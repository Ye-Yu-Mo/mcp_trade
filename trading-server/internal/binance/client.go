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

// UserTrade is one fill from the account trade history.
type UserTrade struct {
	Symbol      string
	Side        string
	Price       float64
	Qty         float64
	RealizedPnl float64
	Commission  float64
	Time        int64
	OrderID     int64
	Maker       bool
}

type userTradeRaw struct {
	Symbol      string `json:"symbol"`
	Side        string `json:"side"`
	Price       string `json:"price"`
	Qty         string `json:"qty"`
	RealizedPnl string `json:"realizedPnl"`
	Commission  string `json:"commission"`
	Time        int64  `json:"time"`
	OrderID     int64  `json:"orderId"`
	Maker       bool   `json:"maker"`
}

// GetUserTrades returns the account's fills for a symbol since startTime (ms).
func (c *Client) GetUserTrades(symbol string, startTime int64, limit int) ([]UserTrade, error) {
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("limit", strconv.Itoa(limit))
	if startTime > 0 {
		params.Set("startTime", strconv.FormatInt(startTime, 10))
	}
	body, err := c.get(context.Background(), "/fapi/v1/userTrades", params, true)
	if err != nil {
		return nil, err
	}
	var raw []userTradeRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse user trades: %w", err)
	}
	out := make([]UserTrade, 0, len(raw))
	for _, r := range raw {
		price, _ := strconv.ParseFloat(r.Price, 64)
		qty, _ := strconv.ParseFloat(r.Qty, 64)
		pnl, _ := strconv.ParseFloat(r.RealizedPnl, 64)
		commission, _ := strconv.ParseFloat(r.Commission, 64)
		out = append(out, UserTrade{
			Symbol: r.Symbol, Side: r.Side, Price: price, Qty: qty,
			RealizedPnl: pnl, Commission: commission, Time: r.Time,
			OrderID: r.OrderID, Maker: r.Maker,
		})
	}
	return out, nil
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
	PositionSide  string `json:"positionSide"`
	StopPrice     string `json:"stopPrice"`
	TimeInForce   string `json:"timeInForce"`
	ReduceOnly    bool   `json:"reduceOnly"`
	ClosePosition bool   `json:"closePosition"`
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
		PositionSide:  r.PositionSide,
		StopPrice:     stopPrice,
		TimeInForce:   r.TimeInForce,
		ReduceOnly:    r.ReduceOnly,
		ClosePosition: r.ClosePosition,
		UpdateTime:    r.UpdateTime,
	}
}

// --- Order methods ---

