package ws

import (
	"math"
	"testing"
)

// Regression test: Binance bookTicker includes both "a" (ask price) and "A"
// (ask quantity). Go's case-insensitive JSON matching must not let "A"
// overwrite the ask price, which previously made every cached price exactly
// half of the real mid price.
func TestParseBookTickerKeepsAskPrice(t *testing.T) {
	cache := NewMarketCache()
	stream := NewMarketStream("", cache, nil, nil)

	msg := []byte(`{"e":"bookTicker","u":1,"s":"BTCUSDT","b":"80234.10","B":"9.876","a":"80234.20","A":"0.242"}`)
	stream.parseEvent("btcusdt@bookTicker", msg)

	price, ok := cache.GetPrice("BTCUSDT")
	if !ok {
		t.Fatal("price not cached")
	}
	want := 80234.15
	if math.Abs(price-want) > 0.001 {
		t.Fatalf("cached price = %.4f, want %.4f", price, want)
	}

	ob, ok := cache.GetOrderBook("BTCUSDT")
	if !ok || len(ob.Asks) != 1 {
		t.Fatalf("order book not cached correctly: %+v", ob)
	}
	if math.Abs(ob.Asks[0].Price-80234.20) > 0.001 {
		t.Fatalf("ask price = %.4f, want 80234.20", ob.Asks[0].Price)
	}
	if math.Abs(ob.Asks[0].Quantity-0.242) > 0.0001 {
		t.Fatalf("ask qty = %.4f, want 0.242", ob.Asks[0].Quantity)
	}
}
