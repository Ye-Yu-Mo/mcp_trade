package paper

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/store"
)

type fakeMarket struct {
	prices map[string]float64
}

func (f *fakeMarket) GetTicker(symbol string) (*binance.Ticker, error) {
	return &binance.Ticker{Symbol: symbol, Price: f.prices[symbol]}, nil
}
func (f *fakeMarket) GetKlines(string, string, int) ([]binance.Kline, error) { return nil, nil }
func (f *fakeMarket) GetOrderBook(string, int) (*binance.OrderBook, error)   { return nil, nil }
func (f *fakeMarket) ScanMarket(int) ([]binance.ScannerResult, error)        { return nil, nil }
func (f *fakeMarket) GetFundingRate(string) (float64, int64, error)          { return 0, 0, nil }
func (f *fakeMarket) GetOpenInterest(string) (float64, error)                { return 0, nil }
func (f *fakeMarket) GetATR(string, string, int) (float64, error)            { return 0, nil }
func (f *fakeMarket) GetCandleInfo(string, string) (*binance.CandleInfo, error) {
	return nil, nil
}
func (f *fakeMarket) GetAllPrices() (map[string]float64, error) {
	out := make(map[string]float64, len(f.prices))
	for k, v := range f.prices {
		out[k] = v
	}
	return out, nil
}

