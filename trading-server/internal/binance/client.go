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
	"strconv"
	"time"
)

// Client wraps the Binance Futures API using raw HTTP.
type Client struct {
	apiKey     string
	apiSecret  string
	baseURL    string
	httpClient *http.Client
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

// get sends a GET request to the given path with query parameters.
// If signed is true, adds timestamp and HMAC signature.
func (c *Client) get(ctx context.Context, path string, params url.Values, signed bool) ([]byte, error) {
	if signed {
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

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if signed {
		req.Header.Set("X-MBX-APIKEY", c.apiKey)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get %s: %w", path, err)
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
