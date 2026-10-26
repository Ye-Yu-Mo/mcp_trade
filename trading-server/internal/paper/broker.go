// Package paper implements a simulated trading venue that speaks the same
// binance.Trader interface as the live client. Market data always comes from
// Binance; fills, fees, stops and PnL are simulated locally and persisted to
// a JSON state file so a paper run survives restarts.
package paper

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/store"
)

const (
	takerFeeRate = 0.0005 // 0.05% taker
	makerFeeRate = 0.0002 // 0.02% maker
	slippageRate = 0.0002 // 2 bps on market fills
)

// MarketData is the subset of the Binance client used for public market data.
type MarketData interface {
	GetTicker(symbol string) (*binance.Ticker, error)
	GetKlines(symbol, interval string, limit int) ([]binance.Kline, error)
	GetOrderBook(symbol string, limit int) (*binance.OrderBook, error)
	ScanMarket(limit int) ([]binance.ScannerResult, error)
	GetFundingRate(symbol string) (float64, int64, error)
	GetOpenInterest(symbol string) (float64, error)
	GetATR(symbol, interval string, period int) (float64, error)
	GetCandleInfo(symbol, interval string) (*binance.CandleInfo, error)
	GetAllPrices() (map[string]float64, error)
}

type paperPosition struct {
	Symbol      string    `json:"symbol"`
	Side        string    `json:"side"` // LONG / SHORT
	Quantity    float64   `json:"quantity"`
	EntryPrice  float64   `json:"entry_price"`
	StopPrice   float64   `json:"stop_price"`
	TakeProfit  float64   `json:"take_profit"`
	Leverage    float64   `json:"leverage"`
	Margin      float64   `json:"margin"`
	EntryFee    float64   `json:"entry_fee"`
	RealizedPnL float64   `json:"realized_pnl"`
	ExitFees    float64   `json:"exit_fees"`
	RiskAmount  float64   `json:"risk_amount"`
	TradeID     int64     `json:"trade_id"`
	OpenedAt    time.Time `json:"opened_at"`
	MFE         float64   `json:"mfe"`
	MAE         float64   `json:"mae"`
	EntryReason string    `json:"entry_reason"`
	Setup       string    `json:"setup"`
	Tags        string    `json:"tags"`
}

type paperOrder struct {
	OrderID       int64      `json:"order_id"`
	Symbol        string     `json:"symbol"`
	Side          string     `json:"side"`
	Type          string     `json:"type"`
	Status        string     `json:"status"`
	Price         float64    `json:"price"`
	StopPrice     float64    `json:"stop_price"`
	Quantity      float64    `json:"quantity"`
	ExecutedQty   float64    `json:"executed_qty"`
	AvgPrice      float64    `json:"avg_price"`
	ReduceOnly    bool       `json:"reduce_only"`
	ClosePosition bool       `json:"close_position"`
	ClientOrderID string     `json:"client_order_id"`
	CreatedAt     time.Time  `json:"created_at"`
	FilledAt      *time.Time `json:"filled_at,omitempty"`

	// Entry metadata captured at placement, used when a LIMIT order fills
	// later and the position/trade record is created.
	TakeProfit  float64 `json:"take_profit"`
	Leverage    float64 `json:"leverage"`
	RiskAmount  float64 `json:"risk_amount"`
	EntryReason string  `json:"entry_reason"`
	Setup       string  `json:"setup"`
	Tags        string  `json:"tags"`
}

type state struct {
	Balance        float64                   `json:"balance"`
	InitialBalance float64                   `json:"initial_balance"`
	Positions      map[string]*paperPosition `json:"positions"`
	Orders         map[int64]*paperOrder     `json:"orders"`
	NextOrderID    int64                     `json:"next_order_id"`
	CreatedAt      time.Time                 `json:"created_at"`
}

// Broker is a simulated Binance Futures account.
type Broker struct {
	mu        sync.Mutex
	market    MarketData
	store     *store.Store
	statePath string
	st        *state
	dirty     bool
	done      chan struct{}

	priceMu sync.RWMutex
	prices  map[string]float64
}

