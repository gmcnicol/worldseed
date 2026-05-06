package sim

import (
	"math/big"
	"math/rand"
)

type State struct {
	UniverseAgeTicks *big.Int
	ArchiveIntegrity float64
	Entropy          float64
	Civilisations    int
}

type Event struct{ Type, Text string }

type Engine struct{ rng *rand.Rand }

func New(seed int64) *Engine { return &Engine{rng: rand.New(rand.NewSource(seed))} }

func (e *Engine) Tick(s State) (State, []Event) {
	if s.UniverseAgeTicks == nil {
		s.UniverseAgeTicks = big.NewInt(0)
	}
	s.UniverseAgeTicks = new(big.Int).Add(s.UniverseAgeTicks, big.NewInt(1))
	s.Entropy += 0.005 + e.rng.Float64()*0.002
	s.ArchiveIntegrity -= 0.002 + e.rng.Float64()*0.001
	if s.ArchiveIntegrity < 0 {
		s.ArchiveIntegrity = 0
	}
	events := []Event{}
	if e.rng.Float64() < 0.25 {
		events = append(events, Event{Type: "civilisation.fragmented", Text: "The Choir Republic fragmented after prolonged archive instability."})
	}
	if s.Civilisations == 0 {
		s.Civilisations = 1
	}
	return s, events
}
