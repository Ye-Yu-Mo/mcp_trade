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

func NewRouter(client binance.Trader, apiToken string, riskMgr *risk.Manager, st *store.Store, cache *ws.MarketCache, alertStore *ws.AlertStore, startTime time.Time, marketStream *ws.MarketStream, userStream *ws.UserDataStream) *chi.Mux {
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
		JSON(w, http.StatusOK, map[string]interface{}{
			"status":      "ok",
			"ws_market":   marketStream.Connected(),
			"ws_userdata": userStream.Connected(),
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

		market := NewMarketHandler(client)
		market.cache = cache
		market.alertStore = alertStore
		account := NewAccountHandler(client)
		account.cache = cache
		order := NewOrderHandler(client, riskMgr, st)
		trade := NewTradeHandler(st)

		r.Route("/market", func(r chi.Router) {
			r.Get("/klines", market.HandleKlines)
			r.Get("/ticker", market.HandleTicker)
			r.Get("/orderbook", market.HandleOrderBook)
			r.Get("/scanner", market.HandleScanner)
			r.Get("/funding", market.HandleFunding)
			r.Get("/oi", market.HandleOI)
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
		})
	})

	return r
}
