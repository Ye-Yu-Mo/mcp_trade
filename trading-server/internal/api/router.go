package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
)

// NewRouter creates and configures the chi router with all API routes.
func NewRouter(client binance.Trader, apiToken string) *chi.Mux {
	r := chi.NewRouter()

	// Public: health check
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		JSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Protected: all API routes require Bearer token
	r.Route("/api/v1", func(r chi.Router) {
		r.Use(AuthMiddleware(apiToken))

		market := NewMarketHandler(client)
		account := NewAccountHandler(client)

		r.Route("/market", func(r chi.Router) {
			r.Get("/klines", market.HandleKlines)
			r.Get("/ticker", market.HandleTicker)
			r.Get("/orderbook", market.HandleOrderBook)
		})
		r.Route("/account", func(r chi.Router) {
			r.Get("/balance", account.HandleBalance)
			r.Get("/positions", account.HandlePositions)
		})
	})

	return r
}
