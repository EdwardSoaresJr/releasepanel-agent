package paths

import (
	"path/filepath"
)

func Enrollment(stateDir string) string {
	return filepath.Join(stateDir, "enrollment.json")
}

func RuntimeState(stateDir string) string {
	return filepath.Join(stateDir, "runtime.json")
}

func InventoryCache(stateDir string) string {
	return filepath.Join(stateDir, "inventory.json")
}

func ConvergenceState(stateDir string) string {
	return filepath.Join(stateDir, "convergence.json")
}

func LocksDir(stateDir string) string {
	return filepath.Join(stateDir, "locks")
}

func DeployStaging(stateDir string) string {
	return filepath.Join(stateDir, "deploy", "staging")
}

func DeployRuns(stateDir string) string {
	return filepath.Join(stateDir, "deploy", "runs")
}

func OutboxDir(stateDir string) string {
	return filepath.Join(stateDir, "outbox")
}

func AgentLog(logDir string) string {
	return filepath.Join(logDir, "agent.log")
}

func EventsLog(logDir string) string {
	return filepath.Join(logDir, "events.jsonl")
}
