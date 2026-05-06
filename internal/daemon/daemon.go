package daemon

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/worldseed/worldseed/internal/packets"
	"github.com/worldseed/worldseed/internal/sim"
	"github.com/worldseed/worldseed/internal/storage"
	"github.com/worldseed/worldseed/internal/universe"
)

type Config struct {
	RootDir, UniverseName string
	TickInterval          time.Duration
}

func Run(ctx context.Context, cfg Config) error {
	db, path, err := storage.OpenUniverseDB(cfg.RootDir, cfg.UniverseName)
	if err != nil {
		return err
	}
	defer db.Close()
	svc := universe.NewService(db)
	seed := universeSeed(db, cfg.UniverseName)
	engine := sim.New(seed)
	if cfg.TickInterval <= 0 {
		cfg.TickInterval = 5 * time.Second
	}
	log.Printf("worldseedd started universe=%s db=%s tick=%s", cfg.UniverseName, path, cfg.TickInterval)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() { defer wg.Done(); _ = packets.StartServer(ctx, cfg.RootDir, cfg.UniverseName, svc) }()

	ticker := time.NewTicker(cfg.TickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return nil
		case t := <-ticker.C:
			_ = tickOnce(ctx, db, cfg.UniverseName, t, engine, svc)
		}
	}
}

func universeSeed(db *sql.DB, id string) int64 {
	var s int64
	_ = db.QueryRow(`SELECT seed FROM universes WHERE id=?`, id).Scan(&s)
	return s
}

func tickOnce(ctx context.Context, db *sql.DB, uid string, t time.Time, engine *sim.Engine, svc *universe.Service) error {
	var st sim.State
	_ = db.QueryRowContext(ctx, `SELECT universe_age_ticks, archive_integrity FROM universes WHERE id=?`, uid).Scan(&st.UniverseAgeTicks, &st.ArchiveIntegrity)
	st.Civilisations = 1
	next, events := engine.Tick(st)
	tx, _ := db.BeginTx(ctx, nil)
	_, _ = tx.ExecContext(ctx, `UPDATE universes SET universe_age_ticks=?, archive_integrity=?, updated_at=? WHERE id=?`, next.UniverseAgeTicks, next.ArchiveIntegrity, t.UTC().Format(time.RFC3339), uid)
	for _, ev := range events {
		payload, _ := json.Marshal(map[string]any{"text": ev.Text})
		_, _ = tx.ExecContext(ctx, `INSERT INTO events (id, universe_id, event_type, valid_time, recorded_time, payload_json, certainty) VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?, ?, 0.74)`, uid, ev.Type, t.UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339), string(payload))
		_, _ = tx.ExecContext(ctx, `INSERT INTO event_log (universe_id, aggregate_type, aggregate_id, event_type, valid_time, recorded_time, payload_json) VALUES (?, 'timeline', ?, ?, ?, ?, ?)`, uid, uid, ev.Type, t.UTC().Format(time.RFC3339), time.Now().UTC().Format(time.RFC3339), string(payload))
	}
	_ = tx.Commit()
	return svc.ResolveInterventions(ctx, uid, t)
}
