package risk

import (
	"fmt"
	"math"
)

// SymbolInfo holds exchange-level constraints for a trading pair.
type SymbolInfo struct {
	TickSize   float64 // 最小价格变动单位
	StepSize   float64 // 最小数量变动单位
	MinNotional float64 // 最小名义价值 (USDT)
}

// Common symbol definitions.
var symbolInfo = map[string]SymbolInfo{
	"BTCUSDT": {TickSize: 0.1, StepSize: 0.001, MinNotional: 50},
	"ETHUSDT": {TickSize: 0.01, StepSize: 0.01, MinNotional: 20},
}

func getSymbolInfo(symbol string) SymbolInfo {
	if info, ok := symbolInfo[symbol]; ok {
		return info
	}
	// Default conservative values for unknown symbols
	return SymbolInfo{TickSize: 0.01, StepSize: 0.01, MinNotional: 10}
}

// ManagerConfig holds risk control parameters.
type ManagerConfig struct {
	MaxPositionPercent float64 // 单笔仓位上限，占余额比例（0-1）
	MaxStopLossPercent float64 // 单笔止损上限，占余额比例（0-1）
	DailyLossLimit     float64 // 每日最大亏损 (USDT)
}

// Manager performs pre-trade risk checks.
type Manager struct {
	cfg ManagerConfig
}

// NewManager creates a risk manager.
func NewManager(cfg ManagerConfig) *Manager {
	return &Manager{cfg: cfg}
}

// CheckInput is the input for a risk check.
type CheckInput struct {
	Symbol    string
	Quantity  float64
	Price     float64
	StopPrice float64 // 0 = no stop
	Balance   float64 // account balance in USDT
	DailyPnL  float64 // running daily PnL (negative = loss)
}

// CheckResult holds the result of risk checks.
type CheckResult struct {
	Passed   bool
	RejectCode   string
	RejectReason string
}

// CheckOrder performs all pre-trade risk checks. Returns nil if order is safe.
func (m *Manager) CheckOrder(in CheckInput) error {
	info := getSymbolInfo(in.Symbol)
	notional := in.Quantity * in.Price

	// 1. Exchange-level: minimum notional
	if notional < info.MinNotional {
		return fmt.Errorf("%s: notional %.2f < min %.2f USDT (qty=%.4f, price=%.2f)",
			"RISK_MIN_NOTIONAL", notional, info.MinNotional, in.Quantity, in.Price)
	}

	// 2. Account-level: position size
	maxNotional := in.Balance * m.cfg.MaxPositionPercent
	if notional > maxNotional {
		return fmt.Errorf("%s: notional %.2f > max %.2f USDT (%.0f%% of balance)",
			"RISK_POSITION_SIZE", notional, maxNotional, m.cfg.MaxPositionPercent*100)
	}

	// 3. Account-level: stop loss (only if stopPrice is set)
	if in.StopPrice > 0 {
		var loss float64
		if in.Price > in.StopPrice {
			loss = (in.Price - in.StopPrice) * in.Quantity // long stop loss
		} else {
			loss = (in.StopPrice - in.Price) * in.Quantity // short stop loss
		}
		maxLoss := in.Balance * m.cfg.MaxStopLossPercent
		if loss > maxLoss {
			return fmt.Errorf("%s: stop loss %.2f > max %.2f USDT (%.0f%% of balance)",
				"RISK_STOP_LOSS", loss, maxLoss, m.cfg.MaxStopLossPercent*100)
		}
	}

	// 4. Account-level: daily loss limit
	if in.DailyPnL < -m.cfg.DailyLossLimit {
		return fmt.Errorf("%s: daily loss %.2f > limit %.2f USDT",
			"RISK_DAILY_LOSS", -in.DailyPnL, m.cfg.DailyLossLimit)
	}

	return nil
}

// RoundPrice rounds a price to the symbol's tick size.
func RoundPrice(symbol string, price float64) float64 {
	info := getSymbolInfo(symbol)
	return math.Round(price/info.TickSize) * info.TickSize
}

// RoundQuantity rounds a quantity down to the symbol's step size.
func RoundQuantity(symbol string, qty float64) float64 {
	info := getSymbolInfo(symbol)
	return math.Floor(qty/info.StepSize) * info.StepSize
}
