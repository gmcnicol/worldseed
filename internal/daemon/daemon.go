package daemon

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/worldseed/worldseed/internal/storage"
)

type Config struct {
	RootDir      string
	UniverseName string
	TickInterval time.Duration
}

func Run(ctx context.Context, cfg Config) error {
	db, path, err := storage.OpenUniverseDB(cfg.RootDir, cfg.UniverseName)
	if err != nil {
		return err
	}
	defer db.Close()

	if cfg.TickInterval <= 0 {
		cfg.TickInterval = 5 * time.Second
	}

	log.Printf("worldseedd started universe=%s db=%s tick=%s", cfg.UniverseName, path, cfg.TickInterval)
	ticker := time.NewTicker(cfg.TickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case t := <-ticker.C:
			if _, err := db.Exec(`INSERT INTO timeline_events (id, universe_id, event_type, valid_time, recorded_time, payload_json, certainty)
VALUES (lower(hex(randomblob(16))), ?, 'universe.tick', ?, ?, '{"kind":"background-pressure"}', 0.8)`, cfg.UniverseName, t.UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339)); err != nil {
				return fmt.Errorf("write tick event: %w", err)
			}
		}
	}
}
