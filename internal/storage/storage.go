package storage

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gmcnicol/worldseed/internal/timeline"
	_ "modernc.org/sqlite"
)

const zeroEventChecksum = "0000000000000000000000000000000000000000000000000000000000000000"

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Store struct {
	db *sql.DB
}

type Universe struct {
	ID               string
	Name             string
	Seed             int64
	CreatedAt        time.Time
	Age              int64
	Entropy          float64
	ArchiveIntegrity float64
}

type Entity struct {
	ID         string
	UniverseID string
	Kind       string
	Name       string
	State      string
	ValidFrom  int64
	RecordedAt time.Time
}

type CivilisationState struct {
	Status    string  `json:"status"`
	Stability float64 `json:"stability"`
	Doctrine  float64 `json:"doctrine"`
}

type Intervention struct {
	ID             int64
	UniverseID     string
	Kind           string
	RequestedAtAge int64
	DueAtAge       int64
	ResolvedAtAge  sql.NullInt64
	RecordedAt     time.Time
	Payload        string
}

type ClientKey struct {
	ID          int64
	UniverseID  string
	Username    string
	Fingerprint string
	PublicKey   string
	FirstSeenAt time.Time
	LastSeenAt  time.Time
	RemoteAddr  string
}

type DashboardState struct {
	Universe            Universe
	ActiveCivilisations []Entity
	RecentEvents        []timeline.Event
	SignalCount         int
	Uptime              time.Duration
}

func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_txlock=immediate")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db}
	if err := store.Migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	entries, err := migrationFiles.ReadDir("migrations")
	if err != nil {
		return err
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Name() < entries[j].Name() })
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".sql") {
			continue
		}
		body, err := migrationFiles.ReadFile("migrations/" + entry.Name())
		if err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, string(body)); err != nil {
			return fmt.Errorf("apply migration %s: %w", entry.Name(), err)
		}
	}
	return s.ensureEventLedger(ctx)
}

func (s *Store) ensureEventLedger(ctx context.Context) error {
	if err := s.ensureEventColumn(ctx, "previous_checksum", "TEXT NOT NULL DEFAULT '"+zeroEventChecksum+"'"); err != nil {
		return err
	}
	if err := s.ensureEventColumn(ctx, "checksum", "TEXT NOT NULL DEFAULT '"+zeroEventChecksum+"'"); err != nil {
		return err
	}
	needsBackfill, err := s.eventChecksumBackfillNeeded(ctx)
	if err != nil {
		return err
	}
	if needsBackfill {
		if _, err := s.db.ExecContext(ctx, `DROP TRIGGER IF EXISTS events_immutable_update`); err != nil {
			return err
		}
		if _, err := s.db.ExecContext(ctx, `DROP TRIGGER IF EXISTS events_immutable_delete`); err != nil {
			return err
		}
		if err := s.backfillEventChecksums(ctx); err != nil {
			return err
		}
	}
	if _, err := s.db.ExecContext(ctx, `CREATE TRIGGER IF NOT EXISTS events_immutable_update
BEFORE UPDATE ON events
BEGIN
	SELECT RAISE(ABORT, 'timeline events are immutable');
END`); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `CREATE TRIGGER IF NOT EXISTS events_immutable_delete
BEFORE DELETE ON events
BEGIN
	SELECT RAISE(ABORT, 'timeline events are immutable');
END`)
	return err
}

func (s *Store) eventChecksumBackfillNeeded(ctx context.Context) (bool, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM events WHERE checksum = ? OR checksum = ''`, zeroEventChecksum).Scan(&count)
	return count > 0, err
}

func (s *Store) ensureEventColumn(ctx context.Context, name, definition string) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(events)`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var columnName string
		var columnType string
		var notNull int
		var defaultValue sql.NullString
		var primaryKey int
		if err := rows.Scan(&cid, &columnName, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			return err
		}
		if columnName == name {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, fmt.Sprintf("ALTER TABLE events ADD COLUMN %s %s", name, definition))
	return err
}

func (s *Store) backfillEventChecksums(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)

	rows, err := tx.QueryContext(ctx, `SELECT id, universe_id, kind, COALESCE(entity_id, ''), valid_time, recorded_at, payload, summary, previous_checksum, checksum FROM events ORDER BY id`)
	if err != nil {
		return err
	}
	defer rows.Close()

	var events []timeline.Event
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return err
		}
		events = append(events, event)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if err := rows.Close(); err != nil {
		return err
	}

	previousByUniverse := map[string]string{}
	for _, event := range events {
		previous := previousByUniverse[event.UniverseID]
		if previous == "" {
			previous = zeroEventChecksum
		}
		event.PreviousChecksum = previous
		event.Checksum = eventChecksum(event)
		if _, err := tx.ExecContext(ctx, `UPDATE events SET previous_checksum = ?, checksum = ? WHERE id = ?`, event.PreviousChecksum, event.Checksum, event.ID); err != nil {
			return err
		}
		previousByUniverse[event.UniverseID] = event.Checksum
	}
	return tx.Commit()
}