// CreateOrder places a new order on Binance Futures.
// Conditional orders (STOP / TAKE_PROFIT / TRAILING_STOP) must use the Algo
// Order API; the regular order endpoint rejects them with -4120.
func (c *Client) CreateOrder(req NewOrderRequest) (*Order, error) {
	orderType := strings.ToUpper(strings.TrimSpace(req.OrderType))
	isConditional := orderType == "STOP_MARKET" || orderType == "TAKE_PROFIT_MARKET" ||
		orderType == "STOP" || orderType == "TAKE_PROFIT" || orderType == "TRAILING_STOP_MARKET"
	if isConditional {
		return c.createAlgoOrder(req)
	}
	if req.ClosePosition {
		return nil, fmt.Errorf("closePosition is only valid for conditional order types")
	}
	if req.Quantity <= 0 {
		return nil, fmt.Errorf("quantity must be positive")
	}
	if orderType == "LIMIT" && req.Price <= 0 {
		return nil, fmt.Errorf("limit price is required")
	}

	params := url.Values{}
	params.Set("symbol", req.Symbol)
	params.Set("side", strings.ToUpper(req.Side))
	if req.PositionSide != "" {
		params.Set("positionSide", strings.ToUpper(req.PositionSide))
	}
	params.Set("type", orderType)
	if orderType == "MARKET" {
		// Futures defaults to ACK, which returns status=NEW and zero fill data
		// even for a market order. RESULT gives the actual fill.
		params.Set("newOrderRespType", "RESULT")
	}
	params.Set("quantity", strconv.FormatFloat(req.Quantity, 'f', -1, 64))
	if req.ClientOrderID != "" {
		params.Set("newClientOrderId", req.ClientOrderID)
	}
	if req.WorkingType != "" {
		params.Set("workingType", strings.ToUpper(req.WorkingType))
	}
	if req.ReduceOnly {
		params.Set("reduceOnly", "true")
	}
	if orderType == "LIMIT" {
		params.Set("price", strconv.FormatFloat(req.Price, 'f', -1, 64))
		if req.ExpireAt > 0 {
			// GTD lets the venue itself drop the order at the deadline; a
			// server-side sweep alone would leave it resting if this process is down.
			params.Set("timeInForce", "GTD")
			params.Set("goodTillDate", strconv.FormatInt(req.ExpireAt, 10))
		} else {
			params.Set("timeInForce", "GTC")
		}
	}

	body, err := c.request(context.Background(), http.MethodPost, "/fapi/v1/order", params, true)
	if err != nil {
		return nil, err
	}

	var raw orderRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse order: %w", err)
	}
	order := raw.toOrder()
	order.ExpireAt = req.ExpireAt
	// Some venue/edge responses still ACK a market order (status=NEW,
	// executedQty=0). Re-query once so callers never mistake a filled entry
	// for an unfilled one.
	if orderType == "MARKET" && order.Status == "NEW" {
		time.Sleep(250 * time.Millisecond)
		if got, err := c.GetOrder(req.Symbol, order.OrderID); err == nil {
			return got, nil
		}
	}
	return order, nil
}

// --- Algo (conditional) orders ---

type algoOrderRaw struct {
	AlgoID        int64  `json:"algoId"`
	ClientAlgoID  string `json:"clientAlgoId"`
	AlgoType      string `json:"algoType"`
	OrderType     string `json:"orderType"`
	Symbol        string `json:"symbol"`
	Side          string `json:"side"`
	PositionSide  string `json:"positionSide"`
	Quantity      string `json:"quantity"`
	Price         string `json:"price"`
	TriggerPrice  string `json:"triggerPrice"`
	WorkingType   string `json:"workingType"`
	ClosePosition bool   `json:"closePosition"`
	ReduceOnly    bool   `json:"reduceOnly"`
	AlgoStatus    string `json:"algoStatus"`
	Status        string `json:"status"`
	ExecutedQty   string `json:"executedQty"`
	AvgPrice      string `json:"avgPrice"`
	TriggerTime   int64  `json:"triggerTime"`
	BookTime      int64  `json:"bookTime"`
	UpdateTime    int64  `json:"updateTime"`
}

func (r algoOrderRaw) toOrder() *Order {
	status := r.AlgoStatus
	if status == "" {
		status = r.Status
	}
	switch status {
	case "FINISHED":
		status = "FILLED"
	case "TRIGGERING", "TRIGGERED":
		status = "NEW"
	}
	qty, _ := strconv.ParseFloat(r.Quantity, 64)
	price, _ := strconv.ParseFloat(r.Price, 64)
	trigger, _ := strconv.ParseFloat(r.TriggerPrice, 64)
	executed, _ := strconv.ParseFloat(r.ExecutedQty, 64)
	avg, _ := strconv.ParseFloat(r.AvgPrice, 64)
	update := r.UpdateTime
	if update == 0 {
		update = r.BookTime
	}
	if update == 0 {
		update = r.TriggerTime
	}
	return &Order{
		OrderID:       r.AlgoID,
		Symbol:        r.Symbol,
		Status:        status,
		ClientOrderID: r.ClientAlgoID,
		Price:         price,
		AvgPrice:      avg,
		OrigQty:       qty,
		ExecutedQty:   executed,
		Type:          r.OrderType,
		Side:          r.Side,
		PositionSide:  r.PositionSide,
		StopPrice:     trigger,
		ReduceOnly:    r.ReduceOnly,
		ClosePosition: r.ClosePosition,
		IsAlgo:        true,
		UpdateTime:    update,
	}
}

