CREATE TABLE IF NOT EXISTS cursors(
    chain TEXT PRIMARY KEY,
    position INTEGER NOT NULL,
    hash TEXT NOT NULL,
    finalized_position INTEGER,
    finalized_boundary INTEGER,
    updated_at INTEGER NOT NULL
);
CREATE TABLE IF NOT EXISTS backfill_cursors(
    chain TEXT PRIMARY KEY,
    position INTEGER NOT NULL,
    hash TEXT NOT NULL,
    updated_at INTEGER NOT NULL
);
