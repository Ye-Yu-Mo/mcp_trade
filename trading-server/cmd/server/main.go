package main

import (
	"log"
	"net/http"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/api"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/config"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/store"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Initialize Binance client
	client := binance.NewClient(cfg.APIKey, cfg.APISecret, cfg.BaseURL)
	log.Printf("binance client initialized: %s", cfg.BaseURL)

	// Initialize SQLite store
	st, err := store.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()
	log.Printf("database initialized: %s", cfg.DBPath)

	// Setup HTTP router
	router := api.NewRouter(client, cfg.APIToken)

	addr := ":" + cfg.ServerPort
	log.Printf("trading server starting on %s (env=%s)", addr, cfg.BaseURL)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server: %v", err)
	}
}
