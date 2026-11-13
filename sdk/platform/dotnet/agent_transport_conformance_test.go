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

var agentConformanceHarness struct {
	sync.Once
	directory string
	dll       string
	err       error
}
var agentConformanceProducer struct {
	sync.Once
	binary string
	err    error
}

func agentConformanceRoot() string {
	_, source, _, _ := runtime.Caller(0)
	return filepath.Clean(filepath.Join(filepath.Dir(source), "../../.."))
}

func agentConformanceCommand(directory string, timeout time.Duration, env []string, program string, args ...string) ([]byte, error) {
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

func agentConformancePrepare(t *testing.T) (string, string) {
	t.Helper()
	agentConformanceHarness.Do(func() {
		agentConformanceHarness.directory, agentConformanceHarness.err = os.MkdirTemp("", "agent-conformance-dotnet-")
		if agentConformanceHarness.err != nil {
			return
		}
		artifacts := filepath.Join(agentConformanceHarness.directory, "artifacts")
		output, err := agentConformanceCommand(agentConformanceRoot(), 3*time.Minute, []string{"DOTNET_CLI_TELEMETRY_OPTOUT=1", "DOTNET_NOLOGO=1"},
			"dotnet", "build", "sdk/platform/dotnet/tests/AgentTransportConformance/AgentTransportConformance.csproj", "--artifacts-path", artifacts, "--nologo")
		log := filepath.Join(agentConformanceHarness.directory, "dotnet-build.log")
		if writeErr := os.WriteFile(log, output, 0600); writeErr != nil {
			agentConformanceHarness.err = writeErr
			return
		}
		if err != nil {
			agentConformanceHarness.err = fmt.Errorf("%w; private evidence: %s", err, log)
			return
		}
		agentConformanceHarness.dll = filepath.Join(artifacts, "bin", "AgentTransportConformance", "debug", "AgentTransportConformance.dll")
	})
	if agentConformanceHarness.err != nil {
		t.Fatal(agentConformanceHarness.err)
	}
	return agentConformanceHarness.dll, agentConformanceHarness.directory
}

func agentConformanceDaemon(t *testing.T) string {
	t.Helper()
	agentConformanceProducer.Do(func() {
		_, directory := agentConformancePrepare(t)
		target := filepath.Join(directory, "rust")
		output, err := agentConformanceCommand(agentConformanceRoot(), 6*time.Minute, []string{"CARGO_BUILD_JOBS=2"}, "cargo", "build",
			"--manifest-path", "sdk/platform/dotnet/tests/AgentTransportProducer/Cargo.toml", "--target-dir", target, "--bin", "agent-transport-producer")
		log := filepath.Join(directory, "daemon-codec-build.log")
		if writeErr := os.WriteFile(log, output, 0600); writeErr != nil {
			agentConformanceProducer.err = writeErr
			return
		}
		if err != nil {
			agentConformanceProducer.err = fmt.Errorf("%w; private evidence: %s", err, log)
			return
		}
		agentConformanceProducer.binary = filepath.Join(target, "debug", "agent-transport-producer")
	})
	if agentConformanceProducer.err != nil {
		t.Fatal(agentConformanceProducer.err)
	}
	return agentConformanceProducer.binary
}

func agentConformanceRun(t *testing.T, name, state string, producer bool) {
	t.Helper()
	dll, directory := agentConformancePrepare(t)
	var env []string
	if producer {
		env = []string{"REPAIR138_DAEMON_CODEC=" + agentConformanceDaemon(t)}
	}
	output, err := agentConformanceCommand(agentConformanceRoot(), time.Minute, env, "dotnet", dll, agentConformanceRoot(), name, state)
	log := filepath.Join(directory, name+".log")
	if writeErr := os.WriteFile(log, output, 0600); writeErr != nil {
		t.Fatal(writeErr)
	}
	if err != nil {
		t.Fatalf("%v; private evidence: %s", err, log)
	}
	t.Logf("%s; private evidence: %s", output, log)
}

func TestAgentConformanceRequestVersion(t *testing.T) {
	agentConformanceRun(t, "request", "", true)
}
func TestAgentConformanceResponseEnvelope(t *testing.T) {
	agentConformanceRun(t, "response", "", true)
}
func TestAgentConformanceVerificationShapes(t *testing.T) {
	agentConformanceRun(t, "verification", "", true)
}
func TestAgentConformanceBootstrapCredential(t *testing.T) {
	agentConformanceRun(t, "credential", "", true)
}
func TestAgentConformancePortableValidatorTrust(t *testing.T) {
	agentConformanceRun(t, "trust", "", false)
}
func TestAgentConformanceStreamUndeliveredCursor(t *testing.T) {
	state := t.TempDir()
	agentConformanceRun(t, "stream", state, false)
	// A separate OS process reopens only the persisted cursor and event journal.
	agentConformanceRun(t, "stream-reopen", state, false)
}
