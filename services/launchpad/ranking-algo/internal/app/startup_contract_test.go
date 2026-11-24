package app

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
)

func startupJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func startupReadJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, value); err != nil {
		t.Fatal(err)
	}
}

func composeStartup(t *testing.T, database internaltest.RuntimeDatabase, binding config.RankingRuntimeBinding, maxConnections int64) {
	t.Helper()
	root := internaltest.RepositoryRoot(t)
	documents := filepath.Join(root, "deploy", "launchpad", "documents")
	values, output := t.TempDir(), t.TempDir()
	var common production.ConnectionBudget
	startupReadJSON(t, filepath.Join(documents, "values", "common.example.json"), &common)
	common.Targets[0].Host = database.Host
	common.Targets[0].Port = database.Port
	common.Targets[0].MaxConnections = maxConnections
	for i := range common.Clients {
		client := &common.Clients[i]
		if client.Role == "rankingd" {
			client.Transport = db.TransportDirect
			client.Pooler = ""
			client.Alias = database.Database
			client.User = database.RuntimeRole
			client.QueryClass = db.Batch
			client.MaxConnections = 3
			t.Setenv("DB_POOL_ID", client.PoolID)
		}
	}
	startupJSON(t, filepath.Join(values, "common.example.json"), common)
	var example map[string]any
	startupReadJSON(t, filepath.Join(documents, "values", "ranking.example.json"), &example)
	raw, err := json.Marshal(binding)
	if err != nil {
		t.Fatal(err)
	}
	var supplied map[string]any
	if err = json.Unmarshal(raw, &supplied); err != nil {
		t.Fatal(err)
	}
	for key := range example {
		if key != "_example" {
			example[key] = supplied[key]
		}
	}
	startupJSON(t, filepath.Join(values, "ranking.example.json"), example)
	if err = os.Mkdir(filepath.Join(values, "expected-schema"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(values, "expected-schema", "ranking.json"), database.ExpectedSchema, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "python3", filepath.Join(documents, "compose.py"), "--examples", values, "--out", output, "--service", "ranking")
	command.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	if result, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compose ranking documents: %v: %s", err, result)
	}
	var composed map[string]string
	startupReadJSON(t, filepath.Join(output, "ranking.json"), &composed)
	var contract map[string]struct {
		Fixed    map[string]string `json:"fixed"`
		Required []string          `json:"required"`
	}
	startupReadJSON(t, filepath.Join(root, "deploy", "launchpad", "railway", "variables.json"), &contract)
	for name, value := range contract["ranking"].Fixed {
		t.Setenv(name, value)
	}
	for name, value := range composed {
		t.Setenv(name, value)
	}
	for _, name := range contract["ranking"].Required {
		if os.Getenv(name) == "" {
			t.Fatalf("ranking requires %s", name)
		}
	}
	for _, name := range []string{"DB_BUDGET_FILE", "EXPECTED_SCHEMA_FILE", "RANKING_RUNTIME_FILE"} {
		t.Setenv(name, "")
	}
}

func provisionStartupBroker(t *testing.T) {
	t.Helper()
	root := internaltest.RepositoryRoot(t)
	if os.Getenv("RANKING_TEST_NATS_URL") == "" {
		t.Fatal("RANKING_TEST_NATS_URL must name the private three-server broker")
	}
	t.Setenv("RANKING_TEST_BROKER_TOKEN", "ranking-test-provisioning")
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "run", "./brokerprovision/cmd/brokerprovision", "provision",
		"--document", filepath.Join(root, "deploy", "launchpad", "documents", "values", "broker.example.json"),
		"--broker-env", "RANKING_TEST_NATS_URL", "--token-env", "RANKING_TEST_BROKER_TOKEN")
	command.Dir = filepath.Join(root, "sdk", "launchpad", "shared")
	if result, err := command.CombinedOutput(); err != nil {
		t.Fatalf("provision deployment streams and durables: %v: %s", err, result)
	}
}

func startupBinary(t *testing.T) string {
	t.Helper()
	binary := filepath.Join(t.TempDir(), "rankingd")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	command := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/rankingd")
	command.Dir = filepath.Join(internaltest.RepositoryRoot(t), "services", "launchpad", "ranking-algo")
	if result, err := command.CombinedOutput(); err != nil {
		t.Fatalf("build ranking entrypoint: %v: %s", err, result)
	}
	return binary
}

type startupProcess struct {
	command *exec.Cmd
	done    chan struct{}
	err     error
}

func startRankingProcess(t *testing.T, binary string) *startupProcess {
	t.Helper()
	output, err := os.CreateTemp(t.TempDir(), "ranking-output-")
	if err != nil {
		t.Fatal(err)
	}
	p := &startupProcess{command: exec.Command(binary), done: make(chan struct{})}
	p.command.Stdout, p.command.Stderr = output, output
	if err = p.command.Start(); err != nil {
		output.Close()
		t.Fatal(err)
	}
	go func() { p.err = p.command.Wait(); close(p.done) }()
	t.Cleanup(func() {
		if err := p.stop(); err != nil {
			t.Errorf("stop ranking process: %v", err)
		}
		if err := output.Close(); err != nil {
			t.Error(err)
		}
		if t.Failed() {
			raw, err := os.ReadFile(output.Name())
			if err != nil {
				t.Error(err)
			} else {
				t.Logf("ranking process output: %s", raw)
			}
		}
	})
	return p
}

func (p *startupProcess) stop() error {
	select {
	case <-p.done:
		return p.err
	default:
	}
	if err := p.command.Process.Signal(syscall.SIGTERM); err != nil {
		return err
	}
	select {
	case <-p.done:
		return p.err
	case <-time.After(45 * time.Second):
		_ = p.command.Process.Kill()
		<-p.done
		return errors.New("ranking process did not stop within its shutdown deadline")
	}
}