func (c *Client) createAlgoOrder(req NewOrderRequest) (*Order, error) {
	orderType := strings.ToUpper(strings.TrimSpace(req.OrderType))
	if req.ClosePosition && req.ReduceOnly {
		return nil, fmt.Errorf("closePosition and reduceOnly are mutually exclusive")
	}
	if req.ClosePosition {
		if orderType != "STOP_MARKET" && orderType != "TAKE_PROFIT_MARKET" {
			return nil, fmt.Errorf("closePosition is only valid for STOP_MARKET/TAKE_PROFIT_MARKET")
		}
	} else if req.Quantity <= 0 {
		return nil, fmt.Errorf("quantity is required when closePosition is false")
	}
	if orderType != "TRAILING_STOP_MARKET" && req.StopPrice <= 0 {
		return nil, fmt.Errorf("triggerPrice is required for %s", orderType)
	}
	if (orderType == "STOP" || orderType == "TAKE_PROFIT") && req.Price <= 0 {
		return nil, fmt.Errorf("price is required for %s", orderType)
	}

	params := url.Values{}
	params.Set("algoType", "CONDITIONAL")
	params.Set("type", orderType)
	params.Set("symbol", req.Symbol)
	params.Set("side", strings.ToUpper(req.Side))
	positionSide := strings.ToUpper(req.PositionSide)
	if positionSide == "" {
		positionSide = "BOTH"
	}
	params.Set("positionSide", positionSide)

	if req.ClosePosition {
		// closePosition cannot be sent with quantity or reduceOnly.
		params.Set("closePosition", "true")
	} else {
		params.Set("quantity", strconv.FormatFloat(req.Quantity, 'f', -1, 64))
		if req.ReduceOnly {
			params.Set("reduceOnly", "true")
		}
	}
	if orderType == "TRAILING_STOP_MARKET" {
		if req.ActivationPrice > 0 {
			params.Set("activatePrice", strconv.FormatFloat(req.ActivationPrice, 'f', -1, 64))
		}
		if req.CallbackRate > 0 {
			params.Set("callbackRate", strconv.FormatFloat(req.CallbackRate, 'f', -1, 64))
		}
	} else {
		params.Set("triggerPrice", strconv.FormatFloat(req.StopPrice, 'f', -1, 64))
	}
	if req.WorkingType != "" {
		params.Set("workingType", strings.ToUpper(req.WorkingType))
	}
	if req.ClientOrderID != "" {
		params.Set("clientAlgoId", req.ClientOrderID)
	}
	if orderType == "STOP" || orderType == "TAKE_PROFIT" {
		params.Set("price", strconv.FormatFloat(req.Price, 'f', -1, 64))
		params.Set("timeInForce", "GTC")
	}
	params.Set("newOrderRespType", "RESULT")

	body, err := c.request(context.Background(), http.MethodPost, "/fapi/v1/algoOrder", params, true)
	if err != nil {
		return nil, err
	}
	var raw algoOrderRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse algo order: %w", err)
	}
	return raw.toOrder(), nil
}

// GetAlgoOrder queries one conditional (algo) order.
func (c *Client) GetAlgoOrder(symbol string, algoID int64) (*Order, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("algoId", strconv.FormatInt(algoID, 10))
	body, err := c.get(context.Background(), "/fapi/v1/algoOrder", params, true)
	if err != nil {
		return nil, err
	}
	var raw algoOrderRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse algo order: %w", err)
	}
	return raw.toOrder(), nil
}

