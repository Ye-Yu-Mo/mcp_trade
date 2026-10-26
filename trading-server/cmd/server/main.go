package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/api"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/binance"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/config"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/paper"
	"github.com/ye-yu-mo/mcp-trade/trading-server/internal/reconcile"
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

	// Market data is always read from mainnet, even when trading on testnet
	// or in paper mode, so simulated/validated prices match the real market.
	marketClient := binance.NewClient(cfg.APIKey, cfg.APISecret, cfg.MarketDataURL)
	log.Printf("[init] market data: %s", cfg.MarketDataURL)

	st, err := store.New(cfg.DBPath)
	if err != nil {
		log.Fatalf("[init] store: %v", err)
	}
	defer st.Close()
	log.Printf("[init] database: %s", cfg.DBPath)

	cache := ws.NewMarketCache()

	var trader binance.Trader
	var paperBroker *paper.Broker
	var userStream interface{ Connected() bool }
	var userData *ws.UserDataStream
	var reconcileDone chan struct{}

	if cfg.PaperMode {
		paperBroker, err = paper.New(marketClient, st, cfg.PaperStatePath, cfg.PaperBalance)
		if err != nil {
			log.Fatalf("[init] paper broker: %v", err)
		}
		paperBroker.Start()
		trader = paperBroker
		log.Printf("[init] PAPER mode: initial %.2f USDT, state %s", cfg.PaperBalance, cfg.PaperStatePath)
	} else {
		tradeClient := binance.NewClient(cfg.APIKey, cfg.APISecret, cfg.BaseURL)
		log.Printf("[init] trading venue: %s", cfg.BaseURL)

		// Account setup is opt-in only. Changing position mode / margin type /
		// leverage on startup can silently alter an existing live account.
		if cfg.SetupAccount {
			if err := tradeClient.SetPositionMode(false); err != nil {
				log.Printf("[init] warn: position mode: %v", err)
			}
			for _, sym := range []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"} {
				if err := tradeClient.SetMarginType(sym, "ISOLATED"); err != nil {
					log.Printf("[init] warn: margin type %s: %v", sym, err)
				}
				if err := tradeClient.SetLeverage(sym, int(cfg.MaxLeverage)); err != nil {
					log.Printf("[init] warn: leverage %s: %v", sym, err)
				}
			}
			log.Println("[init] account setup done")
		} else {
			log.Println("[init] account setup skipped (SETUP_ACCOUNT != true)")
		}
		dual, modeErr := tradeClient.GetPositionMode()
		if modeErr != nil {
			log.Printf("[init] warn: cannot read position mode: %v", modeErr)
		} else if dual {
			log.Fatalf("[init] hedge (dual-side) mode is active; this build requires one-way mode. " +
				"Switch in Binance or run once with SETUP_ACCOUNT=true.")
		} else {
			log.Println("[init] position mode: one-way")
		}

		// Close trade rows that the exchange already closed while the server
		// was offline, then keep reconciling in case the user-data stream
		// misses a stop/TP event.
		if n, rerr := reconcile.Once(tradeClient, st); rerr != nil {
			log.Printf("[init] warn: initial reconcile: %v", rerr)
		} else if n > 0 {
			log.Printf("[init] reconciled %d stale OPEN trade(s)", n)
		}
		reconcileDone = make(chan struct{})
		go func() {
			ticker := time.NewTicker(15 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-reconcileDone:
					return
				case <-ticker.C:
					if n, err := reconcile.Once(tradeClient, st); n > 0 || err != nil {
						if err != nil {
							log.Printf("reconcile: %v", err)
						} else {
							log.Printf("reconcile: closed %d trade(s)", n)
						}
					}
				}
			}
		}()
		trader = tradeClient

		// Reconcile trade rows with the exchange so a close that happened while
		// the server was offline does not leave a permanent OPEN trade (and a
		// permanently consumed open-risk budget).
		if positions, err := tradeClient.GetPositions(); err == nil {
			active := make(map[string]bool, len(positions))
			for _, p := range positions {
				if p.Quantity > 0 {
					active[p.Symbol] = true
				}
			}
			n, rerr := st.ReconcileMissingOpenTrades(active)
			if rerr != nil {
				log.Printf("[init] warn: reconcile trades: %v", rerr)
			} else if n > 0 {
				log.Printf("[init] reconciled %d stale OPEN trade(s) as CLOSED", n)
			}
		} else {
			log.Printf("[init] warn: cannot reconcile trades (positions unavailable): %v", err)
		}

		userData = ws.NewUserDataStream(tradeClient, st, cache, cfg.BaseURL)
		userData.Start()
		userStream = userData
	}

	riskMgr := risk.NewManager(risk.ManagerConfig{
		RiskPerTradePct: cfg.RiskPerTradePct,
		MaxTotalRiskPct: cfg.MaxTotalRiskPct,
		MaxPositions:    cfg.MaxPositions,
		MaxLeverage:     cfg.MaxLeverage,
		MaxNotionalPct:  cfg.MaxNotionalPct,
		DailyLossPct:    cfg.DailyLossPct,
		WeeklyLossPct:   cfg.WeeklyLossPct,
	})

	// Exchange filters come from the exchange, not a hardcoded table; they are
	// required for correct rounding and minimum notional checks on altcoins.
	var exchange map[string]binance.SymbolInfo
	for attempt := 1; attempt <= 3; attempt++ {
		exchange, err = marketClient.GetExchangeInfo()
		if err == nil {
			break
		}
		log.Printf("[init] exchangeInfo attempt %d failed: %v", attempt, err)
		time.Sleep(2 * time.Second)
	}
	if err != nil {
		log.Fatalf("[init] exchangeInfo unavailable; refusing to start without symbol filters: %v", err)
	}
	symbols := make(map[string]risk.SymbolInfo, len(exchange))
	for sym, info := range exchange {
		symbols[sym] = risk.SymbolInfo{
			Status:      info.Status,
			TickSize:    info.TickSize,
			StepSize:    info.StepSize,
			MinNotional: info.MinNotional,
		}
	}
	riskMgr.SetSymbolInfo(symbols)
	log.Printf("[init] loaded %d symbol filters", len(symbols))

	alertStore := ws.NewAlertStore(cache, st, trader)
	marketStream := ws.NewMarketStream(cfg.MarketDataURL, cache, []string{"BTCUSDT", "ETHUSDT"}, marketClient)
	marketStream.Start()
	log.Println("[init] ws streams started")

	router := api.NewRouter(
		marketClient, trader, cfg.APIToken, riskMgr, st, cache, alertStore,
		startTime, marketStream, userStream, cfg.PaperMode,
	)

	srv := &http.Server{Addr: net.JoinHostPort(cfg.BindAddr, cfg.ServerPort), Handler: router}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Printf("[server] listening on %s:%s (paper=%v)", cfg.BindAddr, cfg.ServerPort, cfg.PaperMode)
		if err := srv.ListenAndServe(); err != http.ErrServerClosed {
			log.Fatalf("[server] %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("[server] shutting down...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	marketStream.Stop()
	if userData != nil {
		userData.Stop()
	}
	if reconcileDone != nil {
		close(reconcileDone)
	}
	if paperBroker != nil {
		paperBroker.Stop()
	}
	srv.Shutdown(shutdownCtx)
	st.Close()
	log.Println("[server] stopped")
}
