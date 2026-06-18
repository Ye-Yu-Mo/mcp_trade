package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/risk"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/store"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/ws"
)

// NewRouter creates and configures the chi router with all API routes.
func NewRouter(client binance.Trader, apiToken string, riskMgr *risk.Manager, st *store.Store, cache *ws.MarketCache) *chi.Mux {
	r := chi.NewRouter()

	// Public: health check
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Serve frontend static files (if built)
	fs := http.FileServer(http.Dir("frontend-dist"))
	r.Handle("/assets/*", fs)
	r.Handle("/favicon.svg", fs)
	r.Handle("/icons.svg", fs)
	r.Get("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "frontend-dist/index.html")
	})

	// Protected: all API routes require Bearer token
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(AuthMiddleware(apiToken))

		market := NewMarketHandler(client)
		market.cache = cache
		account := NewAccountHandler(client)
		account.cache = cache
		order := NewOrderHandler(client, riskMgr, st)
		trade := NewTradeHandler(st)

		r.Route("/market", func(r chi.Router) {
			r.Get("/klines", market.HandleKlines)
			r.Get("/ticker", market.HandleTicker)
			r.Get("/orderbook", market.HandleOrderBook)
			r.Get("/watch", market.HandleWatch)
		})
		r.Route("/account", func(r chi.Router) {
			r.Get("/balance", account.HandleBalance)
			r.Get("/positions", account.HandlePositions)
		})
		r.Route("/order", func(r chi.Router) {
			r.Post("/place", order.HandlePlaceOrder)
			r.Delete("/cancel", order.HandleCancelOrder)
			r.Get("/list", order.HandleListOrders)
			r.Get("/status", order.HandleOrderStatus)
		})
		r.Route("/trade", func(r chi.Router) {
			r.Get("/history", trade.HandleHistory)
			r.Post("/journal", trade.HandleJournal)
			r.Get("/performance", trade.HandlePerformance)
		})
	})

	return r
}
