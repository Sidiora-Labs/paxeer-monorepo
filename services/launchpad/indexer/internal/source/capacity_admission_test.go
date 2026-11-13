package source

import (
	"bufio"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestCapacityRunRejectsUnavailableInitialStateAndCapture(t *testing.T) {
	ctx := context.Background()
	node := newCapacityChain(t)
	live := repair997Open(t, fopts(node.server.URL))
	dsn, pool := canonicaltest.New(t)
	explorer := capacityExplorer(t, dsn, pool, live, 100)
	st := store.New(pool)
	fence, err := st.AcquireLease(ctx, 125, "capacity-admission", "capacity", 30*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks VALUES(125,'capacity-admission',100,100)`); err != nil {
		t.Fatal(err)
	}
	workload := &canonicalCapacityWorkload{source: live, store: st, fence: fence, stream: "capacity-admission"}
	providers := CapacityProviders{Live: live, Historical: &HistoricalSource{RPC: live, Explorer: explorer}, Head: live, Processor: workload, Pool: pool}
	base := capacityBoundOptions(t, live)
	base.StreamID = "capacity-admission"
	base.Duration = 300 * time.Millisecond
	base.Profile = "harness"
	base.TargetEventsPerSecond = 1
	base.MaxHeapBytes = 1 << 30
	base.MaxCaptureBytes = 16 << 20
	base.HistoricalFrom = 1
	base.HistoricalTo = 90
	base.ParityFrom = 1
	base.ParityTo = 10000
	base.MutationApproved = true
	base.Workload = capacityOutputCommand(t, `{"Kind":"event","Value":{"chainId":125}}`)
	for _, tc := range []struct{ name, want string }{
		{"head mismatch", "capacity-initial-cursor"}, {"directory collision", "not a directory"}, {"capture exists", "file exists"}, {"binding revision", "capacity-revision"}, {"capture byte budget", "capacity-input"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := base
			opts.OutputDirectory = t.TempDir()
			switch tc.name {
			case "head mismatch":
				node.head.Store(99)
				defer node.head.Store(100)
			case "directory collision":
				path := filepath.Join(opts.OutputDirectory, "occupied")
				if err := os.WriteFile(path, []byte("retained"), 0600); err != nil {
					t.Fatal(err)
				}
				opts.OutputDirectory = filepath.Join(path, "capture")
			case "capture exists":
				if err := os.WriteFile(filepath.Join(opts.OutputDirectory, "source-capacity.raw.jsonl"), []byte("retained"), 0600); err != nil {
					t.Fatal(err)
				}
			case "binding revision":
				opts.Binding.Revision = "invalid"
			case "capture byte budget":
				opts.MaxCaptureBytes = 1
			}
			report, err := RunSourceCapacity(ctx, opts, providers)
			if err == nil || !strings.Contains(err.Error(), tc.want) || report.HarnessPassed || report.CapacityTargetQualified {
				t.Fatalf("capacity admission: %v (passed=%v qualified=%v)", err, report.HarnessPassed, report.CapacityTargetQualified)
			}
		})
	}
	closedPool, err := pgxpool.NewWithConfig(ctx, pool.Config())
	if err != nil {
		t.Fatal(err)
	}
	closedPool.Close()
	providers.Pool = closedPool
	base.OutputDirectory = t.TempDir()
	if report, err := RunSourceCapacity(ctx, base, providers); err == nil || !strings.Contains(err.Error(), "capacity-commit-time") || report.HarnessPassed {
		t.Fatalf("unavailable commit clock %+v: %v", report, err)
	}
}

func TestCapacityWorkloadBoundsActiveProcessAndFaultDelivery(t *testing.T) {
	shell, err := exec.LookPath("sh")
	if err != nil {
		t.Fatal(err)
	}
	shell, err = filepath.EvalSymlinks(shell)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"active process", "blocked fault", "bounded record"} {
		t.Run(mode, func(t *testing.T) {
			capture, err := newCapacityCapture(filepath.Join(t.TempDir(), "capture.jsonl"), 1<<20)
			if err != nil {
				t.Fatal(err)
			}
			var command CapacityCommand
			var faults chan CapacityFaultObservation
			ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
			defer cancel()
			switch mode {
			case "active process":
				command = CapacityCommand{Executable: shell, SHA256: capacityFileDigest(t, shell), Args: []string{"-c", "sleep 10"}}
			case "blocked fault":
				command = capacityOutputCommand(t, `{"Kind":"fault","Fault":{"Kind":"throttle","SourceID":"primary","State":"injected","ObservedAt":"2026-01-01T00:00:00Z"}}`)
				faults = make(chan CapacityFaultObservation)
			case "bounded record":
				command = CapacityCommand{Executable: shell, SHA256: capacityFileDigest(t, shell), Args: []string{"-c", `head -c 2097153 /dev/zero | tr '\000' x`}}
			}
			err = capacityWorkload(ctx, command, capture, faults)
			if mode == "bounded record" {
				if err == nil || !errors.Is(err, bufio.ErrTooLong) {
					t.Fatalf("unbounded process record: %v", err)
				}
			} else if mode == "blocked fault" {
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("blocked fault shutdown: %v", err)
				}
			} else {
				var se *SourceError
				if !errors.As(err, &se) || se.Kind != IncompleteError {
					t.Fatalf("active workload shutdown: %v", err)
				}
			}
			if _, _, err = capture.finish(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCapacityBindingRequiresCompleteSourceInventory(t *testing.T) {
	live := repair997Open(t, fopts(fixture(t, "0x7d").URL))
	opts := capacityBoundOptions(t, live)
	providers := CapacityProviders{Live: live, Historical: &HistoricalSource{RPC: live}}
	for _, mode := range []string{"missing source", "missing module", "unavailable source", "source directory"} {
		t.Run(mode, func(t *testing.T) {
			changed := opts
			changed.Binding.Files = append([]CapacityBoundFile(nil), opts.Binding.Files...)
			switch mode {
			case "missing source":
				changed.Binding.Files = changed.Binding.Files[1:]
			case "missing module":
				for i, file := range changed.Binding.Files {
					if file.Path == "services/launchpad/indexer/go.sum" {
						changed.Binding.Files = append(changed.Binding.Files[:i], changed.Binding.Files[i+1:]...)
						break
					}
				}
			case "unavailable source":
				changed.Binding.Files[0].Path = "services/launchpad/indexer/unavailable.go"
			case "source directory":
				changed.Binding.Files[0].Path = "services/launchpad/indexer/internal/source"
			}
			if err := verifyCapacityBinding(context.Background(), changed, providers); err == nil {
				t.Fatal("incomplete source inventory admitted")
			}
		})
	}
}

func TestLagRecoveryRejectsUnavailableObservationAndCapture(t *testing.T) {
	ctx := context.Background()
	node := newCapacityChain(t)
	live := repair997Open(t, fopts(node.server.URL))
	_, pool := canonicaltest.New(t)
	p := CapacityProviders{Live: live, Historical: &HistoricalSource{RPC: live}, Pool: pool}
	base := capacityBoundOptions(t, live)
	base.StreamID = "lag-admission"
	base.Duration = 95 * time.Second
	for _, mode := range []string{"binding", "directory", "capture", "stale head", "watermark", "canceled"} {
		t.Run(mode, func(t *testing.T) {
			opts := base
			opts.OutputDirectory = t.TempDir()
			live.mu.Lock()
			live.authoritative.ObservedAt = time.Now().UTC()
			live.mu.Unlock()
			switch mode {
			case "binding":
				opts.Binding.Revision = "invalid"
			case "directory":
				occupied := filepath.Join(opts.OutputDirectory, "occupied")
				if err := os.WriteFile(occupied, []byte("retained"), 0600); err != nil {
					t.Fatal(err)
				}
				opts.OutputDirectory = filepath.Join(occupied, "capture")
			case "capture":
				if err := os.WriteFile(filepath.Join(opts.OutputDirectory, "source-lag-recovery.raw.jsonl"), []byte("retained"), 0600); err != nil {
					t.Fatal(err)
				}
			case "stale head":
				live.mu.Lock()
				live.authoritative.ObservedAt = time.Now().Add(-3 * time.Second)
				live.mu.Unlock()
			}
			run, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
			defer cancel()
			if mode == "canceled" {
				go func() {
					for run.Err() == nil {
						if _, e := os.Stat(filepath.Join(opts.OutputDirectory, "source-lag-recovery.raw.jsonl")); e == nil {
							cancel()
							return
						}
						time.Sleep(time.Millisecond)
					}
				}()
			}
			report, err := MeasureSourceLagRecovery(run, opts, p)
			if err == nil || report.TransitionsValidated {
				t.Fatalf("unavailable lag observation accepted: %+v %v", report, err)
			}
			want := map[string]string{"binding": "capacity-revision", "directory": "not a directory", "capture": "file exists", "stale head": "capacity-lag-head-stale", "watermark": "capacity-lag-watermark", "canceled": "context canceled"}[mode]
			if !strings.Contains(err.Error(), want) {
				t.Fatalf("expected %s: %v", want, err)
			}
		})
	}
}