// New creates or restores a paper broker.
func New(market MarketData, st *store.Store, statePath string, initialBalance float64) (*Broker, error) {
	if initialBalance <= 0 {
		initialBalance = 33.12
	}
	b := &Broker{
		market:    market,
		store:     st,
		statePath: statePath,
		done:      make(chan struct{}),
		prices:    make(map[string]float64),
		st: &state{
			Balance:        initialBalance,
			InitialBalance: initialBalance,
			Positions:      make(map[string]*paperPosition),
			Orders:         make(map[int64]*paperOrder),
			NextOrderID:    1,
			CreatedAt:      time.Now().UTC(),
		},
	}
	if err := b.load(); err != nil {
		return nil, err
	}
	return b, nil
}

func (b *Broker) load() error {
	data, err := os.ReadFile(b.statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var s state
	if err := json.Unmarshal(data, &s); err != nil {
		return fmt.Errorf("parse paper state: %w", err)
	}
	if s.Positions == nil {
		s.Positions = make(map[string]*paperPosition)
	}
	if s.Orders == nil {
		s.Orders = make(map[int64]*paperOrder)
	}
	if s.Balance <= 0 {
		// Keep a blown account blown; do not silently revive it on restart.
		s.Balance = 0
	}
	if s.InitialBalance <= 0 {
		s.InitialBalance = b.st.InitialBalance
	}
	if s.NextOrderID <= 0 {
		s.NextOrderID = 1
	}
	for id := range s.Orders {
		if id >= s.NextOrderID {
			s.NextOrderID = id + 1
		}
	}
	b.st = &s
	log.Printf("paper: restored %d positions, %d orders, balance %.2f",
		len(s.Positions), len(s.Orders), s.Balance)
	return nil
}

func (b *Broker) saveLocked() {
	if b.statePath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(b.statePath), 0o755); err != nil {
		log.Printf("paper: mkdir state: %v", err)
		return
	}
	data, err := json.MarshalIndent(b.st, "", "  ")
	if err != nil {
		log.Printf("paper: marshal state: %v", err)
		return
	}
	tmp := b.statePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		log.Printf("paper: write state: %v", err)
		return
	}
	if err := os.Rename(tmp, b.statePath); err != nil {
		log.Printf("paper: rename state: %v", err)
	}
	b.dirty = false
}

// Start launches the price/fill/stop monitor.
func (b *Broker) Start() { go b.loop() }

// Stop stops the monitor and persists state.
func (b *Broker) Stop() {
	close(b.done)
	b.mu.Lock()
	b.saveLocked()
	b.mu.Unlock()
}

// Reset clears the paper account to a fresh balance.
func (b *Broker) Reset(balance float64) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if balance <= 0 {
		balance = b.st.InitialBalance
	}
	b.st = &state{
		Balance:        balance,
		InitialBalance: balance,
		Positions:      make(map[string]*paperPosition),
		Orders:         make(map[int64]*paperOrder),
		NextOrderID:    1,
		CreatedAt:      time.Now().UTC(),
	}
	b.dirty = true
	b.saveLocked()
}

func (b *Broker) loop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	lastPrices := time.Time{}

	for {
		select {
		case <-b.done:
			return
		case <-ticker.C:
			if time.Since(lastPrices) >= 2*time.Second {
				b.refreshPrices()
				lastPrices = time.Now()
			}
			b.mu.Lock()
			b.markPositionsLocked()
			b.checkOrdersLocked()
			b.checkLiquidationLocked()
			if b.dirty {
				b.saveLocked()
			}
			b.mu.Unlock()
		}
	}
}

func (b *Broker) refreshPrices() {
	prices, err := b.market.GetAllPrices()
	if err != nil {
		// Fall back to per-symbol tickers for symbols we care about.
		b.mu.Lock()
		symbols := b.activeSymbolsLocked()
		b.mu.Unlock()
		prices = make(map[string]float64)
		for _, s := range symbols {
			if t, err := b.market.GetTicker(s); err == nil {
				prices[s] = t.Price
			}
		}
	}
	if len(prices) == 0 {
		return
	}
	b.priceMu.Lock()
	b.prices = prices
	b.priceMu.Unlock()
}

