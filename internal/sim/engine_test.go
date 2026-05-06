package sim

import "testing"

func TestDeterministicTick(t *testing.T) {
	e1 := New(42)
	e2 := New(42)
	s1 := State{ArchiveIntegrity: 0.8, Civilisations: 1}
	s2 := State{ArchiveIntegrity: 0.8, Civilisations: 1}
	for i := 0; i < 5; i++ {
		var ev1, ev2 []Event
		s1, ev1 = e1.Tick(s1)
		s2, ev2 = e2.Tick(s2)
		if s1 != s2 || len(ev1) != len(ev2) {
			t.Fatalf("non deterministic")
		}
	}
}
