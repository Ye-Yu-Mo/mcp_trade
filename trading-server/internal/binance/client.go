package binance

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Client wraps the Binance Futures API using raw HTTP.
type Client struct {
	apiKey       string
	apiSecret    string
	baseURL      string
	httpClient   *http.Client
	lastSignedAt time.Time
	mu           sync.Mutex
}

// NewClient creates a new Binance Futures API client.
func NewClient(apiKey, apiSecret, baseURL string) *Client {
	return &Client{
		apiKey:    apiKey,
		apiSecret: apiSecret,
		baseURL:   baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// rateLimitSigned enforces a minimum 250ms between signed requests to avoid Binance IP bans.
func (c *Client) rateLimitSigned() {
	c.mu.Lock()
	defer c.mu.Unlock()
	elapsed := time.Since(c.lastSignedAt)
	if minWait := 250 * time.Millisecond; elapsed < minWait {
		time.Sleep(minWait - elapsed)
	}
	c.lastSignedAt = time.Now()
}

// request sends an HTTP request to the given path with query parameters.
// If signed is true, adds timestamp and HMAC signature.
func (c *Client) request(ctx context.Context, method, path string, params url.Values, signed bool) ([]byte, error) {
	if signed {
		c.rateLimitSigned()
		if params == nil {
			params = url.Values{}
		}
		params.Set("timestamp", strconv.FormatInt(time.Now().UnixMilli(), 10))
		params.Set("recvWindow", "5000")
		params.Set("signature", c.sign(params))
	}

	reqURL := c.baseURL + path
	if len(params) > 0 {
		reqURL += "?" + params.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if signed {
		req.Header.Set("X-MBX-APIKEY", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("binance API error %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}

// get is a convenience wrapper for GET requests.
func (c *Client) get(ctx context.Context, path string, params url.Values, signed bool) ([]byte, error) {
	return c.request(ctx, http.MethodGet, path, params, signed)
}

// sign computes the HMAC-SHA256 signature for the given parameters.
func (c *Client) sign(params url.Values) string {
	mac := hmac.New(sha256.New, []byte(c.apiSecret))
	mac.Write([]byte(params.Encode()))
	return hex.EncodeToString(mac.Sum(nil))
}

// ------- Kline types for JSON unmarshaling -------

type klineRaw [12]interface{}

func (k klineRaw) toKline() (Kline, error) {
	var kl Kline
	if v, ok := k[0].(float64); ok {
		kl.OpenTime = int64(v)
	}
	if v, ok := k[1].(string); ok {
		kl.Open, _ = strconv.ParseFloat(v, 64)
	}
	if v, ok := k[2].(string); ok {
		kl.High, _ = strconv.ParseFloat(v, 64)
	}
	if v, ok := k[3].(string); ok {
		kl.Low, _ = strconv.ParseFloat(v, 64)
	}
	if v, ok := k[4].(string); ok {
		kl.Close, _ = strconv.ParseFloat(v, 64)
	}
	if v, ok := k[5].(string); ok {
		kl.Volume, _ = strconv.ParseFloat(v, 64)
	}
	if v, ok := k[6].(float64); ok {
		kl.CloseTime = int64(v)
	}
	return kl, nil
}

type tickerPriceRaw struct {
	Symbol string `json:"symbol"`
	Price  string `json:"price"`
}

type balanceRaw struct {
	Asset            string `json:"asset"`
	Balance          string `json:"balance"`
	AvailableBalance string `json:"availableBalance"`
}

type positionRaw struct {
	Symbol           string `json:"symbol"`
	PositionAmt      string `json:"positionAmt"`
	EntryPrice       string `json:"entryPrice"`
	MarkPrice        string `json:"markPrice"`
	UnRealizedProfit string `json:"unRealizedProfit"`
	Leverage         string `json:"leverage"`
}

type orderBookRaw struct {
	Bids [][]string `json:"bids"`
	Asks [][]string `json:"asks"`
}

// ------- Public methods -------

// GetKlines fetches candlestick/kline data for a symbol.
func (c *Client) GetKlines(symbol, interval string, limit int) ([]Kline, error) {
	if limit <= 0 {
		limit = 500
	}
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("interval", interval)
	params.Set("limit", strconv.Itoa(limit))

	body, err := c.get(context.Background(), "/fapi/v1/klines", params, false)
	if err != nil {
		return nil, err
	}

	var raw []klineRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse klines: %w", err)
	}

	klines := make([]Kline, 0, len(raw))
	for _, r := range raw {
		k, err := r.toKline()
		if err != nil {
			return nil, fmt.Errorf("convert kline: %w", err)
		}
		klines = append(klines, k)
	}
	return klines, nil
}

// GetTicker returns the latest price for a symbol.
func (c *Client) GetTicker(symbol string) (*Ticker, error) {
	params := url.Values{}
	params.Set("symbol", symbol)

	body, err := c.get(context.Background(), "/fapi/v1/ticker/price", params, false)
	if err != nil {
		return nil, err
	}

	var raw tickerPriceRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse ticker: %w", err)
	}

	price, _ := strconv.ParseFloat(raw.Price, 64)
	return &Ticker{
		Symbol: raw.Symbol,
		Price:  price,
	}, nil
}

// GetBalance returns all asset balances with non-zero total balance.
func (c *Client) GetBalance() ([]Balance, error) {
	body, err := c.get(context.Background(), "/fapi/v2/balance", nil, true)
	if err != nil {
		return nil, err
	}

	var raw []balanceRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse balance: %w", err)
	}

	balances := make([]Balance, 0, len(raw))
	for _, r := range raw {
		total, _ := strconv.ParseFloat(r.Balance, 64)
		if total == 0 {
			continue
		}
		available, _ := strconv.ParseFloat(r.AvailableBalance, 64)
		balances = append(balances, Balance{
			Asset:            r.Asset,
			AvailableBalance: available,
			TotalBalance:     total,
		})
	}
	return balances, nil
}

// GetPositions returns all open positions (quantity != 0).
func (c *Client) GetPositions() ([]Position, error) {
	body, err := c.get(context.Background(), "/fapi/v2/positionRisk", nil, true)
	if err != nil {
		return nil, err
	}

	var raw []positionRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse positions: %w", err)
	}

	positions := make([]Position, 0, len(raw))
	for _, r := range raw {
		qty, _ := strconv.ParseFloat(r.PositionAmt, 64)
		if qty == 0 {
			continue
		}
		entryPrice, _ := strconv.ParseFloat(r.EntryPrice, 64)
		markPrice, _ := strconv.ParseFloat(r.MarkPrice, 64)
		unrealizedPnL, _ := strconv.ParseFloat(r.UnRealizedProfit, 64)
		leverage, _ := strconv.Atoi(r.Leverage)

		side := "LONG"
		if qty < 0 {
			side = "SHORT"
			qty = -qty
		}

		positions = append(positions, Position{
			Symbol:        r.Symbol,
			Side:          side,
			Quantity:      qty,
			EntryPrice:    entryPrice,
			MarkPrice:     markPrice,
			UnrealizedPnL: unrealizedPnL,
			Leverage:      leverage,
		})
	}
	return positions, nil
}

// GetOrderBook returns the order book depth for a symbol.
// limit: 5, 10, 20, 50, 100, 500, 1000. Default 100.
func (c *Client) GetOrderBook(symbol string, limit int) (*OrderBook, error) {
	if limit <= 0 {
		limit = 100
	}
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("limit", strconv.Itoa(limit))

	body, err := c.get(context.Background(), "/fapi/v1/depth", params, false)
	if err != nil {
		return nil, err
	}

	var raw orderBookRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse orderbook: %w", err)
	}

	parseLevels := func(data [][]string) []OrderBookLevel {
		levels := make([]OrderBookLevel, 0, len(data))
		for _, entry := range data {
			if len(entry) < 2 {
				continue
			}
			price, _ := strconv.ParseFloat(entry[0], 64)
			qty, _ := strconv.ParseFloat(entry[1], 64)
			levels = append(levels, OrderBookLevel{Price: price, Quantity: qty})
		}
		return levels
	}

	return &OrderBook{
		Symbol: symbol,
		Bids:   parseLevels(raw.Bids),
		Asks:   parseLevels(raw.Asks),
	}, nil
}

// --- Order raw types ---

type orderRaw struct {
	OrderID       int64  `json:"orderId"`
	Symbol        string `json:"symbol"`
	Status        string `json:"status"`
	ClientOrderID string `json:"clientOrderId"`
	Price         string `json:"price"`
	AvgPrice      string `json:"avgPrice"`
	OrigQty       string `json:"origQty"`
	ExecutedQty   string `json:"executedQty"`
	Type          string `json:"type"`
	Side          string `json:"side"`
	StopPrice     string `json:"stopPrice"`
	TimeInForce   string `json:"timeInForce"`
	UpdateTime    int64  `json:"updateTime"`
}

func (r orderRaw) toOrder() *Order {
	price, _ := strconv.ParseFloat(r.Price, 64)
	avgPrice, _ := strconv.ParseFloat(r.AvgPrice, 64)
	origQty, _ := strconv.ParseFloat(r.OrigQty, 64)
	executedQty, _ := strconv.ParseFloat(r.ExecutedQty, 64)
	stopPrice, _ := strconv.ParseFloat(r.StopPrice, 64)
	return &Order{
		OrderID:       r.OrderID,
		Symbol:        r.Symbol,
		Status:        r.Status,
		ClientOrderID: r.ClientOrderID,
		Price:         price,
		AvgPrice:      avgPrice,
		OrigQty:       origQty,
		ExecutedQty:   executedQty,
		Type:          r.Type,
		Side:          r.Side,
		StopPrice:     stopPrice,
		TimeInForce:   r.TimeInForce,
		UpdateTime:    r.UpdateTime,
	}
}

// --- Order methods ---

// CreateOrder places a new order on Binance Futures.
func (c *Client) CreateOrder(req NewOrderRequest) (*Order, error) {
	params := url.Values{}
	params.Set("symbol", req.Symbol)
	params.Set("side", req.Side)
	if req.PositionSide != "" {
		params.Set("positionSide", req.PositionSide)
	}
	params.Set("type", req.OrderType)
	params.Set("quantity", strconv.FormatFloat(req.Quantity, 'f', -1, 64))

	// STOP_MARKET and TAKE_PROFIT_MARKET use Algo endpoint on mainnet; testnet unsupported
	isConditional := req.OrderType == "STOP_MARKET" || req.OrderType == "TAKE_PROFIT_MARKET" || req.OrderType == "STOP"
	isTestnet := strings.Contains(c.baseURL, "testnet")

	if isConditional && isTestnet {
		return nil, fmt.Errorf("条件单(%s)在testnet不可用，请改用 LIMIT/MARKET + reduce_only=true 或设 market.alerts 提醒", req.OrderType)
	}

	path := "/fapi/v1/order"
	if isConditional && !isTestnet {
		path = "/fapi/v1/algo/order"
	}

	if req.OrderType == "LIMIT" || req.OrderType == "STOP" {
		params.Set("timeInForce", "GTC")
		params.Set("price", strconv.FormatFloat(req.Price, 'f', -1, 64))
	}
	if isConditional {
		params.Set("stopPrice", strconv.FormatFloat(req.StopPrice, 'f', -1, 64))
		params.Set("reduceOnly", strconv.FormatBool(req.ReduceOnly))
	} else if req.ReduceOnly {
		params.Set("reduceOnly", "true")
	}

	body, err := c.request(context.Background(), http.MethodPost, path, params, true)
	if err != nil {
		return nil, err
	}

	var raw orderRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse order: %w", err)
	}
	return raw.toOrder(), nil
}

// CancelOrder cancels an active order by ID.
func (c *Client) CancelOrder(symbol string, orderID int64) (*Order, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("orderId", strconv.FormatInt(orderID, 10))

	body, err := c.request(context.Background(), http.MethodDelete, "/fapi/v1/order", params, true)
	if err != nil {
		return nil, err
	}

	var raw orderRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse cancel: %w", err)
	}
	return raw.toOrder(), nil
}

