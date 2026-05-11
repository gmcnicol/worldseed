CREATE TABLE IF NOT EXISTS universes (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL UNIQUE,
    seed INTEGER NOT NULL,
    created_at TEXT NOT NULL,
    age INTEGER NOT NULL DEFAULT 0,
    entropy REAL NOT NULL DEFAULT 0,
    archive_integrity REAL NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS entities (
    id TEXT PRIMARY KEY,
    universe_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    state TEXT NOT NULL,
    valid_from INTEGER NOT NULL,
    recorded_at TEXT NOT NULL,
    FOREIGN KEY (universe_id) REFERENCES universes(id)
);

CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    universe_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    entity_id TEXT,
    valid_time INTEGER NOT NULL,
    recorded_at TEXT NOT NULL,
    payload TEXT NOT NULL,
    summary TEXT NOT NULL,
    previous_checksum TEXT NOT NULL DEFAULT '0000000000000000000000000000000000000000000000000000000000000000',
    checksum TEXT NOT NULL DEFAULT '0000000000000000000000000000000000000000000000000000000000000000',
    FOREIGN KEY (universe_id) REFERENCES universes(id)
);

CREATE INDEX IF NOT EXISTS idx_events_universe_valid_time ON events(universe_id, valid_time, id);

CREATE TABLE IF NOT EXISTS facts (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    universe_id TEXT NOT NULL,
    entity_id TEXT,
    key TEXT NOT NULL,
    value TEXT NOT NULL,
    valid_from INTEGER NOT NULL,
    recorded_at TEXT NOT NULL,
    FOREIGN KEY (universe_id) REFERENCES universes(id)
);

CREATE TABLE IF NOT EXISTS interventions (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    universe_id TEXT NOT NULL,
    kind TEXT NOT NULL,
    requested_at_age INTEGER NOT NULL,
    due_at_age INTEGER NOT NULL,
    resolved_at_age INTEGER,
    recorded_at TEXT NOT NULL,
    payload TEXT NOT NULL,
    FOREIGN KEY (universe_id) REFERENCES universes(id)
);

CREATE INDEX IF NOT EXISTS idx_interventions_due ON interventions(universe_id, due_at_age, resolved_at_age);

CREATE TABLE IF NOT EXISTS signals (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    universe_id TEXT NOT NULL,
    received_at_age INTEGER NOT NULL,
    recorded_at TEXT NOT NULL,
    source TEXT NOT NULL,
    payload TEXT NOT NULL,
    FOREIGN KEY (universe_id) REFERENCES universes(id)
);

CREATE TABLE IF NOT EXISTS client_keys (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    universe_id TEXT NOT NULL,
    username TEXT NOT NULL,
    fingerprint TEXT NOT NULL,
    public_key TEXT NOT NULL,
    first_seen_at TEXT NOT NULL,
    last_seen_at TEXT NOT NULL,
    remote_addr TEXT NOT NULL,
    FOREIGN KEY (universe_id) REFERENCES universes(id),
    UNIQUE (universe_id, fingerprint)
);

CREATE INDEX IF NOT EXISTS idx_client_keys_universe ON client_keys(universe_id, last_seen_at);
