package api

import (
	"net/http"
	"strconv"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/store"
)

// TradeHandler handles trade journal endpoints.
type TradeHandler struct {
	store  *store.Store
	client binance.Trader
	paper  bool
}

// NewTradeHandler creates a TradeHandler.
func NewTradeHandler(st *store.Store, client binance.Trader, paper bool) *TradeHandler {
	return &TradeHandler{store: st, client: client, paper: paper}
}

// HandleHistory handles GET /api/v1/trade/history
func (h *TradeHandler) HandleHistory(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	limitStr := r.URL.Query().Get("limit")
	limit := 50
	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}

	records, err := h.store.QueryTrades(symbol, limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeQueryFailed, err.Error())
		return
	}

	if records == nil {
		records = []store.TradeRecord{}
	}
	JSON(w, http.StatusOK, records)
}

// HandleJournal handles POST /api/v1/trade/journal
func (h *TradeHandler) HandleJournal(w http.ResponseWriter, r *http.Request) {
	entryType := r.FormValue("entry_type")
	reason := r.FormValue("reason")
	tags := r.FormValue("tags")

	if entryType == "" {
		Error(w, http.StatusBadRequest, CodeMissingParam, "entry_type is required")
		return
	}

	var tradeID *int64
	if tidStr := r.FormValue("trade_id"); tidStr != "" {
		tid, err := strconv.ParseInt(tidStr, 10, 64)
		if err == nil {
			tradeID = &tid
		}
	}

	id, err := h.store.InsertJournal(entryType, reason, tags, tradeID)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeInsertFailed, err.Error())
		return
	}

	JSON(w, http.StatusOK, map[string]int64{"id": id})
}

// HandleExchangeIncome handles GET /api/v1/trade/exchange — income records
// straight from the venue. Trades placed outside this system (manually on
// Binance) never reach our DB, but they do appear here.
func (h *TradeHandler) HandleExchangeIncome(w http.ResponseWriter, r *http.Request) {
	if h.paper {
		Error(w, http.StatusBadRequest, CodeInvalidParam, "paper mode has no exchange income records")
		return
	}
	getter, ok := h.client.(interface {
		GetIncome(symbol, incomeType string, startTime, endTime int64, limit int) ([]binance.Income, error)
	})
	if !ok {
		Error(w, http.StatusInternalServerError, CodeNoCache, "exchange client does not expose income records")
		return
	}

	q := r.URL.Query()
	startTime, _ := strconv.ParseInt(q.Get("start_time"), 10, 64)
	endTime, _ := strconv.ParseInt(q.Get("end_time"), 10, 64)
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 1000 {
		limit = 1000
	}

	records, err := getter.GetIncome(q.Get("symbol"), q.Get("income_type"), startTime, endTime, limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeBinanceError, err.Error())
		return
	}
	if records == nil {
		records = []binance.Income{}
	}
	JSON(w, http.StatusOK, records)
}

// HandleExchangeFills handles GET /api/v1/trade/fills — raw fills for one
// symbol, straight from the venue.
func (h *TradeHandler) HandleExchangeFills(w http.ResponseWriter, r *http.Request) {
	if h.paper {
		Error(w, http.StatusBadRequest, CodeInvalidParam, "paper mode has no exchange fills")
		return
	}
	getter, ok := h.client.(interface {
		GetUserTrades(symbol string, startTime int64, limit int) ([]binance.UserTrade, error)
	})
	if !ok {
		Error(w, http.StatusInternalServerError, CodeNoCache, "exchange client does not expose fills")
		return
	}
	q := r.URL.Query()
	symbol := q.Get("symbol")
	if symbol == "" {
		Error(w, http.StatusBadRequest, CodeMissingParam, "symbol is required")
		return
	}
	startTime, _ := strconv.ParseInt(q.Get("start_time"), 10, 64)
	limit, _ := strconv.Atoi(q.Get("limit"))
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	fills, err := getter.GetUserTrades(symbol, startTime, limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeBinanceError, err.Error())
		return
	}
	if fills == nil {
		fills = []binance.UserTrade{}
	}
	JSON(w, http.StatusOK, fills)
}

// HandlePerformance handles GET /api/v1/trade/performance
func (h *TradeHandler) HandlePerformance(w http.ResponseWriter, r *http.Request) {
	perf, err := h.store.GetPerformance()
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeQueryFailed, err.Error())
		return
	}

	JSON(w, http.StatusOK, perf)
}

// HandleJournalList handles GET /api/v1/trade/journal
func (h *TradeHandler) HandleJournalList(w http.ResponseWriter, r *http.Request) {
	limitStr := r.URL.Query().Get("limit")
	limit := 20
	if limitStr != "" {
		limit, _ = strconv.Atoi(limitStr)
	}
	entryType := r.URL.Query().Get("entry_type")
	tags := r.URL.Query().Get("tags")
	entries, err := h.store.QueryJournals(limit, entryType, tags)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeQueryFailed, err.Error())
		return
	}
	if entries == nil {
		entries = []store.JournalEntry{}
	}
	JSON(w, http.StatusOK, entries)
}
