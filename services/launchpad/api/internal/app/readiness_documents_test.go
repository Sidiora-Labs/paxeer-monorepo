package app

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func readinessDocumentsRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source path unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func TestReadinessDocuments(t *testing.T) {
	root := readinessDocumentsRoot(t)
	valuesDir := t.TempDir()
	for _, name := range []string{"common.example.json", "api.example.json", "network-authority.json"} {
		raw, err := os.ReadFile(filepath.Join(root, "deploy", "launchpad", "documents", "values", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(valuesDir, name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(valuesDir, "expected-schema"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(valuesDir, "expected-schema", "api.json"), []byte(`{"service":"core/api"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	command := exec.Command("python3", filepath.Join(root, "deploy", "launchpad", "documents", "compose.py"), "--values", valuesDir, "--out", out, "--service", "api")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compose API examples: %v: %s", err, output)
	}
	raw, err := os.ReadFile(filepath.Join(out, "api.json"))
	if err != nil {
		t.Fatal(err)
	}
	var values map[string]string
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatal(err)
	}
	decode := func(name string, target any) {
		t.Helper()
		encoded, ok := values[name]
		if !ok {
			t.Fatalf("composed API document missing %s", name)
		}
		document, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil {
			t.Fatalf("decode %s: %v", name, err)
		}
		if err := json.Unmarshal(document, target); err != nil {
			t.Fatalf("decode %s into runtime type: %v", name, err)
		}
	}
	var relay streamRelayDocument
	decode("STREAM_RELAY_CONFIG_DOCUMENT", &relay)
	if err := relay.validate(); err != nil {
		t.Fatalf("stream relay document: %v", err)
	}
	var discovery discoveryRuntimeDocument
	decode("DISCOVERY_RUNTIME_CONFIG_DOCUMENT", &discovery)
	if err := discovery.validate(); err != nil {
		t.Fatalf("discovery runtime document: %v", err)
	}

	contractRaw, err := os.ReadFile(filepath.Join(root, "deploy", "launchpad", "railway", "variables.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contract map[string]struct {
		Required        []string          `json:"required"`
		DocumentSources map[string]string `json:"documentSources"`
	}
	if err := json.Unmarshal(contractRaw, &contract); err != nil {
		t.Fatal(err)
	}
	api := contract["api"]
	required := map[string]bool{}
	for _, name := range api.Required {
		required[name] = true
	}
	for name := range api.DocumentSources {
		required[name] = true
	}
	for _, name := range []string{
		"NETWORK_AUTHORITY_CONFIG_DOCUMENT", "NETWORK_AUTHORITY_CONFIG_SHA256",
		"STREAM_RELAY_CONFIG_DOCUMENT", "STREAM_RELAY_CONFIG_SHA256",
		"DISCOVERY_RUNTIME_CONFIG_DOCUMENT", "DISCOVERY_RUNTIME_CONFIG_SHA256",
	} {
		if !required[name] {
			t.Fatalf("rest readiness document %s is absent from the API contract", name)
		}
	}
	for _, check := range []struct{ name, document string }{
		{"network_authority", "NETWORK_AUTHORITY_CONFIG_DOCUMENT"},
		{"stream_relay", "STREAM_RELAY_CONFIG_DOCUMENT"},
		{"discovery_notifications", "DISCOVERY_RUNTIME_CONFIG_DOCUMENT"},
	} {
		if !required[check.document] {
			t.Fatalf("readiness check %s lacks required document %s", check.name, check.document)
		}
	}
}
