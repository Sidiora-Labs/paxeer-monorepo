package internaltest

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
)

func LivestreamDocuments(t testing.TB, runtime coordinatortest.Runtime) []string {
	t.Helper()
	documents := filepath.Join(coordinatortest.RepositoryRoot(t), "deploy", "launchpad", "documents")
	values, output := t.TempDir(), t.TempDir()
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	write := func(path string, raw []byte) {
		t.Helper()
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	budget, err := production.ParseConnectionBudget(read(filepath.Join(documents, "values", "common.example.json")))
	if err != nil {
		t.Fatal(err)
	}
	endpoint, err := url.Parse(runtime.DSN)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.ParseUint(endpoint.Port(), 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	for i := range budget.Targets {
		budget.Targets[i].Host, budget.Targets[i].Port = endpoint.Hostname(), uint16(port)
	}
	found := false
	for i := range budget.Clients {
		client := &budget.Clients[i]
		if client.Role == "livestreamd" {
			client.Transport, client.Pooler = shareddb.TransportDirect, ""
			client.Alias, client.User = strings.TrimPrefix(endpoint.Path, "/"), endpoint.User.Username()
			client.MaxConnections = 5
			found = true
		}
	}
	if !found {
		t.Fatal("livestream database client is required")
	}
	raw, err := json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := production.ParseConnectionBudget(raw); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(values, "common.json"), raw)
	write(filepath.Join(values, "livestream.json"), read(filepath.Join(documents, "values", "livestream.example.json")))
	if err := os.Mkdir(filepath.Join(values, "expected-schema"), 0700); err != nil {
		t.Fatal(err)
	}
	write(filepath.Join(values, "expected-schema", "livestream.json"), runtime.ExpectedSchema)
	command := exec.Command("python3", filepath.Join(documents, "compose.py"), "--values", values, "--out", output, "--service", "livestream")
	if raw, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compose livestream documents: %v: %s", err, raw)
	}
	var composed map[string]string
	if err := json.Unmarshal(read(filepath.Join(output, "livestream.json")), &composed); err != nil {
		t.Fatal(err)
	}
	var environment []string
	for _, name := range []string{"DB_BUDGET", "EXPECTED_SCHEMA"} {
		raw, err := base64.StdEncoding.DecodeString(composed[name+"_DOCUMENT"])
		if err != nil || len(raw) == 0 || production.Checksum(raw) != composed[name+"_SHA256"] {
			t.Fatalf("%s document binding: %v", name, err)
		}
		environment = append(environment, name+"_DOCUMENT="+composed[name+"_DOCUMENT"], name+"_SHA256="+composed[name+"_SHA256"])
	}
	return environment
}
