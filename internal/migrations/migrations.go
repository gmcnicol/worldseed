package migrations

import (
	"database/sql"
	"fmt"
)

type migration struct {
	id   int
	name string
	sql  string
}

var migrationSet = []migration{
	{
		id:   1,
		name: "phase1_core_schema",
		sql: `
CREATE TABLE IF NOT EXISTS schema_migrations (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS universes (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL UNIQUE,
  seed INTEGER NOT NULL,
  entropy_profile TEXT NOT NULL,
  universe_age_ticks INTEGER NOT NULL DEFAULT 0,
  archive_integrity REAL NOT NULL DEFAULT 0.70,
  created_at TEXT NOT NULL,
  updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS entities (
  id TEXT PRIMARY KEY,
  universe_id TEXT NOT NULL,
  kind TEXT NOT NULL,
  name TEXT NOT NULL,
  state_json TEXT NOT NULL,
  created_valid_time TEXT NOT NULL,
  archived_at TEXT,
  certainty REAL NOT NULL,
  FOREIGN KEY(universe_id) REFERENCES universes(id)
);

CREATE TABLE IF NOT EXISTS events (
  id TEXT PRIMARY KEY,
  universe_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  valid_time TEXT NOT NULL,
  recorded_time TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  certainty REAL NOT NULL,
  FOREIGN KEY(universe_id) REFERENCES universes(id)
);

CREATE TABLE IF NOT EXISTS facts (
  id TEXT PRIMARY KEY,
  universe_id TEXT NOT NULL,
  subject_type TEXT NOT NULL,
  subject_id TEXT NOT NULL,
  predicate TEXT NOT NULL,
  object_json TEXT NOT NULL,
  valid_time TEXT NOT NULL,
  recorded_time TEXT NOT NULL,
  FOREIGN KEY(universe_id) REFERENCES universes(id)
);

CREATE TABLE IF NOT EXISTS interventions (
  id TEXT PRIMARY KEY,
  universe_id TEXT NOT NULL,
  intervention_type TEXT NOT NULL,
  status TEXT NOT NULL,
  created_time TEXT NOT NULL,
  due_time TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  FOREIGN KEY(universe_id) REFERENCES universes(id)
);

CREATE TABLE IF NOT EXISTS signals (
  id TEXT PRIMARY KEY,
  universe_id TEXT NOT NULL,
  signal_type TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  observed_time TEXT NOT NULL,
  FOREIGN KEY(universe_id) REFERENCES universes(id)
);

CREATE TABLE IF NOT EXISTS event_log (
  seq INTEGER PRIMARY KEY AUTOINCREMENT,
  universe_id TEXT NOT NULL,
  aggregate_type TEXT NOT NULL,
  aggregate_id TEXT NOT NULL,
  event_type TEXT NOT NULL,
  valid_time TEXT NOT NULL,
  recorded_time TEXT NOT NULL,
  payload_json TEXT NOT NULL,
  FOREIGN KEY(universe_id) REFERENCES universes(id)
);
`,
	},
}

func Apply(db *sql.DB) error { /* unchanged */
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS schema_migrations (
  id INTEGER PRIMARY KEY,
  name TEXT NOT NULL,
  applied_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);`); err != nil {
		return fmt.Errorf("ensure schema_migrations: %w", err)
	}
	for _, m := range migrationSet {
		var exists int
		err := db.QueryRow("SELECT 1 FROM schema_migrations WHERE id = ?", m.id).Scan(&exists)
		if err == nil {
			continue
		}
		if err != sql.ErrNoRows {
			return fmt.Errorf("check migration %d: %w", m.id, err)
		}
		tx, err := db.Begin()
		if err != nil {
			return fmt.Errorf("begin migration %d: %w", m.id, err)
		}
		if _, err := tx.Exec(m.sql); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("apply migration %d (%s): %w", m.id, m.name, err)
		}
		if _, err := tx.Exec(`INSERT INTO schema_migrations (id, name) VALUES (?, ?)`, m.id, m.name); err != nil {
			_ = tx.Rollback()
			return fmt.Errorf("record migration %d: %w", m.id, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("commit migration %d: %w", m.id, err)
		}
	}
	return nil
}
