package store

import (
	"database/sql"
	"os"
	"path/filepath"

	_ "github.com/marcboeker/go-duckdb"
)

// Store wraps the DuckDB database for trade journal persistence.
type Store struct {
	db *sql.DB
}

// New opens (or creates) the DuckDB database at the given path and runs migrations.
func New(dbPath string) (*Store, error) {
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("duckdb", dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Ping(); err != nil {
		return nil, err
	}

	if err := migrate(db); err != nil {
		return nil, err
	}

	return &Store{db: db}, nil
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying sql.DB for direct queries.
func (s *Store) DB() *sql.DB {
	return s.db
}

// migrate runs the schema DDL. DuckDB uses sequences for auto-increment IDs.
func migrate(db *sql.DB) error {
	_, err := db.Exec(schema)
	return err
}

const schema = `
CREATE SEQUENCE IF NOT EXISTS seq_trade_id;

CREATE TABLE IF NOT EXISTS trades (
    id          BIGINT PRIMARY KEY DEFAULT nextval('seq_trade_id'),
    symbol      TEXT NOT NULL,
    side        TEXT NOT NULL,
    quantity    DOUBLE NOT NULL,
    price       DOUBLE NOT NULL,
    order_id    TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'OPEN',
    created_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at  TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE SEQUENCE IF NOT EXISTS seq_journal_id;

CREATE TABLE IF NOT EXISTS journal_entries (
    id               BIGINT PRIMARY KEY DEFAULT nextval('seq_journal_id'),
    trade_id         BIGINT,
    entry_type       TEXT NOT NULL,
    reason           TEXT NOT NULL DEFAULT '',
    screenshot_url   TEXT NOT NULL DEFAULT '',
    market_snapshot  TEXT NOT NULL DEFAULT '{}',
    created_at       TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);
`