// GetOpenAlgoOrders returns open conditional orders for a symbol (or all).
func (c *Client) GetOpenAlgoOrders(symbol string) ([]Order, error) {
	params := url.Values{}
	if symbol != "" {
		params.Set("symbol", symbol)
	}
	body, err := c.get(context.Background(), "/fapi/v1/openAlgoOrders", params, true)
	if err != nil {
		return nil, err
	}
	var raw []algoOrderRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		var wrapper struct {
			Orders []algoOrderRaw `json:"orders"`
		}
		if err2 := json.Unmarshal(body, &wrapper); err2 != nil {
			return nil, fmt.Errorf("parse open algo orders: %w", err)
		}
		raw = wrapper.Orders
	}
	out := make([]Order, 0, len(raw))
	for _, r := range raw {
		o := r.toOrder()
		if o.Status == "NEW" {
			out = append(out, *o)
		}
	}
	return out, nil
}

// CancelAlgoOrder cancels a conditional order by algo ID.
func (c *Client) CancelAlgoOrder(symbol string, algoID int64) (*Order, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("algoId", strconv.FormatInt(algoID, 10))
	body, err := c.request(context.Background(), http.MethodDelete, "/fapi/v1/algoOrder", params, true)
	if err != nil {
		return nil, err
	}
	var raw algoOrderRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse algo cancel: %w", err)
	}
	return raw.toOrder(), nil
}

// CancelAllAlgoOpenOrders cancels every open conditional order for a symbol.
func (c *Client) CancelAllAlgoOpenOrders(symbol string) error {
	params := url.Values{}
	params.Set("symbol", symbol)
	_, err := c.request(context.Background(), http.MethodDelete, "/fapi/v1/algoOpenOrders", params, true)
	return err
}

// CancelOrder cancels an active order by ID. Algo (conditional) orders use a
// separate endpoint, so the regular attempt is followed by an algo attempt.
func (c *Client) CancelOrder(symbol string, orderID int64) (*Order, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("orderId", strconv.FormatInt(orderID, 10))

	body, err := c.request(context.Background(), http.MethodDelete, "/fapi/v1/order", params, true)
	if err == nil {
		var raw orderRaw
		if uerr := json.Unmarshal(body, &raw); uerr != nil {
			return nil, fmt.Errorf("parse cancel: %w", uerr)
		}
		return raw.toOrder(), nil
	}
	normalErr := err
	if algo, aerr := c.CancelAlgoOrder(symbol, orderID); aerr == nil {
		return algo, nil
	} else {
		return nil, fmt.Errorf("cancel order %d: regular: %v; algo: %v", orderID, normalErr, aerr)
	}
}

// GetOpenOrders returns all open regular and algo orders for a symbol.
// Empty string = all symbols.
func (c *Client) GetOpenOrders(symbol string) ([]Order, error) {
	params := url.Values{}
	if symbol != "" {
		params.Set("symbol", symbol)
	}

	body, err := c.get(context.Background(), "/fapi/v1/openOrders", params, true)
	var regular []Order
	if err == nil {
		var raw []orderRaw
		if uerr := json.Unmarshal(body, &raw); uerr != nil {
			return nil, fmt.Errorf("parse open orders: %w", uerr)
		}
		regular = make([]Order, 0, len(raw))
		for _, r := range raw {
			regular = append(regular, *r.toOrder())
		}
	}

	algo, algoErr := c.GetOpenAlgoOrders(symbol)
	if err != nil && algoErr != nil {
		return nil, err
	}
	if err != nil {
		return algo, nil
	}
	if algoErr != nil {
		return regular, nil
	}
	return append(regular, algo...), nil
}

// GetOrder returns details for a specific order (regular or algo).
func (c *Client) GetOrder(symbol string, orderID int64) (*Order, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("orderId", strconv.FormatInt(orderID, 10))

	body, err := c.get(context.Background(), "/fapi/v1/order", params, true)
	if err == nil {
		var raw orderRaw
		if uerr := json.Unmarshal(body, &raw); uerr != nil {
			return nil, fmt.Errorf("parse order: %w", uerr)
		}
		return raw.toOrder(), nil
	}
	normalErr := err
	if algo, aerr := c.GetAlgoOrder(symbol, orderID); aerr == nil {
		return algo, nil
	} else {
		return nil, fmt.Errorf("get order %d: regular: %v; algo: %v", orderID, normalErr, aerr)
	}
}

