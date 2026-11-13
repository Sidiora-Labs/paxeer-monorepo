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

var durabilityProbe struct {
	once   sync.Once
	binary string
	output []byte
	err    error
}

func TestMain(m *testing.M) {
	code := m.Run()
	if durabilityProbe.binary != "" {
		if err := os.RemoveAll(filepath.Dir(filepath.Dir(durabilityProbe.binary))); err != nil {
			os.Stderr.WriteString(err.Error() + "\n")
			code = 1
		}
	}
	os.Exit(code)
}

func runDurabilityScenario(t *testing.T, scenario string) {
	t.Helper()
	durabilityProbe.once.Do(func() {
		_, source, _, ok := runtime.Caller(0)
		if !ok {
			durabilityProbe.err = os.ErrNotExist
			return
		}
		target, err := os.MkdirTemp("", "perfdb-durability-target-")
		if err != nil {
			durabilityProbe.err = err
			return
		}
		durabilityProbe.binary = filepath.Join(target, "debug", "durability-probe")
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "cargo", "build", "--quiet", "--manifest-path", filepath.Join(filepath.Dir(source), "durability_probe", "Cargo.toml"), "--target-dir", target)
		durabilityProbe.output, durabilityProbe.err = cmd.CombinedOutput()
	})
	if durabilityProbe.err != nil {
		t.Fatalf("compile real Rust storage probe: %v\n%s", durabilityProbe.err, durabilityProbe.output)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	output, err := exec.CommandContext(ctx, durabilityProbe.binary, scenario, t.TempDir()).CombinedOutput()
	if err != nil {
		t.Fatalf("real storage restart scenario %s: %v\n%s", scenario, err, output)
	}
	if !strings.Contains(string(output), "durability-probe:"+scenario+":ok") {
		t.Fatalf("scenario did not finish assertions: %s", output)
	}
}

func TestWALRecoveredTailWrite(t *testing.T)      { runDurabilityScenario(t, "wal") }
func TestSnapshotDurablePublish(t *testing.T)     { runDurabilityScenario(t, "snapshot") }
func TestRelationalStatePersistence(t *testing.T) { runDurabilityScenario(t, "relational") }