// GetOpenOrders returns all open orders for a symbol. Empty string = all symbols.
func (c *Client) GetOpenOrders(symbol string) ([]Order, error) {
	params := url.Values{}
	if symbol != "" {
		params.Set("symbol", symbol)
	}

	body, err := c.get(context.Background(), "/fapi/v1/openOrders", params, true)
	if err != nil {
		return nil, err
	}

	var raw []orderRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse open orders: %w", err)
	}

	orders := make([]Order, 0, len(raw))
	for _, r := range raw {
		orders = append(orders, *r.toOrder())
	}
	return orders, nil
}

// GetOrder returns details for a specific order.
func (c *Client) GetOrder(symbol string, orderID int64) (*Order, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("orderId", strconv.FormatInt(orderID, 10))

	body, err := c.get(context.Background(), "/fapi/v1/order", params, true)
	if err != nil {
		return nil, err
	}

	var raw orderRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse order: %w", err)
	}
	return raw.toOrder(), nil
}

// --- Account setup ---

// SetPositionMode sets hedge mode. dual=false means one-way mode (单向持仓).
func (c *Client) SetPositionMode(dual bool) error {
	params := url.Values{}
	params.Set("dualSidePosition", strconv.FormatBool(dual))
	_, err := c.request(context.Background(), http.MethodPost, "/fapi/v1/positionSide/dual", params, true)
	return err
}

