package ws

import (
	"encoding/json"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
)

// MarketStream connects to Binance WebSocket for real-time market data.
type MarketStream struct {
	baseURL string
	cache   *MarketCache
	symbols []string
	done    chan struct{}
}

// NewMarketStream creates a market stream for the given symbols.
func NewMarketStream(baseURL string, cache *MarketCache, symbols []string) *MarketStream {
	return &MarketStream{
		baseURL: baseURL,
		cache:   cache,
		symbols: symbols,
		done:    make(chan struct{}),
	}
}

// Start connects to the WebSocket and begins processing messages.
// It auto-reconnects on disconnect with exponential backoff.
func (m *MarketStream) Start() {
	go m.loop()
}

// Stop signals the stream to shut down.
func (m *MarketStream) Stop() {
	close(m.done)
}

func (m *MarketStream) loop() {
	backoff := 1 * time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-m.done:
			return
		default:
		}

		if err := m.connect(); err != nil {
			log.Printf("ws market: %v, reconnecting in %v", err, backoff)
			time.Sleep(backoff)
			backoff *= 2
			if backoff > maxBackoff {
				backoff = maxBackoff
			}
			continue
		}
		backoff = 1 * time.Second // reset on clean disconnect
	}
}

func (m *MarketStream) connect() error {
	// Build combined stream URL: wss://host/stream?streams=sym1@stream1/sym2@stream2
	wsHost := "wss://fstream.binance.com/stream"
	if strings.Contains(m.baseURL, "testnet") {
		wsHost = "wss://testnet.binancefuture.com/stream"
	}

	streams := make([]string, 0, len(m.symbols)*3)
	for _, sym := range m.symbols {
		s := strings.ToLower(sym)
		streams = append(streams,
			s+"@bookTicker",
			s+"@kline_1h",
			s+"@depth20@100ms",
		)
	}
	wsURL := wsHost + "?streams=" + strings.Join(streams, "/")

	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial %s: %w", wsURL, err)
	}
	defer conn.Close()
	log.Printf("ws market: connected to %s (%d streams)", wsHost, len(streams))

	// Read messages
	for {
		select {
		case <-m.done:
			return nil
		default:
		}

		conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		_, msg, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}

		m.handleMessage(msg)
	}
}

func (m *MarketStream) handleMessage(msg []byte) {
	// Try to parse as a combined stream or single event
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(msg, &raw); err != nil {
		return
	}

	// Combined stream: {"stream":"btcusdt@bookTicker","data":{...}}
	if streamName, ok := raw["stream"]; ok {
		var name string
		json.Unmarshal(streamName, &name)
		if data, ok := raw["data"]; ok {
			m.parseEvent(name, data)
		}
		return
	}

	// Single event (from direct subscription): try to identify type
	if _, ok := raw["e"]; ok {
		var eventType string
		json.Unmarshal(raw["e"], &eventType)
		m.parseSingleEvent(eventType, msg)
	}
}

func (m *MarketStream) parseEvent(streamName string, data json.RawMessage) {
	symbol := extractSymbol(streamName)

	switch {
	case strings.Contains(streamName, "@bookTicker"):
		var evt struct {
			BidPrice string `json:"b"`
			BidQty   string `json:"B"`
			AskPrice string `json:"a"`
			AskQty   string `json:"A"`
		}
		if json.Unmarshal(data, &evt) == nil {
			bid, _ := strconv.ParseFloat(evt.BidPrice, 64)
			bidQty, _ := strconv.ParseFloat(evt.BidQty, 64)
			ask, _ := strconv.ParseFloat(evt.AskPrice, 64)
			askQty, _ := strconv.ParseFloat(evt.AskQty, 64)
			// Update price from mid-price or last traded
			// bookTicker gives best bid/ask, use mid for price estimate
			mid := (bid + ask) / 2
			m.cache.SetPrice(symbol, mid)
			m.cache.SetOrderBook(symbol, binance.OrderBook{
				Symbol: symbol,
				Bids:   []binance.OrderBookLevel{{Price: bid, Quantity: bidQty}},
				Asks:   []binance.OrderBookLevel{{Price: ask, Quantity: askQty}},
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
				Closed    bool   `json:"x"`
			} `json:"k"`
		}
		if json.Unmarshal(data, &evt) == nil {
			o, _ := strconv.ParseFloat(evt.Kline.Open, 64)
			h, _ := strconv.ParseFloat(evt.Kline.High, 64)
			l, _ := strconv.ParseFloat(evt.Kline.Low, 64)
			c, _ := strconv.ParseFloat(evt.Kline.Close, 64)
			v, _ := strconv.ParseFloat(evt.Kline.Volume, 64)
			k := binance.Kline{
				OpenTime:  evt.Kline.StartTime,
				Open:      o,
				High:      h,
				Low:       l,
				Close:     c,
				Volume:    v,
				CloseTime: evt.Kline.CloseTime,
			}
			// Extract interval from stream name, e.g., "btcusdt@kline_1h"
			interval := "1h"
			if idx := strings.LastIndex(streamName, "_"); idx > 0 {
				interval = streamName[idx+1:]
			}
			m.cache.SetKline(symbol, interval, k)
			// Also update price from kline close
			m.cache.SetPrice(symbol, c)
		}

	case strings.Contains(streamName, "@depth"):
		var evt struct {
			Bids [][]string `json:"bids"`
			Asks [][]string `json:"asks"`
		}
		if json.Unmarshal(data, &evt) == nil {
			parseLevels := func(data [][]string) []binance.OrderBookLevel {
				levels := make([]binance.OrderBookLevel, 0, len(data))
				for _, entry := range data {
					if len(entry) < 2 {
						continue
					}
					price, _ := strconv.ParseFloat(entry[0], 64)
					qty, _ := strconv.ParseFloat(entry[1], 64)
					levels = append(levels, binance.OrderBookLevel{Price: price, Quantity: qty})
				}
				return levels
			}
			m.cache.SetOrderBook(symbol, binance.OrderBook{
				Symbol: symbol,
				Bids:   parseLevels(evt.Bids),
				Asks:   parseLevels(evt.Asks),
			})
		}
	}
}

// extractSymbol extracts the trading pair from a stream name.
func extractSymbol(streamName string) string {
	// "btcusdt@bookTicker" → "BTCUSDT"
	parts := strings.SplitN(streamName, "@", 2)
	return strings.ToUpper(parts[0])
}

func (m *MarketStream) parseSingleEvent(eventType string, msg []byte) {
	// Handle non-combined stream events if needed
	_ = eventType
	_ = msg
}
