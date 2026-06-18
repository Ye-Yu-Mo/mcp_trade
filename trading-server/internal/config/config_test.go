package config

import (
	"os"
	"testing"
)

func setupEnv(t *testing.T) {
	t.Helper()
	os.Setenv("BINANCE_API_KEY", "test_key")
	os.Setenv("BINANCE_API_SECRET", "test_secret")
	os.Setenv("API_TOKEN", "test_token")
	t.Cleanup(func() {
		os.Unsetenv("BINANCE_API_KEY")
		os.Unsetenv("BINANCE_API_SECRET")
		os.Unsetenv("API_TOKEN")
		os.Unsetenv("TRADE_ENV")
		os.Unsetenv("SERVER_PORT")
	})
}

func TestLoadConfig(t *testing.T) {
	setupEnv(t)

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
	if cfg.APIToken != "test_token" {
		t.Errorf("APIToken = %q, want %q", cfg.APIToken, "test_token")
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
	os.Setenv("API_TOKEN", "test_token")
	defer os.Unsetenv("BINANCE_API_SECRET")
	defer os.Unsetenv("API_TOKEN")
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
	os.Setenv("API_TOKEN", "test_token")
	defer os.Unsetenv("BINANCE_API_KEY")
	defer os.Unsetenv("API_TOKEN")
	os.Unsetenv("BINANCE_API_SECRET")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for missing API secret, got nil")
	}
	if err.Error() != "BINANCE_API_SECRET is required" {
		t.Errorf("error = %q, want %q", err.Error(), "BINANCE_API_SECRET is required")
	}
}

func TestLoadConfig_MissingAPIToken(t *testing.T) {
	os.Setenv("BINANCE_API_KEY", "test_key")
	os.Setenv("BINANCE_API_SECRET", "test_secret")
	defer os.Unsetenv("BINANCE_API_KEY")
	defer os.Unsetenv("BINANCE_API_SECRET")
	os.Unsetenv("API_TOKEN")

	_, err := Load()
	if err == nil {
		t.Fatal("Load() expected error for missing API token, got nil")
	}
	if err.Error() != "API_TOKEN is required" {
		t.Errorf("error = %q, want %q", err.Error(), "API_TOKEN is required")
	}
}

func TestLoadConfig_TestnetDefault(t *testing.T) {
	setupEnv(t)
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
	setupEnv(t)
	os.Setenv("TRADE_ENV", "mainnet")

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
	setupEnv(t)
	os.Setenv("SERVER_PORT", "9090")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() unexpected error: %v", err)
	}
	if cfg.ServerPort != "9090" {
		t.Errorf("ServerPort = %q, want %q", cfg.ServerPort, "9090")
	}
}
