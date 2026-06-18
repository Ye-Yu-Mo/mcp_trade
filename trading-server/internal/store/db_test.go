package store

import (
	"encoding/json"
	"os"
	"testing"
)

func TestNew_CreatesDatabase(t *testing.T) {
	path := "/tmp/test_m4_trade.duckdb"
	os.Remove(path)
	defer os.Remove(path)

	st, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer st.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("database file was not created")
	}
}

func TestInsertAndQueryTrade(t *testing.T) {
	path := "/tmp/test_m4_trade2.duckdb"
	os.Remove(path)
	defer os.Remove(path)

	st, _ := New(path)
	defer st.Close()

	snapshot, _ := json.Marshal(map[string]float64{"btc_price": 65000.0})
	id, err := st.InsertTrade("BTCUSDT", "BUY", 0.002, 65000.0, "12345", "trend breakout", string(snapshot))
	if err != nil {
		t.Fatalf("InsertTrade: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	records, err := st.QueryTrades("BTCUSDT", 10)
	if err != nil {
		t.Fatalf("QueryTrades: %v", err)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if records[0].EntryReason != "trend breakout" {
		t.Errorf("entry_reason = %q", records[0].EntryReason)
	}
}

func TestUpdateTradePnL(t *testing.T) {
	path := "/tmp/test_m4_pnl.duckdb"
	os.Remove(path)
	defer os.Remove(path)

	st, _ := New(path)
	defer st.Close()

	st.InsertTrade("ETHUSDT", "SELL", 0.1, 3200.0, "order_abc", "", "{}")
	err := st.UpdateTradePnL("order_abc", 15.50)
	if err != nil {
		t.Fatalf("UpdateTradePnL: %v", err)
	}

	records, _ := st.QueryTrades("", 10)
	if records[0].Status != "FILLED" {
		t.Errorf("status = %q, want FILLED", records[0].Status)
	}
	if records[0].PnL != 15.50 {
		t.Errorf("pnl = %f, want 15.50", records[0].PnL)
	}
}

func TestGetDailyPnL(t *testing.T) {
	path := "/tmp/test_m4_dailypnl.duckdb"
	os.Remove(path)
	defer os.Remove(path)

	st, _ := New(path)
	defer st.Close()

	st.InsertTrade("BTCUSDT", "BUY", 0.001, 65000.0, "o1", "", "{}")
	st.UpdateTradePnL("o1", 10.0)
	st.InsertTrade("BTCUSDT", "SELL", 0.001, 64000.0, "o2", "", "{}")
	st.UpdateTradePnL("o2", -5.0)

	pnl, err := st.GetDailyPnL()
	if err != nil {
		t.Fatalf("GetDailyPnL: %v", err)
	}
	if pnl != 5.0 {
		t.Errorf("daily pnl = %f, want 5.0", pnl)
	}
}

func TestInsertAndQueryJournal(t *testing.T) {
	path := "/tmp/test_m4_journal.duckdb"
	os.Remove(path)
	defer os.Remove(path)

	st, _ := New(path)
	defer st.Close()

	id, err := st.InsertJournal("REVIEW", "stopped out too early", "止损过紧,趋势交易", nil)
	if err != nil {
		t.Fatalf("InsertJournal: %v", err)
	}
	if id == 0 {
		t.Fatal("expected non-zero id")
	}

	entries, err := st.QueryJournals(10)
	if err != nil {
		t.Fatalf("QueryJournals: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Tags != "止损过紧,趋势交易" {
		t.Errorf("tags = %q", entries[0].Tags)
	}
}

func TestGetPerformance(t *testing.T) {
	path := "/tmp/test_m4_perf.duckdb"
	os.Remove(path)
	defer os.Remove(path)

	st, _ := New(path)
	defer st.Close()

	// 3 wins, 2 losses
	st.InsertTrade("A", "BUY", 1, 100, "w1", "", "{}"); st.UpdateTradePnL("w1", 20.0)
	st.InsertTrade("A", "BUY", 1, 100, "w2", "", "{}"); st.UpdateTradePnL("w2", 30.0)
	st.InsertTrade("A", "BUY", 1, 100, "w3", "", "{}"); st.UpdateTradePnL("w3", 10.0)
	st.InsertTrade("A", "BUY", 1, 100, "l1", "", "{}"); st.UpdateTradePnL("l1", -15.0)
	st.InsertTrade("A", "BUY", 1, 100, "l2", "", "{}"); st.UpdateTradePnL("l2", -5.0)

	perf, err := st.GetPerformance()
	if err != nil {
		t.Fatalf("GetPerformance: %v", err)
	}
	if perf.TotalTrades != 5 {
		t.Errorf("total = %d, want 5", perf.TotalTrades)
	}
	if perf.WinTrades != 3 {
		t.Errorf("wins = %d, want 3", perf.WinTrades)
	}
	if perf.WinRate != 60.0 {
		t.Errorf("win_rate = %f, want 60.0", perf.WinRate)
	}
	if perf.TotalPnL != 40.0 {
		t.Errorf("total_pnl = %f, want 40.0", perf.TotalPnL)
	}
	if perf.ProfitFactor != 3.0 { // (20+30+10) / (15+5) = 60/20 = 3.0
		t.Errorf("profit_factor = %f, want 3.0", perf.ProfitFactor)
	}
}

func TestNew_AutoCreatesDirectory(t *testing.T) {
	path := "/tmp/mcp_trade_m4/subdir/test_auto.duckdb"
	os.RemoveAll("/tmp/mcp_trade_m4")
	defer os.RemoveAll("/tmp/mcp_trade_m4")

	st, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	st.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("database file was not created in auto-created directory")
	}
}
