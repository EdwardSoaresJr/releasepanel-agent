package runtimeprobe

// Observation states for POST ping runtime_dependency_reports (convergence vocabulary).
const (
	StateObserved    = "observed"
	StateMissing     = "missing"
	StateUnreachable = "unreachable"
	StateFailed      = "failed"
)

// Spec is a normalized, explicit probe description (no discovery).
type Spec struct {
	Dependency string

	SocketPath string
	TCPAddr    string

	SystemctlArgv []string
	PidFile       string
}
