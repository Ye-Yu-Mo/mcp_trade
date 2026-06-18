package config

import (
	"fmt"
	"os"
)

// Config holds all configuration for the Trading Server.
type Config struct {
	APIKey     string
	APISecret  string
	BaseURL    string
	ServerPort string
	DBPath     string
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

	return &Config{
		APIKey:     apiKey,
		APISecret:  apiSecret,
		BaseURL:    baseURL,
		ServerPort: serverPort,
		DBPath:     dbPath,
	}, nil
}
