package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

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

// HandleKlines handles GET /api/v1/market/klines — supports comma-separated symbols.
func (h *MarketHandler) HandleKlines(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		Error(w, http.StatusBadRequest, CodeMissingParam, "symbol is required")
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
			Error(w, http.StatusBadRequest, CodeInvalidParam, "limit must be 1-1500")
			return
		}
	}

	symbols := strings.Split(symbol, ",")
	if len(symbols) == 1 {
		klines, err := h.client.GetKlines(symbols[0], interval, limit)
		if err != nil {
			Error(w, http.StatusInternalServerError, CodeBinanceError, err.Error())
			return
		}
		JSON(w, http.StatusOK, klines)
		return
	}

	result := make(map[string][]binance.Kline)
	for _, sym := range symbols {
		sym = strings.TrimSpace(sym)
		if sym == "" {
			continue
		}
		klines, err := h.client.GetKlines(sym, interval, limit)
		if err != nil {
			Error(w, http.StatusInternalServerError, CodeBinanceError, fmt.Sprintf("%s: %v", sym, err))
			return
		}
		result[sym] = klines
	}
	JSON(w, http.StatusOK, result)
}

// HandleTicker handles GET /api/v1/market/ticker
func (h *MarketHandler) HandleTicker(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		Error(w, http.StatusBadRequest, CodeMissingParam, "symbol is required")
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
		Error(w, http.StatusInternalServerError, CodeBinanceError, err.Error())
		return
	}
	JSON(w, http.StatusOK, ticker)
}

// HandleWatch handles GET /api/v1/market/watch — returns all cached market data.
func (h *MarketHandler) HandleWatch(w http.ResponseWriter, r *http.Request) {
	if h.cache == nil {
		Error(w, http.StatusInternalServerError, CodeNoCache, "market cache not initialized")
		return
	}
	JSON(w, http.StatusOK, h.cache.Snapshot())
}

// HandleCalendar handles GET /api/v1/market/calendar — returns upcoming economic events.
func (h *MarketHandler) HandleCalendar(w http.ResponseWriter, _ *http.Request) {
	events := getUpcomingEvents()
	JSON(w, http.StatusOK, events)
}

// HandleOrderBook handles GET /api/v1/market/orderbook
func (h *MarketHandler) HandleOrderBook(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		Error(w, http.StatusBadRequest, CodeMissingParam, "symbol is required")
		return
	}

	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		var err error
		limit, err = strconv.Atoi(limitStr)
		if err != nil || limit <= 0 || limit > 1000 {
			Error(w, http.StatusBadRequest, CodeInvalidParam, "limit must be 1-1000")
			return
		}
	}

	ob, err := h.client.GetOrderBook(symbol, limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeBinanceError, err.Error())
		return
	}

	JSON(w, http.StatusOK, ob)
}
