package binance

// Kline represents a single candlestick/kline data point.
type Kline struct {
	OpenTime  int64   // K线开始时间 (Unix ms)
	Open      float64 // 开盘价
	High      float64 // 最高价
	Low       float64 // 最低价
	Close     float64 // 收盘价
	Volume    float64 // 成交量
	CloseTime int64   // K线结束时间 (Unix ms)
}

// Ticker represents the latest price for a symbol.
type Ticker struct {
	Symbol string
	Price  float64
}

// Balance represents the balance of a single asset.
type Balance struct {
	Asset            string
	AvailableBalance float64 // 可用余额
	TotalBalance     float64 // 总余额（含未实现盈亏）
}

// Position represents a current open position.
type Position struct {
	Symbol        string
	Side          string  // LONG / SHORT
	Quantity      float64 // 持仓数量（正数）
	EntryPrice    float64 // 开仓均价
	MarkPrice     float64 // 标记价格
	UnrealizedPnL float64 // 未实现盈亏
	Leverage      int     // 杠杆倍数
}

// OrderBookLevel represents a single price level in the order book.
type OrderBookLevel struct {
	Price    float64
	Quantity float64
}

// OrderBook represents the order book depth for a symbol.
type OrderBook struct {
	Symbol string
	Bids   []OrderBookLevel // 买盘，价格从高到低
	Asks   []OrderBookLevel // 卖盘，价格从低到高
}

// NewOrderRequest is the input for creating a new order.
type NewOrderRequest struct {
	Symbol       string  // 交易对
	Side         string  // BUY / SELL
	PositionSide string  // LONG / SHORT / BOTH（默认 BOTH，单向模式）
	OrderType    string  // LIMIT / MARKET / STOP_MARKET
	Quantity     float64 // 数量
	Price        float64 // 限价（LIMIT 必填）
	StopPrice    float64 // 止损价（STOP_MARKET 必填）
}

// Order represents a single order returned from Binance.
type Order struct {
	OrderID      int64   `json:"orderId"`
	Symbol       string  `json:"symbol"`
	Status       string  `json:"status"`
	ClientOrderID string `json:"clientOrderId"`
	Price        float64 `json:"price,string"`
	AvgPrice     float64 `json:"avgPrice,string"`
	OrigQty      float64 `json:"origQty,string"`
	ExecutedQty  float64 `json:"executedQty,string"`
	Type         string  `json:"type"`
	Side         string  `json:"side"`
	StopPrice    float64 `json:"stopPrice,string"`
	TimeInForce  string  `json:"timeInForce"`
	UpdateTime   int64   `json:"updateTime"`
}

// OrderPreview is returned by the Plan phase (without confirm).
// It shows what the order will look like and the risk assessment.
type OrderPreview struct {
	PlanID   string      `json:"plan_id"`
	Symbol   string      `json:"symbol"`
	Side     string      `json:"side"`
	Type     string      `json:"type"`
	Quantity float64     `json:"quantity"`
	Price    float64     `json:"price"`
	StopPrice float64    `json:"stop_price"`
	Risk     RiskCheck   `json:"risk"`
}

// RiskCheck holds the result of pre-trade risk checks.
type RiskCheck struct {
	Passed  bool     `json:"passed"`
	Checks  []string `json:"checks"`
	Warnings []string `json:"warnings"`
}
