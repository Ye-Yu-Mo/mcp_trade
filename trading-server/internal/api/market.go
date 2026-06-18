package api

import (
	"net/http"
	"strconv"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/ws"
)

// MarketHandler handles market data endpoints.
type MarketHandler struct {
	client binance.Trader
	cache  *ws.MarketCache
}

// NewMarketHandler creates a MarketHandler.
func NewMarketHandler(client binance.Trader) *MarketHandler {
	return &MarketHandler{client: client}
}

// HandleKlines handles GET /api/v1/market/klines
func (h *MarketHandler) HandleKlines(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		Error(w, http.StatusBadRequest, "MISSING_PARAM", "symbol is required")
		return
	}

	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "1h"
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 500
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 || limit > 1500 {
			Error(w, http.StatusBadRequest, "INVALID_PARAM", "limit must be 1-1500")
			return
		}
	}

	klines, err := h.client.GetKlines(symbol, interval, limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "BINANCE_ERROR", err.Error())
		return
	}

	JSON(w, http.StatusOK, klines)
}

// HandleTicker handles GET /api/v1/market/ticker
func (h *MarketHandler) HandleTicker(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		Error(w, http.StatusBadRequest, "MISSING_PARAM", "symbol is required")
		return
	}

	// Cache-first
	if h.cache != nil {
		if price, ok := h.cache.GetPrice(symbol); ok {
			JSON(w, http.StatusOK, &binance.Ticker{Symbol: symbol, Price: price})
			return
		}
	}

	ticker, err := h.client.GetTicker(symbol)
	if err != nil {
		Error(w, http.StatusInternalServerError, "BINANCE_ERROR", err.Error())
		return
	}
	JSON(w, http.StatusOK, ticker)
}

// HandleWatch handles GET /api/v1/market/watch — returns all cached market data.
func (h *MarketHandler) HandleWatch(w http.ResponseWriter, r *http.Request) {
	if h.cache == nil {
		Error(w, http.StatusInternalServerError, "NO_CACHE", "market cache not initialized")
		return
	}
	JSON(w, http.StatusOK, h.cache.Snapshot())
}

// HandleOrderBook handles GET /api/v1/market/orderbook
func (h *MarketHandler) HandleOrderBook(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		Error(w, http.StatusBadRequest, "MISSING_PARAM", "symbol is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 || limit > 1000 {
			Error(w, http.StatusBadRequest, "INVALID_PARAM", "limit must be 1-1000")
			return
		}
	}

	ob, err := h.client.GetOrderBook(symbol, limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, "BINANCE_ERROR", err.Error())
		return
	}

	JSON(w, http.StatusOK, ob)
}
