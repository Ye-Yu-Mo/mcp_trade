package api

import (
	"net/http"
	"strconv"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
)

// MarketHandler handles market data endpoints.
type MarketHandler struct {
	client binance.Trader
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

	ticker, err := h.client.GetTicker(symbol)
	if err != nil {
		Error(w, http.StatusInternalServerError, "BINANCE_ERROR", err.Error())
		return
	}

	JSON(w, http.StatusOK, ticker)
}
