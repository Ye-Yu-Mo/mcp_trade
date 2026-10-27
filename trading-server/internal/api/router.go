package api

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/risk"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/store"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/ws"
)

func NewRouter(marketData binance.Trader, trader binance.Trader, apiToken string, riskMgr *risk.Manager, st *store.Store, cache *ws.MarketCache, alertStore *ws.AlertStore, startTime time.Time, marketStream *ws.MarketStream, userStream interface{ Connected() bool }, paperMode bool) *chi.Mux {
	r := chi.NewRouter()

	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		snap := cache.Snapshot()
		prices := snap["prices"].(map[string]float64)
		count := 0
		for k := range prices {
			if !strings.Contains(k, "_balance") {
				count++
			}
		}
		userConnected := false
		if userStream != nil {
			userConnected = userStream.Connected()
		}
		JSON(w, http.StatusOK, map[string]interface{}{
			"status":      "ok",
			"paper":       paperMode,
			"ws_market":   marketStream.Connected(),
			"ws_userdata": userConnected,
			"cache_items": count,
			"uptime":      fmt.Sprintf("%.0fs", time.Since(startTime).Seconds()),
		})
	})

	fs := http.FileServer(http.Dir("frontend-dist"))
	r.Handle("/assets/*", fs)
	r.Handle("/favicon.svg", fs)
	r.Handle("/icons.svg", fs)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "frontend-dist/index.html")
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(AuthMiddleware(apiToken))

		market := NewMarketHandler(marketData)
		market.cache = cache
		market.alertStore = alertStore
		account := NewAccountHandler(trader)
		account.cache = cache
		order := NewOrderHandler(trader, riskMgr, st, paperMode)
		trade := NewTradeHandler(st, trader, paperMode)

		r.Route("/market", func(r chi.Router) {
			r.Get("/klines", market.HandleKlines)
			r.Get("/ticker", market.HandleTicker)
			r.Get("/orderbook", market.HandleOrderBook)
			r.Get("/scanner", market.HandleScanner)
			r.Get("/funding", market.HandleFunding)
			r.Get("/oi", market.HandleOI)
			r.Get("/atr", market.HandleATR)
			r.Get("/candle_info", market.HandleCandleInfo)
			r.Get("/calendar", market.HandleCalendar)
			r.Post("/alert", market.HandleSetAlert)
			r.Get("/alerts", market.HandleListAlerts)
			r.Delete("/alert", market.HandleRemoveAlert)
			r.Get("/watch", market.HandleWatch)
		})
		r.Route("/account", func(r chi.Router) {
			r.Get("/balance", account.HandleBalance)
			r.Get("/positions", account.HandlePositions)
		})
		r.Route("/order", func(r chi.Router) {
			r.Post("/place", order.HandlePlaceOrder)
			r.Post("/close", order.HandleClose)
			r.Post("/modify_stop", order.HandleModifyStop)
			r.Delete("/cancel", order.HandleCancelOrder)
			r.Post("/oco", order.HandleOCOOrder)
			r.Delete("/oco", order.HandleCancelOCO)
			r.Get("/list", order.HandleListOrders)
			r.Get("/status", order.HandleOrderStatus)
		})
		r.Route("/trade", func(r chi.Router) {
			r.Get("/history", trade.HandleHistory)
			r.Post("/journal", trade.HandleJournal)
			r.Get("/journal", trade.HandleJournalList)
			r.Get("/performance", trade.HandlePerformance)
			r.Get("/exchange", trade.HandleExchangeIncome)
		})
	})

	return r
}
