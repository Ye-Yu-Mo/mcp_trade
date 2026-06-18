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
	DBPath     string
	APIToken   string

	// Risk control
	MaxPositionPercent float64
	MaxStopLossPercent float64
	DailyLossLimit     float64
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

	baseURL := "https://testnet.binancefuture.com"
	if env == "mainnet" {
		baseURL = "https://fapi.binance.com"
	}

	serverPort := os.Getenv("SERVER_PORT")
	if serverPort == "" {
		serverPort = "8877"
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

	return &Config{
		APIKey:             apiKey,
		APISecret:          apiSecret,
		BaseURL:            baseURL,
		ServerPort:         serverPort,
		DBPath:             dbPath,
		APIToken:           apiToken,
		MaxPositionPercent: maxPositionPercent,
		MaxStopLossPercent: maxStopLossPercent,
		DailyLossLimit:     dailyLossLimit,
	}, nil
}
