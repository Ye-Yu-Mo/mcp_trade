// Package reconcile closes position-level trade rows that the exchange has
// already closed but the user-data stream missed (disconnects, server restart,
// or events that never arrived). It uses the account's own fill history so the
// recorded PnL and fees come from Binance rather than an estimate.
package reconcile

import (
	"log"
	"time"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/store"
)

// Trader is the exchange surface needed for reconciliation.
type Trader interface {
	GetPositions() ([]binance.Position, error)
	GetUserTrades(symbol string, startTime int64, limit int) ([]binance.UserTrade, error)
}

// Store is the persistence surface needed for reconciliation.
type Store interface {
	OpenTrades() ([]store.TradeRecord, error)
	CloseTrade(tradeID int64, exitPrice, pnl, fees, rMultiple float64, exitReason string) error
}

// Once closes OPEN trades whose symbol no longer has an exchange position.
// It returns the number of trades closed.
func Once(client Trader, st Store) (int, error) {
	trades, err := st.OpenTrades()
	if err != nil {
		return 0, err
	}
	if len(trades) == 0 {
		return 0, nil
	}
	positions, err := client.GetPositions()
	if err != nil {
		return 0, err
	}
	active := make(map[string]bool, len(positions))
	for _, p := range positions {
		if p.Quantity > 0 {
			active[p.Symbol] = true
		}
	}

	closed := 0
	for _, t := range trades {
		if active[t.Symbol] {
			continue
		}
		start := t.CreatedAt.Add(-10 * time.Second).UnixMilli()
		fills, err := client.GetUserTrades(t.Symbol, start, 1000)
		if err != nil {
			log.Printf("reconcile: userTrades %s: %v", t.Symbol, err)
			continue
		}

		var realized, commission, exitQty, exitNotional float64
		for _, f := range fills {
			commission += f.Commission
			if f.RealizedPnl != 0 {
				realized += f.RealizedPnl
				exitQty += f.Qty
				exitNotional += f.Price * f.Qty
			}
		}
		exitPrice := 0.0
		if exitQty > 0 {
			exitPrice = exitNotional / exitQty
		}
		net := realized - commission
		rMultiple := 0.0
		if t.RiskAmount > 0 {
			rMultiple = net / t.RiskAmount
		}
		reason := "RECONCILED"
		if len(fills) == 0 {
			reason = "RECONCILED_NO_FILLS"
		}
		if err := st.CloseTrade(t.ID, exitPrice, net, commission, rMultiple, reason); err != nil {
			log.Printf("reconcile: close trade %d %s: %v", t.ID, t.Symbol, err)
			continue
		}
		closed++
		log.Printf("reconcile: closed trade %d %s pnl=%.4f fees=%.4f (%.2fR) reason=%s",
			t.ID, t.Symbol, net, commission, rMultiple, reason)
	}
	return closed, nil
}