// --- Account setup ---

// SetPositionMode sets hedge mode. dual=false means one-way mode (单向持仓).
func (c *Client) SetPositionMode(dual bool) error {
	params := url.Values{}
	params.Set("dualSidePosition", strconv.FormatBool(dual))
	_, err := c.request(context.Background(), http.MethodPost, "/fapi/v1/positionSide/dual", params, true)
	return err
}

// GetPositionMode reports whether the account is in hedge (dual-side) mode.
func (c *Client) GetPositionMode() (bool, error) {
	body, err := c.get(context.Background(), "/fapi/v1/positionSide/dual", nil, true)
	if err != nil {
		return false, err
	}
	var raw struct {
		DualSidePosition bool `json:"dualSidePosition"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return false, fmt.Errorf("parse position mode: %w", err)
	}
	return raw.DualSidePosition, nil
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

// GetFundingRate returns the current funding rate and the next funding time.
// /fapi/v1/fundingRate is a historical endpoint (its latest row can be hours
// old), so use premiumIndex which carries the current lastFundingRate and
// nextFundingTime.
func (c *Client) GetFundingRate(symbol string) (float64, int64, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	body, err := c.get(context.Background(), "/fapi/v1/premiumIndex", params, false)
	if err != nil {
		return 0, 0, err
	}
	var raw struct {
		LastFundingRate string `json:"lastFundingRate"`
		NextFundingTime int64  `json:"nextFundingTime"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return 0, 0, fmt.Errorf("parse funding rate: %w", err)
	}
	rate, _ := strconv.ParseFloat(raw.LastFundingRate, 64)
	return rate, raw.NextFundingTime, nil
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

// GetExchangeInfo returns trading rules for every USDⓈ-M futures symbol.
// These filters (tick size, step size, min notional) must come from the
// exchange rather than a hardcoded table, especially for altcoins.
func (c *Client) GetExchangeInfo() (map[string]SymbolInfo, error) {
	body, err := c.get(context.Background(), "/fapi/v1/exchangeInfo", nil, false)
	if err != nil {
		return nil, err
	}
	var raw struct {
		Symbols []struct {
			Symbol            string `json:"symbol"`
			Status            string `json:"status"`
			PricePrecision    int    `json:"pricePrecision"`
			QuantityPrecision int    `json:"quantityPrecision"`
			Filters           []struct {
				FilterType  string `json:"filterType"`
				TickSize    string `json:"tickSize"`
				StepSize    string `json:"stepSize"`
				Notional    string `json:"notional"`
				MinNotional string `json:"minNotional"`
			} `json:"filters"`
		} `json:"symbols"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse exchangeInfo: %w", err)
	}

	out := make(map[string]SymbolInfo, len(raw.Symbols))
	for _, s := range raw.Symbols {
		info := SymbolInfo{
			Symbol:            s.Symbol,
			Status:            s.Status,
			PricePrecision:    s.PricePrecision,
			QuantityPrecision: s.QuantityPrecision,
		}
		for _, f := range s.Filters {
			switch f.FilterType {
			case "PRICE_FILTER":
				info.TickSize, _ = strconv.ParseFloat(f.TickSize, 64)
			case "LOT_SIZE":
				info.StepSize, _ = strconv.ParseFloat(f.StepSize, 64)
			case "MIN_NOTIONAL":
				info.MinNotional, _ = strconv.ParseFloat(f.Notional, 64)
				if info.MinNotional == 0 {
					info.MinNotional, _ = strconv.ParseFloat(f.MinNotional, 64)
				}
			}
		}
		out[s.Symbol] = info
	}
	return out, nil
}

// GetAllPrices returns the latest price for every symbol in one request.
func (c *Client) GetAllPrices() (map[string]float64, error) {
	body, err := c.get(context.Background(), "/fapi/v1/ticker/price", nil, false)
	if err != nil {
		return nil, err
	}
	var raw []tickerPriceRaw
	if err := json.Unmarshal(body, &raw); err != nil {
		return nil, fmt.Errorf("parse all prices: %w", err)
	}
	out := make(map[string]float64, len(raw))
	for _, r := range raw {
		p, err := strconv.ParseFloat(r.Price, 64)
		if err == nil {
			out[r.Symbol] = p
		}
	}
	return out, nil
}

// CancelAllOpenOrders cancels every open order for a symbol.
func (c *Client) CancelAllOpenOrders(symbol string) error {
	params := url.Values{}
	params.Set("symbol", symbol)
	_, normalErr := c.request(context.Background(), http.MethodDelete, "/fapi/v1/allOpenOrders", params, true)
	algoErr := c.CancelAllAlgoOpenOrders(symbol)
	if normalErr != nil && algoErr != nil {
		return fmt.Errorf("cancel all: regular: %v; algo: %v", normalErr, algoErr)
	}
	return nil
}

// --- OCO Order ---

type ocoOrderRaw struct {
	OrderListID  int64      `json:"orderListId"`
	Orders       []orderRaw `json:"orders"`
	OrderReports []orderRaw `json:"orderReports"`
}

// OCOOrder represents a One-Cancels-Other order pair.
type OCOOrder struct {
	OrderListID int64  `json:"orderListId"`
	StopOrder   *Order `json:"stop_order"`
	LimitOrder  *Order `json:"limit_order"`
}

// CreateOCOOrder places an OCO (One-Cancels-Other) order — a take-profit limit
// and a stop-loss stop-market order. When one triggers, the other cancels.
func (c *Client) CreateOCOOrder(symbol, side string, quantity, price, stopPrice float64) (*OCOOrder, error) {
	params := url.Values{}
	params.Set("symbol", symbol)
	params.Set("side", side)
	params.Set("quantity", strconv.FormatFloat(quantity, 'f', -1, 64))
	params.Set("price", strconv.FormatFloat(price, 'f', -1, 64))         // take-profit limit
	params.Set("stopPrice", strconv.FormatFloat(stopPrice, 'f', -1, 64)) // stop-loss trigger
	params.Set("stopLimitPrice", "0")                                    // 0 = stop-market
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

// --- ATR (Average True Range) & Volatility ---

// GetATR calculates the Average True Range over a given period.
func (c *Client) GetATR(symbol, interval string, period int) (float64, error) {
	if period <= 0 {
		period = 14
	}
	// Need period+1 klines to calculate ATR for `period` candles
	klines, err := c.GetKlines(symbol, interval, period+1)
	if err != nil {
		return 0, err
	}
	if len(klines) < 2 {
		return 0, fmt.Errorf("not enough klines for ATR")
	}

	var sum float64
	for i := 1; i < len(klines); i++ {
		high := klines[i].High
		low := klines[i].Low
		prevClose := klines[i-1].Close
		tr := high - low
		if hc := high - prevClose; hc > tr {
			tr = hc
		}
		if lc := prevClose - low; lc > tr {
			tr = lc
		}
		sum += tr
	}
	return sum / float64(len(klines)-1), nil
}

// --- Candle Info ---

// CandleInfo holds current candle timing info.
type CandleInfo struct {
	Symbol       string `json:"symbol"`
	Interval     string `json:"interval"`
	OpenTime     int64  `json:"open_time"`
	CloseTime    int64  `json:"close_time"`
	RemainingSec int64  `json:"remaining_sec"`
}

// GetCandleInfo returns timing info for the current candle.
func (c *Client) GetCandleInfo(symbol, interval string) (*CandleInfo, error) {
	klines, err := c.GetKlines(symbol, interval, 1)
	if err != nil || len(klines) == 0 {
		return nil, err
	}
	k := klines[0]
	now := time.Now().UnixMilli()
	remaining := (k.CloseTime - now) / 1000
	if remaining < 0 {
		remaining = 0
	}
	return &CandleInfo{
		Symbol: symbol, Interval: interval,
		OpenTime: k.OpenTime, CloseTime: k.CloseTime,
		RemainingSec: remaining,
	}, nil
}
