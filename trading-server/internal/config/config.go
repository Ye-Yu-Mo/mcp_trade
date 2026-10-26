package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config holds all configuration for the Trading Server.
type Config struct {
	APIKey     string
	APISecret  string
	BaseURL    string
	ServerPort string
	BindAddr   string
	DBPath     string
	APIToken   string

	// Risk control
	MaxPositionPercent float64
	MaxStopLossPercent float64
	DailyLossLimit     float64

	// SetupAccount controls whether startup is allowed to change exchange
	// account settings (position mode, margin type, leverage). Default false:
	// the server must never mutate the account unless explicitly asked.
	SetupAccount bool

	// Paper trading
	PaperMode      bool
	MarketDataURL  string
	PaperBalance   float64
	PaperStatePath string

	// Risk v2 (fractions of equity)
	RiskPerTradePct float64
	MaxTotalRiskPct float64
	MaxPositions    int
	MaxLeverage     float64
	MaxNotionalPct  float64
	DailyLossPct    float64
	WeeklyLossPct   float64
}

// Load reads configuration from environment variables.
// Required: BINANCE_API_KEY, BINANCE_API_SECRET.
// Optional: TRADE_ENV (default "testnet"), SERVER_PORT (default "8080"), DB_PATH (default "data/trade.db").
func Load() (*Config, error) {
	apiKey := os.Getenv("BINANCE_API_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("BINANCE_API_KEY is required")
	}

	apiSecret := os.Getenv("BINANCE_API_SECRET")
	if apiSecret == "" {
		return nil, fmt.Errorf("BINANCE_API_SECRET is required")
	}

	env := os.Getenv("TRADE_ENV")
	if env == "" {
		env = "testnet"
	}
	paperMode := env == "paper"

	baseURL := "https://testnet.binancefuture.com"
	if env == "mainnet" || paperMode {
		baseURL = "https://fapi.binance.com"
	}

	// Market data is deliberately decoupled from the trading venue so paper
	// and testnet runs still see real mainnet prices.
	marketDataURL := os.Getenv("MARKET_DATA_URL")
	if marketDataURL == "" {
		marketDataURL = "https://fapi.binance.com"
	}

	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8877"
	}
	bindAddr := os.Getenv("BIND_ADDR")
	if bindAddr == "" {
		bindAddr = "127.0.0.1"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "data/trade.duckdb"
	}

	apiToken := os.Getenv("API_TOKEN")
	if apiToken == "" {
		return nil, fmt.Errorf("API_TOKEN is required")
	}

	maxPositionPercent := 0.1
	if v := os.Getenv("MAX_POSITION_PERCENT"); v != "" {
		maxPositionPercent, _ = strconv.ParseFloat(v, 64)
	}

	maxStopLossPercent := 0.02
	if v := os.Getenv("MAX_STOP_LOSS_PERCENT"); v != "" {
		maxStopLossPercent, _ = strconv.ParseFloat(v, 64)
	}

	dailyLossLimit := 100.0
	if v := os.Getenv("DAILY_LOSS_LIMIT"); v != "" {
		dailyLossLimit, _ = strconv.ParseFloat(v, 64)
	}

	setupAccount := os.Getenv("SETUP_ACCOUNT") == "true"

	paperBalance := 33.12
	if v := os.Getenv("PAPER_BALANCE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil && f > 0 {
			paperBalance = f
		}
	}
	paperStatePath := os.Getenv("PAPER_STATE_PATH")
	if paperStatePath == "" {
		paperStatePath = "data/paper_state.json"
	}

	riskPerTradePct := envFloat("RISK_PER_TRADE_PCT", 0.01)
	maxTotalRiskPct := envFloat("MAX_TOTAL_RISK_PCT", 0.03)
	maxPositions := envInt("MAX_POSITIONS", 2)
	maxLeverage := envFloat("MAX_LEVERAGE", 5)
	maxNotionalPct := envFloat("MAX_NOTIONAL_PCT", 1.5)
	dailyLossPct := envFloat("DAILY_LOSS_PCT", 0.03)
	weeklyLossPct := envFloat("WEEKLY_LOSS_PCT", 0.08)

	return &Config{
		APIKey:             apiKey,
		APISecret:          apiSecret,
		BaseURL:            baseURL,
		ServerPort:         serverPort,
		BindAddr:           bindAddr,
		DBPath:             dbPath,
		APIToken:           apiToken,
		MaxPositionPercent: maxPositionPercent,
		MaxStopLossPercent: maxStopLossPercent,
		DailyLossLimit:     dailyLossLimit,
		SetupAccount:       setupAccount,
		PaperMode:          paperMode,
		MarketDataURL:      marketDataURL,
		PaperBalance:       paperBalance,
		PaperStatePath:     paperStatePath,
		RiskPerTradePct:    riskPerTradePct,
		MaxTotalRiskPct:    maxTotalRiskPct,
		MaxPositions:       maxPositions,
		MaxLeverage:        maxLeverage,
		MaxNotionalPct:     maxNotionalPct,
		DailyLossPct:       dailyLossPct,
		WeeklyLossPct:      weeklyLossPct,
	}, nil
}

func envFloat(key string, fallback float64) float64 {
	if v := os.Getenv(key); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return fallback
}
