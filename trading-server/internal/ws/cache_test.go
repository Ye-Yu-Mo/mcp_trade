package ws

import (
	"sync"
	"testing"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
)

func TestCache_SetGetPrice(t *testing.T) {
	c := NewMarketCache()
	c.SetPrice("BTCUSDT", 65000.0)

	p, ok := c.GetPrice("BTCUSDT")
	if !ok {
		t.Fatal("price not found")
	}
	if p != 65000.0 {
		t.Errorf("price = %f, want 65000.0", p)
	}
}

func TestCache_CaseInsensitive(t *testing.T) {
	c := NewMarketCache()
	c.SetPrice("btcusdt", 65000.0)

	p, ok := c.GetPrice("BTCUSDT")
	if !ok {
		t.Fatal("case-insensitive lookup failed")
	}
	if p != 65000.0 {
		t.Errorf("price = %f", p)
	}
}

func TestCache_SetGetKline(t *testing.T) {
	c := NewMarketCache()
	k := binance.Kline{OpenTime: 100, Open: 100, High: 110, Low: 95, Close: 105, Volume: 1000}
	c.SetKline("BTCUSDT", "1h", k)

	got, ok := c.GetKline("BTCUSDT", "1h")
	if !ok {
		t.Fatal("kline not found")
	}
	if got.Close != 105 {
		t.Errorf("close = %f, want 105", got.Close)
	}
}

func TestCache_ConcurrentReadWrite(t *testing.T) {
	c := NewMarketCache()
	var wg sync.WaitGroup

	// 100 concurrent writers
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			c.SetPrice("BTCUSDT", float64(65000+n))
		}(i)
	}

	// 100 concurrent readers
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c.GetPrice("BTCUSDT")
			c.GetKline("ETHUSDT", "1h")
			c.GetOrderBook("BTCUSDT")
		}()
	}

	wg.Wait()
	// No race = pass
}

func TestCache_MissingReturnsFalse(t *testing.T) {
	c := NewMarketCache()

	if _, ok := c.GetPrice("DEADCOIN"); ok {
		t.Error("expected false for missing symbol")
	}
	if _, ok := c.GetKline("DEADCOIN", "1h"); ok {
		t.Error("expected false for missing kline")
	}
}
