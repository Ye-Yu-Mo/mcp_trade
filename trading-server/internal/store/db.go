package store

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "github.com/marcboeker/go-duckdb"
)

// Store wraps the DuckDB database for trade journal persistence.
type Store struct {
	db *sql.DB
}

// TradeRecord represents a row in the trades table.
type TradeRecord struct {
	ID             int64
	Symbol         string
	Side           string
	Quantity       float64
	Price          float64
	OrderID        string
	Status         string
	EntryReason    string
	ExitReason     string
	PnL            float64
	MarketSnapshot string
	CreatedAt      time.Time
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
	_, err := db.Exec(schema)
	return err
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

// UpdateTradePnL updates the trade with exit info when the position is closed.
func (s *Store) UpdateTradePnL(orderID string, pnl float64) error {
	_, err := s.db.Exec(
		`UPDATE trades SET status='FILLED', pnl=?, updated_at=CURRENT_TIMESTAMP WHERE order_id=? AND status='OPEN'`,
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

// GetDailyPnL returns the sum of realized PnL for today (UTC).
func (s *Store) GetDailyPnL() (float64, error) {
	var pnl sql.NullFloat64
	err := s.db.QueryRow(
		`SELECT SUM(pnl) FROM trades WHERE status='FILLED' AND updated_at >= CURRENT_DATE`,
	).Scan(&pnl)
	if !pnl.Valid {
		return 0, err
	}
	return pnl.Float64, err
}

// QueryTrades returns trades filtered by symbol and time range, ordered by most recent.
func (s *Store) QueryTrades(symbol string, limit int) ([]TradeRecord, error) {
	query := `SELECT id, symbol, side, quantity, price, order_id, status, entry_reason, exit_reason, pnl, market_snapshot, created_at, updated_at FROM trades`
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
		var r TradeRecord
		if err := rows.Scan(&r.ID, &r.Symbol, &r.Side, &r.Quantity, &r.Price, &r.OrderID, &r.Status,
			&r.EntryReason, &r.ExitReason, &r.PnL, &r.MarketSnapshot, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
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
	TotalTrades   int     `json:"total_trades"`
	WinTrades     int     `json:"win_trades"`
	LossTrades    int     `json:"loss_trades"`
	WinRate       float64 `json:"win_rate"`
	TotalPnL      float64 `json:"total_pnl"`
	AvgPnL        float64 `json:"avg_pnl"`
	MaxWin        float64 `json:"max_win"`
	MaxLoss       float64 `json:"max_loss"`
	ProfitFactor  float64 `json:"profit_factor"`
}

// GetPerformance computes aggregated trading statistics from FILLED trades.
func (s *Store) GetPerformance() (*Performance, error) {
	p := &Performance{}
	rows, err := s.db.Query(`SELECT pnl FROM trades WHERE status='FILLED'`)
	if err != nil {
		return p, err
	}
	defer rows.Close()

	var totalWin, totalLoss float64
	for rows.Next() {
		var pnl float64
		if err := rows.Scan(&pnl); err != nil {
			return p, err
		}
		p.TotalTrades++
		p.TotalPnL += pnl
		if pnl > 0 {
			p.WinTrades++
			totalWin += pnl
			if pnl > p.MaxWin {
				p.MaxWin = pnl
			}
		} else {
			p.LossTrades++
			totalLoss += -pnl
			if -pnl > p.MaxLoss {
				p.MaxLoss = -pnl
			}
		}
	}

	if p.TotalTrades > 0 {
		p.WinRate = float64(p.WinTrades) / float64(p.TotalTrades) * 100
		p.AvgPnL = p.TotalPnL / float64(p.TotalTrades)
		if totalLoss > 0 {
			p.ProfitFactor = totalWin / totalLoss
		} else if totalWin > 0 {
			p.ProfitFactor = totalWin // all wins, no losses
		}
	}

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
`