func (s *Store) CreateUniverse(ctx context.Context, u Universe, firstEntity Entity, event timeline.Event) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)

	_, err = tx.ExecContext(ctx, `INSERT INTO universes (id, name, seed, created_at, age, entropy, archive_integrity) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Name, u.Seed, u.CreatedAt.UTC().Format(time.RFC3339Nano), u.Age, u.Entropy, u.ArchiveIntegrity)
	if err != nil {
		return err
	}
	if err := insertEntity(ctx, tx, firstEntity); err != nil {
		return err
	}
	if _, err := insertEvent(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) LoadUniverse(ctx context.Context) (Universe, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, seed, created_at, age, entropy, archive_integrity FROM universes LIMIT 1`)
	return scanUniverse(row)
}

func (s *Store) UpdateUniverse(ctx context.Context, u Universe) error {
	_, err := s.db.ExecContext(ctx, `UPDATE universes SET age = ?, entropy = ?, archive_integrity = ? WHERE id = ?`,
		u.Age, u.Entropy, u.ArchiveIntegrity, u.ID)
	return err
}

func (s *Store) AppendEvent(ctx context.Context, event timeline.Event) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer rollback(tx)
	id, err := insertEvent(ctx, tx, event)
	if err != nil {
		return 0, err
	}
	return id, tx.Commit()
}

func (s *Store) ApplyTick(ctx context.Context, u Universe, entities []Entity, events []timeline.Event, resolved map[int64]int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)
	_, err = tx.ExecContext(ctx, `UPDATE universes SET age = ?, entropy = ?, archive_integrity = ? WHERE id = ?`,
		u.Age, u.Entropy, u.ArchiveIntegrity, u.ID)
	if err != nil {
		return err
	}
	for _, entity := range entities {
		if err := upsertEntity(ctx, tx, entity); err != nil {
			return err
		}
	}
	for _, event := range events {
		if _, err := insertEvent(ctx, tx, event); err != nil {
			return err
		}
	}
	for id, age := range resolved {
		_, err := tx.ExecContext(ctx, `UPDATE interventions SET resolved_at_age = ? WHERE id = ?`, age, id)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ActiveCivilisations(ctx context.Context, universeID string) ([]Entity, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, universe_id, kind, name, state, valid_from, recorded_at FROM entities WHERE universe_id = ? AND kind = 'civilisation' ORDER BY name`, universeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entities []Entity
	for rows.Next() {
		entity, err := scanEntity(rows)
		if err != nil {
			return nil, err
		}
		state, err := DecodeCivilisationState(entity.State)
		if err != nil {
			return nil, err
		}
		if state.Status == "active" {
			entities = append(entities, entity)
		}
	}
	return entities, rows.Err()
}

func (s *Store) AllCivilisations(ctx context.Context, universeID string) ([]Entity, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, universe_id, kind, name, state, valid_from, recorded_at FROM entities WHERE universe_id = ? AND kind = 'civilisation' ORDER BY valid_from, name`, universeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entities []Entity
	for rows.Next() {
		entity, err := scanEntity(rows)
		if err != nil {
			return nil, err
		}
		entities = append(entities, entity)
	}
	return entities, rows.Err()
}

func (s *Store) RecentEvents(ctx context.Context, universeID string, limit int) ([]timeline.Event, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, universe_id, kind, COALESCE(entity_id, ''), valid_time, recorded_at, payload, summary, previous_checksum, checksum FROM events WHERE universe_id = ? ORDER BY valid_time DESC, id DESC LIMIT ?`, universeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var events []timeline.Event
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) PendingInterventions(ctx context.Context, universeID string, age int64) ([]Intervention, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, universe_id, kind, requested_at_age, due_at_age, resolved_at_age, recorded_at, payload FROM interventions WHERE universe_id = ? AND resolved_at_age IS NULL AND due_at_age <= ? ORDER BY due_at_age, id`, universeID, age)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var interventions []Intervention
	for rows.Next() {
		intervention, err := scanIntervention(rows)
		if err != nil {
			return nil, err
		}
		interventions = append(interventions, intervention)
	}
	return interventions, rows.Err()
}

