package sim

import (
	"math/big"
	"testing"
)

func TestDeterministicTick(t *testing.T) {
	e1 := New(42)
	e2 := New(42)
	s1 := State{UniverseAgeTicks: big.NewInt(0), ArchiveIntegrity: 0.8, Civilisations: 1}
	s2 := State{UniverseAgeTicks: big.NewInt(0), ArchiveIntegrity: 0.8, Civilisations: 1}
	for i := 0; i < 5; i++ {
		var ev1, ev2 []Event
		s1, ev1 = e1.Tick(s1)
		s2, ev2 = e2.Tick(s2)
		if s1.UniverseAgeTicks.Cmp(s2.UniverseAgeTicks) != 0 || s1.ArchiveIntegrity != s2.ArchiveIntegrity || len(ev1) != len(ev2) {
			t.Fatalf("non deterministic")
		}
	}
}
