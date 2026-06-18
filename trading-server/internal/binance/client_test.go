package binance

import (
	"os"
	"testing"
)

// testClient creates a Client for Testnet integration tests.
// Skip tests if credentials are not set.
func testClient(t *testing.T) *Client {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	apiKey := os.Getenv("BINANCE_API_KEY")
	apiSecret := os.Getenv("BINANCE_API_SECRET")
	if apiKey == "" || apiSecret == "" {
		t.Skip("BINANCE_API_KEY and BINANCE_API_SECRET required for integration test")
	}
	baseURL := os.Getenv("BINANCE_BASE_URL")
	if baseURL == "" {
		baseURL = "https://testnet.binancefuture.com"
	}
	return NewClient(apiKey, apiSecret, baseURL)
}

func TestGetKlines(t *testing.T) {
	client := testClient(t)
	klines, err := client.GetKlines("BTCUSDT", "1h", 10)
	if err != nil {
		t.Fatalf("GetKlines() error: %v", err)
	}
	if len(klines) != 10 {
		t.Errorf("GetKlines() returned %d klines, want 10", len(klines))
	}
	for i, k := range klines {
		if k.OpenTime == 0 {
			t.Errorf("kline[%d].OpenTime is zero", i)
		}
		if k.Open <= 0 {
			t.Errorf("kline[%d].Open = %f, want > 0", i, k.Open)
		}
		if k.High < k.Low {
			t.Errorf("kline[%d].High (%f) < Low (%f)", i, k.High, k.Low)
		}
		if k.Close < k.Low || k.Close > k.High {
			t.Errorf("kline[%d].Close (%f) not in [Low, High]", i, k.Close)
		}
		if k.Volume < 0 {
			t.Errorf("kline[%d].Volume = %f, want >= 0", i, k.Volume)
		}
	}
}

func TestGetKlines_DefaultLimit(t *testing.T) {
	client := testClient(t)
	klines, err := client.GetKlines("BTCUSDT", "1h", 0)
	if err != nil {
		t.Fatalf("GetKlines(limit=0) error: %v", err)
	}
	if len(klines) != 500 {
		t.Errorf("GetKlines(limit=0) returned %d klines, want 500 (default)", len(klines))
	}
}

func TestGetKlines_InvalidSymbol(t *testing.T) {
	client := testClient(t)
	_, err := client.GetKlines("DEADCOIN", "1h", 10)
	if err == nil {
		t.Fatal("GetKlines(DEADCOIN) expected error, got nil")
	}
}

func TestGetTicker(t *testing.T) {
	client := testClient(t)
	ticker, err := client.GetTicker("BTCUSDT")
	if err != nil {
		t.Fatalf("GetTicker() error: %v", err)
	}
	if ticker.Symbol != "BTCUSDT" {
		t.Errorf("Symbol = %q, want %q", ticker.Symbol, "BTCUSDT")
	}
	if ticker.Price <= 0 {
		t.Errorf("Price = %f, want > 0", ticker.Price)
	}
}

func TestGetBalance(t *testing.T) {
	client := testClient(t)
	balances, err := client.GetBalance()
	if err != nil {
		t.Fatalf("GetBalance() error: %v", err)
	}
	if len(balances) == 0 {
		t.Log("GetBalance() returned empty balance list — this is normal for a fresh Testnet account")
	}
	for _, b := range balances {
		if b.Asset == "" {
			t.Error("balance.Asset is empty")
		}
		if b.TotalBalance < 0 {
			t.Errorf("balance[%s].TotalBalance = %f, want >= 0", b.Asset, b.TotalBalance)
		}
	}
}

func TestGetPositions(t *testing.T) {
	client := testClient(t)
	positions, err := client.GetPositions()
	if err != nil {
		t.Fatalf("GetPositions() error: %v", err)
	}
	for _, p := range positions {
		if p.Symbol == "" {
			t.Error("position.Symbol is empty")
		}
		if p.Quantity == 0 {
			t.Error("position.Quantity is 0 — only non-zero positions should be returned")
		}
		if p.Side != "LONG" && p.Side != "SHORT" {
			t.Errorf("position.Side = %q, want LONG or SHORT", p.Side)
		}
	}
}

