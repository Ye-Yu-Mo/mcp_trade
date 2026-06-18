-- trades: 交易记录
CREATE TABLE IF NOT EXISTS trades (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol      TEXT NOT NULL,
    side        TEXT NOT NULL CHECK(side IN ('BUY', 'SELL')),
    quantity    REAL NOT NULL,
    price       REAL NOT NULL,
    order_id    TEXT NOT NULL DEFAULT '',
    status      TEXT NOT NULL DEFAULT 'OPEN',
    created_at  TEXT NOT NULL DEFAULT (datetime('now')),
    updated_at  TEXT NOT NULL DEFAULT (datetime('now'))
);

-- journal_entries: 交易日志/经验记录
CREATE TABLE IF NOT EXISTS journal_entries (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    trade_id         INTEGER REFERENCES trades(id),
    entry_type       TEXT NOT NULL CHECK(entry_type IN ('ENTRY', 'EXIT', 'REVIEW')),
    reason           TEXT NOT NULL DEFAULT '',
    screenshot_url   TEXT NOT NULL DEFAULT '',
    market_snapshot  TEXT NOT NULL DEFAULT '{}',
    created_at       TEXT NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_trades_symbol ON trades(symbol);
CREATE INDEX IF NOT EXISTS idx_trades_created ON trades(created_at);
CREATE INDEX IF NOT EXISTS idx_journal_trade ON journal_entries(trade_id);
