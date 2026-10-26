package risk

import (
	"math"
	"testing"
)

func testManager() *Manager {
	m := NewManager(ManagerConfig{
		RiskPerTradePct: 0.01,
		MaxTotalRiskPct: 0.03,
		MaxPositions:    2,
		MaxLeverage:     5,
		DailyLossPct:    0.03,
		WeeklyLossPct:   0.08,
	})
	m.SetSymbolInfo(map[string]SymbolInfo{
		"BTCUSDT": {Status: "TRADING", TickSize: 0.1, StepSize: 0.001, MinNotional: 50},
		"ETHUSDT": {Status: "TRADING", TickSize: 0.01, StepSize: 0.01, MinNotional: 20},
		"SOLUSDT": {Status: "TRADING", TickSize: 0.01, StepSize: 0.01, MinNotional: 5},
	})
	return m
}

func TestEntrySizesFromRiskPct(t *testing.T) {
	m := testManager()
	d := m.CheckEntry(AccountState{Equity: 10000, Available: 10000}, OrderIntent{
		Symbol: "BTCUSDT", Side: "BUY", Type: "MARKET",
		Price: 65000, StopPrice: 64000, RiskPct: 0.5, Leverage: 5,
	})
	if !d.Passed {
		t.Fatalf("expected pass, got %s: %s", d.Code, d.Reason)
	}
	// 0.5% of 10000 = 50 USDT risk; 1000 USDT stop distance => 0.05 BTC.
	if math.Abs(d.Quantity-0.05) > 1e-9 {
		t.Fatalf("quantity = %f, want 0.05", d.Quantity)
	}
	if math.Abs(d.RiskAmount-50) > 1e-6 {
		t.Fatalf("risk = %f, want 50", d.RiskAmount)
	}
}

func TestEntryRequiresStop(t *testing.T) {
	m := testManager()
	d := m.CheckEntry(AccountState{Equity: 10000, Available: 10000}, OrderIntent{
		Symbol: "BTCUSDT", Side: "BUY", Type: "MARKET", Quantity: 0.001, Price: 65000,
	})
	if d.Passed || d.Code != "RISK_STOP_REQUIRED" {
		t.Fatalf("expected RISK_STOP_REQUIRED, got passed=%v code=%s", d.Passed, d.Code)
	}
}

func TestEntryStopOnWrongSide(t *testing.T) {
	m := testManager()
	d := m.CheckEntry(AccountState{Equity: 10000, Available: 10000}, OrderIntent{
		Symbol: "BTCUSDT", Side: "BUY", Type: "MARKET", Quantity: 0.001,
		Price: 65000, StopPrice: 66000,
	})
	if d.Passed || d.Code != "RISK_STOP_SIDE" {
		t.Fatalf("expected RISK_STOP_SIDE, got passed=%v code=%s", d.Passed, d.Code)
	}
}

func TestEntryRiskPerTradeExceeded(t *testing.T) {
	m := testManager()
	// 1% of 10000 = 100 USDT; 0.2 BTC * 1000 = 200 USDT risk.
	d := m.CheckEntry(AccountState{Equity: 10000, Available: 10000}, OrderIntent{
		Symbol: "BTCUSDT", Side: "BUY", Type: "MARKET", Quantity: 0.2,
		Price: 65000, StopPrice: 64000,
	})
	if d.Passed || d.Code != "RISK_STOP_LOSS" {
		t.Fatalf("expected RISK_STOP_LOSS, got passed=%v code=%s", d.Passed, d.Code)
	}
}

func TestEntryMinNotional(t *testing.T) {
	m := testManager()
	d := m.CheckEntry(AccountState{Equity: 10000, Available: 10000}, OrderIntent{
		Symbol: "BTCUSDT", Side: "BUY", Type: "MARKET", Quantity: 0.001,
		Price: 1000, StopPrice: 990,
	})
	if d.Passed || d.Code != "RISK_MIN_NOTIONAL" {
		t.Fatalf("expected RISK_MIN_NOTIONAL, got passed=%v code=%s", d.Passed, d.Code)
	}
}

func TestEntryTotalRiskExceeded(t *testing.T) {
	m := testManager()
	// 3% of 10000 = 300; existing 250 + new 100 > 300.
	d := m.CheckEntry(AccountState{
		Equity: 10000, Available: 10000, OpenRisk: 250, OpenPositions: 1,
	}, OrderIntent{
		Symbol: "BTCUSDT", Side: "BUY", Type: "MARKET", Quantity: 0.1,
		Price: 65000, StopPrice: 64000,
	})
	if d.Passed || d.Code != "RISK_TOTAL_RISK" {
		t.Fatalf("expected RISK_TOTAL_RISK, got passed=%v code=%s", d.Passed, d.Code)
	}
}

func TestEntryMaxPositions(t *testing.T) {
	m := testManager()
	d := m.CheckEntry(AccountState{
		Equity: 10000, Available: 10000, OpenPositions: 2,
	}, OrderIntent{
		Symbol: "BTCUSDT", Side: "BUY", Type: "MARKET", Quantity: 0.01,
		Price: 65000, StopPrice: 64000,
	})
	if d.Passed || d.Code != "RISK_MAX_POSITIONS" {
		t.Fatalf("expected RISK_MAX_POSITIONS, got passed=%v code=%s", d.Passed, d.Code)
	}
}

