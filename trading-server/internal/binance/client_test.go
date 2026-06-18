package binance

import (
	"os"
	"testing"
)

// testClient creates a Client for Testnet integration tests.
// Skip tests if credentials are not set.
func testClient(t *testing.T) *Client {
	t.Helper()
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
