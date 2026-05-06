package sim

import (
	"math/rand"
)

type State struct {
	UniverseAgeTicks int64
	ArchiveIntegrity float64
	Entropy          float64
	Civilisations    int
}

type Event struct{ Type, Text string }

type Engine struct{ rng *rand.Rand }

func New(seed int64) *Engine { return &Engine{rng: rand.New(rand.NewSource(seed))} }

func (e *Engine) Tick(s State) (State, []Event) {
	s.UniverseAgeTicks++
	s.Entropy += 0.005 + e.rng.Float64()*0.002
	s.ArchiveIntegrity -= 0.002 + e.rng.Float64()*0.001
	if s.ArchiveIntegrity < 0 {
		s.ArchiveIntegrity = 0
	}
	events := []Event{}
	if e.rng.Float64() < 0.25 {
		txt := "The Choir Republic fragmented after prolonged archive instability."
		events = append(events, Event{Type: "civilisation.fragmented", Text: txt})
	}
	if s.Civilisations == 0 {
		s.Civilisations = 1
	}
	return s, events
}
