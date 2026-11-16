package config

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func platformDocumentsRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../.."))
}

func composeSocialDocuments(t *testing.T) map[string]string {
	t.Helper()
	root := platformDocumentsRepositoryRoot(t)
	values := t.TempDir()
	if err := os.WriteFile(filepath.Join(values, "common.example.json"), mustReadPlatformDocument(t, filepath.Join(root, "deploy/launchpad/documents/values/common.example.json")), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(values, "social.example.json"), mustReadPlatformDocument(t, filepath.Join(root, "deploy/launchpad/documents/values/social.example.json")), 0o600); err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(values, "expected-schema")
	if err := os.Mkdir(expected, 0o700); err != nil {
		t.Fatal(err)
	}
	schema := []byte(`{"service":"media/social","runtime_role":"sociald","registry_digest":"` + fmt.Sprintf("%064x", 0) + `","interface_digest":"` + fmt.Sprintf("%064x", 0) + `","reader_version":1,"migrations":{"1":"` + fmt.Sprintf("%064x", 0) + `"},"schemas":{"chat":"` + fmt.Sprintf("%064x", 0) + `"},"epoch":1,"region":"mainnet-beta","privileges":[]}`)
	if err := os.WriteFile(filepath.Join(expected, "social.json"), schema, 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out")
	cmd := exec.Command("python3", filepath.Join(root, "deploy/launchpad/documents/compose.py"), "--values", values, "--out", out, "--service", "social")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compose: %v\n%s", err, output)
	}
	var documents map[string]string
	if err := json.Unmarshal(mustReadPlatformDocument(t, filepath.Join(out, "social.json")), &documents); err != nil {
		t.Fatal(err)
	}
	return documents
}

func mustReadPlatformDocument(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func setPlatformDocumentEnvironment(t *testing.T, documents map[string]string) {
	t.Helper()
	baseEnv(t)
	for name, value := range map[string]string{
		"NODE_ENV":                      "production",
		"RUNTIME_PROFILE":               "container",
		"BIND_ADDRESS":                  "0.0.0.0",
		"SECRETS_PROVIDER":              "env",
		"SOCIAL_PRIVATE_LISTEN_ADDRESS": "0.0.0.0:8443",
		"PORT":                          "8080",
		"DATABASE_URL":                  "postgres://social:social@database.internal:5432/social?sslmode=disable",
		"REDIS_URL":                     "redis://redis.internal:6379/0",
		"DB_POOL_ID":                    "sociald_pool",
		"INSTANCE_ID":                   "social-1",
		"ZONE":                          "mainnet-beta",
	} {
		t.Setenv(name, value)
	}
	for name, value := range documents {
		t.Setenv(name, value)
	}
}

func TestPlatformDocumentsComposeAndLoad(t *testing.T) {
	documents := composeSocialDocuments(t)
	setPlatformDocumentEnvironment(t, documents)
	if _, err := Load(); err != nil {
		t.Fatalf("Load composed platform documents: %v", err)
	}
}

func TestPlatformDocumentsRejectTampering(t *testing.T) {
	for _, documentName := range []string{"DB_BUDGET_DOCUMENT", "EXPECTED_SCHEMA_DOCUMENT", "SOCIAL_RUNTIME_DOCUMENT"} {
		t.Run(documentName, func(t *testing.T) {
			documents := composeSocialDocuments(t)
			raw, err := base64.StdEncoding.Strict().DecodeString(documents[documentName])
			if err != nil {
				t.Fatal(err)
			}
			if len(raw) == 0 {
				t.Fatal("composed document is empty")
			}
			raw[0] ^= 1
			documents[documentName] = base64.StdEncoding.EncodeToString(raw)
			setPlatformDocumentEnvironment(t, documents)
			if _, err := Load(); err == nil {
				t.Fatalf("Load accepted tampered %s", documentName)
			}
		})
	}
}

func TestPlatformDocumentsExpectedSchemaFileForm(t *testing.T) {
	root := platformDocumentsRepositoryRoot(t)
	values := t.TempDir()
	for _, name := range []string{"common.example.json", "social.example.json"} {
		if err := os.WriteFile(filepath.Join(values, name), mustReadPlatformDocument(t, filepath.Join(root, "deploy/launchpad/documents/values", name)), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	expected := filepath.Join(values, "expected-schema")
	if err := os.Mkdir(expected, 0o700); err != nil {
		t.Fatal(err)
	}
	schema := []byte(`{"service":"media/social","runtime_role":"sociald","registry_digest":"` + fmt.Sprintf("%064x", 0) + `","interface_digest":"` + fmt.Sprintf("%064x", 0) + `","reader_version":1,"migrations":{"1":"` + fmt.Sprintf("%064x", 0) + `"},"schemas":{"chat":"` + fmt.Sprintf("%064x", 0) + `"},"epoch":1,"region":"mainnet-beta","privileges":[]}`)
	schemaPath := filepath.Join(expected, "social.json")
	if err := os.WriteFile(schemaPath, schema, 0o600); err != nil {
		t.Fatal(err)
	}
	runtimePath := filepath.Join(t.TempDir(), "expected-schema.json")
	paths, err := json.Marshal(map[string]string{"social": runtimePath})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(expected, "runtime-paths.json"), paths, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(runtimePath, schema, 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "out")
	cmd := exec.Command("python3", filepath.Join(root, "deploy/launchpad/documents/compose.py"), "--values", values, "--out", out, "--service", "social")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compose file form: %v\n%s", err, output)
	}
	var documents map[string]string
	if err := json.Unmarshal(mustReadPlatformDocument(t, filepath.Join(out, "social.json")), &documents); err != nil {
		t.Fatal(err)
	}
	if documents["EXPECTED_SCHEMA_FILE"] != runtimePath || documents["EXPECTED_SCHEMA_DOCUMENT"] != "" {
		t.Fatalf("expected schema file form = %#v", documents)
	}
	setPlatformDocumentEnvironment(t, documents)
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load file-form platform documents: %v", err)
	}
	bound, err := boundBytes(cfg.ExpectedSchemaFile, cfg.ExpectedSchemaDocument, cfg.ExpectedSchemaSHA256)
	if err != nil || !bytes.Equal(bound, schema) {
		t.Fatalf("file-form expected schema bytes = %q, error = %v", bound, err)
	}
	changed := append([]byte(nil), schema...)
	changed[len(changed)-1] ^= 1
	if err := os.WriteFile(runtimePath, changed, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(); err == nil || !strings.Contains(err.Error(), "runtime_document_binding_mismatch") {
		t.Fatalf("changed expected schema file was accepted: %v", err)
	}
}
