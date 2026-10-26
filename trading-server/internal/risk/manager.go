package risk

import (
	"fmt"
	"math"
	"strings"
)

// SymbolInfo holds exchange rules for one symbol.
type SymbolInfo struct {
	Status      string
	TickSize    float64
	StepSize    float64
	MinNotional float64
	MaxLeverage float64
}

// ManagerConfig holds the account-level risk budget.
type ManagerConfig struct {
	RiskPerTradePct    float64 // fraction of equity risked per trade (0.01 = 1%)
	MaxTotalRiskPct    float64 // max sum of open risk (0.03 = 3%)
	MaxPositions       int     // max concurrent positions
	MaxLeverage        float64 // cap on order leverage
	DailyLossPct       float64 // stop new entries after this loss (0.03 = 3%)
	WeeklyLossPct      float64 // trailing 7-day loss stop (0.08 = 8%)
	MinNotionalDefault float64
}

// Manager performs pre-trade checks. It never mutates account state.
type Manager struct {
	cfg     ManagerConfig
	symbols map[string]SymbolInfo
}

// NewManager creates a risk manager with sane defaults for any zero field.
func NewManager(cfg ManagerConfig) *Manager {
	if cfg.RiskPerTradePct <= 0 {
		cfg.RiskPerTradePct = 0.01
	}
	if cfg.MaxTotalRiskPct <= 0 {
		cfg.MaxTotalRiskPct = 0.03
	}
	if cfg.MaxPositions <= 0 {
		cfg.MaxPositions = 2
	}
	if cfg.MaxLeverage <= 0 {
		cfg.MaxLeverage = 5
	}
	if cfg.DailyLossPct <= 0 {
		cfg.DailyLossPct = 0.03
	}
	if cfg.WeeklyLossPct <= 0 {
		cfg.WeeklyLossPct = 0.08
	}
	if cfg.MinNotionalDefault <= 0 {
		cfg.MinNotionalDefault = 5
	}
	return &Manager{cfg: cfg, symbols: make(map[string]SymbolInfo)}
}

// SetSymbolInfo replaces the exchange rule table (normally loaded from
// /fapi/v1/exchangeInfo at startup).
func (m *Manager) SetSymbolInfo(symbols map[string]SymbolInfo) {
	m.symbols = symbols
}

// Config returns the effective risk configuration.
func (m *Manager) Config() ManagerConfig { return m.cfg }

// Symbol returns the rules for a symbol, falling back to conservative defaults.
func (m *Manager) Symbol(symbol string) SymbolInfo {
	if info, ok := m.symbols[strings.ToUpper(symbol)]; ok {
		if info.MinNotional <= 0 {
			info.MinNotional = m.cfg.MinNotionalDefault
		}
		return info
	}
	return SymbolInfo{
		TickSize:    1e-8,
		StepSize:    1e-6,
		MinNotional: m.cfg.MinNotionalDefault,
	}
}

// AccountState is a snapshot used for risk checks.
type AccountState struct {
	Equity        float64 // wallet + unrealized
	Wallet        float64
	Available     float64
	DailyPnL      float64
	WeeklyPnL     float64
	OpenPositions int
	OpenRisk      float64
}

// OrderIntent describes the order being checked.
type OrderIntent struct {
	Symbol        string
	Side          string // BUY / SELL
	Type          string // MARKET / LIMIT
	Quantity      float64
	Price         float64 // entry price (market price for MARKET)
	StopPrice     float64
	TakeProfit    float64
	RiskPct       float64 // optional: size from this fraction of equity
	ReduceOnly    bool
	ClosePosition bool
	Leverage      float64
}

// Decision is the result of a risk check.
type Decision struct {
	Passed     bool
	Code       string
	Reason     string
	Quantity   float64
	EntryPrice float64
	StopPrice  float64
	TakeProfit float64
	Notional   float64
	RiskAmount float64
	RiskPct    float64
	Margin     float64
	Leverage   float64
	Warnings   []string
}

func reject(code, format string, args ...interface{}) Decision {
	return Decision{Passed: false, Code: code, Reason: fmt.Sprintf(format, args...)}
}

