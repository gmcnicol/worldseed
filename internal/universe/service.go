package universe

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type Service struct{ db *sql.DB }
type CreateInput struct {
	ID, Name, EntropyProfile string
	Seed                     int64
}

type Snapshot struct {
	Name             string   `json:"name"`
	EntropyProfile   string   `json:"entropy_profile"`
	UniverseAgeTicks int64    `json:"universe_age_ticks"`
	ArchiveIntegrity float64  `json:"archive_integrity"`
	RecentEvents     []string `json:"recent_events"`
}

func NewService(db *sql.DB) *Service { return &Service{db: db} }

func (s *Service) Create(ctx context.Context, in CreateInput) error {
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO universes (id, name, seed, entropy_profile, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`, in.ID, in.Name, in.Seed, in.EntropyProfile, now, now)
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	payload, _ := json.Marshal(map[string]any{"name": in.Name, "seed": in.Seed})
	if err := appendLog(ctx, tx, in.ID, "universe", in.ID, "universe.created", now, string(payload)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func appendLog(ctx context.Context, tx *sql.Tx, uid, at, aid, et, vt, payload string) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO event_log (universe_id, aggregate_type, aggregate_id, event_type, valid_time, recorded_time, payload_json) VALUES (?, ?, ?, ?, ?, ?, ?)`, uid, at, aid, et, vt, vt, payload)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO events (id, universe_id, event_type, valid_time, recorded_time, payload_json, certainty) VALUES (lower(hex(randomblob(16))), ?, ?, ?, ?, ?, ?)`, uid, et, vt, vt, payload, 0.8)
	return err
}

func (s *Service) Snapshot(ctx context.Context, universeID string) (Snapshot, error) {
	var out Snapshot
	if err := s.db.QueryRowContext(ctx, `SELECT name, entropy_profile, universe_age_ticks, archive_integrity FROM universes WHERE id=?`, universeID).Scan(&out.Name, &out.EntropyProfile, &out.UniverseAgeTicks, &out.ArchiveIntegrity); err != nil {
		return out, err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT payload_json FROM events WHERE universe_id=? ORDER BY recorded_time DESC LIMIT 6`, universeID)
	if err != nil {
		return out, err
	}
	defer rows.Close()
	for rows.Next() {
		var pj string
		_ = rows.Scan(&pj)
		var m map[string]any
		_ = json.Unmarshal([]byte(pj), &m)
		if t, ok := m["text"].(string); ok {
			out.RecentEvents = append(out.RecentEvents, t)
		}
	}
	return out, nil
}

func (s *Service) PreserveArchive(ctx context.Context, universeID string) error {
	now := time.Now().UTC()
	due := now.Add(30 * time.Second)
	tx, _ := s.db.BeginTx(ctx, nil)
	_, err := tx.ExecContext(ctx, `INSERT INTO interventions (id, universe_id, intervention_type, status, created_time, due_time, payload_json) VALUES (lower(hex(randomblob(16))), ?, 'preserve_archive', 'pending', ?, ?, '{}')`, universeID, now.Format(time.RFC3339), due.Format(time.RFC3339))
	if err != nil {
		_ = tx.Rollback()
		return err
	}
	_, _ = tx.ExecContext(ctx, `UPDATE universes SET archive_integrity = MIN(1.0, archive_integrity + 0.08), updated_at=? WHERE id=?`, now.Format(time.RFC3339), universeID)
	payload, _ := json.Marshal(map[string]any{"text": "Archive preservation lattice deployed."})
	if err := appendLog(ctx, tx, universeID, "intervention", universeID, "intervention.preserve_archive", now.Format(time.RFC3339), string(payload)); err != nil {
		_ = tx.Rollback()
		return err
	}
	return tx.Commit()
}

func (s *Service) ResolveInterventions(ctx context.Context, universeID string, now time.Time) error {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM interventions WHERE universe_id=? AND status='pending' AND due_time<=?`, universeID, now.UTC().Format(time.RFC3339))
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		_ = rows.Scan(&id)
		tx, _ := s.db.BeginTx(ctx, nil)
		_, _ = tx.ExecContext(ctx, `UPDATE interventions SET status='resolved' WHERE id=?`, id)
		payload, _ := json.Marshal(map[string]any{"text": "Preservation efforts unintentionally strengthened doctrinal rigidity."})
		_ = appendLog(ctx, tx, universeID, "intervention", id, "intervention.preserve_archive.consequence", now.UTC().Format(time.RFC3339), string(payload))
		_ = tx.Commit()
	}
	return nil
}

var _ = fmt.Sprintf
