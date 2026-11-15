package config_test

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/config"
	dbproduction "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/nats-io/nkeys"
)

var platformAccountEvents = []string{
	"profile.updated", "preferences.updated", "watchlist.changed", "consent.updated",
	"endpoint.bound", "endpoint.revoked", "account.export.requested",
	"account.deletion.requested", "account.deletion.job", "account.backfill.completed",
}

func platformValues(t *testing.T) (string, string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locate user configuration tests")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
	documents := filepath.Join(root, "deploy", "launchpad", "documents")
	values := t.TempDir()
	for _, name := range []string{"common.example.json", "user.example.json"} {
		data, err := os.ReadFile(filepath.Join(documents, "values", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(values, name), data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(values, "expected-schema"), 0o700); err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("0", 64)
	schema := dbproduction.ExpectedSchema{
		Service: "user", RuntimeRole: "userd", RegistryDigest: digest, InterfaceDigest: digest,
		ReaderVersion: 1, Migrations: map[int64]string{1: digest}, Schemas: map[string]string{"users": digest},
		Epoch: 1, Region: "mainnet-beta",
		Privileges: []dbproduction.Grant{{Schema: "users", Object: "user_profiles", Privileges: []string{"SELECT"}}},
	}
	platformWriteJSON(t, filepath.Join(values, "expected-schema", "user.json"), schema)
	return root, values
}

func platformWriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func platformCompose(t *testing.T, root, values, out string) *exec.Cmd {
	t.Helper()
	return exec.Command("python3", filepath.Join(root, "deploy", "launchpad", "documents", "compose.py"),
		"--values", values, "--out", out, "--service", "user")
}

func platformEnvironment(t *testing.T, root string, composed map[string]string) {
	t.Helper()
	fields := reflect.TypeOf(config.Config{})
	for i := 0; i < fields.NumField(); i++ {
		name := strings.Split(fields.Field(i).Tag.Get("env"), ",")[0]
		if name != "" {
			t.Setenv(name, "")
			if err := os.Unsetenv(name); err != nil {
				t.Fatal(err)
			}
		}
	}
	var contract map[string]struct {
		Fixed    map[string]string `json:"fixed"`
		Required []string          `json:"required"`
	}
	data, err := os.ReadFile(filepath.Join(root, "deploy", "launchpad", "railway", "variables.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &contract); err != nil {
		t.Fatal(err)
	}
	entry, ok := contract["user"]
	if !ok {
		t.Fatal("user variable contract missing")
	}
	pki := testpki.New(t)
	leaf := pki.Leaf("user", "user.example")
	creds := filepath.Join(t.TempDir(), "user.creds")
	accountKey, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	defer accountKey.Wipe()
	userKey, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	defer userKey.Wipe()
	issuer, err := accountKey.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	user, err := userKey.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	seed, err := userKey.Seed()
	if err != nil {
		t.Fatal(err)
	}
	claims, err := json.Marshal(map[string]any{
		"iss": issuer, "sub": user, "name": "test-only-user", "iat": 1,
		"nats": map[string]any{"type": "user", "version": 2},
	})
	if err != nil {
		t.Fatal(err)
	}
	claimDigest := sha256.Sum256(claims)
	var identifiedClaims map[string]any
	if err := json.Unmarshal(claims, &identifiedClaims); err != nil {
		t.Fatal(err)
	}
	identifiedClaims["jti"] = base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(claimDigest[:])
	claims, err = json.Marshal(identifiedClaims)
	if err != nil {
		t.Fatal(err)
	}
	jwt := base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"JWT","alg":"ed25519-nkey"}`)) + "." + base64.RawURLEncoding.EncodeToString(claims)
	signature, err := accountKey.Sign([]byte(jwt))
	if err != nil {
		t.Fatal(err)
	}
	jwt += "." + base64.RawURLEncoding.EncodeToString(signature)
	credentials := "-----BEGIN NATS USER JWT-----\n" + jwt + "\n------END NATS USER JWT------\n\n-----BEGIN USER NKEY SEED-----\n" + string(seed) + "\n------END USER NKEY SEED------\n"
	if err := os.WriteFile(creds, []byte(credentials), 0o600); err != nil {
		t.Fatal(err)
	}
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatal(err)
	}
	encodedKey := base64.StdEncoding.EncodeToString(key)
	values := map[string]string{
		"ACCOUNT_AUDIT_KEY": encodedKey, "ACCOUNT_CURSOR_KEY": encodedKey,
		"ACCOUNT_ENDPOINT_FINGERPRINT_KEY": encodedKey, "ACCOUNT_ENDPOINT_KEY": encodedKey,
		"ACCOUNT_ENDPOINT_PROOF_PUBLIC_KEY": encodedKey, "ACCOUNT_EXPORT_KEY": encodedKey,
		"ACCOUNT_GATEWAY_PUBLIC_KEY": encodedKey, "ACCOUNT_SERVICE_SIGNING_SEED": encodedKey,
		"ACCOUNT_GATEWAY_CERT_SHA256": leaf.SHA256, "IDENTITY_CERT_SHA256": leaf.SHA256,
		"METADATA_CERT_SHA256": leaf.SHA256, "NOTIFICATION_CERT_SHA256": leaf.SHA256,
		"ACCOUNT_CHAIN_RPC_URL":     "https://api5.mainnet-beta.paxeer.network",
		"ACCOUNT_DOWNLOAD_BASE_URL": "https://downloads.example", "ACCOUNT_EXPORT_BUCKET": "user-exports",
		"ACCOUNT_NATS_CA_FILE": pki.CAFile, "ACCOUNT_NATS_CERT_FILE": leaf.CertFile,
		"ACCOUNT_NATS_CREDENTIALS_FILE": creds, "ACCOUNT_NATS_KEY_FILE": leaf.KeyFile,
		"ACCOUNT_NATS_STREAM": "USERS", "ACCOUNT_NATS_URL": "tls://nats.example:4222",
		"ACCOUNT_TLS_CA_FILE": pki.CAFile, "ACCOUNT_TLS_CERT_FILE": leaf.CertFile, "ACCOUNT_TLS_KEY_FILE": leaf.KeyFile,
		"DATABASE_URL": "postgres://userd_user:test-only-password@pgbouncer:6432/userd",
		"DB_POOL_ID":   "userd_pool", "INSTANCE_ID": "user-1",
		"IDENTITY_SERVICE_URL": "https://identity.example", "METADATA_SERVICE_URL": "https://metadata.example",
		"NOTIFICATION_D11_TOKEN":     strings.Repeat("test-only-inbound-token", 2),
		"NOTIFICATION_SERVICE_TOKEN": strings.Repeat("test-only-service-token", 2),
		"NOTIFICATION_SERVICE_URL":   "https://notifications.example", "REDIS_URL": "redis://redis.example:6379/0",
		"S3_ACCESS_KEY_ID": "test-only-access-key", "S3_BUCKET": "user-assets",
		"S3_ENDPOINT": "https://storage.example", "S3_SECRET_ACCESS_KEY": "test-only-secret-key", "ZONE": "mainnet-beta",
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
			t.Fatalf("required variable %s has no test value", name)
		}
		t.Setenv(name, value)
	}
	for name := range composed {
		found := false
		for _, required := range entry.Required {
			found = found || name == required
		}
		if !found {
			t.Fatalf("composer emitted variable %s outside the contract", name)
		}
	}
}

func TestPlatformDocuments(t *testing.T) {
	root, values := platformValues(t)
	out := t.TempDir()
	if output, err := platformCompose(t, root, values, out).CombinedOutput(); err != nil {
		t.Fatalf("compose user documents: %v: %s", err, output)
	}
	data, err := os.ReadFile(filepath.Join(out, "user.json"))
	if err != nil {
		t.Fatal(err)
	}
	var composed map[string]string
	if err := json.Unmarshal(data, &composed); err != nil {
		t.Fatal(err)
	}
	if len(composed) != 6 {
		t.Fatalf("composed variable count = %d, want 6", len(composed))
	}
	platformEnvironment(t, root, composed)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load composed platform documents: %v", err)
	}
	if cfg.NodeEnv != "production" || cfg.RuntimeProfile != "container" || cfg.Port != 8080 || cfg.BindAddress != "0.0.0.0" || cfg.SecretsProvider != "env" {
		t.Fatal("fixed platform configuration differs")
	}
	if err := cfg.ValidateAccountRuntime(); err != nil {
		t.Fatalf("composed account runtime: %v", err)
	}
	for _, name := range []string{"DB_BUDGET", "EXPECTED_SCHEMA", "ACCOUNT_SUBJECTS"} {
		raw, err := base64.StdEncoding.Strict().DecodeString(composed[name+"_DOCUMENT"])
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		if composed[name+"_SHA256"] != hex.EncodeToString(sum[:]) {
			t.Fatalf("%s document digest differs", name)
		}
		switch name {
		case "DB_BUDGET":
			budget, err := dbproduction.LoadConnectionBudget(raw, cfg.RuntimeBudgetSHA256)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := budget.BudgetPoolOptions("userd", cfg.RuntimePoolID, cfg.RuntimeInstanceID); err != nil {
				t.Fatal(err)
			}
		case "EXPECTED_SCHEMA":
			schema, err := dbproduction.LoadExpectedSchema(raw, cfg.ExpectedSchemaSHA256)
			if err != nil || schema.Service != "user" || schema.RuntimeRole != "userd" {
				t.Fatalf("expected schema: service=%s role=%s error=%v", schema.Service, schema.RuntimeRole, err)
			}
		case "ACCOUNT_SUBJECTS":
			var subjects map[string]string
			if err := config.ReadBoundDocument(cfg.AccountSubjectsFile, cfg.AccountSubjectsSHA256, &subjects); err != nil {
				t.Fatal(err)
			}
			var example struct {
				Subjects map[string]string `json:"subjects"`
			}
			data, err := os.ReadFile(filepath.Join(values, "user.example.json"))
			if err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal(data, &example); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(subjects, example.Subjects) {
				t.Fatal("account subjects differ from deployment values")
			}
			seen := map[string]bool{}
			literal := regexp.MustCompile(`^[A-Za-z0-9_-]+(\.[A-Za-z0-9_-]+)*$`)
			for _, kind := range platformAccountEvents {
				subject := subjects[kind]
				if !literal.MatchString(subject) || len(subject) > 256 || seen[subject] {
					t.Fatalf("invalid subject for %s", kind)
				}
				seen[subject] = true
			}
		}
		t.Run(name+"_changed_byte", func(t *testing.T) {
			changed := bytes.Clone(raw)
			changed[len(changed)/2] ^= 1
			t.Setenv(name+"_DOCUMENT", base64.StdEncoding.EncodeToString(changed))
			if _, err := config.Load(); err == nil || !strings.Contains(err.Error(), name+": runtime_document_binding_mismatch") {
				t.Fatalf("changed %s document: %v", name, err)
			}
		})
		t.Run(name+"_invalid_base64", func(t *testing.T) {
			t.Setenv(name+"_DOCUMENT", "!invalid-base64!")
			if _, err := config.Load(); err == nil || !strings.Contains(err.Error(), name+": runtime_document_invalid") {
				t.Fatalf("malformed %s document: %v", name, err)
			}
		})
		t.Run(name+"_file", func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "document.json")
			if err := os.WriteFile(path, raw, 0o600); err != nil {
				t.Fatal(err)
			}
			t.Setenv(name+"_DOCUMENT", "")
			t.Setenv(name+"_FILE", path)
			if _, err := config.Load(); err != nil {
				t.Fatalf("file %s document: %v", name, err)
			}
		})
	}
}

func TestPlatformDocumentsRejectInvalidValues(t *testing.T) {
	root, values := platformValues(t)
	data, err := os.ReadFile(filepath.Join(values, "user.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	var example struct {
		Subjects map[string]string `json:"subjects"`
	}
	if err := json.Unmarshal(data, &example); err != nil {
		t.Fatal(err)
	}
	for _, kind := range platformAccountEvents {
		t.Run("missing_"+kind, func(t *testing.T) {
			subjects := map[string]any{}
			for key, value := range example.Subjects {
				if key != kind {
					subjects[key] = value
				}
			}
			platformWriteJSON(t, filepath.Join(values, "user.json"), map[string]any{"subjects": subjects})
			if output, err := platformCompose(t, root, values, t.TempDir()).CombinedOutput(); err == nil || !strings.Contains(string(output), "subjects."+kind) {
				t.Fatalf("missing subjects.%s: error=%v output=%s", kind, err, output)
			}
		})
	}
	for _, value := range []any{nil, 125, true, "", "users.*", "users.>", "users..event", "users.event ", strings.Repeat("a", 257), example.Subjects["endpoint.bound"]} {
		t.Run("invalid_subject", func(t *testing.T) {
			subjects := map[string]any{}
			for key, subject := range example.Subjects {
				subjects[key] = subject
			}
			subjects["profile.updated"] = value
			platformWriteJSON(t, filepath.Join(values, "user.json"), map[string]any{"subjects": subjects})
			if output, err := platformCompose(t, root, values, t.TempDir()).CombinedOutput(); err == nil || !strings.Contains(string(output), "subjects.profile.updated") {
				t.Fatalf("invalid subjects.profile.updated: error=%v output=%s", err, output)
			}
		})
	}
	t.Run("missing_subjects", func(t *testing.T) {
		platformWriteJSON(t, filepath.Join(values, "user.json"), map[string]any{})
		if output, err := platformCompose(t, root, values, t.TempDir()).CombinedOutput(); err == nil || !strings.Contains(string(output), "subjects") {
			t.Fatalf("missing subjects: error=%v output=%s", err, output)
		}
	})
	for _, value := range []any{nil, "users", []string{"users.profile.updated"}} {
		t.Run("invalid_subjects_object", func(t *testing.T) {
			platformWriteJSON(t, filepath.Join(values, "user.json"), map[string]any{"subjects": value})
			if output, err := platformCompose(t, root, values, t.TempDir()).CombinedOutput(); err == nil || !strings.Contains(string(output), "subjects") {
				t.Fatalf("invalid subjects: error=%v output=%s", err, output)
			}
		})
	}
	t.Run("deployment_values", func(t *testing.T) {
		subjects := map[string]string{}
		for kind := range example.Subjects {
			subjects[kind] = "custom.users." + kind
		}
		platformWriteJSON(t, filepath.Join(values, "user.json"), map[string]any{"subjects": subjects})
		out := t.TempDir()
		if output, err := platformCompose(t, root, values, out).CombinedOutput(); err != nil {
			t.Fatalf("deployment subjects: %v: %s", err, output)
		}
		data, err := os.ReadFile(filepath.Join(out, "user.json"))
		if err != nil {
			t.Fatal(err)
		}
		var composed map[string]string
		if err := json.Unmarshal(data, &composed); err != nil {
			t.Fatal(err)
		}
		raw, err := base64.StdEncoding.Strict().DecodeString(composed["ACCOUNT_SUBJECTS_DOCUMENT"])
		if err != nil {
			t.Fatal(err)
		}
		var actual map[string]string
		if err := json.Unmarshal(raw, &actual); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(actual, subjects) {
			t.Fatal("composed subjects differ from user.json values")
		}
	})
}
