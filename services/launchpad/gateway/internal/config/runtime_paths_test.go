package config

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	dbproduction "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
)

func TestProductionSecretsBindAndDetectRotation(t *testing.T) {
	values := map[string]string{}
	for _, name := range []string{"REDIS_URL", "GATEWAY_JWT_SECRET", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY", "DATABASE_URL", "IDENTITY_SERVICE_TOKEN", "IDENTITY_LIMITER_KEY", "IDENTITY_SIGNING_SECRET", "RPC_URL"} {
		value := strings.Repeat(strings.ToLower(name[:1]), 32)
		t.Setenv(name, value)
		values[name] = value
	}
	config := Config{NodeEnv: nodeEnvironmentProduction, SecretsProvider: "env", SecretsRefreshSeconds: 60}
	check, stop, err := config.ResolveProductionSecrets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if config.RedisURL != values["REDIS_URL"] || config.IdentitySigningSecret != values["IDENTITY_SIGNING_SECRET"] || config.DatabaseURL != values["DATABASE_URL"] || config.RPCURL != values["RPC_URL"] {
		t.Fatal("required production values were not bound")
	}
	if err := check(context.Background()); err != nil {
		t.Fatalf("unchanged required values: %v", err)
	}
	t.Setenv("RPC_URL", strings.Repeat("r", 33))
	if err := check(context.Background()); err == nil || !strings.Contains(err.Error(), "required_secret_rotated_restart_required") {
		t.Fatalf("rotated required value: %v", err)
	}
	if err := check(context.Background()); err == nil {
		t.Fatal("rotation failure did not remain observable")
	}
}

