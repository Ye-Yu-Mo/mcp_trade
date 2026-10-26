package reconcile

import (
	"errors"
	"math"
	"testing"
	"time"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/store"
)

type fakeTrader struct {
	positions []binance.Position
	fills     map[string][]binance.UserTrade
	err       error
}

func (f *fakeTrader) GetPositions() ([]binance.Position, error) {
	return f.positions, f.err
}
func (f *fakeTrader) GetUserTrades(symbol string, startTime int64, limit int) ([]binance.UserTrade, error) {
	return f.fills[symbol], nil
}

type closedTrade struct {
	id, exitPrice, pnl, fees, r float64
	reason                      string
}

type fakeStore struct {
	trades []store.TradeRecord
	closed []closedTrade
}

func (s *fakeStore) OpenTrades() ([]store.TradeRecord, error) { return s.trades, nil }
func (s *fakeStore) CloseTrade(id int64, exitPrice, pnl, fees, rMultiple float64, reason string) error {
	s.closed = append(s.closed, closedTrade{float64(id), exitPrice, pnl, fees, rMultiple, reason})
	return nil
}

func TestOnceClosesFlatTradeWithExchangeFills(t *testing.T) {
	now := time.Now().UTC()
	trade := store.TradeRecord{
		ID: 1, Symbol: "SOLUSDT", Side: "SHORT", Quantity: 0.06,
		Price: 100.93, Status: "OPEN", RiskAmount: 10, CreatedAt: now.Add(-time.Minute),
	}
	tr := &fakeTrader{fills: map[string][]binance.UserTrade{
		"SOLUSDT": {
			{Symbol: "SOLUSDT", Price: 100.93, Qty: 0.06, RealizedPnl: 0, Commission: 0.01, Time: now.Add(-50 * time.Second).UnixMilli()},
			{Symbol: "SOLUSDT", Price: 101, Qty: 0.06, RealizedPnl: 1.0, Commission: 0.02, Time: now.Add(-10 * time.Second).UnixMilli()},
		},
	}}
	st := &fakeStore{trades: []store.TradeRecord{trade}}

	n, err := Once(tr, st)
	if err != nil || n != 1 {
		t.Fatalf("Once = %d, %v; want 1, nil", n, err)
	}
	if len(st.closed) != 1 {
		t.Fatalf("closed = %d, want 1", len(st.closed))
	}
	c := st.closed[0]
	if math.Abs(c.exitPrice-101) > 1e-9 || math.Abs(c.pnl-0.97) > 1e-9 ||
		math.Abs(c.fees-0.03) > 1e-9 || math.Abs(c.r-0.097) > 1e-9 {
		t.Fatalf("unexpected close: %+v", c)
	}
	if c.reason != "RECONCILED" {
		t.Fatalf("reason = %s, want RECONCILED", c.reason)
	}
}

func TestOnceSkipsActivePosition(t *testing.T) {
	tr := &fakeTrader{positions: []binance.Position{{Symbol: "SOLUSDT", Quantity: 0.06}}}
	st := &fakeStore{trades: []store.TradeRecord{{
		ID: 1, Symbol: "SOLUSDT", Status: "OPEN", CreatedAt: time.Now().Add(-time.Minute),
	}}}
	n, err := Once(tr, st)
	if err != nil || n != 0 || len(st.closed) != 0 {
		t.Fatalf("Once = %d, %v, closed=%d; want 0, nil, 0", n, err, len(st.closed))
	}
}

func TestOnceDoesNotCloseWhenPositionsUnavailable(t *testing.T) {
	tr := &fakeTrader{err: errors.New("network")}
	st := &fakeStore{trades: []store.TradeRecord{{
		ID: 1, Symbol: "SOLUSDT", Status: "OPEN", CreatedAt: time.Now().Add(-time.Minute),
	}}}
	if _, err := Once(tr, st); err == nil {
		t.Fatal("expected error when positions are unavailable")
	}
	if len(st.closed) != 0 {
		t.Fatal("must not close trades without a confirmed flat position")
	}
}
