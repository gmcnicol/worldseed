package storage

import (
	"database/sql"
	"fmt"
	"path/filepath"

	_ "github.com/mattn/go-sqlite3"
)

// OpenUniverseDB opens or creates the SQLite database for a universe and enables sensible defaults.
func OpenUniverseDB(rootDir, universeName string) (*sql.DB, string, error) {
	dbPath := filepath.Join(rootDir, universeName, "universe.sqlite")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, "", fmt.Errorf("open sqlite: %w", err)
	}

	if _, err := db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
		_ = db.Close()
		return nil, "", fmt.Errorf("configure sqlite pragmas: %w", err)
	}

	return db, dbPath, nil
}
