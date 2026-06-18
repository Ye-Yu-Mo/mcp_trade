package binance

// Trader defines the interface for Binance Futures operations.
// This allows mocking in tests without depending on the real API.
type Trader interface {
	GetKlines(symbol, interval string, limit int) ([]Kline, error)
	GetTicker(symbol string) (*Ticker, error)
	GetOrderBook(symbol string, limit int) (*OrderBook, error)
	GetBalance() ([]Balance, error)
	GetPositions() ([]Position, error)
}

// Compile-time check: Client implements Trader.
var _ Trader = (*Client)(nil)
