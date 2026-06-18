package api

import (
	"github.com/go-chi/chi/v5"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
)

// NewRouter creates and configures the chi router with all API routes.
func NewRouter(client binance.Trader) *chi.Mux {
	r := chi.NewRouter()

	market := NewMarketHandler(client)
	account := NewAccountHandler(client)

	r.Route("/api/v1", func(r chi.Router) {
		r.Route("/market", func(r chi.Router) {
			r.Get("/klines", market.HandleKlines)
			r.Get("/ticker", market.HandleTicker)
		})
		r.Route("/account", func(r chi.Router) {
			r.Get("/balance", account.HandleBalance)
			r.Get("/positions", account.HandlePositions)
		})
	})

	return r
}
