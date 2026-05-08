package packets

type Command string

const (
	CommandDashboard Command = "dashboard"
)

type InterventionRequest struct {
	Kind string `json:"kind"`
}
