package store

import (
	"os"
	"testing"
)

func TestNew_CreatesDatabase(t *testing.T) {
	path := "/tmp/test_trade.duckdb"
	os.Remove(path) // clean up from previous run
	defer os.Remove(path)

	st, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer st.Close()

	// Verify file exists
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("database file was not created")
	}
}

func TestNew_CreatesSchema(t *testing.T) {
	path := "/tmp/test_schema.duckdb"
	os.Remove(path)
	defer os.Remove(path)

	st, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer st.Close()

	// Verify trades table exists by inserting and querying
	db := st.DB()

	// Get sequence next value first, then insert
	var tradeID int64
	err = db.QueryRow("SELECT nextval('seq_trade_id')").Scan(&tradeID)
	if err != nil {
		t.Fatalf("sequence query: %v", err)
	}

	_, err = db.Exec(`INSERT INTO trades (id, symbol, side, quantity, price) VALUES (?, 'BTCUSDT', 'BUY', 0.01, 65000.0)`, tradeID)
	if err != nil {
		t.Fatalf("insert trade: %v", err)
	}

	var symbol string
	var quantity float64
	err = db.QueryRow("SELECT symbol, quantity FROM trades WHERE id = ?", tradeID).Scan(&symbol, &quantity)
	if err != nil {
		t.Fatalf("query trade: %v", err)
	}
	if symbol != "BTCUSDT" {
		t.Errorf("symbol = %q, want %q", symbol, "BTCUSDT")
	}
	if quantity != 0.01 {
		t.Errorf("quantity = %f, want 0.01", quantity)
	}
}

func TestNew_CreatesJournalTable(t *testing.T) {
	path := "/tmp/test_journal.duckdb"
	os.Remove(path)
	defer os.Remove(path)

	st, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	defer st.Close()

	db := st.DB()

	var journalID int64
	err = db.QueryRow("SELECT nextval('seq_journal_id')").Scan(&journalID)
	if err != nil {
		t.Fatalf("sequence query: %v", err)
	}

	_, err = db.Exec(`INSERT INTO journal_entries (id, trade_id, entry_type, reason) VALUES (?, NULL, 'REVIEW', 'test entry')`, journalID)
	if err != nil {
		t.Fatalf("insert journal: %v", err)
	}

	var reason string
	err = db.QueryRow("SELECT reason FROM journal_entries WHERE id = ?", journalID).Scan(&reason)
	if err != nil {
		t.Fatalf("query journal: %v", err)
	}
	if reason != "test entry" {
		t.Errorf("reason = %q, want %q", reason, "test entry")
	}
}

func TestNew_AutoCreatesDirectory(t *testing.T) {
	path := "/tmp/mcp_trade_test/subdir/test_auto.duckdb"
	os.RemoveAll("/tmp/mcp_trade_test")
	defer os.RemoveAll("/tmp/mcp_trade_test")

	st, err := New(path)
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	st.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("database file was not created in auto-created directory")
	}
}
