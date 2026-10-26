package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "github.com/marcboeker/go-duckdb"
)

// Store wraps the DuckDB database for trade journal persistence.
type Store struct {
	db         *sql.DB
	baselineMu sync.Mutex
}

// TradeRecord represents a row in the trades table.
// One row is one position lifecycle: entry -> (partial exits) -> close.
type TradeRecord struct {
	ID             int64
	Symbol         string
	Side           string // LONG / SHORT
	Quantity       float64
	Price          float64 // entry price
	OrderID        string  // entry order id
	Status         string  // OPEN / CLOSED / CANCELED
	EntryReason    string
	ExitReason     string
	PnL            float64
	MarketSnapshot string
	StopPrice      float64
	TakeProfit     float64
	ExitPrice      float64
	Setup          string
	Tags           string
	Leverage       float64
	RiskAmount     float64
	RMultiple      float64
	Fees           float64
	CreatedAt      time.Time
	ClosedAt       *time.Time
	UpdatedAt      time.Time
}

// JournalEntry represents a row in the journal_entries table.
type JournalEntry struct {
	ID             int64
	TradeID        *int64
	EntryType      string
	Reason         string
	Tags           string
	ScreenshotURL  string
	MarketSnapshot string
	CreatedAt      time.Time
}

// New opens (or creates) the DuckDB database at the given path and runs migrations.
func New(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

func (s *Store) Close() error { return s.db.Close() }
func (s *Store) DB() *sql.DB  { return s.db }

func migrate(db *sql.DB) error {
	if _, err := db.Exec(schema); err != nil {
		return err
	}
	// Additive migrations for existing databases. Each statement is run
	// separately so an existing column does not abort the whole migration.
	for _, stmt := range migrations {
		if _, err := db.Exec(stmt); err != nil {
			msg := strings.ToLower(err.Error())
			if strings.Contains(msg, "already exists") || strings.Contains(msg, "duplicate") {
				continue
			}
			return err
		}
	}
	return nil
}

// --- Store methods ---

// InsertTrade creates a new OPEN trade record. Returns the new ID.
func (s *Store) InsertTrade(symbol, side string, quantity, price float64, orderID, entryReason, marketSnapshot string) (int64, error) {
	var id int64
	err := s.db.QueryRow(
		`INSERT INTO trades (symbol, side, quantity, price, order_id, status, entry_reason, market_snapshot)
		 VALUES (?, ?, ?, ?, ?, 'OPEN', ?, ?) RETURNING id`,
		symbol, side, quantity, price, orderID, entryReason, marketSnapshot,
	).Scan(&id)
	return id, err
}

// UpdateTradePnL closes the trade matched by entry order ID with its realized
// PnL. It is kept for compatibility with older callers; new code should use
// CloseTrade with the trade ID.
func (s *Store) UpdateTradePnL(orderID string, pnl float64) error {
	_, err := s.db.Exec(
		`UPDATE trades SET status='CLOSED', pnl=?, closed_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP
		 WHERE order_id=? AND status='OPEN'`,
		pnl, orderID,
	)
	return err
}

// UpdateTradeByOrderID updates a trade's status, pnl, and avg price by order ID.
func (s *Store) UpdateTradeByOrderID(orderID, status string, pnl, avgPrice float64) error {
	_, err := s.db.Exec(
		`UPDATE trades SET status=?, pnl=?, price=CASE WHEN ? > 0 THEN ? ELSE price END, updated_at=CURRENT_TIMESTAMP WHERE order_id=? AND status='OPEN'`,
		status, pnl, avgPrice, avgPrice, orderID,
	)
	return err
}

// EnsureEquityBaseline returns the start-of-day and 7-day-start equity used
// for loss limits. The first call of the day records the current equity. This
// captures unrealized and partially realized losses that closed-trade sums
// would miss.
func (s *Store) EnsureEquityBaseline(equity float64) (dailyStart, weeklyStart float64, err error) {
	s.baselineMu.Lock()
	defer s.baselineMu.Unlock()

	if err = s.ensureBaselineRowLocked(equity); err != nil {
		return 0, 0, err
	}
	if err = s.db.QueryRow(
		`SELECT start_equity FROM daily_equity WHERE day = CURRENT_DATE`,
	).Scan(&dailyStart); err != nil {
		return 0, 0, err
	}
	err = s.db.QueryRow(
		`SELECT start_equity FROM daily_equity
		 WHERE day >= CURRENT_DATE - INTERVAL 6 DAY
		 ORDER BY day ASC LIMIT 1`,
	).Scan(&weeklyStart)
	if err == sql.ErrNoRows {
		weeklyStart = dailyStart
		err = nil
	}
	return dailyStart, weeklyStart, err
}

func (s *Store) ensureBaselineRowLocked(equity float64) error {
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM daily_equity WHERE day = CURRENT_DATE`).Scan(&count); err != nil {
		return err
	}
	if count > 0 {
		return nil
	}
	_, err := s.db.Exec(
		`INSERT INTO daily_equity (day, start_equity) VALUES (CURRENT_DATE, ?)`,
		equity,
	)
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "duplicate") || strings.Contains(msg, "unique") || strings.Contains(msg, "primary key") {
			return nil
		}
	}
	return err
}

// GetDailyPnL returns the sum of realized PnL for today (UTC).
func (s *Store) GetDailyPnL() (float64, error) {
	var pnl float64
	err := s.db.QueryRow(
		`SELECT COALESCE(SUM(pnl), 0) FROM trades WHERE status='CLOSED' AND closed_at >= CURRENT_DATE`,
	).Scan(&pnl)
	return pnl, err
}

// GetWeeklyPnL returns realized PnL over the trailing 7 days.
func (s *Store) GetWeeklyPnL() (float64, error) {
	var pnl float64
	err := s.db.QueryRow(
		`SELECT COALESCE(SUM(pnl), 0) FROM trades WHERE status='CLOSED' AND closed_at >= CURRENT_DATE - INTERVAL 7 DAY`,
	).Scan(&pnl)
	return pnl, err
}

// QueryTrades returns trades filtered by symbol and time range, ordered by most recent.
func (s *Store) QueryTrades(symbol string, limit int) ([]TradeRecord, error) {
	query := `SELECT ` + tradeColumns + ` FROM trades`
	args := []interface{}{}
	if symbol != "" {
		query += ` WHERE symbol = ?`
		args = append(args, symbol)
	}
	query += ` ORDER BY created_at DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []TradeRecord
	for rows.Next() {
		r, err := scanTrade(rows)
		if err != nil {
			return nil, err
		}
		records = append(records, *r)
	}
	return records, rows.Err()
}

// OpenTradeParams is the input for opening a position-level trade record.
type OpenTradeParams struct {
	Symbol         string
	Side           string // LONG / SHORT
	Quantity       float64
	EntryPrice     float64
	StopPrice      float64
	TakeProfit     float64
	Leverage       float64
	RiskAmount     float64
	OrderID        string
	EntryReason    string
	Setup          string
	Tags           string
	MarketSnapshot string
}

// OpenTrade creates an OPEN trade record for a new position.
func (s *Store) OpenTrade(p OpenTradeParams) (int64, error) {
	if p.MarketSnapshot == "" {
		p.MarketSnapshot = "{}"
	}
	var id int64
	err := s.db.QueryRow(
		`INSERT INTO trades
		 (symbol, side, quantity, price, order_id, status, entry_reason, market_snapshot,
		  stop_price, take_profit, setup, tags, leverage, risk_amount)
		 VALUES (?, ?, ?, ?, ?, 'OPEN', ?, ?, ?, ?, ?, ?, ?, ?) RETURNING id`,
		p.Symbol, strings.ToUpper(p.Side), p.Quantity, p.EntryPrice, p.OrderID, p.EntryReason,
		p.MarketSnapshot, p.StopPrice, p.TakeProfit, p.Setup, p.Tags, p.Leverage, p.RiskAmount,
	).Scan(&id)
	return id, err
}

// CloseTrade closes an OPEN trade record with its realized result.
func (s *Store) CloseTrade(tradeID int64, exitPrice, pnl, fees, rMultiple float64, exitReason string) error {
	_, err := s.db.Exec(
		`UPDATE trades SET status='CLOSED', exit_price=?, pnl=?, fees=?, r_multiple=?,
		 exit_reason=?, closed_at=CURRENT_TIMESTAMP, updated_at=CURRENT_TIMESTAMP
		 WHERE id=? AND status='OPEN'`,
		exitPrice, pnl, fees, rMultiple, exitReason, tradeID,
	)
	return err
}

// ReconcileMissingOpenTrades closes OPEN trade rows whose symbol no longer has
// an exchange position (e.g. the position was closed while the server was
// down). It returns how many rows were reconciled.
func (s *Store) ReconcileMissingOpenTrades(activeSymbols map[string]bool) (int, error) {
	open, err := s.OpenTrades()
	if err != nil {
		return 0, err
	}
	reconciled := 0
	for _, t := range open {
		if activeSymbols[t.Symbol] {
			continue
		}
		if err := s.CloseTrade(t.ID, 0, 0, 0, 0, "RECONCILED_MISSING"); err != nil {
			return reconciled, err
		}
		reconciled++
	}
	return reconciled, nil
}

// UpdateOpenTradeStop updates the stop price on the symbol's OPEN trade.
func (s *Store) UpdateOpenTradeStop(symbol string, stop float64) error {
	_, err := s.db.Exec(
		`UPDATE trades SET stop_price=?, updated_at=CURRENT_TIMESTAMP WHERE symbol=? AND status='OPEN'`,
		stop, symbol,
	)
	return err
}

// FindOpenTradeBySymbol returns the most recent OPEN trade for a symbol.
func (s *Store) FindOpenTradeBySymbol(symbol string) (*TradeRecord, error) {
	row := s.db.QueryRow(
		`SELECT `+tradeColumns+` FROM trades WHERE symbol=? AND status='OPEN' ORDER BY created_at DESC LIMIT 1`,
		symbol,
	)
	r, err := scanTrade(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return r, err
}

// OpenTrades returns all OPEN trade records, oldest first.
func (s *Store) OpenTrades() ([]TradeRecord, error) {
	rows, err := s.db.Query(`SELECT ` + tradeColumns + ` FROM trades WHERE status='OPEN' ORDER BY created_at ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TradeRecord
	for rows.Next() {
		r, err := scanTrade(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

const tradeColumns = `id, symbol, side, quantity, price, order_id, status,
	entry_reason, exit_reason, pnl, market_snapshot, created_at, updated_at,
	stop_price, take_profit, exit_price, setup, tags, leverage, risk_amount,
	r_multiple, fees, closed_at`

func scanTrade(row interface{ Scan(...interface{}) error }) (*TradeRecord, error) {
	var r TradeRecord
	var closed sql.NullTime
	err := row.Scan(
		&r.ID, &r.Symbol, &r.Side, &r.Quantity, &r.Price, &r.OrderID, &r.Status,
		&r.EntryReason, &r.ExitReason, &r.PnL, &r.MarketSnapshot, &r.CreatedAt, &r.UpdatedAt,
		&r.StopPrice, &r.TakeProfit, &r.ExitPrice, &r.Setup, &r.Tags, &r.Leverage,
		&r.RiskAmount, &r.RMultiple, &r.Fees, &closed,
	)
	if err != nil {
		return nil, err
	}
	if closed.Valid {
		r.ClosedAt = &closed.Time
	}
	return &r, nil
}

// InsertJournal creates a new journal entry.
func (s *Store) InsertJournal(entryType, reason, tags string, tradeID *int64) (int64, error) {
	var id int64
	err := s.db.QueryRow(
		`INSERT INTO journal_entries (trade_id, entry_type, reason, tags)
		 VALUES (?, ?, ?, ?) RETURNING id`,
		tradeID, entryType, reason, tags,
	).Scan(&id)
	return id, err
}

// QueryJournals returns journal entries, optionally filtered by type and tags. Most recent first.
func (s *Store) QueryJournals(limit int, entryType, tags string) ([]JournalEntry, error) {
	query := `SELECT id, trade_id, entry_type, reason, tags, screenshot_url, market_snapshot, created_at FROM journal_entries WHERE 1=1`
	args := []interface{}{}
	if entryType != "" {
		query += ` AND entry_type = ?`
		args = append(args, entryType)
	}
	if tags != "" {
		query += ` AND tags LIKE ?`
		args = append(args, "%"+tags+"%")
	}
	query += ` ORDER BY created_at DESC LIMIT ?`
	args = append(args, limit)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []JournalEntry
	for rows.Next() {
		var j JournalEntry
		if err := rows.Scan(&j.ID, &j.TradeID, &j.EntryType, &j.Reason, &j.Tags,
			&j.ScreenshotURL, &j.MarketSnapshot, &j.CreatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, j)
	}
	return entries, nil
}

// Performance holds aggregated trading statistics.
type Performance struct {
	TotalTrades          int     `json:"total_trades"`
	WinTrades            int     `json:"win_trades"`
	LossTrades           int     `json:"loss_trades"`
	BreakevenTrades      int     `json:"breakeven_trades"`
	WinRate              float64 `json:"win_rate"`
	TotalPnL             float64 `json:"total_pnl"`
	AvgPnL               float64 `json:"avg_pnl"`
	MaxWin               float64 `json:"max_win"`
	MaxLoss              float64 `json:"max_loss"`
	ProfitFactor         float64 `json:"profit_factor"`
	ProfitFactorInfinite bool    `json:"profit_factor_infinite"`
	AvgR                 float64 `json:"avg_r"`
	ExpectancyR          float64 `json:"expectancy_r"`
	MaxDrawdown          float64 `json:"max_drawdown"`
}

// GetPerformance computes aggregated statistics from CLOSED trades only.
// Entry orders and protective orders are never counted.
func (s *Store) GetPerformance() (*Performance, error) {
	p := &Performance{}
	rows, err := s.db.Query(
		`SELECT pnl, COALESCE(r_multiple, 0) FROM trades WHERE status='CLOSED' ORDER BY closed_at ASC, id ASC`,
	)
	if err != nil {
		return p, err
	}
	defer rows.Close()

	var totalWin, totalLoss, sumR float64
	var graded int
	var cumulative, peak, maxDD float64

	for rows.Next() {
		var pnl, rMultiple float64
		if err := rows.Scan(&pnl, &rMultiple); err != nil {
			return p, err
		}
		p.TotalTrades++
		p.TotalPnL += pnl
		sumR += rMultiple
		graded++
		switch {
		case pnl > 0:
			p.WinTrades++
			totalWin += pnl
			if pnl > p.MaxWin {
				p.MaxWin = pnl
			}
		case pnl < 0:
			p.LossTrades++
			totalLoss += -pnl
			if -pnl > p.MaxLoss {
				p.MaxLoss = -pnl
			}
		default:
			p.BreakevenTrades++
		}

		cumulative += pnl
		if cumulative > peak {
			peak = cumulative
		}
		if dd := peak - cumulative; dd > maxDD {
			maxDD = dd
		}
	}
	if err := rows.Err(); err != nil {
		return p, err
	}

	decided := p.WinTrades + p.LossTrades
	if decided > 0 {
		p.WinRate = float64(p.WinTrades) / float64(decided) * 100
	}
	if p.TotalTrades > 0 {
		p.AvgPnL = p.TotalPnL / float64(p.TotalTrades)
	}
	if graded > 0 {
		p.AvgR = sumR / float64(graded)
		p.ExpectancyR = p.AvgR
	}
	if totalLoss > 0 {
		p.ProfitFactor = totalWin / totalLoss
	} else if totalWin > 0 {
		p.ProfitFactorInfinite = true
	}
	p.MaxDrawdown = maxDD

	return p, nil
}

// --- Alert persistence ---

// InsertAlert creates a new alert in the database.
func (s *Store) InsertAlert(id, symbol, direction, message string, price float64) error {
	_, err := s.db.Exec(
		`INSERT INTO alerts (id, symbol, price, direction, message) VALUES (?, ?, ?, ?, ?)`,
		id, symbol, price, direction, message,
	)
	return err
}

// QueryAlerts returns all alerts, most recent first.
func (s *Store) QueryAlerts() ([]Alert, error) {
	rows, err := s.db.Query(`SELECT id, symbol, price, direction, message, triggered, created_at FROM alerts ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var alerts []Alert
	for rows.Next() {
		var a Alert
		if err := rows.Scan(&a.ID, &a.Symbol, &a.Price, &a.Direction, &a.Message, &a.Triggered, &a.CreatedAt); err != nil {
			return nil, err
		}
		alerts = append(alerts, a)
	}
	return alerts, nil
}

// UpdateAlertTriggered marks an alert as triggered.
func (s *Store) UpdateAlertTriggered(id string) error {
	_, err := s.db.Exec(`UPDATE alerts SET triggered=TRUE WHERE id=?`, id)
	return err
}

// DeleteAlert removes an alert by ID.
func (s *Store) DeleteAlert(id string) error {
	_, err := s.db.Exec(`DELETE FROM alerts WHERE id=?`, id)
	return err
}

// Alert is a persisted price alert.
type Alert struct {
	ID        string    `json:"id"`
	Symbol    string    `json:"symbol"`
	Price     float64   `json:"price"`
	Direction string    `json:"direction"`
	Message   string    `json:"message"`
	Triggered bool      `json:"triggered"`
	CreatedAt time.Time `json:"created_at"`
}

const schema = `
CREATE SEQUENCE IF NOT EXISTS seq_trade_id;

CREATE TABLE IF NOT EXISTS trades (
    id              BIGINT PRIMARY KEY DEFAULT nextval('seq_trade_id'),
    symbol          TEXT NOT NULL,
    side            TEXT NOT NULL,
    quantity        DOUBLE NOT NULL,
    price           DOUBLE NOT NULL,
    order_id        TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'OPEN',
    entry_reason    TEXT NOT NULL DEFAULT '',
    exit_reason     TEXT NOT NULL DEFAULT '',
    pnl             DOUBLE NOT NULL DEFAULT 0,
    market_snapshot TEXT NOT NULL DEFAULT '{}',
    created_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at      TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS seq_journal_id;

CREATE TABLE IF NOT EXISTS journal_entries (
    id               BIGINT PRIMARY KEY DEFAULT nextval('seq_journal_id'),
    trade_id         BIGINT,
    entry_type       TEXT NOT NULL,
    reason           TEXT NOT NULL DEFAULT '',
    tags             TEXT NOT NULL DEFAULT '',
    screenshot_url   TEXT NOT NULL DEFAULT '',
    market_snapshot  TEXT NOT NULL DEFAULT '{}',
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS alerts (
    id          TEXT PRIMARY KEY,
    symbol      TEXT NOT NULL,
    price       DOUBLE NOT NULL,
    direction   TEXT NOT NULL,
    message     TEXT NOT NULL DEFAULT '',
    triggered   BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS daily_equity (
    day           DATE PRIMARY KEY,
    start_equity  DOUBLE NOT NULL
);
`

// migrations are additive schema changes for databases created by older
// versions. DuckDB supports ADD COLUMN IF NOT EXISTS; migrate() also ignores
// "already exists" errors for safety.
var migrations = []string{
	`ALTER TABLE trades ADD COLUMN IF NOT EXISTS stop_price DOUBLE DEFAULT 0`,
	`ALTER TABLE trades ADD COLUMN IF NOT EXISTS take_profit DOUBLE DEFAULT 0`,
	`ALTER TABLE trades ADD COLUMN IF NOT EXISTS exit_price DOUBLE DEFAULT 0`,
	`ALTER TABLE trades ADD COLUMN IF NOT EXISTS setup TEXT DEFAULT ''`,
	`ALTER TABLE trades ADD COLUMN IF NOT EXISTS tags TEXT DEFAULT ''`,
	`ALTER TABLE trades ADD COLUMN IF NOT EXISTS leverage DOUBLE DEFAULT 1`,
	`ALTER TABLE trades ADD COLUMN IF NOT EXISTS risk_amount DOUBLE DEFAULT 0`,
	`ALTER TABLE trades ADD COLUMN IF NOT EXISTS r_multiple DOUBLE DEFAULT 0`,
	`ALTER TABLE trades ADD COLUMN IF NOT EXISTS fees DOUBLE DEFAULT 0`,
	`ALTER TABLE trades ADD COLUMN IF NOT EXISTS closed_at TIMESTAMP`,
}
