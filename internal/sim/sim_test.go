package sim

import (
	"context"
	"testing"

	"github.com/gmcnicol/worldseed/internal/storage"
	"github.com/gmcnicol/worldseed/internal/timeline"
	"github.com/gmcnicol/worldseed/internal/universe"
)

func TestTickSeedDeterministic(t *testing.T) {
	first := TickSeed(42, 100)
	second := TickSeed(42, 100)
	if first != second {
		t.Fatalf("expected stable tick seed, got %d and %d", first, second)
	}
	if first == TickSeed(42, 101) {
		t.Fatalf("expected age to affect tick seed")
	}
}

func TestClampBounds(t *testing.T) {
	if got := clamp(-0.5); got != 0 {
		t.Fatalf("negative value should clamp to 0, got %f", got)
	}
	if got := clamp(1.5); got != 1 {
		t.Fatalf("high value should clamp to 1, got %f", got)
	}
	if got := clamp(0.42); got != 0.42 {
		t.Fatalf("in-range value changed to %f", got)
	}
}

func TestTicksAreDeterministicAcrossIdenticalUniverses(t *testing.T) {
	ctx := context.Background()
	left := createTestStore(t, "deterministic", 4242)
	right := createTestStore(t, "deterministic", 4242)

	leftEngine := NewEngine(left)
	rightEngine := NewEngine(right)
	for i := 0; i < 12; i++ {
		leftResult, err := leftEngine.Tick(ctx, 1)
		if err != nil {
			t.Fatalf("left tick %d: %v", i, err)
		}
		rightResult, err := rightEngine.Tick(ctx, 1)
		if err != nil {
			t.Fatalf("right tick %d: %v", i, err)
		}
		assertSameUniverseProjection(t, leftResult.Universe, rightResult.Universe)
		assertSameEvents(t, leftResult.Events, rightResult.Events)
	}

	leftCivilisations, err := left.AllCivilisations(ctx, "uni_deterministic")
	if err != nil {
		t.Fatal(err)
	}
	rightCivilisations, err := right.AllCivilisations(ctx, "uni_deterministic")
	if err != nil {
		t.Fatal(err)
	}
	if len(leftCivilisations) != len(rightCivilisations) {
		t.Fatalf("civilisation count mismatch: %d != %d", len(leftCivilisations), len(rightCivilisations))
	}
	for i := range leftCivilisations {
		if leftCivilisations[i].State != rightCivilisations[i].State {
			t.Fatalf("civilisation state mismatch at %d: %s != %s", i, leftCivilisations[i].State, rightCivilisations[i].State)
		}
	}
}

func TestPreserveArchiveHasDelayedConsequence(t *testing.T) {
	ctx := context.Background()
	store := createTestStore(t, "intervention", 99)
	engine := NewEngine(store)

	if err := RequestPreserveArchive(ctx, store); err != nil {
		t.Fatal(err)
	}

	var sawConsequence bool
	for i := 0; i < 4; i++ {
		result, err := engine.Tick(ctx, 1)
		if err != nil {
			t.Fatalf("tick %d: %v", i, err)
		}
		for _, event := range result.Events {
			if event.Kind == timeline.EventInterventionConsequence {
				sawConsequence = true
			}
		}
	}
	if !sawConsequence {
		t.Fatalf("expected delayed intervention consequence after due age")
	}

	pending, err := store.PendingInterventions(ctx, "uni_intervention", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Fatalf("expected intervention to resolve, got %d pending", len(pending))
	}

	active, err := store.ActiveCivilisations(ctx, "uni_intervention")
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 {
		t.Fatalf("expected one active civilisation, got %d", len(active))
	}
	state, err := storage.DecodeCivilisationState(active[0].State)
	if err != nil {
		t.Fatal(err)
	}
	if state.Doctrine < 0.40 {
		t.Fatalf("expected preservation consequence to strengthen doctrine, got %.3f", state.Doctrine)
	}
}

func createTestStore(t *testing.T, name string, seed int64) *storage.Store {
	t.Helper()
	path, err := universe.Create(context.Background(), universe.CreateOptions{
		DataDir: t.TempDir(),
		Name:    name,
		Seed:    seed,
	})
	if err != nil {
		t.Fatal(err)
	}
	store, err := storage.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	return store
}

func assertSameUniverseProjection(t *testing.T, left, right storage.Universe) {
	t.Helper()
	if left.ID != right.ID || left.Name != right.Name || left.Seed != right.Seed || left.Age != right.Age {
		t.Fatalf("universe identity mismatch: %#v != %#v", left, right)
	}
	if left.Entropy != right.Entropy {
		t.Fatalf("entropy mismatch: %.12f != %.12f", left.Entropy, right.Entropy)
	}
	if left.ArchiveIntegrity != right.ArchiveIntegrity {
		t.Fatalf("archive integrity mismatch: %.12f != %.12f", left.ArchiveIntegrity, right.ArchiveIntegrity)
	}
}

func assertSameEvents(t *testing.T, left, right []timeline.Event) {
	t.Helper()
	if len(left) != len(right) {
		t.Fatalf("event count mismatch: %d != %d", len(left), len(right))
	}
	for i := range left {
		if left[i].Kind != right[i].Kind || left[i].EntityID != right[i].EntityID || left[i].ValidTime != right[i].ValidTime || left[i].Summary != right[i].Summary {
			t.Fatalf("event mismatch at %d: %#v != %#v", i, left[i], right[i])
		}
	}
}
