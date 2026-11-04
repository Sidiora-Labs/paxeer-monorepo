package storage

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

var repair139Probe struct {
	once   sync.Once
	binary string
	output []byte
	err    error
}

func TestMain(m *testing.M) {
	code := m.Run()
	if repair139Probe.binary != "" {
		if err := os.RemoveAll(filepath.Dir(filepath.Dir(repair139Probe.binary))); err != nil {
			os.Stderr.WriteString(err.Error() + "\n")
			code = 1
		}
	}
	os.Exit(code)
}

func runRepair139(t *testing.T, scenario string) {
	t.Helper()
	repair139Probe.once.Do(func() {
		_, source, _, ok := runtime.Caller(0)
		if !ok {
			repair139Probe.err = os.ErrNotExist
			return
		}
		target, err := os.MkdirTemp("", "perfdb-repair139-target-")
		if err != nil {
			repair139Probe.err = err
			return
		}
		repair139Probe.binary = filepath.Join(target, "debug", "repair139")
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "cargo", "build", "--quiet", "--manifest-path", filepath.Join(filepath.Dir(source), "repair_139", "Cargo.toml"), "--target-dir", target)
		repair139Probe.output, repair139Probe.err = cmd.CombinedOutput()
	})
	if repair139Probe.err != nil {
		t.Fatalf("compile real Rust storage probe: %v\n%s", repair139Probe.err, repair139Probe.output)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	output, err := exec.CommandContext(ctx, repair139Probe.binary, scenario, t.TempDir()).CombinedOutput()
	if err != nil {
		t.Fatalf("real storage restart scenario %s: %v\n%s", scenario, err, output)
	}
	if !strings.Contains(string(output), "repair139:"+scenario+":ok") {
		t.Fatalf("scenario did not finish assertions: %s", output)
	}
}

func TestRepair_139_wal_recovered_tail_write(t *testing.T)     { runRepair139(t, "wal") }
func TestRepair_139_snapshot_durable_publish(t *testing.T)     { runRepair139(t, "snapshot") }
func TestRepair_139_relational_state_persistence(t *testing.T) { runRepair139(t, "relational") }
