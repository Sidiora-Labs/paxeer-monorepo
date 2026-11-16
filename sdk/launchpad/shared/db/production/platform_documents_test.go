package production

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"os"
	osexec "os/exec"
	"path/filepath"
	"testing"
)

func TestPlatformDocuments(t *testing.T) {
	services := []string{"api", "api-sockets", "indexer", "charts", "stats", "pnl", "ranking", "gateway", "user", "social", "metadata", "livestream"}
	registryServices := map[string]string{"api": "core/api", "api-sockets": "core/api", "indexer": "core/indexer", "charts": "core/trading-charts", "stats": "core/stats-workers", "pnl": "core/pnl-tracker", "ranking": "core/ranking-algo", "gateway": "media/gateway", "user": "media/user", "social": "media/social", "metadata": "media/metadata", "livestream": "media/livestream"}
	roles := map[string]string{"api": "apid_rest", "api-sockets": "apid_sockets", "indexer": "indexerd", "charts": "chartsd", "stats": "statsd", "pnl": "pnld", "ranking": "rankingd", "gateway": "mediagatewayd", "user": "userd", "social": "sociald", "metadata": "metadatad", "livestream": "livestreamd"}
	root := filepath.Join("..", "..", "..", "..", "..")
	values := t.TempDir()
	if err := os.Mkdir(filepath.Join(values, "expected-schema"), 0o755); err != nil {
		t.Fatal(err)
	}
	example, err := os.ReadFile(filepath.Join(root, "deploy", "launchpad", "documents", "values", "common.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(values, "common.json"), example, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, service := range services {
		example, err := os.ReadFile(filepath.Join(root, "deploy", "launchpad", "documents", "values", service+".example.json"))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(values, service+".example.json"), example, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	digest := "0000000000000000000000000000000000000000000000000000000000000000"
	for _, service := range services {
		want := map[string]any{
			"service": registryServices[service], "runtime_role": roles[service], "registry_digest": digest, "interface_digest": digest,
			"reader_version": 1, "migrations": map[string]string{"1": digest}, "schemas": map[string]string{"app": digest},
			"epoch": 1, "region": "mainnet", "privileges": []map[string]any{{"schema": "app", "object": "items", "privileges": []string{"SELECT"}}},
		}
		raw, err := json.Marshal(want)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(values, "expected-schema", service+".json"), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	out := t.TempDir()
	args := []string{filepath.Join(root, "deploy", "launchpad", "documents", "compose.py"), "--values", values, "--out", out}
	for _, service := range services {
		args = append(args, "--service", service)
	}
	command := osexec.Command("python3", args...)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compose: %v: %s", err, output)
	}

	var budget []byte
	for _, service := range services {
		data := readComposedDocument(t, filepath.Join(out, service+".json"), "DB_BUDGET_DOCUMENT")
		digest := readComposedValue(t, filepath.Join(out, service+".json"), "DB_BUDGET_SHA256")
		if _, err := LoadConnectionBudget(data, digest); err != nil {
			t.Fatalf("%s budget: %v", service, err)
		}
		if budget == nil {
			budget = data
		} else if !bytes.Equal(budget, data) {
			t.Fatalf("%s has a different budget", service)
		}
		schemaPath := filepath.Join(out, service+".json")
		schema := readComposedDocument(t, schemaPath, "EXPECTED_SCHEMA_DOCUMENT")
		schemaDigest := readComposedValue(t, schemaPath, "EXPECTED_SCHEMA_SHA256")
		if _, err := LoadExpectedSchema(schema, schemaDigest); err != nil {
			t.Fatalf("%s schema: %v", service, err)
		}
		changed := append([]byte(nil), data...)
		changed[0] ^= 1
		if _, err := LoadConnectionBudget(changed, digest); err == nil {
			t.Fatalf("%s accepted changed budget", service)
		}
		changed = append([]byte(nil), schema...)
		changed[0] ^= 1
		if _, err := LoadExpectedSchema(changed, schemaDigest); err == nil {
			t.Fatalf("%s accepted changed schema", service)
		}
	}
}

func readComposedValue(t *testing.T, path, name string) string {
	t.Helper()
	var values map[string]string
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatal(err)
	}
	value, ok := values[name]
	if !ok {
		t.Fatalf("%s missing from %s", name, path)
	}
	return value
}

func readComposedDocument(t *testing.T, path, name string) []byte {
	t.Helper()
	value := readComposedValue(t, path, name)
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
