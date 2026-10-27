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

	// Order-flow fields. TakerBuyVolume is the part of Volume that hit the
	// ask (aggressive buys). Delta = 2*TakerBuyVolume - Volume tells you
	// whether a move had buyers behind it or was short covering.
	QuoteVolume         float64
	Trades              int64
	TakerBuyVolume      float64
	TakerBuyQuoteVolume float64
}

// Delta returns taker buy minus taker sell volume (both in base units).
func (k Kline) Delta() float64 { return 2*k.TakerBuyVolume - k.Volume }

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
	Symbol          string  // 交易对
	Side            string  // BUY / SELL
	PositionSide    string  // LONG / SHORT / BOTH（默认 BOTH，单向模式）
	OrderType       string  // LIMIT / MARKET / STOP_MARKET / TAKE_PROFIT_MARKET / TRAILING_STOP_MARKET
	Quantity        float64 // 数量
	Price           float64 // 限价（LIMIT 必填）
	StopPrice       float64 // 止损价（STOP_MARKET 必填）
	TakeProfit      float64 // 纸面盘元数据：入场时已知的止盈价（实盘客户端不发送）
	ReduceOnly      bool    // 仅减仓（平仓用）
	ClosePosition   bool    // 条件单触发时平掉整个仓位（不能与 Quantity/ReduceOnly 同用）
	ClientOrderID   string  // 自定义订单号，用于幂等与追踪
	WorkingType     string  // MARK_PRICE / CONTRACT_PRICE，默认 CONTRACT_PRICE
	ActivationPrice float64 // TRAILING_STOP_MARKET 激活价（可选）
	CallbackRate    float64 // TRAILING_STOP_MARKET 回调比例（百分比，如 1.0 = 1%）
	ExpireAt        int64   // 限价单自动撤销时间（Unix ms，0 = GTC）。实盘用 GTD，纸面盘由 broker 扫描

	// Metadata used by the paper broker and trade journal. The live client
	// ignores these fields.
	Leverage    float64
	RiskAmount  float64
	EntryReason string
	Setup       string
	Tags        string
}

// SymbolInfo holds exchange trading rules for one symbol.
type SymbolInfo struct {
	Symbol            string
	Status            string
	TickSize          float64
	StepSize          float64
	MinNotional       float64
	MaxLeverage       float64
	PricePrecision    int
	QuantityPrecision int
}

// Order represents a single order returned from Binance.
type Order struct {
	OrderID       int64   `json:"orderId"`
	Symbol        string  `json:"symbol"`
	Status        string  `json:"status"`
	ClientOrderID string  `json:"clientOrderId"`
	Price         float64 `json:"price,string"`
	AvgPrice      float64 `json:"avgPrice,string"`
	OrigQty       float64 `json:"origQty,string"`
	ExecutedQty   float64 `json:"executedQty,string"`
	Type          string  `json:"type"`
	Side          string  `json:"side"`
	PositionSide  string  `json:"positionSide"`
	StopPrice     float64 `json:"stopPrice,string"`
	TimeInForce   string  `json:"timeInForce"`
	ReduceOnly    bool    `json:"reduceOnly"`
	ClosePosition bool    `json:"closePosition"`
	IsAlgo        bool    `json:"isAlgo,omitempty"`
	ExpireAt      int64   `json:"expireAt,omitempty"`
	UpdateTime    int64   `json:"updateTime"`
}

// OrderPreview is returned by the Plan phase (without confirm).
// It shows what the order will look like and the risk assessment.
type OrderPreview struct {
	PlanID    string    `json:"plan_id"`
	Symbol    string    `json:"symbol"`
	Side      string    `json:"side"`
	Type      string    `json:"type"`
	Quantity  float64   `json:"quantity"`
	Price     float64   `json:"price"`
	StopPrice float64   `json:"stop_price"`
	Risk      RiskCheck `json:"risk"`
}

// RiskCheck holds the result of pre-trade risk checks.
type RiskCheck struct {
	Passed   bool     `json:"passed"`
	Checks   []string `json:"checks"`
	Warnings []string `json:"warnings"`
}

// ScannerResult represents a single market scan entry.
type ScannerResult struct {
	Symbol         string  `json:"symbol"`
	LastPrice      float64 `json:"last_price"`
	PriceChangePct float64 `json:"change_24h_pct"`
	HighPrice      float64 `json:"high_24h"`
	LowPrice       float64 `json:"low_24h"`
	Volume         float64 `json:"volume_24h"`
	QuoteVolume    float64 `json:"quote_volume_24h"`
}