func (b *Broker) activeSymbolsLocked() []string {
	set := make(map[string]bool)
	for s := range b.st.Positions {
		set[s] = true
	}
	for _, o := range b.st.Orders {
		if o.Status == "NEW" {
			set[o.Symbol] = true
		}
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

func (b *Broker) price(symbol string) (float64, error) {
	symbol = strings.ToUpper(symbol)
	b.priceMu.RLock()
	p := b.prices[symbol]
	b.priceMu.RUnlock()
	if p > 0 {
		return p, nil
	}
	t, err := b.market.GetTicker(symbol)
	if err != nil {
		return 0, err
	}
	b.priceMu.Lock()
	b.prices[symbol] = t.Price
	b.priceMu.Unlock()
	return t.Price, nil
}

// --- binance.Trader implementation ---

func (b *Broker) GetKlines(symbol, interval string, limit int) ([]binance.Kline, error) {
	return b.market.GetKlines(symbol, interval, limit)
}

func (b *Broker) GetTicker(symbol string) (*binance.Ticker, error) {
	p, err := b.price(symbol)
	if err != nil {
		return nil, err
	}
	return &binance.Ticker{Symbol: strings.ToUpper(symbol), Price: p}, nil
}

func (b *Broker) GetOrderBook(symbol string, limit int) (*binance.OrderBook, error) {
	return b.market.GetOrderBook(symbol, limit)
}

func (b *Broker) GetBalance() ([]binance.Balance, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	margin := 0.0
	unrealized := 0.0
	for _, p := range b.st.Positions {
		margin += p.Margin
		mark := b.markPriceLocked(p.Symbol, p.EntryPrice)
		unrealized += p.unrealized(mark)
	}
	available := math.Max(0, b.st.Balance+unrealized-margin)
	return []binance.Balance{{
		Asset:            "USDT",
		AvailableBalance: available,
		TotalBalance:     b.st.Balance,
	}}, nil
}

func (b *Broker) GetPositions() ([]binance.Position, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.markPositionsLocked()
	out := make([]binance.Position, 0, len(b.st.Positions))
	for _, p := range b.st.Positions {
		mark := b.markPriceLocked(p.Symbol, p.EntryPrice)
		pnl := p.unrealized(mark)
		out = append(out, binance.Position{
			Symbol:        p.Symbol,
			Side:          p.Side,
			Quantity:      p.Quantity,
			EntryPrice:    p.EntryPrice,
			MarkPrice:     mark,
			UnrealizedPnL: pnl,
			Leverage:      int(math.Round(p.Leverage)),
		})
	}
	return out, nil
}

func (b *Broker) CreateOrder(req binance.NewOrderRequest) (*binance.Order, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	symbol := strings.ToUpper(strings.TrimSpace(req.Symbol))
	side := strings.ToUpper(strings.TrimSpace(req.Side))
	orderType := strings.ToUpper(strings.TrimSpace(req.OrderType))
	if symbol == "" || side == "" || orderType == "" {
		return nil, fmt.Errorf("symbol, side and type are required")
	}
	if req.ClientOrderID != "" {
		for _, o := range b.st.Orders {
			if o.ClientOrderID == req.ClientOrderID {
				return o.toOrder(), nil
			}
		}
	}

	order := &paperOrder{
		OrderID:       b.st.NextOrderID,
		Symbol:        symbol,
		Side:          side,
		Type:          orderType,
		Status:        "NEW",
		Price:         req.Price,
		StopPrice:     req.StopPrice,
		Quantity:      req.Quantity,
		ReduceOnly:    req.ReduceOnly,
		ClosePosition: req.ClosePosition,
		ClientOrderID: req.ClientOrderID,
		CreatedAt:     time.Now().UTC(),
		TakeProfit:    req.TakeProfit,
		Leverage:      req.Leverage,
		RiskAmount:    req.RiskAmount,
		EntryReason:   req.EntryReason,
		Setup:         req.Setup,
		Tags:          req.Tags,
	}
	b.st.NextOrderID++

	switch orderType {
	case "MARKET":
		price, err := b.price(symbol)
		if err != nil {
			return nil, err
		}
		fill := applySlippage(price, side)
		if err := b.fillLocked(order, fill, takerFeeRate, "MARKET", req); err != nil {
			return nil, err
		}
	case "LIMIT":
		if req.Price <= 0 {
			return nil, fmt.Errorf("limit price is required")
		}
		mark, err := b.price(symbol)
		if err != nil {
			return nil, err
		}
		if marketable(side, req.Price, mark) {
			// An immediately marketable limit order crosses the spread and
			// pays the taker fee; only a resting limit earns maker fees.
			if err := b.fillLocked(order, req.Price, takerFeeRate, "LIMIT", req); err != nil {
				return nil, err
			}
		} else {
			b.st.Orders[order.OrderID] = order
			b.dirty = true
		}
	case "STOP_MARKET", "TAKE_PROFIT_MARKET":
		if req.StopPrice <= 0 {
			return nil, fmt.Errorf("stop price is required for %s", orderType)
		}
		if pos := b.st.Positions[symbol]; pos != nil {
			if orderType == "STOP_MARKET" {
				pos.StopPrice = req.StopPrice
			} else {
				pos.TakeProfit = req.StopPrice
			}
		}
		b.st.Orders[order.OrderID] = order
		b.dirty = true
		b.checkOrderLocked(order)
	default:
		return nil, fmt.Errorf("unsupported paper order type %s", orderType)
	}

	if order.Status == "FILLED" {
		now := time.Now().UTC()
		order.FilledAt = &now
	}
	if _, exists := b.st.Orders[order.OrderID]; !exists {
		b.st.Orders[order.OrderID] = order
	}
	b.saveLocked()
	return order.toOrder(), nil
}

func (b *Broker) CancelOrder(symbol string, orderID int64) (*binance.Order, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	o, ok := b.st.Orders[orderID]
	if !ok {
		return nil, fmt.Errorf("order %d not found", orderID)
	}
	if o.Status == "NEW" {
		o.Status = "CANCELED"
		b.dirty = true
		b.saveLocked()
	}
	return o.toOrder(), nil
}

// CancelAllOpenOrders cancels every NEW order for a symbol.
func (b *Broker) CancelAllOpenOrders(symbol string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, o := range b.st.Orders {
		if o.Symbol == strings.ToUpper(symbol) && o.Status == "NEW" {
			o.Status = "CANCELED"
			b.dirty = true
		}
	}
	return nil
}

func (b *Broker) GetOpenOrders(symbol string) ([]binance.Order, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	symbol = strings.ToUpper(strings.TrimSpace(symbol))
	out := []binance.Order{}
	ids := make([]int64, 0, len(b.st.Orders))
	for id := range b.st.Orders {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		o := b.st.Orders[id]
		if o.Status != "NEW" {
			continue
		}
		if symbol != "" && o.Symbol != symbol {
			continue
		}
		out = append(out, *o.toOrder())
	}
	return out, nil
}

func (b *Broker) GetOrder(symbol string, orderID int64) (*binance.Order, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	o, ok := b.st.Orders[orderID]
	if !ok {
		return nil, fmt.Errorf("order %d not found", orderID)
	}
	return o.toOrder(), nil
}

func (b *Broker) ScanMarket(limit int) ([]binance.ScannerResult, error) {
	return b.market.ScanMarket(limit)
}

func (b *Broker) GetFundingRate(symbol string) (float64, int64, error) {
	return b.market.GetFundingRate(symbol)
}

func (b *Broker) GetOpenInterest(symbol string) (float64, error) {
	return b.market.GetOpenInterest(symbol)
}

func (b *Broker) GetATR(symbol, interval string, period int) (float64, error) {
	return b.market.GetATR(symbol, interval, period)
}

func (b *Broker) GetCandleInfo(symbol, interval string) (*binance.CandleInfo, error) {
	return b.market.GetCandleInfo(symbol, interval)
}

// Binance Futures has no OCO product; use the bracket flow in order.place.
func (b *Broker) CreateOCOOrder(symbol, side string, quantity, price, stopPrice float64) (*binance.OCOOrder, error) {
	return nil, fmt.Errorf("OCO is not supported on Binance Futures; use order.place with stop_price and take_profit_price")
}

func (b *Broker) CancelOCOOrder(symbol string, orderListID int64) error {
	return fmt.Errorf("OCO is not supported on Binance Futures")
}

// --- simulation internals (caller must hold b.mu) ---

func (b *Broker) fillLocked(o *paperOrder, fillPrice, feeRate float64, reason string, req binance.NewOrderRequest) error {
	if o.ClosePosition || o.ReduceOnly {
		pos := b.st.Positions[o.Symbol]
		if pos == nil {
			return fmt.Errorf("no open position for %s", o.Symbol)
		}
		if !o.ClosePosition && o.Quantity <= 0 {
			return fmt.Errorf("reduce quantity is required")
		}
		closed, _, err := b.closePositionLocked(o.Symbol, o.Quantity, fillPrice, reason, o.ClosePosition)
		if err != nil {
			return err
		}
		o.Status = "FILLED"
		o.ExecutedQty = closed
		o.AvgPrice = fillPrice
		b.dirty = true
		return nil
	}

	if err := b.openPositionLocked(o.Symbol, o.Side, o.Quantity, fillPrice, feeRate, req, o.OrderID); err != nil {
		return err
	}
	o.Status = "FILLED"
	o.ExecutedQty = o.Quantity
	o.AvgPrice = fillPrice
	b.dirty = true
	return nil
}

func (b *Broker) openPositionLocked(symbol, side string, qty, price, feeRate float64, req binance.NewOrderRequest, orderID int64) error {
	if qty <= 0 {
		return fmt.Errorf("quantity must be positive")
	}
	posSide := "LONG"
	if side == "SELL" {
		posSide = "SHORT"
	}
	if existing := b.st.Positions[symbol]; existing != nil {
		return fmt.Errorf("position already open on %s (%s); close or reduce it first", symbol, existing.Side)
	}

	leverage := req.Leverage
	if leverage <= 0 {
		leverage = 5
	}
	notional := qty * price
	margin := notional / leverage
	fee := notional * feeRate
	b.st.Balance -= fee

	stop := req.StopPrice
	riskAmount := req.RiskAmount
	if riskAmount <= 0 && stop > 0 {
		riskAmount = math.Abs(price-stop) * qty
	}

	pos := &paperPosition{
		Symbol:      symbol,
		Side:        posSide,
		Quantity:    qty,
		EntryPrice:  price,
		StopPrice:   stop,
		TakeProfit:  req.TakeProfit,
		Leverage:    leverage,
		Margin:      margin,
		EntryFee:    fee,
		RiskAmount:  riskAmount,
		OpenedAt:    time.Now().UTC(),
		EntryReason: req.EntryReason,
		Setup:       req.Setup,
		Tags:        req.Tags,
	}
	b.st.Positions[symbol] = pos

	if b.store != nil {
		id, err := b.store.OpenTrade(store.OpenTradeParams{
			Symbol:      symbol,
			Side:        posSide,
			Quantity:    qty,
			EntryPrice:  price,
			StopPrice:   stop,
			TakeProfit:  req.TakeProfit,
			Leverage:    leverage,
			RiskAmount:  riskAmount,
			OrderID:     fmt.Sprintf("paper-%d", orderID),
			EntryReason: req.EntryReason,
			Setup:       req.Setup,
			Tags:        req.Tags,
		})
		if err != nil {
			log.Printf("paper: open trade record: %v", err)
		} else {
			pos.TradeID = id
		}
	}
	b.dirty = true
	return nil
}

func (b *Broker) closePositionLocked(symbol string, qty, price float64, reason string, closeAll bool) (float64, float64, error) {
	pos := b.st.Positions[symbol]
	if pos == nil {
		return 0, 0, fmt.Errorf("no open position for %s", symbol)
	}
	if closeAll || qty <= 0 || qty > pos.Quantity {
		qty = pos.Quantity
	}
	if qty <= 0 {
		return 0, 0, fmt.Errorf("quantity must be positive")
	}

	var gross float64
	if pos.Side == "LONG" {
		gross = (price - pos.EntryPrice) * qty
	} else {
		gross = (pos.EntryPrice - price) * qty
	}
	exitFee := qty * price * takerFeeRate
	b.st.Balance += gross - exitFee

	pos.RealizedPnL += gross
	pos.ExitFees += exitFee
	releasedMargin := pos.Margin * (qty / pos.Quantity)
	pos.Margin -= releasedMargin
	pos.Quantity -= qty

	closedQty := qty
	if pos.Quantity <= 1e-9 {
		net := pos.RealizedPnL - pos.EntryFee - pos.ExitFees
		fees := pos.EntryFee + pos.ExitFees
		rMultiple := 0.0
		if pos.RiskAmount > 0 {
			rMultiple = net / pos.RiskAmount
		}
		if b.store != nil && pos.TradeID > 0 {
			if err := b.store.CloseTrade(pos.TradeID, price, net, fees, rMultiple, reason); err != nil {
				log.Printf("paper: close trade record: %v", err)
			}
		}
		delete(b.st.Positions, symbol)
		b.cancelProtectiveLocked(symbol)
		// Remove the filled protective order too so the UI does not show it.
		b.dirty = true
	} else {
		b.dirty = true
	}
	return closedQty, gross - exitFee, nil
}

func (b *Broker) cancelProtectiveLocked(symbol string) {
	for _, o := range b.st.Orders {
		if o.Symbol == symbol && o.Status == "NEW" &&
			(o.ReduceOnly || o.ClosePosition || strings.Contains(o.Type, "STOP") || strings.Contains(o.Type, "TAKE_PROFIT")) {
			o.Status = "CANCELED"
		}
	}
}

func (b *Broker) markPositionsLocked() {
	for _, p := range b.st.Positions {
		mark := b.markPriceLocked(p.Symbol, p.EntryPrice)
		u := p.unrealized(mark)
		if u > p.MFE {
			p.MFE = u
		}
		if u < p.MAE {
			p.MAE = u
		}
	}
}

func (b *Broker) checkLiquidationLocked() {
	for symbol, p := range b.st.Positions {
		mark := b.markPriceLocked(symbol, p.EntryPrice)
		if p.Margin > 0 && p.unrealized(mark) <= -p.Margin*0.95 {
			log.Printf("paper: liquidating %s at %.8f", symbol, mark)
			b.closePositionLocked(symbol, p.Quantity, mark, "LIQUIDATION", true)
		}
	}
}

func (b *Broker) checkOrdersLocked() {
	// Process in placement order: an entry LIMIT (lower ID) must fill before
	// its protective stop/TP is evaluated, otherwise a gap could cancel the
	// stop while the entry fills unprotected.
	ids := make([]int64, 0, len(b.st.Orders))
	for id, o := range b.st.Orders {
		if o.Status == "NEW" {
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range ids {
		b.checkOrderLocked(b.st.Orders[id])
	}
}

func (b *Broker) checkOrderLocked(o *paperOrder) {
	if o.Status != "NEW" {
		return
	}
	mark, err := b.price(o.Symbol)
	if err != nil || mark <= 0 {
		return
	}

	switch o.Type {
	case "LIMIT":
		if marketable(o.Side, o.Price, mark) {
			if o.ClosePosition || o.ReduceOnly {
				if b.st.Positions[o.Symbol] != nil {
					b.fillOrderLocked(o, o.Price, makerFeeRate, "LIMIT")
				}
			} else {
				b.fillOrderLocked(o, o.Price, makerFeeRate, "LIMIT")
			}
		}
	case "STOP_MARKET":
		if triggeredStop(o.Side, o.StopPrice, mark) {
			b.fillOrderLocked(o, applySlippage(mark, o.Side), takerFeeRate, "STOP")
		}
	case "TAKE_PROFIT_MARKET":
		if triggeredTakeProfit(o.Side, o.StopPrice, mark) {
			b.fillOrderLocked(o, applySlippage(mark, o.Side), takerFeeRate, "TAKE_PROFIT")
		}
	}
}

func (b *Broker) fillOrderLocked(o *paperOrder, fill float64, feeRate float64, reason string) {
	pos := b.st.Positions[o.Symbol]
	if o.ClosePosition || o.ReduceOnly {
		if pos == nil {
			o.Status = "CANCELED"
			b.dirty = true
			return
		}
		closed, _, err := b.closePositionLocked(o.Symbol, o.Quantity, fill, reason, o.ClosePosition)
		if err != nil {
			o.Status = "CANCELED"
			b.dirty = true
			return
		}
		o.Status = "FILLED"
		o.ExecutedQty = closed
		o.AvgPrice = fill
		now := time.Now().UTC()
		o.FilledAt = &now
		b.dirty = true
		return
	}

	// A resting LIMIT entry fills here.
	if pos != nil {
		o.Status = "CANCELED"
		b.dirty = true
		return
	}
	req := binance.NewOrderRequest{
		Symbol:      o.Symbol,
		Side:        o.Side,
		OrderType:   "LIMIT",
		Quantity:    o.Quantity,
		Price:       o.Price,
		StopPrice:   o.StopPrice,
		TakeProfit:  o.TakeProfit,
		Leverage:    o.Leverage,
		RiskAmount:  o.RiskAmount,
		EntryReason: o.EntryReason,
		Setup:       o.Setup,
		Tags:        o.Tags,
	}
	if err := b.openPositionLocked(o.Symbol, o.Side, o.Quantity, fill, feeRate, req, o.OrderID); err != nil {
		log.Printf("paper: fill limit order %d: %v", o.OrderID, err)
		o.Status = "CANCELED"
		b.dirty = true
		return
	}
	o.Status = "FILLED"
	o.ExecutedQty = o.Quantity
	o.AvgPrice = fill
	now := time.Now().UTC()
	o.FilledAt = &now
	b.dirty = true
}

func (b *Broker) markPriceLocked(symbol string, fallback float64) float64 {
	if p, err := b.price(symbol); err == nil && p > 0 {
		return p
	}
	return fallback
}

func (p *paperPosition) unrealized(mark float64) float64 {
	if p.Side == "LONG" {
		return (mark - p.EntryPrice) * p.Quantity
	}
	return (p.EntryPrice - mark) * p.Quantity
}

func (o *paperOrder) toOrder() *binance.Order {
	t := o.CreatedAt.UnixMilli()
	if o.FilledAt != nil {
		t = o.FilledAt.UnixMilli()
	}
	return &binance.Order{
		OrderID:       o.OrderID,
		Symbol:        o.Symbol,
		Status:        o.Status,
		ClientOrderID: o.ClientOrderID,
		Price:         o.Price,
		AvgPrice:      o.AvgPrice,
		OrigQty:       o.Quantity,
		ExecutedQty:   o.ExecutedQty,
		Type:          o.Type,
		Side:          o.Side,
		StopPrice:     o.StopPrice,
		ReduceOnly:    o.ReduceOnly,
		ClosePosition: o.ClosePosition,
		UpdateTime:    t,
	}
}

func marketable(side string, limit, mark float64) bool {
	if side == "BUY" {
		return mark <= limit
	}
	return mark >= limit
}

func triggeredStop(side string, stop, mark float64) bool {
	if side == "SELL" {
		return mark <= stop
	}
	return mark >= stop
}

func triggeredTakeProfit(side string, stop, mark float64) bool {
	if side == "SELL" {
		return mark >= stop
	}
	return mark <= stop
}

func applySlippage(price float64, side string) float64 {
	if side == "BUY" {
		return price * (1 + slippageRate)
	}
	return price * (1 - slippageRate)
}
