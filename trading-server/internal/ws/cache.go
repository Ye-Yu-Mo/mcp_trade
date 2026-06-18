package ws

import (
	"strings"
	"sync"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
)

// MarketCache holds the latest market data from WebSocket streams.
// Thread-safe via sync.RWMutex.
type MarketCache struct {
	mu        sync.RWMutex
	prices    map[string]float64          // symbol → latest price
	klines    map[string]binance.Kline    // symbol:interval → latest kline
	orderBooks map[string]binance.OrderBook // symbol → latest order book
}

// NewMarketCache creates an empty market cache.
func NewMarketCache() *MarketCache {
	return &MarketCache{
		prices:     make(map[string]float64),
		klines:     make(map[string]binance.Kline),
		orderBooks: make(map[string]binance.OrderBook),
	}
}

// SetPrice updates the latest price for a symbol.
func (c *MarketCache) SetPrice(symbol string, price float64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.prices[strings.ToUpper(symbol)] = price
}

// GetPrice returns the latest price, false if not found.
func (c *MarketCache) GetPrice(symbol string) (float64, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.prices[strings.ToUpper(symbol)]
	return p, ok
}

// SetKline updates the latest kline for a symbol+interval.
func (c *MarketCache) SetKline(symbol, interval string, k binance.Kline) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.klines[strings.ToUpper(symbol)+":"+interval] = k
}

// GetKline returns the latest kline, false if not found.
func (c *MarketCache) GetKline(symbol, interval string) (binance.Kline, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	k, ok := c.klines[strings.ToUpper(symbol)+":"+interval]
	return k, ok
}

// SetOrderBook updates the latest order book for a symbol.
func (c *MarketCache) SetOrderBook(symbol string, ob binance.OrderBook) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.orderBooks[strings.ToUpper(symbol)] = ob
}

// GetOrderBook returns the latest order book, false if not found.
func (c *MarketCache) GetOrderBook(symbol string) (binance.OrderBook, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	ob, ok := c.orderBooks[strings.ToUpper(symbol)]
	return ob, ok
}

// Snapshot returns all cached data for the market.watch endpoint.
func (c *MarketCache) Snapshot() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return map[string]interface{}{
		"prices":     c.prices,
		"klines":     c.klines,
		"orderBooks": c.orderBooks,
	}
}
