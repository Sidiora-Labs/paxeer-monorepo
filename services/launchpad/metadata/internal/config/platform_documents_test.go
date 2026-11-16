package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	dbproduction "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
)

func platformDocumentValues(t *testing.T) (string, string) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	documents := filepath.Join(root, "deploy", "launchpad", "documents")
	values := t.TempDir()
	for _, name := range []string{"common.example.json", "metadata.example.json"} {
		raw, err := os.ReadFile(filepath.Join(documents, "values", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(values, name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(values, "expected-schema"), 0o700); err != nil {
		t.Fatal(err)
	}
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte("metadata-schema")))
	schema := dbproduction.ExpectedSchema{
		Service: "media/metadata", RuntimeRole: "metadatad", RegistryDigest: sum, InterfaceDigest: sum,
		ReaderVersion: 1, Migrations: map[int64]string{1: sum}, Schemas: map[string]string{"metadata": sum},
		Epoch: 1, Region: "mainnet-beta",
		Privileges: []dbproduction.Grant{{Schema: "metadata", Object: "assets", Privileges: []string{"SELECT"}}},
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(values, "expected-schema", "metadata.json"), raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return root, values
}

func composePlatformDocuments(t *testing.T, root, values string) map[string]string {
	t.Helper()
	out := t.TempDir()
	command := exec.Command("python3", filepath.Join(root, "deploy", "launchpad", "documents", "compose.py"),
		"--values", values, "--out", out, "--service", "metadata")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compose metadata: %v: %s", err, output)
	}
	path := filepath.Join(out, "metadata.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("document environment permissions = %o", info.Mode().Perm())
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var composed map[string]string
	if err := json.Unmarshal(raw, &composed); err != nil {
		t.Fatal(err)
	}
	if len(composed) != 6 {
		t.Fatalf("document environment contains %d variables, want 6", len(composed))
	}
	for _, name := range []string{"DB_BUDGET_DOCUMENT", "EXPECTED_SCHEMA_DOCUMENT", "METADATA_PUBLICATION_CONFIG_DOCUMENT"} {
		data, err := base64.StdEncoding.Strict().DecodeString(composed[name])
		if err != nil || len(data) == 0 {
			t.Fatalf("%s: empty or invalid base64: %v", name, err)
		}
		digestName := strings.TrimSuffix(name, "_DOCUMENT") + "_SHA256"
		if got := fmt.Sprintf("%x", sha256.Sum256(data)); composed[digestName] != got {
			t.Fatalf("%s does not bind %s", digestName, name)
		}
	}
	return composed
}

func setPlatformDocumentEnvironment(t *testing.T, root string, composed map[string]string) {
	t.Helper()
	configType := reflect.TypeOf(Config{})
	for i := 0; i < configType.NumField(); i++ {
		name := strings.Split(configType.Field(i).Tag.Get("env"), ",")[0]
		if name == "" {
			continue
		}
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	}
	var contract map[string]struct {
		Fixed    map[string]string `json:"fixed"`
		Required []string          `json:"required"`
	}
	raw, err := os.ReadFile(filepath.Join(root, "deploy", "launchpad", "railway", "variables.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, &contract); err != nil {
		t.Fatal(err)
	}
	entry, ok := contract["metadata"]
	if !ok || len(entry.Fixed) == 0 || len(entry.Required) == 0 {
		t.Fatal("metadata variable contract is required")
	}
	values := map[string]string{
		"CLAMAV_HOST":  "clamav.railway.internal",
		"DATABASE_URL": "postgres://metadatad_user:test-password@postgres:5432/launchpad",
		"DB_POOL_ID":   "metadatad_pool", "INSTANCE_ID": "metadata_1", "ZONE": "mainnet-beta",
		"IDENTITY_AUDIENCE": "metadata", "IDENTITY_SERVICE_URL": "http://gateway.railway.internal:8080",
		"IDENTITY_SERVICE_TOKEN":         "test-only-metadata-identity-service-token",
		"METADATA_PUBLICATION_AUDIT_KEY": base64.StdEncoding.EncodeToString(bytes.Repeat([]byte("k"), 32)),
		"PUBLIC_URL":                     "https://metadata.example", "REDIS_URL": "redis://redis:6379",
		"RPC_URL":          "https://rpc.example",
		"S3_ACCESS_KEY_ID": "test-only-access-key", "S3_SECRET_ACCESS_KEY": "test-only-secret-key",
		"S3_APPROVED_BUCKET": "metadata-approved", "S3_QUARANTINE_BUCKET": "metadata-quarantine",
		"S3_BUCKET": "metadata-approved", "S3_ENDPOINT": "https://objects.example",
	}
	for name, value := range entry.Fixed {
		t.Setenv(name, value)
	}
	for _, name := range entry.Required {
		value, ok := composed[name]
		if !ok {
			value, ok = values[name]
		}
		if !ok || value == "" {
			t.Fatalf("required variable %s has no value", name)
		}
		t.Setenv(name, value)
	}
}

func TestPlatformDocuments(t *testing.T) {
	root, values := platformDocumentValues(t)
	composed := composePlatformDocuments(t, root, values)
	setPlatformDocumentEnvironment(t, root, composed)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidateMediaRuntime(); err != nil {
		t.Fatal(err)
	}
	runtime, key, err := cfg.ReadPublicationRuntime()
	if err != nil || runtime.SchemaVersion != 3 || runtime.Environment != "paxeer125-mainnet-beta" ||
		runtime.EnvelopeHeader != "X-Kindle-Operation" || runtime.SignatureHeader != "X-Kindle-Operation-Signature" ||
		!bytes.Equal(key, bytes.Repeat([]byte("k"), 32)) {
		t.Fatalf("publication runtime = %+v, audit key bytes = %d, error = %v", runtime, len(key), err)
	}
	schema, err := cfg.readExpectedRuntimeSchema()
	if err != nil || schema.Service != "media/metadata" || schema.RuntimeRole != "metadatad" {
		t.Fatalf("expected schema = %+v, error = %v", schema, err)
	}

	for _, name := range []string{"DB_BUDGET_DOCUMENT", "EXPECTED_SCHEMA_DOCUMENT", "METADATA_PUBLICATION_CONFIG_DOCUMENT"} {
		t.Run("changed_"+name, func(t *testing.T) {
			raw, err := base64.StdEncoding.Strict().DecodeString(composed[name])
			if err != nil {
				t.Fatal(err)
			}
			raw[2] ^= 1
			t.Setenv(name, base64.StdEncoding.EncodeToString(raw))
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), name) || !strings.Contains(err.Error(), "runtime_document_binding_mismatch") {
				t.Fatalf("changed %s: %v", name, err)
			}
		})
		t.Run("invalid_"+name, func(t *testing.T) {
			raw := []byte("{}")
			t.Setenv(name, base64.StdEncoding.EncodeToString(raw))
			t.Setenv(strings.TrimSuffix(name, "_DOCUMENT")+"_SHA256", fmt.Sprintf("%x", sha256.Sum256(raw)))
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), name) {
				t.Fatalf("invalid %s: %v", name, err)
			}
		})
	}

	t.Run("file_sources", func(t *testing.T) {
		for document, file := range map[string]string{
			"DB_BUDGET_DOCUMENT": "DB_BUDGET_FILE", "EXPECTED_SCHEMA_DOCUMENT": "EXPECTED_SCHEMA_FILE",
			"METADATA_PUBLICATION_CONFIG_DOCUMENT": "METADATA_PUBLICATION_CONFIG_FILE",
		} {
			raw, err := base64.StdEncoding.Strict().DecodeString(composed[document])
			if err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "document.json")
			if err := os.WriteFile(path, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv(document, "")
			t.Setenv(file, path)
		}
		if _, err := Load(); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("other_service", func(t *testing.T) {
		raw, err := base64.StdEncoding.Strict().DecodeString(composed["EXPECTED_SCHEMA_DOCUMENT"])
		if err != nil {
			t.Fatal(err)
		}
		raw = bytes.Replace(raw, []byte(`"service":"media/metadata"`), []byte(`"service":"media/social"`), 1)
		t.Setenv("EXPECTED_SCHEMA_DOCUMENT", base64.StdEncoding.EncodeToString(raw))
		t.Setenv("EXPECTED_SCHEMA_SHA256", fmt.Sprintf("%x", sha256.Sum256(raw)))
		if _, err := Load(); err == nil || !strings.Contains(err.Error(), "runtime_schema_service_mismatch") {
			t.Fatalf("other service schema: %v", err)
		}
	})
}

func TestPlatformDocumentsValues(t *testing.T) {
	root, values := platformDocumentValues(t)
	path := filepath.Join(values, "metadata.json")
	for _, tc := range []struct {
		name, raw, key string
	}{
		{"missing_environment", `{}`, "environment"},
		{"empty_environment", `{"environment":""}`, "environment"},
		{"invalid_environment", `{"environment":"mainnet beta"}`, "environment"},
		{"long_environment", `{"environment":"` + strings.Repeat("a", 129) + `"}`, "environment"},
		{"numeric_environment", `{"environment":125}`, "environment"},
		{"null_environment", `{"environment":null}`, "environment"},
		{"unknown_key", `{"environment":"paxeer125-mainnet-beta","extra":1}`, "extra"},
		{"invalid_json", `{`, "metadata.json"},
		{"non_object", `[]`, "metadata.json"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.raw), 0o600); err != nil {
				t.Fatal(err)
			}
			command := exec.Command("python3", filepath.Join(root, "deploy", "launchpad", "documents", "compose.py"),
				"--values", values, "--out", t.TempDir(), "--service", "metadata")
			output, err := command.CombinedOutput()
			if err == nil || !strings.Contains(string(output), tc.key) {
				t.Fatalf("values %s: error = %v, output = %s", tc.name, err, output)
			}
		})
	}
	if err := os.WriteFile(path, []byte(`{"environment":"paxeer125-small"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	composed := composePlatformDocuments(t, root, values)
	setPlatformDocumentEnvironment(t, root, composed)
	cfg, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	runtime, _, err := cfg.ReadPublicationRuntime()
	if err != nil || runtime.Environment != "paxeer125-small" {
		t.Fatalf("deployment environment = %q, error = %v", runtime.Environment, err)
	}
}
