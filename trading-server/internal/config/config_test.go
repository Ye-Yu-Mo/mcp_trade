package config

import (
	"os"
	"testing"
)

func TestLoadConfig(t *testing.T) {
	// 设置环境变量
	os.Setenv("BINANCE_API_KEY", "test_key")
	os.Setenv("BINANCE_API_SECRET", "test_secret")
	defer os.Unsetenv("BINANCE_API_KEY")
	defer os.Unsetenv("BINANCE_API_SECRET")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.APIKey != "test_key" {
		t.Errorf("APIKey = %q, want %q", cfg.APIKey, "test_key")
	}
	if cfg.APISecret != "test_secret" {
		t.Errorf("APISecret = %q, want %q", cfg.APISecret, "test_secret")
	}
	if cfg.ServerPort != "8877" {
		t.Errorf("ServerPort = %q, want %q", cfg.ServerPort, "8877")
	}
	if cfg.DBPath != "data/trade.duckdb" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "data/trade.duckdb")
	}
}

func TestLoadConfig_MissingAPIKey(t *testing.T) {
	os.Setenv("BINANCE_API_SECRET", "test_secret")
	defer os.Unsetenv("BINANCE_API_SECRET")
	os.Unsetenv("BINANCE_API_KEY")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for missing API key, got nil")
	}
	if err.Error() != "BINANCE_API_KEY is required" {
		t.Errorf("error = %q, want %q", err.Error(), "BINANCE_API_KEY is required")
	}
}

func TestLoadConfig_MissingAPISecret(t *testing.T) {
	os.Setenv("BINANCE_API_KEY", "test_key")
	defer os.Unsetenv("BINANCE_API_KEY")
	os.Unsetenv("BINANCE_API_SECRET")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for missing API secret, got nil")
	}
	if err.Error() != "BINANCE_API_SECRET is required" {
		t.Errorf("error = %q, want %q", err.Error(), "BINANCE_API_SECRET is required")
	}
}

func TestLoadConfig_TestnetDefault(t *testing.T) {
	os.Setenv("BINANCE_API_KEY", "test_key")
	os.Setenv("BINANCE_API_SECRET", "test_secret")
	defer os.Unsetenv("BINANCE_API_KEY")
	defer os.Unsetenv("BINANCE_API_SECRET")

	// 不设 TRADE_ENV
	os.Unsetenv("TRADE_ENV")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	expected := "https://testnet.binancefuture.com"
	if cfg.BaseURL != expected {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, expected)
	}
}

func TestLoadConfig_Mainnet(t *testing.T) {
	os.Setenv("BINANCE_API_KEY", "test_key")
	os.Setenv("BINANCE_API_SECRET", "test_secret")
	os.Setenv("TRADE_ENV", "mainnet")
	defer os.Unsetenv("BINANCE_API_KEY")
	defer os.Unsetenv("BINANCE_API_SECRET")
	defer os.Unsetenv("TRADE_ENV")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	expected := "https://fapi.binance.com"
	if cfg.BaseURL != expected {
		t.Errorf("BaseURL = %q, want %q", cfg.BaseURL, expected)
	}
}

func TestLoadConfig_CustomPort(t *testing.T) {
	os.Setenv("BINANCE_API_KEY", "test_key")
	os.Setenv("BINANCE_API_SECRET", "test_secret")
	os.Setenv("SERVER_PORT", "9090")
	defer os.Unsetenv("BINANCE_API_KEY")
	defer os.Unsetenv("BINANCE_API_SECRET")
	defer os.Unsetenv("SERVER_PORT")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.ServerPort != "9090" {
		t.Errorf("ServerPort = %q, want %q", cfg.ServerPort, "9090")
	}
}
