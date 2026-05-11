package storage

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/gmcnicol/worldseed/internal/timeline"
)

func TestEventsAreHashChainedAndImmutable(t *testing.T) {
	ctx := context.Background()
	store, err := Open(t.TempDir() + "/universe.sqlite")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()

	now := time.Date(2026, 5, 8, 13, 0, 0, 0, time.UTC)
	u := Universe{
		ID:               "universe-1",
		Name:             "ledger",
		Seed:             1,
		CreatedAt:        now,
		Age:              0,
		Entropy:          0.1,
		ArchiveIntegrity: 1,
	}
	entity := Entity{
		ID:         "entity-1",
		UniverseID: u.ID,
		Kind:       "civilisation",
		Name:       "Ledger State",
		State:      EncodeCivilisationState(CivilisationState{Status: "active"}),
		ValidFrom:  0,
		RecordedAt: now,
	}
	first := timeline.Event{
		UniverseID: u.ID,
		Kind:       timeline.EventUniverseCreated,
		EntityID:   entity.ID,
		ValidTime:  0,
		RecordedAt: now,
		Payload:    "{}",
		Summary:    "Ledger opened.",
	}
	if err := store.CreateUniverse(ctx, u, entity, first); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AppendEvent(ctx, timeline.Event{
		UniverseID: u.ID,
		Kind:       timeline.EventClockAdvanced,
		ValidTime:  1,
		RecordedAt: now.Add(time.Second),
		Payload:    "{}",
		Summary:    "Clock advanced.",
	}); err != nil {
		t.Fatal(err)
	}

	events, err := store.EventChain(ctx, u.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("expected two events, got %d", len(events))
	}
	if events[0].PreviousChecksum != zeroEventChecksum {
		t.Fatalf("expected genesis previous checksum, got %s", events[0].PreviousChecksum)
	}
	if len(events[0].Checksum) != 64 || events[0].Checksum == zeroEventChecksum {
		t.Fatalf("expected first event checksum, got %s", events[0].Checksum)
	}
	if events[1].PreviousChecksum != events[0].Checksum {
		t.Fatalf("expected second event to link to first checksum")
	}
	if err := store.VerifyEventChain(ctx, u.ID); err != nil {
		t.Fatal(err)
	}

	if _, err := store.db.ExecContext(ctx, `UPDATE events SET summary = 'tampered' WHERE id = ?`, events[0].ID); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutable update error, got %v", err)
	}
	if _, err := store.db.ExecContext(ctx, `DELETE FROM events WHERE id = ?`, events[0].ID); err == nil || !strings.Contains(err.Error(), "immutable") {
		t.Fatalf("expected immutable delete error, got %v", err)
	}
}
