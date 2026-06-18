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
