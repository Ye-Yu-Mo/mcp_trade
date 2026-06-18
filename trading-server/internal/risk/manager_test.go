package risk

import (
	"math"
	"testing"
)

func TestCheckNotional_Pass(t *testing.T) {
	rm := NewManager(ManagerConfig{
		MaxPositionPercent: 0.1,
		MaxStopLossPercent: 0.02,
		DailyLossLimit:     100,
	})
	err := rm.CheckOrder(CheckInput{
		Symbol:   "BTCUSDT",
		Quantity: 0.002,
		Price:    30000,
		Balance:  10000,
		DailyPnL: 0,
	})
	if err != nil {
		t.Errorf("expected pass, got: %v", err)
	}
}

func TestCheckNotional_TooSmall(t *testing.T) {
	rm := NewManager(ManagerConfig{
		MaxPositionPercent: 0.1,
		MaxStopLossPercent: 0.02,
		DailyLossLimit:     100,
	})
	err := rm.CheckOrder(CheckInput{
		Symbol:   "BTCUSDT",
		Quantity: 0.001,
		Price:    1000, // notional = 1 USDT, below min
		Balance:  10000,
		DailyPnL: 0,
	})
	if err == nil {
		t.Fatal("expected error for too small notional")
	}
}

func TestCheckPositionSize_Exceeded(t *testing.T) {
	rm := NewManager(ManagerConfig{
		MaxPositionPercent: 0.1, // 10% of 10000 = 1000
		MaxStopLossPercent: 0.02,
		DailyLossLimit:     100,
	})
	err := rm.CheckOrder(CheckInput{
		Symbol:   "BTCUSDT",
		Quantity: 0.1,
		Price:    65000, // notional = 6500 > 1000
		Balance:  10000,
		DailyPnL: 0,
	})
	if err == nil {
		t.Fatal("expected error for position size exceeded")
	}
}

func TestCheckStopLoss_Exceeded(t *testing.T) {
	rm := NewManager(ManagerConfig{
		MaxPositionPercent: 0.1,
		MaxStopLossPercent: 0.02, // 2% of 10000 = 200
		DailyLossLimit:     100,
	})
	// 0.01 BTC at 65000, stop at 64000 => notional = 650 <= 1000, loss = 0.01 * 1000 = 10 USDT, under 200
	err := rm.CheckOrder(CheckInput{
		Symbol:    "BTCUSDT",
		Quantity:  0.01,
		Price:     65000,
		StopPrice: 64000,
		Balance:   10000,
		DailyPnL:  0,
	})
	if err != nil {
		t.Errorf("expected pass for stop loss 10 USDT, got: %v", err)
	}

	// 0.005 BTC at 65000, stop at 63000 => notional = 325 <= 1000, loss = 0.005 * 2000 = 10, still under 200
	err = rm.CheckOrder(CheckInput{
		Symbol:    "BTCUSDT",
		Quantity:  0.005,
		Price:     65000,
		StopPrice: 63000,
		Balance:   10000,
		DailyPnL:  0,
	})
	if err != nil {
		t.Errorf("expected pass for stop loss 10 USDT (wide stop), got: %v", err)
	}

	// 0.005 BTC at 65000, stop at 50000 => loss = 0.005 * 15000 = 75 USDT, under 200
	err = rm.CheckOrder(CheckInput{
		Symbol:    "BTCUSDT",
		Quantity:  0.005,
		Price:     65000,
		StopPrice: 50000,
		Balance:   10000,
		DailyPnL:  0,
	})
	if err != nil {
		t.Errorf("expected pass for stop loss 75 USDT, got: %v", err)
	}

	// 0.01 BTC at 65000, stop at 40000 => loss = 0.01 * 25000 = 250, exceeds 200
	err = rm.CheckOrder(CheckInput{
		Symbol:    "BTCUSDT",
		Quantity:  0.01,
		Price:     65000,
		StopPrice: 40000,
		Balance:   10000,
		DailyPnL:  0,
	})
	if err == nil {
		t.Fatal("expected error for stop loss exceeded")
	}
}

func TestCheckDailyLoss_Exceeded(t *testing.T) {
	rm := NewManager(ManagerConfig{
		MaxPositionPercent: 0.1,
		MaxStopLossPercent: 0.02,
		DailyLossLimit:     100,
	})
	err := rm.CheckOrder(CheckInput{
		Symbol:   "BTCUSDT",
		Quantity: 0.002,
		Price:    30000,
		Balance:  10000,
		DailyPnL: -150, // already lost 150 today > 100 limit
	})
	if err == nil {
		t.Fatal("expected error for daily loss exceeded")
	}
}

func TestRoundToTickSize(t *testing.T) {
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
		got := RoundPrice(tt.symbol, tt.price)
		if math.Abs(got-tt.expected) > 0.001 {
			t.Errorf("RoundPrice(%s, %f) = %f, want %f", tt.symbol, tt.price, got, tt.expected)
		}
	}
}

func TestRoundToStepSize(t *testing.T) {
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
		got := RoundQuantity(tt.symbol, tt.qty)
		if got != tt.expected {
			t.Errorf("RoundQuantity(%s, %f) = %f, want %f", tt.symbol, tt.qty, got, tt.expected)
		}
	}
}