func (s *Store) RequestIntervention(ctx context.Context, universeID, kind string, requestedAge, dueAge int64, payload string, event timeline.Event) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer rollback(tx)
	_, err = tx.ExecContext(ctx, `INSERT INTO interventions (universe_id, kind, requested_at_age, due_at_age, recorded_at, payload) VALUES (?, ?, ?, ?, ?, ?)`,
		universeID, kind, requestedAge, dueAge, time.Now().UTC().Format(time.RFC3339Nano), payload)
	if err != nil {
		return err
	}
	if _, err := insertEvent(ctx, tx, event); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) SignalCount(ctx context.Context, universeID string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM signals WHERE universe_id = ?`, universeID).Scan(&count)
	return count, err
}

func (s *Store) RecordClientKey(ctx context.Context, key ClientKey) error {
	if key.FirstSeenAt.IsZero() {
		key.FirstSeenAt = key.LastSeenAt
	}
	if key.LastSeenAt.IsZero() {
		key.LastSeenAt = key.FirstSeenAt
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO client_keys (universe_id, username, fingerprint, public_key, first_seen_at, last_seen_at, remote_addr)
VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(universe_id, fingerprint) DO UPDATE SET
	username = excluded.username,
	public_key = excluded.public_key,
	last_seen_at = excluded.last_seen_at,
	remote_addr = excluded.remote_addr`,
		key.UniverseID,
		key.Username,
		key.Fingerprint,
		key.PublicKey,
		key.FirstSeenAt.UTC().Format(time.RFC3339Nano),
		key.LastSeenAt.UTC().Format(time.RFC3339Nano),
		key.RemoteAddr)
	return err
}

func (s *Store) ClientKeys(ctx context.Context, universeID string) ([]ClientKey, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, universe_id, username, fingerprint, public_key, first_seen_at, last_seen_at, remote_addr FROM client_keys WHERE universe_id = ? ORDER BY last_seen_at DESC, id DESC`, universeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []ClientKey
	for rows.Next() {
		key, err := scanClientKey(rows)
		if err != nil {
			return nil, err
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func (s *Store) EventChain(ctx context.Context, universeID string) ([]timeline.Event, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, universe_id, kind, COALESCE(entity_id, ''), valid_time, recorded_at, payload, summary, previous_checksum, checksum FROM events WHERE universe_id = ? ORDER BY id`, universeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var events []timeline.Event
	for rows.Next() {
		event, err := scanEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) VerifyEventChain(ctx context.Context, universeID string) error {
	events, err := s.EventChain(ctx, universeID)
	if err != nil {
		return err
	}
	previous := zeroEventChecksum
	for _, event := range events {
		if event.PreviousChecksum != previous {
			return fmt.Errorf("event %d previous checksum mismatch", event.ID)
		}
		if checksum := eventChecksum(event); event.Checksum != checksum {
			return fmt.Errorf("event %d checksum mismatch", event.ID)
		}
		previous = event.Checksum
	}
	return nil
}

func EncodeCivilisationState(state CivilisationState) string {
	body, _ := json.Marshal(state)
	return string(body)
}

func DecodeCivilisationState(raw string) (CivilisationState, error) {
	var state CivilisationState
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return CivilisationState{}, err
	}
	return state, nil
}

func scanUniverse(row interface{ Scan(...any) error }) (Universe, error) {
	var u Universe
	var created string
	if err := row.Scan(&u.ID, &u.Name, &u.Seed, &created, &u.Age, &u.Entropy, &u.ArchiveIntegrity); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Universe{}, fmt.Errorf("universe not found")
		}
		return Universe{}, err
	}
	createdAt, err := time.Parse(time.RFC3339Nano, created)
	if err != nil {
		return Universe{}, err
	}
	u.CreatedAt = createdAt
	return u, nil
}

func scanEntity(row interface{ Scan(...any) error }) (Entity, error) {
	var entity Entity
	var recorded string
	err := row.Scan(&entity.ID, &entity.UniverseID, &entity.Kind, &entity.Name, &entity.State, &entity.ValidFrom, &recorded)
	if err != nil {
		return Entity{}, err
	}
	recordedAt, err := time.Parse(time.RFC3339Nano, recorded)
	if err != nil {
		return Entity{}, err
	}
	entity.RecordedAt = recordedAt
	return entity, nil
}

func scanEvent(row interface{ Scan(...any) error }) (timeline.Event, error) {
	var event timeline.Event
	var recorded string
	err := row.Scan(&event.ID, &event.UniverseID, &event.Kind, &event.EntityID, &event.ValidTime, &recorded, &event.Payload, &event.Summary, &event.PreviousChecksum, &event.Checksum)
	if err != nil {
		return timeline.Event{}, err
	}
	recordedAt, err := time.Parse(time.RFC3339Nano, recorded)
	if err != nil {
		return timeline.Event{}, err
	}
	event.RecordedAt = recordedAt
	return event, nil
}

func scanIntervention(row interface{ Scan(...any) error }) (Intervention, error) {
	var intervention Intervention
	var recorded string
	err := row.Scan(&intervention.ID, &intervention.UniverseID, &intervention.Kind, &intervention.RequestedAtAge, &intervention.DueAtAge, &intervention.ResolvedAtAge, &recorded, &intervention.Payload)
	if err != nil {
		return Intervention{}, err
	}
	recordedAt, err := time.Parse(time.RFC3339Nano, recorded)
	if err != nil {
		return Intervention{}, err
	}
	intervention.RecordedAt = recordedAt
	return intervention, nil
}

