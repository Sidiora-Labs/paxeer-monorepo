package app

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func productionConstructorSchema(t *testing.T, db internaltest.RuntimeDatabase) (string, string) {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = conn.Close(ctx) }()
	root := internaltest.RepositoryRoot(t)
	base := filepath.Join(root, "migrations")
	registry, err := os.ReadFile(filepath.Join(base, "registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	roles, err := os.ReadFile(filepath.Join(base, "grants.json"))
	if err != nil {
		t.Fatal(err)
	}
	var grants struct {
		Roles []production.RoleManifest `json:"roles"`
	}
	if err := json.Unmarshal(roles, &grants); err != nil {
		t.Fatal(err)
	}
	want := production.ExpectedSchema{Service: "core/api", RuntimeRole: db.RuntimeRole, RegistryDigest: production.Checksum(registry), InterfaceDigest: production.Checksum(roles), ReaderVersion: 1, Migrations: map[int64]string{}, Schemas: map[string]string{}, Epoch: db.Epoch, Region: db.Region}
	for _, role := range grants.Roles {
		if role.Service == "core/api" && role.RuntimeRole == "api_rest_runtime" {
			for _, grant := range role.Grants {
				if grant.Schema == "api_stream" || grant.Schema == "recovery" || grant.Schema == "migration_control" {
					want.Privileges = append(want.Privileges, grant)
				}
			}
			break
		}
	}
	rows, err := conn.Query(ctx, "SELECT migration_id,checksum FROM migration_control.applied_migrations")
	if err != nil {
		t.Fatal(err)
	}
	for rows.Next() {
		var id int64
		var checksum string
		if err := rows.Scan(&id, &checksum); err != nil {
			t.Fatal(err)
		}
		want.Migrations[id] = checksum
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	rows.Close()
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, schema := range []string{"api_stream", "recovery"} {
		fingerprint, err := production.CatalogFingerprint(ctx, tx, schema)
		if err != nil {
			t.Fatal(err)
		}
		want.Schemas[schema] = fingerprint
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := production.RecordRuntimeReadiness(ctx, conn, want); err != nil {
		t.Fatal(err)
	}
	role := pgx.Identifier{db.RuntimeRole}.Sanitize()
	for _, statement := range []string{
		"GRANT USAGE ON SCHEMA migration_control TO " + role,
		"GRANT EXECUTE ON FUNCTION migration_control.readiness(text) TO " + role,
	} {
		if _, err := conn.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "expected-schema.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path, production.Checksum(raw)
}

func productionConstructorBudget(t *testing.T, runtimeURL string) (string, string) {
	t.Helper()
	parsed, err := pgxpool.ParseConfig(runtimeURL)
	if err != nil {
		t.Fatal(err)
	}
	roles := []string{"apid:rest", "apid:sockets", "indexerd", "chartsd", "statsd", "pnld", "rankingd", "mediagatewayd", "userd", "sociald", "metadatad", "livestreamd"}
	budget := production.ConnectionBudget{Identity: "api_runtime_fixture", Targets: []production.PostgresTarget{{ID: "primary", Member: "postgres", Host: parsed.ConnConfig.Host, Port: parsed.ConnConfig.Port, MaxConnections: 100, ProtectedSlots: 10}}, Operational: []production.OperationalBudget{{ID: "migrator", Target: "primary", Category: "migration", Connections: 1}, {ID: "admin", Target: "primary", Category: "admin", Connections: 1}}}
	for _, role := range roles {
		user := strings.ReplaceAll(role, ":", "_")
		if role == "apid:rest" {
			user = parsed.ConnConfig.User
		}
		budget.Clients = append(budget.Clients, production.ClientBudget{Role: role, PoolID: "runtime", Target: "primary", Transport: runtimedb.TransportDirect, Alias: parsed.ConnConfig.Database, User: user, QueryClass: runtimedb.Interactive, Processes: production.LiveProcesses{Replicas: 1, Maximum: 1}, MaxConnections: 1})
	}
	if err := budget.Validate(); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "connection-budget.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path, fmt.Sprintf("%x", sha256.Sum256(raw))
}

func TestProductionGatewayRuntimeBindingAndStatus(t *testing.T) {
	database := internaltest.NewAPIRuntimeDatabase(t, "api_rest_runtime", "api_stream", "recovery")
	t.Setenv("DATABASE_URL", database.RuntimeDSN)
	t.Setenv("REDIS_URL", internaltest.NewRedisURL(t))
	t.Setenv("STREAM_SIGNING_KEY", strings.Repeat("k", 32))
	t.Setenv("IDENTITY_SIGNING_KEY", strings.Repeat("i", 32))
	budgetPath, budgetDigest := productionConstructorBudget(t, database.RuntimeDSN)
	cfg := config.Config{NodeEnv: "production", APIRole: "rest", RuntimeProfile: "owned_local", BindAddress: "127.0.0.1", Port: 8080, RuntimeMetricsEnvironment: "qualification", RuntimeZone: "zone-a", SecretsProvider: "env", SecretsRefreshSeconds: 30,
		RuntimeBudgetFile: budgetPath, RuntimeBudgetSHA256: budgetDigest, RuntimePoolID: "runtime", RuntimeInstanceID: "api_fixture", AdmissionCapacity: 100, AdmissionMaxWaiting: 100}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := New(context.Background(), &cfg, logger)
	if app != nil || err == nil || !strings.Contains(err.Error(), "runtime_schema_configuration_missing") {
		t.Fatalf("unbound production schema: app=%t err=%v", app != nil, err)
	}
	cfg.ExpectedSchemaFile, cfg.ExpectedSchemaSHA256 = productionConstructorSchema(t, database)
	cfg.RuntimeInstanceID = "api_fixture_bound"
	app, err = New(context.Background(), &cfg, logger)
	if app != nil || err == nil || !strings.Contains(err.Error(), "IDENTITY_CA_FILE and IDENTITY_CA_PEM is required") {
		t.Fatalf("bound production schema: app=%t err=%v", app != nil, err)
	}
	pki := testpki.New(t)
	server := pki.Leaf("identity-server", "identity.test")
	client := pki.Leaf("api-client", "api.test")
	ca, err := os.ReadFile(pki.CAFile)
	if err != nil {
		t.Fatal(err)
	}
	identity := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	identity.TLS = &tls.Config{Certificates: []tls.Certificate{server.Pair}}
	identity.StartTLS()
	defer identity.Close()
	stats := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer stats.Close()
	t.Setenv("IDENTITY_SIGNING_KEY", base64.RawStdEncoding.EncodeToString(testpki.Ed25519(t).Private))
	cfg.IdentityURL = identity.URL
	cfg.IdentityPeerCertSHA256 = server.SHA256
	cfg.IdentityCAPEM = string(ca)
	cfg.IdentityClientCertPEM = string(client.CertPEM)
	cfg.IdentityClientKeyPEM = string(client.KeyPEM)
	cfg.IdentityIssuer = "api"
	cfg.IdentityAudience = "identity"
	cfg.StatsURL = stats.URL
	cfg.StreamSchema = "api_stream"
	cfg.RateLimitMax = 100
	cfg.RateLimitWindowSec = 60
	cfg.MaxInFlightRequests = 100
	cfg.WSMaxConnections, cfg.WSMaxPerIP = 10, 10
	cfg.SSEMaxConnections, cfg.SSEMaxPerIP = 10, 10
	cfg.ClientSendBuffer, cfg.CoalesceFlushMS = 16, 20
	readConfig := store.AccountingReadConfig{ChartsSchema: "candles", StatsSchema: "stats", ChartsDeliverySchema: "charts_delivery", StatsDeliverySchema: "stats_delivery", StreamSchema: "api_stream", ProjectionVersion: "v1", ProjectionGeneration: "g1", ChartsConsumerID: "charts", StatsConsumerID: "stats", SourceStream: "chain125-canonical", SourcePartitionSchema: "indexer"}
	readDocument, err := json.Marshal(readConfig)
	if err != nil {
		t.Fatal(err)
	}
	cfg.AccountingReadFile = filepath.Join(t.TempDir(), "accounting-read.json")
	if err := os.WriteFile(cfg.AccountingReadFile, readDocument, 0600); err != nil {
		t.Fatal(err)
	}
	cfg.AccountingReadSHA256 = production.Checksum(readDocument)
	cfg.RuntimeInstanceID = "api_fixture_complete"
	app, err = New(context.Background(), &cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/status", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), `"status":"ok"`) || app.privateStream == nil || app.authority == nil || app.sqlDB == nil {
		t.Fatalf("production runtime: status=%d body=%s stream=%t authority=%t sql=%t", response.Code, response.Body.String(), app.privateStream != nil, app.authority != nil, app.sqlDB != nil)
	}
}
