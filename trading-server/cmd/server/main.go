package main

import (
	"log"
	"net/http"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/api"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/config"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/risk"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/store"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/ws"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	// Initialize Binance client
	client := binance.NewClient(cfg.APIKey, cfg.APISecret, cfg.BaseURL)
	log.Printf("binance client initialized: %s", cfg.BaseURL)

	// Setup safe account defaults: one-way mode, cross margin, 1x leverage
	if err := client.SetPositionMode(false); err != nil {
		log.Printf("warn: set position mode (one-way): %v", err)
	}
	for _, sym := range []string{"BTCUSDT", "ETHUSDT"} {
		if err := client.SetMarginType(sym, "CROSSED"); err != nil {
			log.Printf("warn: set margin type %s: %v", sym, err)
		}
		if err := client.SetLeverage(sym, 1); err != nil {
			log.Printf("warn: set leverage %s: %v", sym, err)
		}
	}
	log.Println("account setup complete: one-way, cross margin, 1x leverage")

	// Initialize risk manager
	riskMgr := risk.NewManager(risk.ManagerConfig{
		MaxPositionPercent: cfg.MaxPositionPercent,
		MaxStopLossPercent: cfg.MaxStopLossPercent,
		DailyLossLimit:     cfg.DailyLossLimit,
	})

	// Initialize DuckDB store
	st, err := store.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer st.Close()
	log.Printf("database initialized: %s", cfg.DBPath)

	// Initialize market data cache and WebSocket streams
	cache := ws.NewMarketCache()

	marketStream := ws.NewMarketStream(cfg.BaseURL, cache, []string{"BTCUSDT", "ETHUSDT"})
	marketStream.Start()
	defer marketStream.Stop()

	userStream := ws.NewUserDataStream(client, st, cache, cfg.BaseURL)
	userStream.Start()
	defer userStream.Stop()

	log.Println("ws streams started: market + userdata")

	// Setup HTTP router with cache
	router := api.NewRouter(client, cfg.APIToken, riskMgr, st, cache)

	addr := ":" + cfg.ServerPort
	log.Printf("trading server starting on %s (env=%s)", addr, cfg.BaseURL)
	if err := http.ListenAndServe(addr, router); err != nil {
		log.Fatalf("server: %v", err)
	}
}
