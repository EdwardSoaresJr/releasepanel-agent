package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strings"
	"syscall"
	"time"

	"releasepanel/agent/internal/central"
	"releasepanel/agent/internal/config"
	"releasepanel/agent/internal/deploy"
	"releasepanel/agent/internal/enroll"
	"releasepanel/agent/internal/health"
	"releasepanel/agent/internal/inventory"
	"releasepanel/agent/internal/logs"
	"releasepanel/agent/internal/paths"
	"releasepanel/agent/internal/state"
	"releasepanel/agent/internal/version"
	"releasepanel/agent/pkg/api"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "enroll":
		cmdEnroll(os.Args[2:])
	case "run":
		cmdRun(os.Args[2:])
	case "inventory":
		cmdInventory(os.Args[2:])
	case "health":
		cmdHealth(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `releasepanel-agent — VPS runtime authority

Commands:
  enroll      Exchange enrollment token for persisted node credentials
  run         Long-running loop (heartbeat, inventory, health; optional manifest reconcile)
  inventory   Print one JSON inventory snapshot to stdout
  health      Print one JSON health snapshot to stdout

`)
}

func cmdEnroll(args []string) {
	fs := flag.NewFlagSet("enroll", flag.ExitOnError)
	cfgPath := fs.String("config", "", "path to config file (default $RELEASEPANEL_CONFIG or /etc/releasepanel-agent/config.yaml)")
	centralURL := fs.String("central-url", "", "override central_base_url from config for this enroll")
	tokenFile := fs.String("token-file", "", "path to enrollment token file")
	_ = fs.Parse(args)

	if strings.TrimSpace(*tokenFile) == "" {
		fmt.Fprintln(os.Stderr, "enroll: --token-file is required")
		os.Exit(2)
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	base := cfg.CentralBaseURL
	if strings.TrimSpace(*centralURL) != "" {
		base = strings.TrimRight(strings.TrimSpace(*centralURL), "/")
	}

	rawTok, err := os.ReadFile(*tokenFile)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	token := strings.TrimSpace(string(rawTok))

	hn, _ := os.Hostname()
	facts := api.NodeFacts{
		Hostname:     hn,
		OS:           runtime.GOOS,
		Arch:         runtime.GOARCH,
		AgentVersion: version.Version,
	}
	if inv, err := inventory.Collect(hn, version.Version); err == nil {
		facts = inv.Facts
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	if err := enroll.Run(ctx, cfg, base, token, facts); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println("enrollment persisted:", paths.Enrollment(cfg.StateDir))
}

func cmdRun(args []string) {
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	cfgPath := fs.String("config", "", "path to config file")
	_ = fs.Parse(args)

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	rec, err := enroll.ReadRecord(cfg.StateDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	sink, err := logs.Open(cfg.LogDir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := state.EnsureTree(0o755, cfg.StateDir, paths.LocksDir(cfg.StateDir), paths.DeployStaging(cfg.StateDir), paths.DeployRuns(cfg.StateDir), paths.OutboxDir(cfg.StateDir), cfg.LogDir); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	clBase, err := central.New(rec.CentralBaseURL, cfg.SkipTLSVerify)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cl := clBase.WithAuth(rec.NodeID, rec.APIKey)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	rtPath := paths.RuntimeState(cfg.StateDir)
	rt := loadRuntime(rtPath)
	rt.SchemaVersion = api.SchemaV1
	if rt.StartedAt.IsZero() {
		rt.StartedAt = time.Now().UTC()
	}

	conv := loadConvergence(paths.ConvergenceState(cfg.StateDir))

	runner := deploy.Runner{StateDir: cfg.StateDir}

	ticker := time.NewTicker(cfg.PollInterval())
	defer ticker.Stop()

	sink.Printf("agent %s starting loop interval=%s node=%s manifest_reconcile=%v", version.Version, cfg.PollInterval(), rec.NodeID, cfg.ManifestReconcileEnabled)

	for {
		rt.LoopCount++
		rt.LastLoopAt = time.Now().UTC()
		rt.LastError = ""

		if err := persistRuntime(rtPath, rt); err != nil {
			sink.Printf("persist runtime: %v", err)
		}

		hb := api.HeartbeatReport{
			SchemaVersion: api.SchemaV1,
			NodeID:        rec.NodeID,
			CollectedAt:   time.Now().UTC().Format(time.RFC3339),
			AgentVersion:  version.Version,
		}
		if err := cl.PostHeartbeat(ctx, hb); err != nil {
			rt.LastError = err.Error()
			sink.Printf("post heartbeat: %v", err)
		}

		inv, err := inventory.Collect("", version.Version)
		if err != nil {
			rt.LastError = err.Error()
			sink.Printf("inventory collect: %v", err)
			_ = persistRuntime(rtPath, rt)
			select {
			case <-ctx.Done():
				sink.Printf("shutdown: %v", ctx.Err())
				return
			case <-ticker.C:
			}
			continue
		}
		inv.NodeID = rec.NodeID

		if shouldInventory(rt, time.Hour) {
			if err := cl.PostInventory(ctx, inv); err != nil {
				rt.LastError = err.Error()
				sink.Printf("post inventory: %v", err)
			} else {
				rt.LastInventoryPostAt = time.Now().UTC()
				_ = state.WriteJSONAtomic(paths.InventoryCache(cfg.StateDir), inv, 0o644)
			}
		}

		hrep := health.Collect(rec.NodeID)
		if err := cl.PostHealth(ctx, hrep); err != nil {
			rt.LastError = err.Error()
			sink.Printf("post health: %v", err)
		} else {
			rt.LastHealthPostAt = time.Now().UTC()
		}

		if cfg.ManifestReconcileEnabled {
			next, err := runner.Reconcile(ctx, cl, conv, rec.NodeID)
			if err != nil {
				rt.LastError = err.Error()
				sink.Printf("reconcile: %v", err)
			}
			conv = next
		}

		if err := persistRuntime(rtPath, rt); err != nil {
			sink.Printf("persist runtime: %v", err)
		}

		_ = sink.AppendEvent(map[string]any{
			"ts":        time.Now().UTC().Format(time.RFC3339),
			"loop":      rt.LoopCount,
			"lastError": rt.LastError,
		})

		select {
		case <-ctx.Done():
			sink.Printf("shutdown: %v", ctx.Err())
			return
		case <-ticker.C:
		}
	}
}

func cmdInventory(args []string) {
	fs := flag.NewFlagSet("inventory", flag.ExitOnError)
	pretty := fs.Bool("pretty", false, "indent JSON")
	_ = fs.Parse(args)

	inv, err := inventory.Collect("", version.Version)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	var out []byte
	if *pretty {
		out, err = json.MarshalIndent(inv, "", "  ")
	} else {
		out, err = json.Marshal(inv)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}

func cmdHealth(args []string) {
	fs := flag.NewFlagSet("health", flag.ExitOnError)
	pretty := fs.Bool("pretty", false, "indent JSON")
	node := fs.String("node-id", "", "optional node id for report embedding")
	_ = fs.Parse(args)

	h := health.Collect(*node)
	var out []byte
	var err error
	if *pretty {
		out, err = json.MarshalIndent(h, "", "  ")
	} else {
		out, err = json.Marshal(h)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Println(string(out))
}

func loadRuntime(path string) state.RuntimeCounters {
	var rt state.RuntimeCounters
	if err := state.ReadJSON(path, &rt); err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "read runtime state: %v\n", err)
		}
	}
	return rt
}

func persistRuntime(path string, rt state.RuntimeCounters) error {
	rt.SchemaVersion = api.SchemaV1
	return state.WriteJSONAtomic(path, rt, 0o644)
}

func loadConvergence(path string) *state.ConvergenceRecord {
	var c state.ConvergenceRecord
	if err := state.ReadJSON(path, &c); err != nil {
		return &state.ConvergenceRecord{SchemaVersion: api.SchemaV1, UpdatedAt: time.Now().UTC()}
	}
	if c.SchemaVersion == 0 {
		c.SchemaVersion = api.SchemaV1
	}
	return &c
}

func shouldInventory(rt state.RuntimeCounters, every time.Duration) bool {
	if rt.LastInventoryPostAt.IsZero() {
		return true
	}
	return time.Since(rt.LastInventoryPostAt) >= every
}