func TestProductionSecretsRefreshObservesUnavailableRequiredValue(t *testing.T) {
	for _, name := range []string{"REDIS_URL", "GATEWAY_JWT_SECRET", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY", "DATABASE_URL", "IDENTITY_SERVICE_TOKEN", "IDENTITY_LIMITER_KEY", "IDENTITY_SIGNING_SECRET", "RPC_URL"} {
		t.Setenv(name, strings.Repeat("v", 32))
	}
	configuration := Config{NodeEnv: nodeEnvironmentProduction, SecretsProvider: "env", SecretsRefreshSeconds: 1}
	check, stop, err := configuration.ResolveProductionSecrets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if err := check(context.Background()); err != nil {
		t.Fatalf("initial bound values: %v", err)
	}
	t.Setenv("RPC_URL", "")
	time.Sleep(1100 * time.Millisecond)
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if err := check(context.Background()); err != nil {
			if !strings.Contains(err.Error(), "required_secret_unavailable: RPC_URL") {
				t.Fatalf("required value removal: %v", err)
			}
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("removed required value remained available")
}

func TestProductionSecretsRejectIncompleteBinding(t *testing.T) {
	for _, tc := range []Config{
		{NodeEnv: "unknown"},
		{NodeEnv: nodeEnvironmentProduction, SecretsRefreshSeconds: 0},
		{NodeEnv: nodeEnvironmentProduction, SecretsRefreshSeconds: 61},
		{NodeEnv: nodeEnvironmentProduction, SecretsRefreshSeconds: 1, SecretsProvider: "unsupported"},
		{NodeEnv: nodeEnvironmentProduction, SecretsRefreshSeconds: 1, SecretsProvider: "env"},
	} {
		if _, stop, err := tc.ResolveProductionSecrets(context.Background()); err == nil {
			if stop != nil {
				stop()
			}
			t.Fatalf("incomplete production binding admitted: %+v", tc)
		}
	}
	dev := Config{NodeEnv: "development"}
	check, stop, err := dev.ResolveProductionSecrets(context.Background())
	if err != nil || check != nil || stop == nil {
		t.Fatalf("development secret path: check available=%t stop available=%t err=%v", check != nil, stop != nil, err)
	}
	stop()
}

func TestBoundRuntimeDocumentReadsExactBytes(t *testing.T) {
	raw := []byte(`{"service":"gateway"}`)
	hash := sha256.Sum256(raw)
	digest := hex.EncodeToString(hash[:])
	path := filepath.Join(t.TempDir(), "runtime.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	fileBytes, err := boundBytes(path, "", digest)
	if err != nil || string(fileBytes) != string(raw) {
		t.Fatalf("file binding: %q %v", fileBytes, err)
	}
	inline := base64.StdEncoding.EncodeToString(raw)
	inlineBytes, err := boundBytes("", inline, digest)
	if err != nil || string(inlineBytes) != string(raw) {
		t.Fatalf("inline binding: %q %v", inlineBytes, err)
	}
	for _, tc := range []struct{ path, inline, digest string }{
		{path, inline, digest},
		{"", "", digest},
		{path, "", strings.Repeat("0", 64)},
		{"", "invalid-base64", digest},
		{filepath.Join(t.TempDir(), "absent"), "", digest},
	} {
		if _, err := boundBytes(tc.path, tc.inline, tc.digest); err == nil {
			t.Fatalf("invalid document binding accepted: %+v", tc)
		}
	}
	var decoded struct{ Service string }
	if err := ReadBoundDocument(path, digest, &decoded); err != nil || decoded.Service != "gateway" {
		t.Fatalf("decoded runtime document: %+v %v", decoded, err)
	}
	trailing := []byte(`{"service":"gateway"} {"service":"other"}`)
	trailingHash := sha256.Sum256(trailing)
	if err := decodeBoundDocument("", base64.StdEncoding.EncodeToString(trailing), hex.EncodeToString(trailingHash[:]), &decoded); err == nil {
		t.Fatal("trailing document accepted")
	}
}

func TestBoundRuntimeDocumentRejectsInvalidSources(t *testing.T) {
	root := t.TempDir()
	raw := []byte(`{"service":"media/gateway"}`)
	actual := sha256.Sum256(raw)
	digest := hex.EncodeToString(actual[:])
	for _, tc := range []struct {
		name, content, want string
	}{
		{"empty", "", "runtime_document_invalid"},
		{"oversized", strings.Repeat("a", 1048577), "runtime_document_invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(root, tc.name)
			if err := os.WriteFile(path, []byte(tc.content), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := boundBytes(path, "", digest); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("invalid source accepted: %v", err)
			}
		})
	}
	if _, err := boundBytes(root, "", digest); err == nil || !strings.Contains(err.Error(), "runtime_document_invalid") {
		t.Fatalf("directory source accepted: %v", err)
	}
	if _, err := boundBytes("", strings.Repeat("a", 1398105), digest); err == nil || !strings.Contains(err.Error(), "runtime_document_invalid") {
		t.Fatalf("unbounded inline source accepted: %v", err)
	}
	for _, content := range []string{`{"service":`, `{"unknown":"value"}`} {
		bytes := []byte(content)
		hash := sha256.Sum256(bytes)
		var document struct{ Service string }
		if err := decodeBoundDocument("", base64.StdEncoding.EncodeToString(bytes), hex.EncodeToString(hash[:]), &document); err == nil {
			t.Fatalf("invalid document accepted: %q", content)
		}
	}
	configuration := Config{GatewayRuntimeDocument: base64.StdEncoding.EncodeToString(raw), GatewayRuntimeSHA256: digest}
	configuration.registerInlineDocuments()
	var document struct{ Service string }
	if err := ReadBoundDocument("", digest, &document); err != nil || document.Service != "media/gateway" {
		t.Fatalf("inline bound document: %+v %v", document, err)
	}
}

func TestRuntimePoolsRejectInvalidProductionProfile(t *testing.T) {
	for _, tc := range []Config{
		{NodeEnv: nodeEnvironmentProduction, RuntimeProfile: "container", Port: 3000, BindAddress: "127.0.0.1"},
		{NodeEnv: nodeEnvironmentProduction, RuntimeProfile: "owned_local", BindAddress: "0.0.0.0"},
		{NodeEnv: nodeEnvironmentProduction, RuntimeProfile: "unknown"},
		{NodeEnv: nodeEnvironmentProduction, RuntimeProfile: "owned_local", BindAddress: "127.0.0.1"},
	} {
		if pool, err := tc.OpenRuntimePool(context.Background(), "mediagatewayd", runtimedb.PoolOptions{}); err == nil {
			if pool != nil {
				pool.Close()
			}
			t.Fatalf("invalid runtime profile accepted: %+v", tc)
		}
	}
	if identity, control, err := (&Config{}).OpenSplitRuntimePools(context.Background(), RuntimePoolSplit{}); err == nil || identity != nil || control != nil {
		t.Fatalf("missing pool split accepted: identity=%v control=%v err=%v", identity, control, err)
	}
}

func TestProductionRuntimeDocumentsRequireBoundSchemaAndBudget(t *testing.T) {
	ctx := context.Background()
	const digest = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	configuration := Config{NodeEnv: nodeEnvironmentProduction, RuntimeProfile: "owned_local", BindAddress: "127.0.0.1", RuntimePoolID: "identity", RuntimeInstanceID: "gateway", RuntimeBudgetFile: filepath.Join(t.TempDir(), "absent-budget.json"), RuntimeBudgetSHA256: digest}
	if pool, err := configuration.OpenRuntimePool(ctx, "mediagatewayd", runtimedb.PoolOptions{}); pool != nil || err == nil || !strings.Contains(err.Error(), "runtime_budget_binding_mismatch") {
		if pool != nil {
			pool.Close()
		}
		t.Fatalf("unbound runtime budget: pool=%v err=%v", pool, err)
	}
	split := RuntimePoolSplit{IdentityPoolID: "identity", IdentityMaxConnections: 1, ControlPoolID: "control", ControlMaxConnections: 1, ControlCredentialReference: "GATEWAY_TEST_CONTROL_DATABASE"}
	if identity, control, err := configuration.OpenSplitRuntimePools(ctx, split); identity != nil || control != nil || err == nil || !strings.Contains(err.Error(), "runtime_budget_binding_mismatch") {
		t.Fatalf("unbound split budget: identity=%v control=%v err=%v", identity, control, err)
	}
	configuration.ExpectedSchemaFile = filepath.Join(t.TempDir(), "absent-schema.json")
	configuration.ExpectedSchemaSHA256 = digest
	if err := configuration.CheckRuntimeSchema(ctx, nil); err == nil {
		t.Fatal("unbound expected schema accepted")
	}
	expected := dbproduction.ExpectedSchema{Service: "other-service", RuntimeRole: "identity_runtime", RegistryDigest: digest, InterfaceDigest: digest, ReaderVersion: 1, Migrations: map[int64]string{1: digest}, Schemas: map[string]string{"gateway": digest}, Epoch: 1, Region: "local", Privileges: []dbproduction.Grant{{Schema: "gateway", Object: "owned", Privileges: []string{"SELECT"}}}}
	raw, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	hash := sha256.Sum256(raw)
	configuration.ExpectedSchemaFile = ""
	configuration.ExpectedSchemaDocument = base64.StdEncoding.EncodeToString(raw)
	configuration.ExpectedSchemaSHA256 = hex.EncodeToString(hash[:])
	if err := configuration.CheckRuntimeSchema(ctx, nil); err == nil || !strings.Contains(err.Error(), "runtime_schema_service_mismatch") {
		t.Fatalf("different expected service: %v", err)
	}
}
