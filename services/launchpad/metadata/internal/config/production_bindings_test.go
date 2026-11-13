package config

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	dbproduction "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
)

func TestDirectConfigurationAndDocumentBounds(t *testing.T) {
	base := Config{DatabaseURL: "postgres://reader:secret@localhost:5432/media", RedisURL: "redis://localhost:6379", S3Endpoint: "https://objects.example", S3Bucket: "media", S3AccessKeyID: "reader", S3SecretAccessKey: "secret", LogLevel: "info", Port: 8080, MaxLogoSizeBytes: 1024, MaxBannerSizeBytes: 2048}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, want string
		change     func(*Config)
	}{
		{"redis", "REDIS_URL", func(c *Config) { c.RedisURL = "" }},
		{"bucket", "S3_BUCKET", func(c *Config) { c.S3Bucket = "" }},
		{"access key", "S3_ACCESS_KEY_ID", func(c *Config) { c.S3AccessKeyID = "" }},
		{"secret key", "S3_SECRET_ACCESS_KEY", func(c *Config) { c.S3SecretAccessKey = "" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := base
			tc.change(&candidate)
			if err := candidate.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("configuration = %v, want %q", err, tc.want)
			}
		})
	}
	document := []byte(`{"schemaVersion":3}`)
	digest := sha256.Sum256(document)
	path := filepath.Join(t.TempDir(), "runtime.json")
	if err := os.WriteFile(path, document, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path, inline, digest string
	}{
		{"missing source", "", "", hex.EncodeToString(digest[:])},
		{"missing file", path + ".missing", "", hex.EncodeToString(digest[:])},
		{"directory", filepath.Dir(path), "", hex.EncodeToString(digest[:])},
		{"oversized inline", "", strings.Repeat("a", 1398105), hex.EncodeToString(digest[:])},
		{"empty inline", "", base64.StdEncoding.EncodeToString(nil), hex.EncodeToString(digest[:])},
		{"digest mismatch", path, "", strings.Repeat("0", 64)},
		{"invalid inline encoding", "", "!invalid-base64!", hex.EncodeToString(digest[:])},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := boundBytes(tc.path, tc.inline, tc.digest); err == nil {
				t.Fatal("unbound document accepted")
			}
		})
	}
	if got, err := boundBytes(path, "", hex.EncodeToString(digest[:])); err != nil || string(got) != string(document) {
		t.Fatalf("bound document = %s, %v", got, err)
	}
}

func TestOwnedDatabaseRoutePinsMetadataSchema(t *testing.T) {
	configuration := Config{MetadataSchema: "metadata_runtime", DatabaseURL: "postgres://reader:secret@localhost:5432/media?sslmode=disable&search_path=public"}
	route, err := configuration.ownedDatabaseURL()
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(route)
	if err != nil || parsed.Query().Get("search_path") != `"metadata_runtime",pg_catalog` || parsed.Query().Get("sslmode") != "disable" {
		t.Fatalf("owned route = %q, %v", route, err)
	}
	for _, schema := range []string{"", "public", "pg_catalog", "Invalid-Schema"} {
		configuration.MetadataSchema = schema
		if _, err := configuration.ownedDatabaseURL(); err == nil {
			t.Fatalf("schema %q admitted", schema)
		}
	}
	configuration.MetadataSchema = "metadata_runtime"
	for _, database := range []string{"", "file:///tmp/media", "postgres://"} {
		configuration.DatabaseURL = database
		if _, err := configuration.ownedDatabaseURL(); err == nil {
			t.Fatalf("database route %q admitted", database)
		}
	}
}

func TestProductionRuntimeBindingErrors(t *testing.T) {
	ctx := context.Background()
	configuration := Config{NodeEnv: "production", RuntimeProfile: "container", Port: 8081, BindAddress: "0.0.0.0"}
	if _, err := configuration.OpenRuntimePool(ctx, "metadatad", runtimedb.PoolOptions{}); err == nil || !strings.Contains(err.Error(), "container_bind_mismatch") {
		t.Fatalf("container route: %v", err)
	}
	configuration.RuntimeProfile = "owned_local"
	if _, err := configuration.OpenRuntimePool(ctx, "metadatad", runtimedb.PoolOptions{}); err == nil || !strings.Contains(err.Error(), "owned_loopback_required") {
		t.Fatalf("owned route: %v", err)
	}
	configuration.BindAddress = "127.0.0.1"
	configuration.RuntimeBudgetDocument = "e30="
	configuration.RuntimePoolID = "direct"
	configuration.RuntimeInstanceID = "replica_1"
	if _, err := configuration.OpenRuntimePool(ctx, "metadatad", runtimedb.PoolOptions{}); err == nil || !strings.Contains(err.Error(), "runtime_budget_binding_mismatch") {
		t.Fatalf("unbound budget: %v", err)
	}
	if err := configuration.CheckRuntimeSchema(ctx, nil); err == nil || !strings.Contains(err.Error(), "runtime_schema_configuration_missing") {
		t.Fatalf("missing expected schema: %v", err)
	}
	configuration.ExpectedSchemaDocument = "e30="
	configuration.ExpectedSchemaSHA256 = strings.Repeat("a", 64)
	if err := configuration.CheckRuntimeSchema(ctx, nil); err == nil {
		t.Fatal("unbound expected schema admitted")
	}
	configuration.ExpectedSchemaSHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte("{}")))
	if err := configuration.CheckRuntimeSchema(ctx, nil); err == nil || !strings.Contains(err.Error(), "complete trusted expected-schema") {
		t.Fatalf("incomplete expected schema = %v", err)
	}
}