func scanClientKey(row interface{ Scan(...any) error }) (ClientKey, error) {
	var key ClientKey
	var firstSeen string
	var lastSeen string
	err := row.Scan(&key.ID, &key.UniverseID, &key.Username, &key.Fingerprint, &key.PublicKey, &firstSeen, &lastSeen, &key.RemoteAddr)
	if err != nil {
		return ClientKey{}, err
	}
	firstSeenAt, err := time.Parse(time.RFC3339Nano, firstSeen)
	if err != nil {
		return ClientKey{}, err
	}
	lastSeenAt, err := time.Parse(time.RFC3339Nano, lastSeen)
	if err != nil {
		return ClientKey{}, err
	}
	key.FirstSeenAt = firstSeenAt
	key.LastSeenAt = lastSeenAt
	return key, nil
}

func insertEntity(ctx context.Context, tx *sql.Tx, entity Entity) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO entities (id, universe_id, kind, name, state, valid_from, recorded_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entity.ID, entity.UniverseID, entity.Kind, entity.Name, entity.State, entity.ValidFrom, entity.RecordedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func upsertEntity(ctx context.Context, tx *sql.Tx, entity Entity) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO entities (id, universe_id, kind, name, state, valid_from, recorded_at) VALUES (?, ?, ?, ?, ?, ?, ?)
ON CONFLICT(id) DO UPDATE SET state = excluded.state, valid_from = excluded.valid_from, recorded_at = excluded.recorded_at`,
		entity.ID, entity.UniverseID, entity.Kind, entity.Name, entity.State, entity.ValidFrom, entity.RecordedAt.UTC().Format(time.RFC3339Nano))
	return err
}

func insertEvent(ctx context.Context, tx *sql.Tx, event timeline.Event) (int64, error) {
	var entityID any
	if event.EntityID != "" {
		entityID = event.EntityID
	}
	id, err := nextEventID(ctx, tx)
	if err != nil {
		return 0, err
	}
	previousChecksum, err := previousEventChecksum(ctx, tx, event.UniverseID)
	if err != nil {
		return 0, err
	}
	event.ID = id
	event.PreviousChecksum = previousChecksum
	event.Checksum = eventChecksum(event)
	_, err = tx.ExecContext(ctx, `INSERT INTO events (id, universe_id, kind, entity_id, valid_time, recorded_at, payload, summary, previous_checksum, checksum) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ID, event.UniverseID, event.Kind, entityID, event.ValidTime, event.RecordedAt.UTC().Format(time.RFC3339Nano), event.Payload, event.Summary, event.PreviousChecksum, event.Checksum)
	return id, err
}

func nextEventID(ctx context.Context, tx *sql.Tx) (int64, error) {
	var id int64
	err := tx.QueryRowContext(ctx, `SELECT MAX(value) + 1 FROM (
SELECT COALESCE(MAX(id), 0) AS value FROM events
UNION ALL
SELECT COALESCE(seq, 0) AS value FROM sqlite_sequence WHERE name = 'events'
)`).Scan(&id)
	return id, err
}

func previousEventChecksum(ctx context.Context, tx *sql.Tx, universeID string) (string, error) {
	var checksum string
	err := tx.QueryRowContext(ctx, `SELECT checksum FROM events WHERE universe_id = ? ORDER BY id DESC LIMIT 1`, universeID).Scan(&checksum)
	if errors.Is(err, sql.ErrNoRows) {
		return zeroEventChecksum, nil
	}
	return checksum, err
}

func eventChecksum(event timeline.Event) string {
	input := struct {
		ID               int64  `json:"id"`
		UniverseID       string `json:"universe_id"`
		Kind             string `json:"kind"`
		EntityID         string `json:"entity_id"`
		ValidTime        int64  `json:"valid_time"`
		RecordedAt       string `json:"recorded_at"`
		Payload          string `json:"payload"`
		Summary          string `json:"summary"`
		PreviousChecksum string `json:"previous_checksum"`
	}{
		ID:               event.ID,
		UniverseID:       event.UniverseID,
		Kind:             event.Kind,
		EntityID:         event.EntityID,
		ValidTime:        event.ValidTime,
		RecordedAt:       event.RecordedAt.UTC().Format(time.RFC3339Nano),
		Payload:          event.Payload,
		Summary:          event.Summary,
		PreviousChecksum: event.PreviousChecksum,
	}
	body, _ := json.Marshal(input)
	sum := sha256.Sum256(body)
	return hex.EncodeToString(sum[:])
}

func rollback(tx *sql.Tx) {
	_ = tx.Rollback()
}
