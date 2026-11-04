package dotnet

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

var repair138Harness struct {
	sync.Once
	directory string
	dll       string
	err       error
}
var repair138Producer struct {
	sync.Once
	binary string
	err    error
}

func repair138Root() string {
	_, source, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(source), "../../.."))
}

func repair138Command(directory string, timeout time.Duration, env []string, program string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	command := exec.CommandContext(ctx, program, args...)
	command.Dir = directory
	command.Env = append(os.Environ(), env...)
	output, err := command.CombinedOutput()
	if err != nil {
		return output, fmt.Errorf("%s %v: %w\n%s", program, args, err, output)
	}
	return output, nil
}

func repair138Prepare(t *testing.T) (string, string) {
	t.Helper()
	repair138Harness.Do(func() {
		repair138Harness.directory, repair138Harness.err = os.MkdirTemp("", "repair138-dotnet-")
		if repair138Harness.err != nil {
			return
		}
		artifacts := filepath.Join(repair138Harness.directory, "artifacts")
		output, err := repair138Command(repair138Root(), 3*time.Minute, []string{"DOTNET_CLI_TELEMETRY_OPTOUT=1", "DOTNET_NOLOGO=1"},
			"dotnet", "build", "sdk/platform/dotnet/tests/Repair138/Repair138.csproj", "--artifacts-path", artifacts, "--nologo")
		log := filepath.Join(repair138Harness.directory, "dotnet-build.log")
		if writeErr := os.WriteFile(log, output, 0600); writeErr != nil {
			repair138Harness.err = writeErr
			return
		}
		if err != nil {
			repair138Harness.err = fmt.Errorf("%w; private evidence: %s", err, log)
			return
		}
		repair138Harness.dll = filepath.Join(artifacts, "bin", "Repair138", "debug", "Repair138.dll")
	})
	if repair138Harness.err != nil {
		t.Fatal(repair138Harness.err)
	}
	return repair138Harness.dll, repair138Harness.directory
}

func repair138Daemon(t *testing.T) string {
	t.Helper()
	repair138Producer.Do(func() {
		_, directory := repair138Prepare(t)
		target := filepath.Join(directory, "rust")
		output, err := repair138Command(repair138Root(), 6*time.Minute, []string{"CARGO_BUILD_JOBS=2"}, "cargo", "build",
			"--manifest-path", "sdk/platform/dotnet/tests/Repair138Producer/Cargo.toml", "--target-dir", target, "--bin", "repair138-producer")
		log := filepath.Join(directory, "daemon-codec-build.log")
		if writeErr := os.WriteFile(log, output, 0600); writeErr != nil {
			repair138Producer.err = writeErr
			return
		}
		if err != nil {
			repair138Producer.err = fmt.Errorf("%w; private evidence: %s", err, log)
			return
		}
		repair138Producer.binary = filepath.Join(target, "debug", "repair138-producer")
	})
	if repair138Producer.err != nil {
		t.Fatal(repair138Producer.err)
	}
	return repair138Producer.binary
}

func repair138Run(t *testing.T, name, state string, producer bool) {
	t.Helper()
	dll, directory := repair138Prepare(t)
	var env []string
	if producer {
		env = []string{"REPAIR138_DAEMON_CODEC=" + repair138Daemon(t)}
	}
	output, err := repair138Command(repair138Root(), time.Minute, env, "dotnet", dll, repair138Root(), name, state)
	log := filepath.Join(directory, name+".log")
	if writeErr := os.WriteFile(log, output, 0600); writeErr != nil {
		t.Fatal(writeErr)
	}
	if err != nil {
		t.Fatalf("%v; private evidence: %s", err, log)
	}
	t.Logf("%s; private evidence: %s", output, log)
}

func TestRepair_138_agent_request_version(t *testing.T) {
	repair138Run(t, "request", "", true)
}
func TestRepair_138_agent_response_envelope(t *testing.T) {
	repair138Run(t, "response", "", true)
}
func TestRepair_138_agent_verification_shapes(t *testing.T) {
	repair138Run(t, "verification", "", true)
}
func TestRepair_138_agent_bootstrap_credential(t *testing.T) {
	repair138Run(t, "credential", "", true)
}
func TestRepair_138_portable_validator_trust(t *testing.T) {
	repair138Run(t, "trust", "", false)
}
func TestRepair_138_stream_undelivered_cursor(t *testing.T) {
	state := t.TempDir()
	repair138Run(t, "stream", state, false)
	// A separate OS process reopens only the persisted cursor and event journal.
	repair138Run(t, "stream-reopen", state, false)
}