func newTestBroker(t *testing.T) (*Broker, *fakeMarket, *store.Store) {
	t.Helper()
	dir := t.TempDir()
	st, err := store.New(filepath.Join(dir, "test.duckdb"))
	if err != nil {
		t.Fatalf("store: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	fm := &fakeMarket{prices: map[string]float64{"BTCUSDT": 65000, "ETHUSDT": 3200}}
	b, err := New(fm, st, filepath.Join(dir, "paper_state.json"), 1000)
	if err != nil {
		t.Fatalf("paper.New: %v", err)
	}
	b.refreshPrices()
	return b, fm, st
}

func TestMarketEntryCreatesPositionAndTrade(t *testing.T) {
	b, _, st := newTestBroker(t)

	o, err := b.CreateOrder(binance.NewOrderRequest{
		Symbol: "BTCUSDT", Side: "BUY", OrderType: "MARKET", Quantity: 0.01,
		StopPrice: 64000, TakeProfit: 67000, Leverage: 5, RiskAmount: 10,
		EntryReason: "test long", Setup: "breakout", Tags: "test",
	})
	if err != nil {
		t.Fatalf("CreateOrder: %v", err)
	}
	if o.Status != "FILLED" || math.Abs(o.ExecutedQty-0.01) > 1e-9 {
		t.Fatalf("unexpected order: %+v", o)
	}
	wantFill := 65000 * (1 + slippageRate)
	if math.Abs(o.AvgPrice-wantFill) > 0.01 {
		t.Fatalf("avg price = %.4f, want %.4f", o.AvgPrice, wantFill)
	}

	positions, _ := b.GetPositions()
	if len(positions) != 1 {
		t.Fatalf("positions = %d, want 1", len(positions))
	}
	p := positions[0]
	if p.Side != "LONG" || math.Abs(p.Quantity-0.01) > 1e-9 || p.EntryPrice != o.AvgPrice {
		t.Fatalf("unexpected position: %+v", p)
	}
	if p.Leverage != 5 {
		t.Fatalf("leverage = %d, want 5", p.Leverage)
	}

	bal, _ := b.GetBalance()
	wantWallet := 1000 - 0.01*wantFill*takerFeeRate
	if math.Abs(bal[0].TotalBalance-wantWallet) > 1e-6 {
		t.Fatalf("wallet = %.6f, want %.6f", bal[0].TotalBalance, wantWallet)
	}
	wantMargin := 0.01 * wantFill / 5
	wantUnrealized := (65000 - wantFill) * 0.01
	wantAvailable := wantWallet - wantMargin + wantUnrealized
	if math.Abs(bal[0].AvailableBalance-wantAvailable) > 1e-6 {
		t.Fatalf("available = %.6f, want %.6f", bal[0].AvailableBalance, wantAvailable)
	}

	trades, err := st.QueryTrades("BTCUSDT", 10)
	if err != nil || len(trades) != 1 {
		t.Fatalf("trades = %d err=%v, want 1", len(trades), err)
	}
	if trades[0].Status != "OPEN" || trades[0].Side != "LONG" ||
		trades[0].StopPrice != 64000 || trades[0].TakeProfit != 67000 ||
		math.Abs(trades[0].RiskAmount-10) > 1e-9 {
		t.Fatalf("unexpected trade record: %+v", trades[0])
	}
}

func TestStopLossClosesPositionAndRecordsR(t *testing.T) {
	b, fm, st := newTestBroker(t)

	_, err := b.CreateOrder(binance.NewOrderRequest{
		Symbol: "BTCUSDT", Side: "BUY", OrderType: "MARKET", Quantity: 0.01,
		StopPrice: 64000, TakeProfit: 67000, Leverage: 5, RiskAmount: 10,
	})
	if err != nil {
		t.Fatalf("entry: %v", err)
	}
	if _, err := b.CreateOrder(binance.NewOrderRequest{
		Symbol: "BTCUSDT", Side: "SELL", OrderType: "STOP_MARKET",
		StopPrice: 64000, ClosePosition: true,
	}); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if _, err := b.CreateOrder(binance.NewOrderRequest{
		Symbol: "BTCUSDT", Side: "SELL", OrderType: "TAKE_PROFIT_MARKET",
		StopPrice: 67000, ClosePosition: true,
	}); err != nil {
		t.Fatalf("tp: %v", err)
	}

	fm.prices["BTCUSDT"] = 63000
	b.refreshPrices()
	b.mu.Lock()
	b.checkOrdersLocked()
	b.mu.Unlock()

	positions, _ := b.GetPositions()
	if len(positions) != 0 {
		t.Fatalf("positions = %d, want 0 after stop", len(positions))
	}

	trades, _ := st.QueryTrades("BTCUSDT", 10)
	if len(trades) != 1 || trades[0].Status != "CLOSED" {
		t.Fatalf("expected one CLOSED trade, got %+v", trades)
	}
	if trades[0].ExitReason != "STOP" {
		t.Fatalf("exit reason = %q, want STOP", trades[0].ExitReason)
	}
	if trades[0].PnL >= 0 {
		t.Fatalf("pnl = %.4f, want a loss", trades[0].PnL)
	}
	// Risk was 10 USDT; the loss must be roughly -2R and must not exceed it
	// grossly (stop slippage + fees).
	if trades[0].RMultiple > -1.9 || trades[0].RMultiple < -2.3 {
		t.Fatalf("r_multiple = %.4f, want about -2.09", trades[0].RMultiple)
	}
	if trades[0].Fees <= 0 {
		t.Fatalf("fees = %.4f, want positive", trades[0].Fees)
	}

	perf, err := st.GetPerformance()
	if err != nil {
		t.Fatalf("performance: %v", err)
	}
	if perf.TotalTrades != 1 || perf.LossTrades != 1 || perf.MaxDrawdown <= 0 {
		t.Fatalf("unexpected performance: %+v", perf)
	}

	orders, _ := b.GetOpenOrders("BTCUSDT")
	if len(orders) != 0 {
		t.Fatalf("open orders = %d, want 0 (TP cancelled with position)", len(orders))
	}
}

func TestLimitEntryFillsWhenPriceCrosses(t *testing.T) {
	b, fm, st := newTestBroker(t)

	o, err := b.CreateOrder(binance.NewOrderRequest{
		Symbol: "BTCUSDT", Side: "BUY", OrderType: "LIMIT", Quantity: 0.01,
		Price: 64000, StopPrice: 63000, TakeProfit: 66000, Leverage: 5, RiskAmount: 10,
	})
	if err != nil {
		t.Fatalf("limit entry: %v", err)
	}
	if o.Status != "NEW" {
		t.Fatalf("status = %s, want NEW before price crosses", o.Status)
	}

	fm.prices["BTCUSDT"] = 63900
	b.refreshPrices()
	b.mu.Lock()
	b.checkOrdersLocked()
	b.mu.Unlock()

	got, _ := b.GetOrder("BTCUSDT", o.OrderID)
	if got.Status != "FILLED" {
		t.Fatalf("status = %s, want FILLED", got.Status)
	}
	if math.Abs(got.AvgPrice-64000) > 1e-9 {
		t.Fatalf("fill = %.4f, want limit 64000", got.AvgPrice)
	}
	positions, _ := b.GetPositions()
	if len(positions) != 1 || math.Abs(positions[0].EntryPrice-64000) > 1e-9 {
		t.Fatalf("unexpected positions: %+v", positions)
	}
	trades, _ := st.QueryTrades("BTCUSDT", 10)
	if len(trades) != 1 || trades[0].Status != "OPEN" || trades[0].StopPrice != 63000 {
		t.Fatalf("unexpected trade record: %+v", trades)
	}
}

func TestExpiredLimitEntryDropsItsBracket(t *testing.T) {
	b, fm, _ := newTestBroker(t)

	entry, err := b.CreateOrder(binance.NewOrderRequest{
		Symbol: "BTCUSDT", Side: "SELL", OrderType: "LIMIT", Quantity: 0.01,
		Price: 66500, StopPrice: 67000, TakeProfit: 65000, Leverage: 5, RiskAmount: 10,
		ExpireAt: time.Now().Add(time.Minute).UnixMilli(),
	})
	if err != nil {
		t.Fatalf("limit entry: %v", err)
	}
	stop, err := b.CreateOrder(binance.NewOrderRequest{
		Symbol: "BTCUSDT", Side: "BUY", OrderType: "STOP_MARKET",
		StopPrice: 67000, ClosePosition: true,
	})
	if err != nil {
		t.Fatalf("stop: %v", err)
	}

	// Nothing has expired yet and price has not reached the entry.
	b.mu.Lock()
	b.checkOrdersLocked()
	b.mu.Unlock()
	if got, _ := b.GetOrder("BTCUSDT", entry.OrderID); got.Status != "NEW" {
		t.Fatalf("entry status = %s, want NEW before deadline", got.Status)
	}

	// Let the deadline pass. Price never returned to 66500.
	b.mu.Lock()
	b.expireOrdersLocked(time.Now().Add(2 * time.Minute).UnixMilli())
	b.mu.Unlock()
	b.refreshPrices()
	_ = fm

	if got, _ := b.GetOrder("BTCUSDT", entry.OrderID); got.Status != "EXPIRED" {
		t.Fatalf("entry status = %s, want EXPIRED", got.Status)
	}
	// The stop existed only to protect that entry; it must not outlive it.
	if got, _ := b.GetOrder("BTCUSDT", stop.OrderID); got.Status != "CANCELED" {
		t.Fatalf("stop status = %s, want CANCELED with its entry", got.Status)
	}
	positions, _ := b.GetPositions()
	if len(positions) != 0 {
		t.Fatalf("expiry must not create a position: %+v", positions)
	}
}

func TestPartialCloseKeepsProtection(t *testing.T) {
	b, _, _ := newTestBroker(t)
	if _, err := b.CreateOrder(binance.NewOrderRequest{
		Symbol: "BTCUSDT", Side: "BUY", OrderType: "MARKET", Quantity: 0.02,
		StopPrice: 64000, Leverage: 5, RiskAmount: 10,
	}); err != nil {
		t.Fatalf("entry: %v", err)
	}
	if _, err := b.CreateOrder(binance.NewOrderRequest{
		Symbol: "BTCUSDT", Side: "SELL", OrderType: "STOP_MARKET",
		StopPrice: 64000, ClosePosition: true,
	}); err != nil {
		t.Fatalf("stop: %v", err)
	}

	if _, err := b.CreateOrder(binance.NewOrderRequest{
		Symbol: "BTCUSDT", Side: "SELL", OrderType: "MARKET",
		Quantity: 0.01, ReduceOnly: true,
	}); err != nil {
		t.Fatalf("partial close: %v", err)
	}

	positions, _ := b.GetPositions()
	if len(positions) != 1 || math.Abs(positions[0].Quantity-0.01) > 1e-9 {
		t.Fatalf("positions after partial close = %+v, want 0.01 remaining", positions)
	}
	orders, _ := b.GetOpenOrders("BTCUSDT")
	foundStop := false
	for _, o := range orders {
		if o.Type == "STOP_MARKET" && o.Status == "NEW" {
			foundStop = true
		}
	}
	if !foundStop {
		t.Fatal("protective stop was cancelled by a partial close")
	}
}

func TestPaperStateSurvivesRestart(t *testing.T) {
	b, _, _ := newTestBroker(t)
	_, err := b.CreateOrder(binance.NewOrderRequest{
		Symbol: "ETHUSDT", Side: "SELL", OrderType: "MARKET", Quantity: 0.1,
		StopPrice: 3300, Leverage: 5, RiskAmount: 10,
	})
	if err != nil {
		t.Fatalf("entry: %v", err)
	}
	b.mu.Lock()
	b.saveLocked()
	b.mu.Unlock()

	b2, err := New(b.market, b.store, b.statePath, 1000)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	positions, _ := b2.GetPositions()
	if len(positions) != 1 || positions[0].Symbol != "ETHUSDT" || positions[0].Side != "SHORT" {
		t.Fatalf("restored positions = %+v", positions)
	}
}