func TestProductionRuntimePoolUsesBoundBudget(t *testing.T) {
	ctx := context.Background()
	_, dsn := internaltest.NewPostgresURL(t)
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	user := parsed.User.Username()
	if user == "" {
		t.Fatal("database fixture requires an identified owner")
	}
	port := uint16(5432)
	if parsed.Port() != "" {
		var n int
		if _, err := fmt.Sscan(parsed.Port(), &n); err != nil || n < 1 || n > 65535 {
			t.Fatalf("database port: %v", err)
		}
		port = uint16(n)
	}
	budget := dbproduction.ConnectionBudget{
		Identity:    "metadata_runtime_budget",
		Targets:     []dbproduction.PostgresTarget{{ID: "primary", Member: "fixture_primary", Host: parsed.Hostname(), Port: port, MaxConnections: 100}},
		Operational: []dbproduction.OperationalBudget{{ID: "migrator", Target: "primary", Category: "migration", Connections: 1}},
	}
	for _, role := range []string{"apid:rest", "apid:sockets", "indexerd", "chartsd", "statsd", "pnld", "rankingd", "mediagatewayd", "userd", "sociald", "metadatad", "livestreamd"} {
		roleUser := strings.ReplaceAll(role, ":", "_")
		poolID := "interactive"
		connections := int64(1)
		if role == "metadatad" {
			roleUser, poolID, connections = user, "direct", 3
		}
		budget.Clients = append(budget.Clients, dbproduction.ClientBudget{Role: role, PoolID: poolID, Target: "primary", Transport: runtimedb.TransportDirect, Alias: strings.TrimPrefix(parsed.Path, "/"), User: roleUser, QueryClass: runtimedb.Interactive, Processes: dbproduction.LiveProcesses{Replicas: 1, Maximum: 1}, MaxConnections: connections})
	}
	if err := budget.Validate(); err != nil {
		t.Fatalf("runtime budget: %v", err)
	}
	raw, err := json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	cfg := Config{NodeEnv: "production", RuntimeProfile: "owned_local", BindAddress: "127.0.0.1", DatabaseURL: dsn, MetadataSchema: "metadata", RuntimePoolID: "direct", RuntimeInstanceID: "replica_1", RuntimeBudgetDocument: base64.StdEncoding.EncodeToString(raw), RuntimeBudgetSHA256: hex.EncodeToString(digest[:])}
	if _, err := cfg.OpenRuntimePool(ctx, "metadatad", runtimedb.PoolOptions{}); err == nil || !strings.Contains(err.Error(), "runtime_schema_configuration_missing") {
		t.Fatalf("unbound schema readiness: %v", err)
	}
	cfg.RuntimeBudgetSHA256 = strings.Repeat("0", 64)
	if _, err := cfg.OpenRuntimePool(ctx, "metadatad", runtimedb.PoolOptions{}); err == nil || !strings.Contains(err.Error(), "runtime_budget_binding_mismatch") {
		t.Fatalf("budget digest: %v", err)
	}
	cfg.RuntimeBudgetSHA256 = hex.EncodeToString(digest[:])
	if _, err := cfg.OpenRuntimePool(ctx, "unbudgeted", runtimedb.PoolOptions{}); err == nil {
		t.Fatal("role without a connection budget admitted")
	}
	invalid := []byte("{}")
	invalidDigest := sha256.Sum256(invalid)
	cfg.RuntimeBudgetDocument = base64.StdEncoding.EncodeToString(invalid)
	cfg.RuntimeBudgetSHA256 = hex.EncodeToString(invalidDigest[:])
	if _, err := cfg.OpenRuntimePool(ctx, "metadatad", runtimedb.PoolOptions{}); err == nil {
		t.Fatal("incomplete connection budget admitted")
	}
}
