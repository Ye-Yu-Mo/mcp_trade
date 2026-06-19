package binance

// Trader defines the interface for Binance Futures operations.
// This allows mocking in tests without depending on the real API.
type Trader interface {
	GetKlines(symbol, interval string, limit int) ([]Kline, error)
	GetTicker(symbol string) (*Ticker, error)
	GetOrderBook(symbol string, limit int) (*OrderBook, error)
	GetBalance() ([]Balance, error)
	GetPositions() ([]Position, error)
	CreateOrder(req NewOrderRequest) (*Order, error)
	CancelOrder(symbol string, orderID int64) (*Order, error)
	GetOpenOrders(symbol string) ([]Order, error)
	GetOrder(symbol string, orderID int64) (*Order, error)
	ScanMarket(limit int) ([]ScannerResult, error)
	GetFundingRate(symbol string) (float64, int64, error)
	GetOpenInterest(symbol string) (float64, error)
	CreateOCOOrder(symbol, side string, quantity, price, stopPrice float64) (*OCOOrder, error)
	CancelOCOOrder(symbol string, orderListID int64) error
	GetATR(symbol, interval string, period int) (float64, error)
	GetCandleInfo(symbol, interval string) (*CandleInfo, error)
}

// Compile-time check: Client implements Trader.
var _ Trader = (*Client)(nil)
