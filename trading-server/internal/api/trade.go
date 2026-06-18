package api

import (
	"net/http"
	"strconv"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/store"
)

// TradeHandler handles trade journal endpoints.
type TradeHandler struct {
	store *store.Store
}

// NewTradeHandler creates a TradeHandler.
func NewTradeHandler(st *store.Store) *TradeHandler {
	return &TradeHandler{store: st}
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
	entries, err := h.store.QueryJournals(limit)
	if err != nil {
		Error(w, http.StatusInternalServerError, CodeQueryFailed, err.Error())
		return
	}
	if entries == nil {
		entries = []store.JournalEntry{}
	}
	JSON(w, http.StatusOK, entries)
}