func TestEntryDailyLossBlocksNewEntry(t *testing.T) {
	m := testManager()
	d := m.CheckEntry(AccountState{
		Equity: 10000, Available: 10000, DailyPnL: -400,
	}, OrderIntent{
		Symbol: "BTCUSDT", Side: "BUY", Type: "MARKET", Quantity: 0.01,
		Price: 65000, StopPrice: 64000,
	})
	if d.Passed || d.Code != "RISK_DAILY_LOSS" {
		t.Fatalf("expected RISK_DAILY_LOSS, got passed=%v code=%s", d.Passed, d.Code)
	}
}

func TestReduceBypassesDailyLoss(t *testing.T) {
	m := testManager()
	d := m.CheckReduce(AccountState{Equity: 10000, DailyPnL: -10000}, OrderIntent{
		Symbol: "BTCUSDT", Side: "SELL", Type: "MARKET", Quantity: 0.01, Price: 65000,
	}, 0.01)
	if !d.Passed {
		t.Fatalf("reduce must pass even after daily loss, got %s: %s", d.Code, d.Reason)
	}
}

func TestReduceCannotExceedPosition(t *testing.T) {
	m := testManager()
	d := m.CheckReduce(AccountState{Equity: 10000}, OrderIntent{
		Symbol: "BTCUSDT", Side: "SELL", Type: "MARKET", Quantity: 0.02, Price: 65000,
	}, 0.01)
	if d.Passed || d.Code != "RISK_QTY" {
		t.Fatalf("expected RISK_QTY, got passed=%v code=%s", d.Passed, d.Code)
	}
}

func TestReduceClosePositionUsesFullSize(t *testing.T) {
	m := testManager()
	d := m.CheckReduce(AccountState{Equity: 10000}, OrderIntent{
		Symbol: "BTCUSDT", Side: "SELL", Type: "MARKET", Price: 65000, ClosePosition: true,
	}, 0.0123)
	if !d.Passed {
		t.Fatalf("expected pass, got %s: %s", d.Code, d.Reason)
	}
	if math.Abs(d.Quantity-0.012) > 1e-9 {
		t.Fatalf("quantity = %f, want 0.012 after step rounding", d.Quantity)
	}
}

func TestEntryLeverageCap(t *testing.T) {
	m := testManager()
	d := m.CheckEntry(AccountState{Equity: 10000, Available: 10000}, OrderIntent{
		Symbol: "BTCUSDT", Side: "BUY", Type: "MARKET", Quantity: 0.01,
		Price: 65000, StopPrice: 64000, Leverage: 10,
	})
	if d.Passed || d.Code != "RISK_LEVERAGE" {
		t.Fatalf("expected RISK_LEVERAGE, got passed=%v code=%s", d.Passed, d.Code)
	}
}

func TestEntryMarginCheck(t *testing.T) {
	m := testManager()
	// 0.01 BTC * 65000 = 650 notional; at 5x margin = 130 > available 100.
	d := m.CheckEntry(AccountState{Equity: 10000, Available: 100}, OrderIntent{
		Symbol: "BTCUSDT", Side: "BUY", Type: "MARKET", Quantity: 0.01,
		Price: 65000, StopPrice: 64000, Leverage: 5,
	})
	if d.Passed || d.Code != "RISK_MARGIN" {
		t.Fatalf("expected RISK_MARGIN, got passed=%v code=%s", d.Passed, d.Code)
	}
}

func TestEntryMarginCheckWhenAvailableIsZero(t *testing.T) {
	m := testManager()
	d := m.CheckEntry(AccountState{Equity: 10000, Available: 0}, OrderIntent{
		Symbol: "BTCUSDT", Side: "BUY", Type: "MARKET", Quantity: 0.01,
		Price: 65000, StopPrice: 64000, Leverage: 5,
	})
	if d.Passed || d.Code != "RISK_MARGIN" {
		t.Fatalf("expected RISK_MARGIN with zero available, got passed=%v code=%s", d.Passed, d.Code)
	}
}

func TestRoundToTickSize(t *testing.T) {
	m := testManager()
	tests := []struct {
		symbol   string
		price    float64
		expected float64
	}{
		{"BTCUSDT", 65000.123, 65000.1},
		{"BTCUSDT", 65000.156, 65000.2},
		{"ETHUSDT", 3200.123, 3200.12},
		{"ETHUSDT", 3200.126, 3200.13},
	}
	for _, tt := range tests {
		got := m.RoundPrice(tt.symbol, tt.price)
		if math.Abs(got-tt.expected) > 0.001 {
			t.Errorf("RoundPrice(%s, %f) = %f, want %f", tt.symbol, tt.price, got, tt.expected)
		}
	}
}

func TestRoundToStepSize(t *testing.T) {
	m := testManager()
	tests := []struct {
		symbol   string
		qty      float64
		expected float64
	}{
		{"BTCUSDT", 0.0015, 0.001},
		{"BTCUSDT", 0.0019, 0.001},
		{"ETHUSDT", 0.015, 0.01},
		{"ETHUSDT", 0.016, 0.01},
	}
	for _, tt := range tests {
		got := m.RoundQuantity(tt.symbol, tt.qty)
		if got != tt.expected {
			t.Errorf("RoundQuantity(%s, %f) = %f, want %f", tt.symbol, tt.qty, got, tt.expected)
		}
	}
}