// CheckEntry validates a new position. A protective stop is mandatory.
func (m *Manager) CheckEntry(acc AccountState, in OrderIntent) Decision {
	side := strings.ToUpper(strings.TrimSpace(in.Side))
	orderType := strings.ToUpper(strings.TrimSpace(in.Type))
	if side != "BUY" && side != "SELL" {
		return reject("RISK_SIDE", "side must be BUY or SELL")
	}
	if orderType != "MARKET" && orderType != "LIMIT" {
		return reject("RISK_TYPE", "entry type must be MARKET or LIMIT")
	}
	if acc.Equity <= 0 {
		return reject("RISK_EQUITY", "account equity is not positive")
	}

	info := m.Symbol(in.Symbol)
	if info.Status != "" && info.Status != "TRADING" {
		return reject("RISK_SYMBOL", "%s is not currently trading (status=%s)", in.Symbol, info.Status)
	}
	if in.Price <= 0 {
		return reject("RISK_PRICE", "entry price is required")
	}
	entry := m.RoundPrice(in.Symbol, in.Price)
	if entry <= 0 {
		return reject("RISK_PRICE", "entry price is invalid")
	}
	if in.StopPrice <= 0 {
		return reject("RISK_STOP_REQUIRED", "a protective stop is mandatory for every entry")
	}
	stop := m.RoundPrice(in.Symbol, in.StopPrice)
	if stop == entry {
		return reject("RISK_STOP_SIDE", "stop price must differ from entry price")
	}
	if side == "BUY" && stop >= entry {
		return reject("RISK_STOP_SIDE", "long stop must be below entry (entry=%.8f stop=%.8f)", entry, stop)
	}
	if side == "SELL" && stop <= entry {
		return reject("RISK_STOP_SIDE", "short stop must be above entry (entry=%.8f stop=%.8f)", entry, stop)
	}

	takeProfit := 0.0
	if in.TakeProfit > 0 {
		takeProfit = m.RoundPrice(in.Symbol, in.TakeProfit)
		if side == "BUY" && takeProfit <= entry {
			return reject("RISK_TP_SIDE", "long take-profit must be above entry")
		}
		if side == "SELL" && takeProfit >= entry {
			return reject("RISK_TP_SIDE", "short take-profit must be below entry")
		}
	}

	maxRisk := acc.Equity * m.cfg.RiskPerTradePct
	stopDistance := math.Abs(entry - stop)
	if stopDistance <= 0 {
		return reject("RISK_STOP_SIDE", "stop distance is zero")
	}

	qty := in.Quantity
	if qty <= 0 {
		if in.RiskPct <= 0 {
			return reject("RISK_QTY", "quantity or risk_pct is required")
		}
		if in.RiskPct > m.cfg.RiskPerTradePct*100+1e-9 {
			return reject("RISK_RISK_PCT", "risk_pct %.2f%% exceeds per-trade limit %.2f%%",
				in.RiskPct, m.cfg.RiskPerTradePct*100)
		}
		qty = (acc.Equity * in.RiskPct / 100) / stopDistance
	}
	qty = m.RoundQuantity(in.Symbol, qty)
	if qty <= 0 {
		return reject("RISK_QTY", "quantity rounds to zero (step=%.8f)", info.StepSize)
	}

	notional := qty * entry
	minNotional := info.MinNotional
	if minNotional <= 0 {
		minNotional = m.cfg.MinNotionalDefault
	}
	if notional < minNotional {
		minQty := m.RoundQuantity(in.Symbol, minNotional/entry+info.StepSize)
		if minQty <= 0 {
			minQty = minNotional / entry
		}
		return reject("RISK_MIN_NOTIONAL",
			"notional %.2f < min %.2f USDT; at least %.8f %s is required",
			notional, minNotional, minQty, in.Symbol)
	}

	leverage := in.Leverage
	if leverage <= 0 {
		leverage = m.cfg.MaxLeverage
	}
	if leverage > m.cfg.MaxLeverage {
		return reject("RISK_LEVERAGE", "leverage %.1fx exceeds cap %.1fx", leverage, m.cfg.MaxLeverage)
	}
	margin := notional / leverage
	if margin > acc.Available+1e-9 {
		return reject("RISK_MARGIN",
			"margin %.2f USDT > available %.2f USDT (leverage %.1fx)",
			margin, acc.Available, leverage)
	}

	riskAmount := stopDistance * qty
	if riskAmount > maxRisk+1e-9 {
		maxQty := m.RoundQuantity(in.Symbol, maxRisk/stopDistance)
		return reject("RISK_STOP_LOSS",
			"risk %.2f USDT (%.2f%% of equity) > per-trade limit %.2f USDT; max qty %.8f",
			riskAmount, riskAmount/acc.Equity*100, maxRisk, maxQty)
	}
	if acc.OpenPositions >= m.cfg.MaxPositions {
		return reject("RISK_MAX_POSITIONS", "already holding %d positions (max %d)",
			acc.OpenPositions, m.cfg.MaxPositions)
	}
	maxTotalRisk := acc.Equity * m.cfg.MaxTotalRiskPct
	if acc.OpenRisk+riskAmount > maxTotalRisk+1e-9 {
		return reject("RISK_TOTAL_RISK",
			"open risk %.2f + new %.2f > total limit %.2f USDT",
			acc.OpenRisk, riskAmount, maxTotalRisk)
	}
	if acc.DailyPnL <= -acc.Equity*m.cfg.DailyLossPct {
		return reject("RISK_DAILY_LOSS",
			"daily loss %.2f reached limit %.2f USDT; new entries disabled",
			acc.DailyPnL, -acc.Equity*m.cfg.DailyLossPct)
	}
	if acc.WeeklyPnL <= -acc.Equity*m.cfg.WeeklyLossPct {
		return reject("RISK_WEEKLY_LOSS",
			"weekly loss %.2f reached limit %.2f USDT; new entries disabled",
			acc.WeeklyPnL, -acc.Equity*m.cfg.WeeklyLossPct)
	}

	warnings := []string{}
	if riskAmount < acc.Equity*0.001 {
		warnings = append(warnings, "risk is below 0.1% of equity; fees may dominate")
	}
	if leverage > 10 {
		warnings = append(warnings, "high leverage; liquidation distance is small")
	}

	return Decision{
		Passed:     true,
		Quantity:   qty,
		EntryPrice: entry,
		StopPrice:  stop,
		TakeProfit: takeProfit,
		Notional:   notional,
		RiskAmount: riskAmount,
		RiskPct:    riskAmount / acc.Equity * 100,
		Margin:     margin,
		Leverage:   leverage,
		Warnings:   warnings,
	}
}

