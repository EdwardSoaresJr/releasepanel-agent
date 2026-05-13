package runtimeprobe

import (
	"context"
	"net"
	"path/filepath"
	"testing"
)

func TestProbeTCPObserved(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	st, reason := probeTCP(context.Background(), ln.Addr().String())
	if st != StateObserved || reason != "" {
		t.Fatalf("got %q %q want observed", st, reason)
	}
}

func TestProbeTCPUnreachable(t *testing.T) {
	t.Parallel()
	st, reason := probeTCP(context.Background(), "127.0.0.1:1")
	if st != StateUnreachable || reason == "" {
		t.Fatalf("got %q %q want unreachable with reason", st, reason)
	}
}

func TestRunAllUnixMissing(t *testing.T) {
	t.Parallel()
	specs := []Spec{
		{Dependency: "php_fpm", SocketPath: filepath.Join(t.TempDir(), "nonexistent.sock")},
	}
	ctx := context.Background()
	rows := RunAll(ctx, specs)
	if len(rows) != 1 {
		t.Fatalf("len=%d", len(rows))
	}
	if rows[0].State != StateMissing {
		t.Fatalf("state=%s", rows[0].State)
	}
}

func TestRunOneTCPFromSpec(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	ctx := context.Background()
	row := runOne(ctx, Spec{Dependency: "redis", TCPAddr: ln.Addr().String()})
	if row.State != StateObserved || row.FailureReason != "" {
		t.Fatalf("%+v", row)
	}
}
