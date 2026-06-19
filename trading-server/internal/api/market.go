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
	client     binance.Trader
	cache      *ws.MarketCache
	alertStore *ws.AlertStore
}

// NewMarketHandler creates a MarketHandler.
func NewMarketHandler(client binance.Trader) *MarketHandler {
	return &MarketHandler{client: client}
}

// HandleKlines handles GET /api/v1/market/klines — supports comma-separated symbols and intervals.
// Single symbol + single interval: returns []Kline
// Multi symbol or multi interval: returns map[string]...{}
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
	intervals := strings.Split(interval, ",")
	multiSym := len(symbols) > 1
	multiInt := len(intervals) > 1

	// Simple case: one symbol, one interval
	if !multiSym && !multiInt {
		klines, err := h.client.GetKlines(symbols[0], intervals[0], limit)
		if err != nil {
			Error(w, http.StatusInternalServerError, CodeBinanceError, err.Error())
			return
		}
		JSON(w, http.StatusOK, klines)
		return
	}

	// Multi-dimensional result: symbol → interval → klines
	result := make(map[string]map[string][]binance.Kline)
	for _, sym := range symbols {
		sym = strings.TrimSpace(sym)
		if sym == "" {
			continue
		}
		result[sym] = make(map[string][]binance.Kline)
		for _, iv := range intervals {
			iv = strings.TrimSpace(iv)
			if iv == "" {
				continue
			}
			klines, err := h.client.GetKlines(sym, iv, limit)
			if err != nil {
				Error(w, http.StatusInternalServerError, CodeBinanceError, fmt.Sprintf("%s/%s: %v", sym, iv, err))
				return
			}
			result[sym][iv] = klines
		}
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

// HandleWatch handles GET /api/v1/market/watch — returns all cached market data + triggered alerts.
func (h *MarketHandler) HandleWatch(w http.ResponseWriter, r *http.Request) {
	if h.cache == nil {
		Error(w, http.StatusInternalServerError, CodeNoCache, "market cache not initialized")
		return
	}
	snapshot := h.cache.Snapshot()
	if h.alertStore != nil {
		snapshot["triggered_alerts"] = h.alertStore.ListTriggered()
	}
	JSON(w, http.StatusOK, snapshot)
}

// HandleCalendar handles GET /api/v1/market/calendar — returns upcoming economic events.
func (h *MarketHandler) HandleCalendar(w http.ResponseWriter, _ *http.Request) {
	events := getUpcomingEvents()
	JSON(w, http.StatusOK, events)
}

// HandleSetAlert handles POST /api/v1/market/alert — creates a price alert.
func (h *MarketHandler) HandleSetAlert(w http.ResponseWriter, r *http.Request) {
	symbol := r.FormValue("symbol")
	priceStr := r.FormValue("price")
	direction := r.FormValue("direction")
	message := r.FormValue("message")

	if symbol == "" || priceStr == "" || direction == "" {
		Error(w, http.StatusBadRequest, CodeMissingParam, "symbol, price, direction are required")
		return
	}
	price, err := strconv.ParseFloat(priceStr, 64)
	if err != nil || price <= 0 {
		Error(w, http.StatusBadRequest, CodeInvalidParam, "invalid price")
		return
	}
	if direction != "ABOVE" && direction != "BELOW" {
		Error(w, http.StatusBadRequest, CodeInvalidParam, "direction must be ABOVE or BELOW")
		return
	}
	if h.alertStore == nil {
		Error(w, http.StatusInternalServerError, CodeNoCache, "alert store not initialized")
		return
	}

	id := h.alertStore.Add(symbol, price, direction, message)
	JSON(w, http.StatusOK, map[string]string{"id": id, "status": "active"})
}

// HandleListAlerts handles GET /api/v1/market/alerts — lists all price alerts.
func (h *MarketHandler) HandleListAlerts(w http.ResponseWriter, r *http.Request) {
	if h.alertStore == nil {
		Error(w, http.StatusInternalServerError, CodeNoCache, "alert store not initialized")
		return
	}
	alerts := h.alertStore.List()
	if alerts == nil {
		alerts = []*ws.PriceAlert{}
	}
	JSON(w, http.StatusOK, alerts)
}

// HandleRemoveAlert handles DELETE /api/v1/market/alert — removes a price alert.
func (h *MarketHandler) HandleRemoveAlert(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		Error(w, http.StatusBadRequest, CodeMissingParam, "alert id is required")
		return
	}
	if h.alertStore == nil {
		Error(w, http.StatusInternalServerError, CodeNoCache, "alert store not initialized")
		return
	}
	if h.alertStore.Remove(id) {
		JSON(w, http.StatusOK, map[string]string{"status": "removed"})
	} else {
		Error(w, http.StatusNotFound, CodeOrderNotFound, "alert not found")
	}
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

// HandleScanner handles GET /api/v1/market/scanner — returns top symbols by volume.
func (h *MarketHandler) HandleScanner(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}
	results, err := h.client.ScanMarket(limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeBinanceError, err.Error())
		return
	}
	JSON(w, http.StatusOK, results)
}

// HandleFunding handles GET /api/v1/market/funding — returns current funding rate.
func (h *MarketHandler) HandleFunding(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTCUSDT"
	}
	rate, fundingTime, err := h.client.GetFundingRate(symbol)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeBinanceError, err.Error())
		return
	}
	JSON(w, http.StatusOK, map[string]interface{}{
		"symbol":       symbol,
		"funding_rate": rate,
		"funding_time": fundingTime,
	})
}

// HandleOI handles GET /api/v1/market/oi — returns current open interest.
func (h *MarketHandler) HandleOI(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTCUSDT"
	}
	oi, err := h.client.GetOpenInterest(symbol)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeBinanceError, err.Error())
		return
	}
	JSON(w, http.StatusOK, map[string]interface{}{
		"symbol":        symbol,
		"open_interest": oi,
	})
}

// HandleATR handles GET /api/v1/market/atr — returns Average True Range.
func (h *MarketHandler) HandleATR(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTCUSDT"
	}
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "1h"
	}
	periodStr := r.URL.Query().Get("period")
	period := 14
	if periodStr != "" {
		period, _ = strconv.Atoi(periodStr)
	}
	atr, err := h.client.GetATR(symbol, interval, period)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeBinanceError, err.Error())
		return
	}
	price, _ := h.client.GetTicker(symbol)
	pct := 0.0
	if price != nil {
		pct = (atr / price.Price) * 100
	}
	JSON(w, http.StatusOK, map[string]interface{}{
		"symbol": symbol, "interval": interval, "period": period,
		"atr": atr, "atr_pct": pct,
	})
}

// HandleCandleInfo handles GET /api/v1/market/candle_info — current candle timing.
func (h *MarketHandler) HandleCandleInfo(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		symbol = "BTCUSDT"
	}
	interval := r.URL.Query().Get("interval")
	if interval == "" {
		interval = "1h"
	}
	info, err := h.client.GetCandleInfo(symbol, interval)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeBinanceError, err.Error())
		return
	}
	JSON(w, http.StatusOK, info)
}
