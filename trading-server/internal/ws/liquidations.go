package ws

import (
	"sort"
	"sync"
	"time"
)

// Liquidation is one forced order from the !forceOrder stream. Side is the
// side of the liquidation order: SELL means a long position was force-closed,
// BUY means a short was. Clusters of one side mark where stops are stacked,
// which is exactly where a wick tends to hunt.
type Liquidation struct {
	Time     int64   `json:"time"`
	Symbol   string  `json:"symbol"`
	Side     string  `json:"side"`
	Price    float64 `json:"price"`
	AvgPrice float64 `json:"avg_price"`
	Qty      float64 `json:"qty"`
	Notional float64 `json:"notional"`
}

// LiquidationStore keeps a bounded, time-ordered window of liquidations.
// The all-market stream is a firehose, so writes must stay cheap: no sorting
// on write, and the oldest entries are dropped once the cap is reached.
type LiquidationStore struct {
	mu    sync.RWMutex
	items []Liquidation
	max   int
}

// NewLiquidationStore creates a store that keeps at most max entries.
func NewLiquidationStore(max int) *LiquidationStore {
	if max <= 0 {
		max = 2000
	}
	return &LiquidationStore{items: make([]Liquidation, 0, max), max: max}
}

// Add appends one liquidation, dropping the oldest when full.
func (s *LiquidationStore) Add(l Liquidation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.items) >= s.max {
		copy(s.items, s.items[1:])
		s.items = s.items[:len(s.items)-1]
	}
	s.items = append(s.items, l)
}

// List returns liquidations newest-first, optionally filtered by symbol and a
// minimum notional. since (Unix ms, 0 = no bound) limits how far back to look.
func (s *LiquidationStore) List(symbol string, minNotional float64, since int64, limit int) []Liquidation {
	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]Liquidation, 0, 64)
	for i := len(s.items) - 1; i >= 0; i-- {
		l := s.items[i]
		if since > 0 && l.Time < since {
			break
		}
		if symbol != "" && l.Symbol != symbol {
			continue
		}
		if minNotional > 0 && l.Notional < minNotional {
			continue
		}
		out = append(out, l)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out
}

// LiquidationSummary aggregates the recent window per symbol.
type LiquidationSummary struct {
	Symbol        string  `json:"symbol"`
	LongLiqs      int     `json:"long_liquidations"`
	ShortLiqs     int     `json:"short_liquidations"`
	LongNotional  float64 `json:"long_notional"`
	ShortNotional float64 `json:"short_notional"`
}

// Summary aggregates every symbol in the window, largest notional first.
func (s *LiquidationStore) Summary(window time.Duration) []LiquidationSummary {
	since := time.Now().Add(-window).UnixMilli()
	s.mu.RLock()
	defer s.mu.RUnlock()

	bySym := make(map[string]*LiquidationSummary)
	for i := len(s.items) - 1; i >= 0; i-- {
		l := s.items[i]
		if since > 0 && l.Time < since {
			break
		}
		e, ok := bySym[l.Symbol]
		if !ok {
			e = &LiquidationSummary{Symbol: l.Symbol}
			bySym[l.Symbol] = e
		}
		if l.Side == "SELL" {
			e.LongLiqs++
			e.LongNotional += l.Notional
		} else {
			e.ShortLiqs++
			e.ShortNotional += l.Notional
		}
	}

	out := make([]LiquidationSummary, 0, len(bySym))
	for _, e := range bySym {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].LongNotional+out[i].ShortNotional > out[j].LongNotional+out[j].ShortNotional
	})
	return out
}
