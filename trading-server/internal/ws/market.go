package ws

import (
	"encoding/json"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
)

// MarketClient abstracts the Binance client for REST fallback.
type MarketClient interface {
	GetTicker(symbol string) (*binance.Ticker, error)
	GetKlines(symbol, interval string, limit int) ([]binance.Kline, error)
	GetOrderBook(symbol string, limit int) (*binance.OrderBook, error)
}

// MarketStream fetches real-time market data via WebSocket with REST fallback.
type MarketStream struct {
	baseURL   string
	cache     *MarketCache
	symbols   []string
	client    MarketClient
	connected bool
	done      chan struct{}
}

func NewMarketStream(baseURL string, cache *MarketCache, symbols []string, client MarketClient) *MarketStream {
	return &MarketStream{baseURL: baseURL, cache: cache, symbols: symbols, client: client, done: make(chan struct{})}
}

func (m *MarketStream) Connected() bool { return m.connected }

func (m *MarketStream) Start() {
	go m.runWS()
	go m.runRESTPoll()
}

func (m *MarketStream) Stop() { close(m.done) }

// --- WebSocket mode (mainnet) ---

func (m *MarketStream) runWS() {
	backoff := 1 * time.Second
	const maxBackoff = 30 * time.Second

	for {
		select {
		case <-m.done:
			return
		default:
		}

		wsHost := m.wsHost()
		streams := make([]string, 0, len(m.symbols)*3)
		for _, sym := range m.symbols {
			s := strings.ToLower(sym)
			streams = append(streams, s+"@bookTicker", s+"@kline_1h", s+"@depth20@100ms")
		}
		wsURL := wsHost + "?streams=" + strings.Join(streams, "/")

		conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
		if err != nil {
			log.Printf("ws market: cannot connect (%v), using REST polling", err)
			m.connected = false
			return // fall back to REST
		}

		log.Printf("ws market: connected to %s", wsHost)
		m.connected = true
		backoff = 1 * time.Second

		for {
			select {
			case <-m.done:
				conn.Close()
				return
			default:
			}
			conn.SetReadDeadline(time.Now().Add(60 * time.Second))
			_, msg, err := conn.ReadMessage()
			if err != nil {
				log.Printf("ws market: read: %v, reconnecting", err)
				m.connected = false
				conn.Close()
				break
			}
			m.parseWSMessage(msg)
		}

		time.Sleep(backoff)
		backoff *= 2
		if backoff > maxBackoff {
			backoff = maxBackoff
		}
	}
}

func (m *MarketStream) wsHost() string {
	if strings.Contains(m.baseURL, "testnet") {
		return "wss://testnet.binancefuture.com/stream"
	}
	return "wss://fstream.binance.com/stream"
}

func (m *MarketStream) parseWSMessage(msg []byte) {
	var raw map[string]json.RawMessage
	if json.Unmarshal(msg, &raw) != nil {
		return
	}
	if data, ok := raw["data"]; ok {
		var streamName string
		json.Unmarshal(raw["stream"], &streamName)
		m.parseEvent(streamName, data)
	}
}

// --- REST polling mode (testnet fallback) ---

func (m *MarketStream) runRESTPoll() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	intervals := []string{"1h", "4h", "1d"}

	for {
		select {
		case <-m.done:
			return
		case <-ticker.C:
			for _, sym := range m.symbols {
				if t, err := m.client.GetTicker(sym); err == nil {
					m.cache.SetPrice(sym, t.Price)
				}
				for _, interval := range intervals {
					if klines, err := m.client.GetKlines(sym, interval, 1); err == nil && len(klines) > 0 {
						m.cache.SetKline(sym, interval, klines[0])
					}
				}
				if ob, err := m.client.GetOrderBook(sym, 20); err == nil {
					m.cache.SetOrderBook(sym, *ob)
				}
			}
		}
	}
}

// --- Event parsing ---

func (m *MarketStream) parseEvent(streamName string, data json.RawMessage) {
	symbol := extractSymbol(streamName)
	switch {
	case strings.Contains(streamName, "@bookTicker"):
		var evt struct {
			BidPrice string `json:"b"`
			BidQty   string `json:"B"`
			AskPrice string `json:"a"`
		}
		if json.Unmarshal(data, &evt) == nil {
			bid, _ := strconv.ParseFloat(evt.BidPrice, 64)
			bidQty, _ := strconv.ParseFloat(evt.BidQty, 64)
			ask, _ := strconv.ParseFloat(evt.AskPrice, 64)
			m.cache.SetPrice(symbol, (bid+ask)/2)
			m.cache.SetOrderBook(symbol, binance.OrderBook{
				Symbol: symbol,
				Bids:   []binance.OrderBookLevel{{Price: bid, Quantity: bidQty}},
				Asks:   []binance.OrderBookLevel{{Price: ask, Quantity: 0}},
			})
		}
	case strings.Contains(streamName, "@kline"):
		var evt struct {
			Kline struct {
				StartTime int64  `json:"t"`
				CloseTime int64  `json:"T"`
				Open      string `json:"o"`
				High      string `json:"h"`
				Low       string `json:"l"`
				Close     string `json:"c"`
				Volume    string `json:"v"`
			} `json:"k"`
		}
		if json.Unmarshal(data, &evt) == nil {
			o, _ := strconv.ParseFloat(evt.Kline.Open, 64)
			h, _ := strconv.ParseFloat(evt.Kline.High, 64)
			l, _ := strconv.ParseFloat(evt.Kline.Low, 64)
			c, _ := strconv.ParseFloat(evt.Kline.Close, 64)
			v, _ := strconv.ParseFloat(evt.Kline.Volume, 64)
			interval := "1h"
			if idx := strings.LastIndex(streamName, "_"); idx > 0 {
				interval = streamName[idx+1:]
			}
			m.cache.SetKline(symbol, interval, binance.Kline{
				OpenTime: evt.Kline.StartTime, Open: o, High: h, Low: l, Close: c, Volume: v,
				CloseTime: evt.Kline.CloseTime,
			})
			m.cache.SetPrice(symbol, c)
		}
	case strings.Contains(streamName, "@depth"):
		var evt struct {
			Bids [][]string `json:"bids"`
			Asks [][]string `json:"asks"`
		}
		if json.Unmarshal(data, &evt) == nil {
			parse := func(d [][]string) []binance.OrderBookLevel {
				lv := make([]binance.OrderBookLevel, 0, len(d))
				for _, e := range d {
					if len(e) >= 2 {
						p, _ := strconv.ParseFloat(e[0], 64)
						q, _ := strconv.ParseFloat(e[1], 64)
						lv = append(lv, binance.OrderBookLevel{Price: p, Quantity: q})
					}
				}
				return lv
			}
			m.cache.SetOrderBook(symbol, binance.OrderBook{Symbol: symbol, Bids: parse(evt.Bids), Asks: parse(evt.Asks)})
		}
	}
}

func extractSymbol(streamName string) string {
	return strings.ToUpper(strings.SplitN(streamName, "@", 2)[0])
}
