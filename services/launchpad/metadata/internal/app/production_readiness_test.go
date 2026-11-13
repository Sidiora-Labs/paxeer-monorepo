package app_test

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	dbproduction "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/app"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
)

func TestProductionStartupRequiresBoundSchema(t *testing.T) {
	_, dsn := internaltest.NewPostgresURL(t)
	redisURL := internaltest.NewRedisURL(t)
	objects := internaltest.NewMinIO(t, "metadata-runtime")
	quarantine := internaltest.NewMinIO(t, "metadata-runtime-quarantine")
	chain := newDevChain(t)
	scannerHost, scannerPort := internaltest.NewClamd(t)
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	port := 5432
	if parsed.Port() != "" {
		port, err = strconv.Atoi(parsed.Port())
		if err != nil || port < 1 || port > 65535 {
			t.Fatalf("database port: %v", err)
		}
	}
	budget := dbproduction.ConnectionBudget{
		Identity:    "metadata_runtime_fixture",
		Targets:     []dbproduction.PostgresTarget{{ID: "primary", Member: "fixture_primary", Host: parsed.Hostname(), Port: uint16(port), MaxConnections: 100}},
		Operational: []dbproduction.OperationalBudget{{ID: "migrator", Target: "primary", Category: "migration", Connections: 1}},
	}
	for _, role := range []string{"apid:rest", "apid:sockets", "indexerd", "chartsd", "statsd", "pnld", "rankingd", "mediagatewayd", "userd", "sociald", "metadatad", "livestreamd"} {
		user, poolID, connections := strings.ReplaceAll(role, ":", "_"), "interactive", int64(1)
		if role == "metadatad" {
			user, poolID, connections = parsed.User.Username(), "direct", 3
		}
		budget.Clients = append(budget.Clients, dbproduction.ClientBudget{Role: role, PoolID: poolID, Target: "primary", Transport: runtimedb.TransportDirect, Alias: strings.TrimPrefix(parsed.Path, "/"), User: user, QueryClass: runtimedb.Interactive, Processes: dbproduction.LiveProcesses{Replicas: 1, Maximum: 1}, MaxConnections: connections})
	}
	if err := budget.Validate(); err != nil {
		t.Fatalf("runtime budget: %v", err)
	}
	budgetBytes, err := json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	budgetHash := sha256.Sum256(budgetBytes)
	publicationDocument := []byte(`{"schemaVersion":3,"environment":"paxeer125-production","envelopeHeader":"X-Kindle-Operation","signatureHeader":"X-Kindle-Operation-Signature"}`)
	publicationHash := sha256.Sum256(publicationDocument)
	secret := make([]byte, 32)
	if _, err := rand.Read(secret); err != nil {
		t.Fatal(err)
	}
	t.Setenv("DATABASE_URL", dsn)
	t.Setenv("REDIS_URL", redisURL)
	t.Setenv("S3_ACCESS_KEY_ID", objects.AccessKeyID)
	t.Setenv("S3_SECRET_ACCESS_KEY", objects.SecretAccessKey)
	t.Setenv("RPC_URL", chain.url)
	t.Setenv("METADATA_PUBLICATION_AUDIT_KEY", base64.StdEncoding.EncodeToString(secret))
	t.Setenv("IDENTITY_SERVICE_TOKEN", hex.EncodeToString(secret))
	worker, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{
		NodeEnv: "production", Ingress: config.IngressEdge, RasterIsolation: config.IsolationRlimit,
		ModerationSource: config.ModerationDatabase, ModerationSchema: "chat", MetadataSchema: "metadata", CanonicalSchema: "indexer", CanonicalStreamID: "paxeer125-canonical",
		IdentityAudience: "https://metadata.sidiora.test", IdentityServiceURL: "https://app.sidiora.test", IdentityServiceToken: hex.EncodeToString(secret),
		QuarantineBucket: quarantine.Bucket, ApprovedBucket: objects.Bucket, RasterExecutable: worker,
		ClamAVHost: scannerHost, ClamAVPort: scannerPort, PublicURL: "https://metadata.sidiora.test",
		PublicationConfigDocument: base64.StdEncoding.EncodeToString(publicationDocument), PublicationConfigSHA256: hex.EncodeToString(publicationHash[:]), PublicationAuditKey: base64.StdEncoding.EncodeToString(secret),
		MediaScanIntervalSeconds: 5, MediaExpiryIntervalSeconds: 300,
		RuntimeProfile: "owned_local", BindAddress: "127.0.0.1", Port: 3000, RuntimePoolID: "direct", RuntimeInstanceID: "replica_1",
		RuntimeBudgetDocument: base64.StdEncoding.EncodeToString(budgetBytes), RuntimeBudgetSHA256: hex.EncodeToString(budgetHash[:]),
		SecretsProvider: "env", SecretsRefreshSeconds: 30, RuntimeMetricsEnvironment: "qualification", RuntimeZone: "zone-a",
		DatabaseURL: dsn, RedisURL: redisURL,
		S3Endpoint: objects.Endpoint, S3Bucket: objects.Bucket, S3Region: "us-east-1",
	}
	if err := cfg.ValidateMediaRuntime(); err != nil {
		t.Fatalf("production media configuration: %v", err)
	}
	instance, err := app.New(context.Background(), cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if instance != nil {
		instance.Close()
		t.Fatal("production startup admitted an unbound schema")
	}
	if err == nil || !strings.Contains(err.Error(), "runtime_schema_configuration_missing") {
		t.Fatalf("production schema readiness: %v", err)
	}
}
