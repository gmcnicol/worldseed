package sim

import (
	"context"
	"encoding/json"
	"fmt"
	"math/rand"
	"strings"
	"time"

	"github.com/gmcnicol/worldseed/internal/entropy"
	"github.com/gmcnicol/worldseed/internal/storage"
	"github.com/gmcnicol/worldseed/internal/timeline"
)

const (
	InterventionPreserveArchive = "preserve_archive"
)

type Engine struct {
	store *storage.Store
}

type TickResult struct {
	Universe storage.Universe
	Events   []timeline.Event
}

func NewEngine(store *storage.Store) *Engine {
	return &Engine{store: store}
}

func (e *Engine) Tick(ctx context.Context, step int64) (TickResult, error) {
	if step <= 0 {
		step = 1
	}
	u, err := e.store.LoadUniverse(ctx)
	if err != nil {
		return TickResult{}, err
	}
	u.Age += step

	r := rand.New(rand.NewSource(TickSeed(u.Seed, u.Age)))
	now := time.Now().UTC()
	entropyDelta := 0.002 + r.Float64()*0.009
	u.Entropy = entropy.Drift(u.Entropy, entropyDelta)
	u.ArchiveIntegrity = entropy.ArchiveIntegrity(u.ArchiveIntegrity, entropyDelta*0.55)

	entities, err := e.store.AllCivilisations(ctx, u.ID)
	if err != nil {
		return TickResult{}, err
	}
	pending, err := e.store.PendingInterventions(ctx, u.ID, u.Age)
	if err != nil {
		return TickResult{}, err
	}

	var changed []storage.Entity
	events := []timeline.Event{clockEvent(u, now)}
	resolved := make(map[int64]int64)
	for i := range entities {
		entity := entities[i]
		state, err := storage.DecodeCivilisationState(entity.State)
		if err != nil {
			return TickResult{}, err
		}
		if state.Status != "active" {
			continue
		}
		state.Stability = clamp(state.Stability - u.Entropy*0.015 + (r.Float64()-0.5)*0.025)
		if r.Float64() < u.Entropy*0.18 {
			state.Doctrine = clamp(state.Doctrine + 0.04 + r.Float64()*0.05)
		}
		entity.State = storage.EncodeCivilisationState(state)
		entity.ValidFrom = u.Age
		entity.RecordedAt = now
		changed = append(changed, entity)

		if ev, ok := civilisationEvent(u, entity, state, r, now); ok {
			events = append(events, ev)
			if ev.Kind == timeline.EventCivilisationCollapse {
				state.Status = "collapsed"
				entity.State = storage.EncodeCivilisationState(state)
				changed[len(changed)-1] = entity
			}
		}
	}

	for _, intervention := range pending {
		if intervention.Kind != InterventionPreserveArchive {
			continue
		}
		u.ArchiveIntegrity = clamp(u.ArchiveIntegrity + 0.08)
		events = append(events, consequenceEvent(u, intervention, now))
		resolved[intervention.ID] = u.Age
		changed = strengthenDoctrine(changed, entities, u.Age, now)
	}

	if len(events) == 1 && r.Float64() < 0.20 {
		events = append(events, atmosphericEvent(u, r, now))
	}
	if err := e.store.ApplyTick(ctx, u, changed, events, resolved); err != nil {
		return TickResult{}, err
	}
	return TickResult{Universe: u, Events: events}, nil
}

func RequestPreserveArchive(ctx context.Context, store *storage.Store) error {
	u, err := store.LoadUniverse(ctx)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	payload, _ := json.Marshal(map[string]any{
		"operator": "local",
		"method":   "preserve_archive",
	})
	event := timeline.Event{
		UniverseID: u.ID,
		Kind:       timeline.EventInterventionRequested,
		ValidTime:  u.Age,
		RecordedAt: now,
		Payload:    string(payload),
		Summary:    "An operator marked vulnerable archive strata for preservation.",
	}
	return store.RequestIntervention(ctx, u.ID, InterventionPreserveArchive, u.Age, u.Age+4, string(payload), event)
}

func TickSeed(seed, age int64) int64 {
	const golden = int64(0x9e3779b97f4a7c15 & 0x7fffffffffffffff)
	return seed ^ (age * golden)
}

func civilisationEvent(u storage.Universe, entity storage.Entity, state storage.CivilisationState, r *rand.Rand, now time.Time) (timeline.Event, bool) {
	collapseChance := (1 - state.Stability) * u.Entropy * 0.22
	splitChance := u.Entropy * state.Doctrine * 0.11
	mutationChance := (1 - u.ArchiveIntegrity) * 0.14
	roll := r.Float64()
	switch {
	case roll < collapseChance:
		return event(u, timeline.EventCivilisationCollapse, entity.ID, now, "%s collapsed after archive pressure made its founding records contradictory.", entity.Name), true
	case roll < collapseChance+splitChance:
		return event(u, timeline.EventCivilisationSplit, entity.ID, now, "%s fragmented after prolonged archive instability.", entity.Name), true
	case roll < collapseChance+splitChance+mutationChance:
		return event(u, timeline.EventCivilisationMutation, entity.ID, now, "%s altered its calendar rites to match a newly recovered stellar ledger.", entity.Name), true
	default:
		return timeline.Event{}, false
	}
}

func consequenceEvent(u storage.Universe, intervention storage.Intervention, now time.Time) timeline.Event {
	return event(u, timeline.EventInterventionConsequence, "", now, "Preservation efforts unintentionally strengthened doctrinal rigidity.")
}

func atmosphericEvent(u storage.Universe, r *rand.Rand, now time.Time) timeline.Event {
	lines := []string{
		"A cold census crossed the observatory lattice without declaring an origin.",
		"Archive sediment shifted; three minor histories now agree where they once diverged.",
		"Entropy instruments recorded a brief symmetry in the civilisational pressure field.",
		"Signal dust accumulated along the outer chronology and was filed without interpretation.",
	}
	return event(u, timeline.EventClockAdvanced, "", now, lines[r.Intn(len(lines))])
}

func clockEvent(u storage.Universe, now time.Time) timeline.Event {
	return event(u, timeline.EventClockAdvanced, "", now, "Archive chronometer advanced; entropy and integrity projections were sealed.")
}

func strengthenDoctrine(changed []storage.Entity, all []storage.Entity, age int64, now time.Time) []storage.Entity {
	seen := make(map[string]int, len(changed))
	for i, entity := range changed {
		seen[entity.ID] = i
	}
	for _, entity := range all {
		state, err := storage.DecodeCivilisationState(entity.State)
		if err != nil || state.Status != "active" {
			continue
		}
		state.Doctrine = clamp(state.Doctrine + 0.12)
		entity.State = storage.EncodeCivilisationState(state)
		entity.ValidFrom = age
		entity.RecordedAt = now
		if index, ok := seen[entity.ID]; ok {
			changed[index] = entity
		} else {
			changed = append(changed, entity)
		}
	}
	return changed
}

func event(u storage.Universe, kind, entityID string, now time.Time, format string, args ...any) timeline.Event {
	summary := fmt.Sprintf(format, args...)
	payload, _ := json.Marshal(map[string]any{
		"entropy":           round(u.Entropy),
		"archive_integrity": round(u.ArchiveIntegrity),
		"text":              summary,
	})
	return timeline.Event{
		UniverseID: u.ID,
		Kind:       kind,
		EntityID:   entityID,
		ValidTime:  u.Age,
		RecordedAt: now,
		Payload:    string(payload),
		Summary:    strings.TrimSpace(summary),
	}
}

func clamp(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func round(v float64) float64 {
	return float64(int(v*1000)) / 1000
}
