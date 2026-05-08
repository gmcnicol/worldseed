package timeline

import "time"

const (
	EventUniverseCreated         = "universe.created"
	EventClockAdvanced           = "clock.advanced"
	EventCivilisationRise        = "civilisation.rise"
	EventCivilisationSplit       = "civilisation.split"
	EventCivilisationCollapse    = "civilisation.collapse"
	EventCivilisationMutation    = "civilisation.mutation"
	EventInterventionRequested   = "intervention.requested"
	EventInterventionConsequence = "intervention.consequence"
)

type Event struct {
	ID         int64     `json:"id"`
	UniverseID string    `json:"universe_id"`
	Kind       string    `json:"kind"`
	EntityID   string    `json:"entity_id,omitempty"`
	ValidTime  int64     `json:"valid_time"`
	RecordedAt time.Time `json:"recorded_at"`
	Payload    string    `json:"payload"`
	Summary    string    `json:"summary"`
}
