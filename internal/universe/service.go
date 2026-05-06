package universe

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

type Service struct {
	db *sql.DB
}

type CreateInput struct {
	ID             string
	Name           string
	Seed           int64
	EntropyProfile string
}

func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

func (s *Service) Create(ctx context.Context, in CreateInput) error {
	now := time.Now().UTC().Format(time.RFC3339)
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO universes (id, name, seed, entropy_profile, created_at) VALUES (?, ?, ?, ?, ?)`,
		in.ID, in.Name, in.Seed, in.EntropyProfile, now); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("insert universe: %w", err)
	}

	payload, _ := json.Marshal(map[string]any{
		"name": in.Name,
		"seed": in.Seed,
	})

	if _, err := tx.ExecContext(ctx, `INSERT INTO event_log (universe_id, aggregate_type, aggregate_id, event_type, valid_time, recorded_time, payload_json) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		in.ID, "universe", in.ID, "universe.created", now, now, string(payload)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("append event_log: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