func TestGetOrderBook(t *testing.T) {
	client := testClient(t)
	ob, err := client.GetOrderBook("BTCUSDT", 10)
	if err != nil {
		t.Fatalf("GetOrderBook() error: %v", err)
	}
	if ob.Symbol != "BTCUSDT" {
		t.Errorf("Symbol = %q, want BTCUSDT", ob.Symbol)
	}
	if len(ob.Bids) == 0 {
		t.Fatal("bids is empty")
	}
	if len(ob.Asks) == 0 {
		t.Fatal("asks is empty")
	}
	// Bids should be descending, asks ascending
	for i := 1; i < len(ob.Bids); i++ {
		if ob.Bids[i].Price > ob.Bids[i-1].Price {
			t.Errorf("bids not sorted descending: bids[%d].Price=%f > bids[%d].Price=%f",
				i, ob.Bids[i].Price, i-1, ob.Bids[i-1].Price)
		}
	}
	for i := 1; i < len(ob.Asks); i++ {
		if ob.Asks[i].Price < ob.Asks[i-1].Price {
			t.Errorf("asks not sorted ascending: asks[%d].Price=%f < asks[%d].Price=%f",
				i, ob.Asks[i].Price, i-1, ob.Asks[i-1].Price)
		}
	}
	// Verify bid < ask (no crossed book)
	if ob.Bids[0].Price >= ob.Asks[0].Price {
		t.Errorf("crossed order book: best bid %f >= best ask %f", ob.Bids[0].Price, ob.Asks[0].Price)
	}
}

func TestGetOrderBook_DefaultLimit(t *testing.T) {
	client := testClient(t)
	ob, err := client.GetOrderBook("BTCUSDT", 0)
	if err != nil {
		t.Fatalf("GetOrderBook(limit=0) error: %v", err)
	}
	if len(ob.Bids) != 100 {
		t.Errorf("default limit: got %d bids, want 100", len(ob.Bids))
	}
}

func TestCreateAndCancelOrder(t *testing.T) {
	client := testClient(t)

	// Place a limit buy far below market — won't fill, but must meet notional >= 5 USDT
	req := NewOrderRequest{
		Symbol:       "BTCUSDT",
		Side:         "BUY",
		PositionSide: "LONG",
		OrderType:    "LIMIT",
		Quantity:     0.002,
		Price:        30000.0,
	}

	order, err := client.CreateOrder(req)
	if err != nil {
		t.Fatalf("CreateOrder() error: %v", err)
	}
	if order.OrderID == 0 {
		t.Fatal("OrderID is 0")
	}
	if order.Symbol != "BTCUSDT" {
		t.Errorf("Symbol = %q, want BTCUSDT", order.Symbol)
	}
	if order.Status != "NEW" {
		t.Errorf("Status = %q, want NEW", order.Status)
	}

	// Cancel the order
	cancelled, err := client.CancelOrder("BTCUSDT", order.OrderID)
	if err != nil {
		t.Fatalf("CancelOrder() error: %v", err)
	}
	if cancelled.Status != "CANCELED" {
		t.Errorf("Status = %q, want CANCELED", cancelled.Status)
	}
}

func TestGetOpenOrders(t *testing.T) {
	client := testClient(t)

	orders, err := client.GetOpenOrders("BTCUSDT")
	if err != nil {
		t.Fatalf("GetOpenOrders() error: %v", err)
	}
	// Should be empty (we cancel all test orders)
	t.Logf("open orders: %d", len(orders))
}

func TestGetOrder(t *testing.T) {
	client := testClient(t)

	req := NewOrderRequest{
		Symbol:       "BTCUSDT",
		Side:         "BUY",
		PositionSide: "LONG",
		OrderType:    "LIMIT",
		Quantity:     0.002,
		Price:        30000.0,
	}

	order, err := client.CreateOrder(req)
	if err != nil {
		t.Fatalf("CreateOrder() error: %v", err)
	}

	// Query the order
	queried, err := client.GetOrder("BTCUSDT", order.OrderID)
	if err != nil {
		t.Fatalf("GetOrder() error: %v", err)
	}
	if queried.OrderID != order.OrderID {
		t.Errorf("OrderID mismatch: %d != %d", queried.OrderID, order.OrderID)
	}

	// Clean up
	client.CancelOrder("BTCUSDT", order.OrderID)
}

func TestScanMarket(t *testing.T) {
	client := testClient(t)
	results, err := client.ScanMarket(10)
	if err != nil {
		t.Fatalf("ScanMarket() error: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected non-empty scanner results")
	}
	if len(results) > 10 {
		t.Errorf("expected max 10 results, got %d", len(results))
	}
	// First entry should have highest volume
	for _, r := range results {
		if r.Symbol == "" {
			t.Error("symbol is empty")
		}
		if r.LastPrice <= 0 {
			t.Errorf("%s: lastPrice = %f", r.Symbol, r.LastPrice)
		}
	}
	// Verify sorted by volume descending
	for i := 1; i < len(results); i++ {
		if results[i].QuoteVolume > results[i-1].QuoteVolume {
			t.Errorf("not sorted by volume: results[%d].vol=%f > results[%d].vol=%f",
				i, results[i].QuoteVolume, i-1, results[i-1].QuoteVolume)
		}
	}
}