// SetMarginType sets the margin type for a symbol. marginType: "ISOLATED" or "CROSSED".
func (c *Client) SetMarginType(symbol, marginType string) error {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("marginType", marginType)
	_, err := c.request(context.Background(), http.MethodPost, "/fapi/v1/marginType", params, true)
	return err
}

// SetLeverage sets the leverage for a symbol. 1-125x.
func (c *Client) SetLeverage(symbol string, leverage int) error {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("leverage", strconv.Itoa(leverage))
	_, err := c.request(context.Background(), http.MethodPost, "/fapi/v1/leverage", params, true)
	return err
}

// GetListenKey creates a new user data stream listen key.
func (c *Client) GetListenKey() (string, error) {
	body, err := c.request(context.Background(), http.MethodPost, "/fapi/v1/listenKey", nil, true)
	if err != nil {
		return "", err
	}
	var result struct {
		ListenKey string `json:"listenKey"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parse listenKey: %w", err)
	}
	return result.ListenKey, nil
}

// KeepAliveListenKey extends the listen key validity by 60 minutes.
func (c *Client) KeepAliveListenKey(listenKey string) error {
	params := url.Values{}
	params.Set("listenKey", listenKey)
	_, err := c.request(context.Background(), http.MethodPut, "/fapi/v1/listenKey", params, true)
	return err
}

// --- Scanner ---

type ticker24hRaw struct {
	Symbol             string `json:"symbol"`
	LastPrice          string `json:"lastPrice"`
	PriceChangePercent string `json:"priceChangePercent"`
	HighPrice          string `json:"highPrice"`
	LowPrice           string `json:"lowPrice"`
	Volume             string `json:"volume"`
	QuoteVolume        string `json:"quoteVolume"`
}

// ScanMarket returns top symbols by 24h volume. limit: max results (default 10).
func (c *Client) ScanMarket(limit int) ([]ScannerResult, error) {
	if limit <= 0 {
		limit = 10
	}
	body, err := c.get(context.Background(), "/fapi/v1/ticker/24hr", nil, false)
	if err != nil {
		return nil, err
	}
	var raw []ticker24hRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse ticker24h: %w", err)
	}

	// Filter USDT pairs only, sort by quoteVolume desc
	results := make([]ScannerResult, 0, len(raw))
	for _, r := range raw {
		if !strings.HasSuffix(r.Symbol, "USDT") {
			continue
		}
		last, _ := strconv.ParseFloat(r.LastPrice, 64)
		pct, _ := strconv.ParseFloat(r.PriceChangePercent, 64)
		high, _ := strconv.ParseFloat(r.HighPrice, 64)
		low, _ := strconv.ParseFloat(r.LowPrice, 64)
		vol, _ := strconv.ParseFloat(r.Volume, 64)
		qvol, _ := strconv.ParseFloat(r.QuoteVolume, 64)
		results = append(results, ScannerResult{
			Symbol: r.Symbol, LastPrice: last, PriceChangePct: pct,
			HighPrice: high, LowPrice: low, Volume: vol, QuoteVolume: qvol,
		})
	}

	// Sort by quoteVolume descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].QuoteVolume > results[j].QuoteVolume
	})

	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

// --- Funding Rate & Open Interest ---

type fundingRaw struct {
	Symbol      string `json:"symbol"`
	FundingRate string `json:"fundingRate"`
	FundingTime int64  `json:"fundingTime"`
}

type oiRaw struct {
	Symbol       string `json:"symbol"`
	OpenInterest string `json:"openInterest"`
}

// GetFundingRate returns the current funding rate for a symbol.
func (c *Client) GetFundingRate(symbol string) (float64, int64, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	body, err := c.get(context.Background(), "/fapi/v1/fundingRate", params, false)
	if err != nil {
		return 0, 0, err
	}
	var raw []fundingRaw
	if err := json.Unmarshal(body, &raw); err != nil || len(raw) == 0 {
		return 0, 0, fmt.Errorf("parse funding rate: %w", err)
	}
	rate, _ := strconv.ParseFloat(raw[0].FundingRate, 64)
	return rate, raw[0].FundingTime, nil
}

// GetOpenInterest returns the current open interest for a symbol.
func (c *Client) GetOpenInterest(symbol string) (float64, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	body, err := c.get(context.Background(), "/fapi/v1/openInterest", params, false)
	if err != nil {
		return 0, err
	}
	var raw oiRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return 0, fmt.Errorf("parse OI: %w", err)
	}
	oi, _ := strconv.ParseFloat(raw.OpenInterest, 64)
	return oi, nil
}

// --- OCO Order ---

type ocoOrderRaw struct {
	OrderListID    int64      `json:"orderListId"`
	Orders         []orderRaw `json:"orders"`
	OrderReports   []orderRaw `json:"orderReports"`
}

// OCOOrder represents a One-Cancels-Other order pair.
type OCOOrder struct {
	OrderListID int64   `json:"orderListId"`
	StopOrder   *Order  `json:"stop_order"`
	LimitOrder  *Order  `json:"limit_order"`
}

// CreateOCOOrder places an OCO (One-Cancels-Other) order — a take-profit limit
// and a stop-loss stop-market order. When one triggers, the other cancels.
func (c *Client) CreateOCOOrder(symbol, side string, quantity, price, stopPrice float64) (*OCOOrder, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("side", side)
	params.Set("quantity", strconv.FormatFloat(quantity, 'f', -1, 64))
	params.Set("price", strconv.FormatFloat(price, 'f', -1, 64))       // take-profit limit
	params.Set("stopPrice", strconv.FormatFloat(stopPrice, 'f', -1, 64)) // stop-loss trigger
	params.Set("stopLimitPrice", "0") // 0 = stop-market
	params.Set("stopLimitTimeInForce", "GTC")

	body, err := c.request(context.Background(), http.MethodPost, "/fapi/v1/order/oco", params, true)
	if err != nil {
		return nil, err
	}
	var raw ocoOrderRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse OCO: %w", err)
	}

	result := &OCOOrder{OrderListID: raw.OrderListID}
	if len(raw.OrderReports) >= 2 {
		result.StopOrder = raw.OrderReports[0].toOrder()
		result.LimitOrder = raw.OrderReports[1].toOrder()
	}
	return result, nil
}

// CancelOCOOrder cancels an OCO order pair by orderListId.
func (c *Client) CancelOCOOrder(symbol string, orderListID int64) error {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("orderListId", strconv.FormatInt(orderListID, 10))
	_, err := c.request(context.Background(), http.MethodDelete, "/fapi/v1/orderList", params, true)
	return err
}
