package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
)

// mockTrader implements binance.Trader for handler tests.
type mockTrader struct {
	klines    []binance.Kline
	ticker    *binance.Ticker
	orderbook *binance.OrderBook
	balances  []binance.Balance
	positions []binance.Position
	err       error
}

func (m *mockTrader) GetKlines(symbol, interval string, limit int) ([]binance.Kline, error) {
	return m.klines, m.err
}
func (m *mockTrader) GetTicker(symbol string) (*binance.Ticker, error) {
	return m.ticker, m.err
}
func (m *mockTrader) GetOrderBook(symbol string, limit int) (*binance.OrderBook, error) {
	return m.orderbook, m.err
}
func (m *mockTrader) GetBalance() ([]binance.Balance, error) {
	return m.balances, m.err
}
func (m *mockTrader) GetPositions() ([]binance.Position, error) {
	return m.positions, m.err
}

// --- Klines handler tests ---

func TestHandleKlines_Success(t *testing.T) {
	mock := &mockTrader{
		klines: []binance.Kline{
			{OpenTime: 100, Open: 100.0, High: 110.0, Low: 95.0, Close: 105.0, Volume: 1000.0, CloseTime: 200},
		},
	}
	m := NewMarketHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/market/klines?symbol=BTCUSDT&interval=1h&limit=10", nil)
	rec := httptest.NewRecorder()

	m.HandleKlines(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp struct {
		Data []binance.Kline `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(resp.Data) != 1 {
		t.Fatalf("len(data) = %d, want 1", len(resp.Data))
	}
	if resp.Data[0].Open != 100.0 {
		t.Errorf("Open = %f, want 100.0", resp.Data[0].Open)
	}
}

func TestHandleKlines_MissingSymbol(t *testing.T) {
	mock := &mockTrader{}
	m := NewMarketHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/market/klines?interval=1h", nil)
	rec := httptest.NewRecorder()

	m.HandleKlines(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}

	var resp struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Error.Code != "MISSING_PARAM" {
		t.Errorf("error code = %q, want %q", resp.Error.Code, "MISSING_PARAM")
	}
}

func TestHandleKlines_InvalidLimit(t *testing.T) {
	mock := &mockTrader{}
	m := NewMarketHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/market/klines?symbol=BTCUSDT&interval=1h&limit=2000", nil)
	rec := httptest.NewRecorder()

	m.HandleKlines(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

func TestHandleKlines_DefaultLimit(t *testing.T) {
	mock := &mockTrader{
		klines: make([]binance.Kline, 500),
	}
	m := NewMarketHandler(mock)

	// No limit param → defaults to 500
	req := httptest.NewRequest(http.MethodGet, "/api/v1/market/klines?symbol=BTCUSDT&interval=1h", nil)
	rec := httptest.NewRecorder()

	m.HandleKlines(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleKlines_BinanceError(t *testing.T) {
	mock := &mockTrader{err: errors.New("API error")}
	m := NewMarketHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/market/klines?symbol=BTCUSDT&interval=1h&limit=10", nil)
	rec := httptest.NewRecorder()

	m.HandleKlines(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}

	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Error.Code != "BINANCE_ERROR" {
		t.Errorf("error code = %q, want %q", resp.Error.Code, "BINANCE_ERROR")
	}
}

// --- Ticker handler tests ---

func TestHandleTicker_Success(t *testing.T) {
	mock := &mockTrader{
		ticker: &binance.Ticker{Symbol: "BTCUSDT", Price: 65000.0},
	}
	m := NewMarketHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/market/ticker?symbol=BTCUSDT", nil)
	rec := httptest.NewRecorder()

	m.HandleTicker(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp struct {
		Data binance.Ticker `json:"data"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if resp.Data.Price != 65000.0 {
		t.Errorf("Price = %f, want 65000.0", resp.Data.Price)
	}
}

func TestHandleTicker_MissingSymbol(t *testing.T) {
	mock := &mockTrader{}
	m := NewMarketHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/market/ticker", nil)
	rec := httptest.NewRecorder()

	m.HandleTicker(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}

// --- Balance handler tests ---

func TestHandleBalance_Success(t *testing.T) {
	mock := &mockTrader{
		balances: []binance.Balance{
			{Asset: "USDT", AvailableBalance: 5000.0, TotalBalance: 5000.0},
		},
	}
	a := NewAccountHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/account/balance", nil)
	rec := httptest.NewRecorder()

	a.HandleBalance(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp struct {
		Data []binance.Balance `json:"data"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data) != 1 {
		t.Fatalf("len(data) = %d, want 1", len(resp.Data))
	}
}

func TestHandleBalance_Empty(t *testing.T) {
	mock := &mockTrader{balances: []binance.Balance{}}
	a := NewAccountHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/account/balance", nil)
	rec := httptest.NewRecorder()

	a.HandleBalance(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestHandleBalance_Error(t *testing.T) {
	mock := &mockTrader{err: errors.New("binance down")}
	a := NewAccountHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/account/balance", nil)
	rec := httptest.NewRecorder()

	a.HandleBalance(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
}

// --- Positions handler tests ---

func TestHandlePositions_Success(t *testing.T) {
	mock := &mockTrader{
		positions: []binance.Position{
			{Symbol: "BTCUSDT", Side: "LONG", Quantity: 0.01, EntryPrice: 64000, MarkPrice: 65000, UnrealizedPnL: 10.0},
		},
	}
	a := NewAccountHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/account/positions", nil)
	rec := httptest.NewRecorder()

	a.HandlePositions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}

	var resp struct {
		Data []binance.Position `json:"data"`
	}
	json.Unmarshal(rec.Body.Bytes(), &resp)
	if len(resp.Data) != 1 {
		t.Fatalf("len(data) = %d, want 1", len(resp.Data))
	}
	if resp.Data[0].Side != "LONG" {
		t.Errorf("Side = %q, want %q", resp.Data[0].Side, "LONG")
	}
}

func TestHandlePositions_Empty(t *testing.T) {
	mock := &mockTrader{positions: []binance.Position{}}
	a := NewAccountHandler(mock)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/account/positions", nil)
	rec := httptest.NewRecorder()

	a.HandlePositions(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}
