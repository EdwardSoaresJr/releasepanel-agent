package logs

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"releasepanel/agent/internal/paths"
)

// Sink appends human-readable lines and optional JSONL events under logDir.
type Sink struct {
	logDir string
	mu     sync.Mutex
}

func Open(logDir string) (*Sink, error) {
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, err
	}
	return &Sink{logDir: logDir}, nil
}

func (s *Sink) Printf(format string, args ...any) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ts := time.Now().UTC().Format(time.RFC3339)
	msg := fmt.Sprintf(format, args...)
	line := ts + " " + msg + "\n"

	fmt.Print(line)

	path := paths.AgentLog(s.logDir)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(line)
}

func (s *Sink) AppendEvent(v any) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path := paths.EventsLog(s.logDir)
	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	enc := json.NewEncoder(f)
	return enc.Encode(v)
}
