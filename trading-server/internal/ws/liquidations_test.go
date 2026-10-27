package ws

import (
	"testing"
	"time"
)

func TestLiquidationStoreFiltersAndSummarizes(t *testing.T) {
	s := NewLiquidationStore(10)
	now := time.Now().UnixMilli()

	s.Add(Liquidation{Time: now - 5000, Symbol: "BTCUSDT", Side: "SELL", Notional: 5000})
	s.Add(Liquidation{Time: now - 4000, Symbol: "BTCUSDT", Side: "BUY", Notional: 1500})
	s.Add(Liquidation{Time: now - 3000, Symbol: "DOODUSDT", Side: "SELL", Notional: 100})
	s.Add(Liquidation{Time: now, Symbol: "BTCUSDT", Side: "SELL", Notional: 9000})

	// Newest first.
	all := s.List("", 0, 0, 0)
	if len(all) != 4 || all[0].Notional != 9000 {
		t.Fatalf("list order wrong: %+v", all)
	}

	// Symbol + notional filters.
	btcBig := s.List("BTCUSDT", 2000, 0, 10)
	if len(btcBig) != 2 {
		t.Fatalf("filtered list = %d, want 2: %+v", len(btcBig), btcBig)
	}

	// Time bound drops the oldest entries.
	recent := s.List("", 0, now-3500, 10)
	if len(recent) != 2 {
		t.Fatalf("since filter = %d, want 2", len(recent))
	}

	sum := s.Summary(time.Hour)
	if len(sum) != 2 || sum[0].Symbol != "BTCUSDT" {
		t.Fatalf("summary = %+v", sum)
	}
	if sum[0].LongLiqs != 2 || sum[0].ShortLiqs != 1 {
		t.Fatalf("BTC aggregation wrong: %+v", sum[0])
	}
	if sum[0].LongNotional != 14000 || sum[0].ShortNotional != 1500 {
		t.Fatalf("BTC notional wrong: %+v", sum[0])
	}
}

func TestLiquidationStoreDropsOldestWhenFull(t *testing.T) {
	s := NewLiquidationStore(2)
	now := time.Now().UnixMilli()
	s.Add(Liquidation{Time: now - 2, Symbol: "A", Side: "SELL"})
	s.Add(Liquidation{Time: now - 1, Symbol: "B", Side: "SELL"})
	s.Add(Liquidation{Time: now, Symbol: "C", Side: "SELL"})

	got := s.List("", 0, 0, 0)
	if len(got) != 2 || got[0].Symbol != "C" || got[1].Symbol != "B" {
		t.Fatalf("ring buffer wrong: %+v", got)
	}
}

func TestParseForceOrderStoresLiquidation(t *testing.T) {
	m := &MarketStream{liqs: NewLiquidationStore(10)}

	// SELL side means a long was force-closed.
	raw := []byte(`{"e":"forceOrder","E":1793070000000,"o":{"s":"DOODUSDT","S":"SELL","p":"0.0025","ap":"0.00249","z":"400000","T":1793070000000}}`)
	m.parseForceOrder(raw)

	got := m.liqs.List("DOODUSDT", 0, 0, 10)
	if len(got) != 1 {
		t.Fatalf("stored %d liquidations, want 1", len(got))
	}
	l := got[0]
	if l.Side != "SELL" || l.Symbol != "DOODUSDT" || l.Qty != 400000 {
		t.Fatalf("unexpected liquidation: %+v", l)
	}
	// Avg price wins over the limit price, and notional = avg * qty.
	if l.Price != 0.00249 {
		t.Fatalf("price = %v, want avg 0.00249", l.Price)
	}
	if diff := l.Notional - 0.00249*400000; diff > 1e-9 || diff < -1e-9 {
		t.Fatalf("notional = %v", l.Notional)
	}

	// Malformed payloads must not panic or store anything.
	m.parseForceOrder([]byte(`{"e":"forceOrder"}`))
	if n := len(m.liqs.List("", 0, 0, 10)); n != 1 {
		t.Fatalf("malformed payload stored extra entries: %d", n)
	}
}