// CheckReduce validates a close/reduce order. Loss limits never block a
// risk-reducing action.
func (m *Manager) CheckReduce(acc AccountState, in OrderIntent, positionQty float64) Decision {
	side := strings.ToUpper(strings.TrimSpace(in.Side))
	if side != "BUY" && side != "SELL" {
		return reject("RISK_SIDE", "side must be BUY or SELL")
	}
	if positionQty <= 0 {
		return reject("RISK_NO_POSITION", "no open position to reduce")
	}
	qty := in.Quantity
	if in.ClosePosition {
		qty = positionQty
	}
	if qty <= 0 {
		return reject("RISK_QTY", "quantity must be positive (or set close_position)")
	}
	if qty > positionQty+1e-9 {
		return reject("RISK_QTY", "reduce quantity %.8f > position %.8f", qty, positionQty)
	}
	qty = m.RoundQuantity(in.Symbol, qty)
	if qty <= 0 {
		return reject("RISK_QTY", "quantity rounds to zero")
	}

	entry := in.Price
	if entry <= 0 {
		entry = 0
	}
	notional := qty * entry
	minNotional := m.Symbol(in.Symbol).MinNotional
	if minNotional <= 0 {
		minNotional = m.cfg.MinNotionalDefault
	}
	warnings := []string{}
	if notional > 0 && notional < minNotional {
		warnings = append(warnings, fmt.Sprintf(
			"reduce notional %.2f is below exchange minimum %.2f; use close_position for the full position",
			notional, minNotional))
	}

	return Decision{
		Passed:     true,
		Quantity:   qty,
		EntryPrice: entry,
		Notional:   notional,
		Warnings:   warnings,
	}
}

// RoundPrice rounds to the symbol tick size.
func (m *Manager) RoundPrice(symbol string, price float64) float64 {
	info := m.Symbol(symbol)
	tick := info.TickSize
	if tick <= 0 {
		return price
	}
	return roundTo(price, tick, false)
}

// RoundQuantity rounds down to the symbol step size (never over-size).
func (m *Manager) RoundQuantity(symbol string, qty float64) float64 {
	info := m.Symbol(symbol)
	step := info.StepSize
	if step <= 0 {
		return qty
	}
	return roundTo(qty, step, true)
}

func roundTo(v, step float64, down bool) float64 {
	if step <= 0 {
		return v
	}
	n := v / step
	if down {
		n = math.Floor(n + 1e-9)
	} else {
		n = math.Round(n)
	}
	return math.Round(n*step*1e8) / 1e8
}
