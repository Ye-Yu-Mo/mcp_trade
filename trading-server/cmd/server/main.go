package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/api"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/config"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/risk"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/store"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/ws"
)

var startTime = time.Now()

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("[init] config: %v", err)
	}

	client := binance.NewClient(cfg.APIKey, cfg.APISecret, cfg.BaseURL)
	log.Printf("[init] binance client: %s", cfg.BaseURL)

	// Account setup (non-fatal on mainnet where mode may already be set)
	if err := client.SetPositionMode(false); err != nil {
		log.Printf("[init] warn: position mode: %v", err)
	}
	for _, sym := range []string{"BTCUSDT", "ETHUSDT"} {
		client.SetMarginType(sym, "CROSSED")
		client.SetLeverage(sym, 1)
	}
	log.Println("[init] account setup done")

	riskMgr := risk.NewManager(risk.ManagerConfig{
		MaxPositionPercent: cfg.MaxPositionPercent,
		MaxStopLossPercent: cfg.MaxStopLossPercent,
		DailyLossLimit:     cfg.DailyLossLimit,
	})

	st, err := store.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("[init] store: %v", err)
	}
	defer st.Close()
	log.Printf("[init] database: %s", cfg.DBPath)

	cache := ws.NewMarketCache()
	alertStore := ws.NewAlertStore(cache)
	marketStream := ws.NewMarketStream(cfg.BaseURL, cache, []string{"BTCUSDT", "ETHUSDT"}, client)
	marketStream.Start()
	userStream := ws.NewUserDataStream(client, st, cache, cfg.BaseURL)
	userStream.Start()
	log.Println("[init] ws streams: market + userdata")

	router := api.NewRouter(client, cfg.APIToken, riskMgr, st, cache, alertStore, startTime, marketStream, userStream)

	srv := &http.Server{Addr: ":" + cfg.ServerPort, Handler: router}

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("[server] listening on :%s (%s)", cfg.ServerPort, cfg.BaseURL)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("[server] %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("[server] shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	marketStream.Stop()
	userStream.Stop()
	srv.Shutdown(shutdownCtx)
	st.Close()
	log.Println("[server] stopped")
}
