package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type productionIdentityDocument struct {
	Issuer         string
	Audience       string
	ActiveKey      string
	Keys           []auth.SigningKey
	Domain         string
	URI            string
	AllowedOrigins []string
}

func productionIdentityDocumentFile(t *testing.T, c config.Config, document productionIdentityDocument) config.Config {
	t.Helper()
	raw, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "identity.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	c.IdentityConfigFile = path
	c.IdentityConfigSHA256 = fmt.Sprintf("%x", sha256.Sum256(raw))
	return c
}

func TestProductionIdentityUsesBoundDocumentAndLiveDependencies(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("L41_POSTGRES_URL")
	rpcURL := os.Getenv("L34_ANVIL_URL")
	if dsn == "" || rpcURL == "" {
		t.Fatal("database and chain fixtures are required")
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	schema := "gateway_identity_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{schema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{schema}.Sanitize()+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	ddl, err := delivery.SchemaDDL(schema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	store, err := delivery.NewStore(pool, schema, delivery.Policy{Subjects: map[string]bool{auth.IdentityRevisionSubject: true}, EventTypes: map[string]bool{auth.IdentityRevisionEventType: true}})
	if err != nil {
		t.Fatal(err)
	}
	connected := &gatewayConnectedRuntime{auditKey: []byte(strings.Repeat("a", 32)), store: store}
	const reference = "GATEWAY_TEST_PRODUCTION_IDENTITY_KEY"
	t.Setenv(reference, strings.Repeat("k", 48))
	configuration := config.Config{NodeEnv: "production", SecretsProvider: "env", CORSAllowedOrigins: "https://kindle.test", RPCURL: rpcURL, IdentityLimiterKey: strings.Repeat("l", 32)}
	document := productionIdentityDocument{
		Issuer: "https://kindle.test", Audience: "gateway-owner", ActiveKey: "owner",
		Keys:   []auth.SigningKey{{ID: "owner", SecretReference: reference, ActiveAt: time.Now().Add(-time.Minute), RetireAt: time.Now().Add(time.Hour)}},
		Domain: "kindle.test", URI: "https://kindle.test", AllowedOrigins: []string{"https://kindle.test"},
	}
	for _, tc := range []struct {
		name string
		edit func(*config.Config, *productionIdentityDocument)
		want string
	}{
		{"missing document", func(c *config.Config, _ *productionIdentityDocument) {
			c.IdentityConfigFile = ""
			c.IdentityConfigSHA256 = ""
		}, "binding"},
		{"origin count", func(_ *config.Config, d *productionIdentityDocument) { d.AllowedOrigins = nil }, "origin bindings disagree"},
		{"origin value", func(_ *config.Config, d *productionIdentityDocument) { d.AllowedOrigins[0] = "https://other.test" }, "origin bindings disagree"},
		{"secret provider", func(c *config.Config, _ *productionIdentityDocument) { c.SecretsProvider = "unsupported" }, "provider"},
		{"signing reference", func(_ *config.Config, d *productionIdentityDocument) {
			d.Keys[0].SecretReference = "GATEWAY_TEST_ABSENT_IDENTITY_KEY"
		}, "identity_signing_secret_unavailable"},
		{"active key", func(_ *config.Config, d *productionIdentityDocument) { d.ActiveKey = "absent" }, "FIELD_INVALID"},
		{"chain endpoint", func(c *config.Config, _ *productionIdentityDocument) { c.RPCURL = "://invalid" }, "missing protocol scheme"},
		{"identity domain", func(_ *config.Config, d *productionIdentityDocument) { d.Domain = "other.test" }, "FIELD_INVALID"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := configuration
			copyDocument := document
			copyDocument.Keys = append([]auth.SigningKey(nil), document.Keys...)
			copyDocument.AllowedOrigins = append([]string(nil), document.AllowedOrigins...)
			tc.edit(&candidate, &copyDocument)
			if tc.name != "missing document" {
				candidate = productionIdentityDocumentFile(t, candidate, copyDocument)
			}
			identity, rpc, check, err := newProductionIdentity(ctx, candidate, pool, connected, newIdentityRuntimeMetrics())
			if rpc != nil {
				rpc.Close()
			}
			if identity != nil || check != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("production identity binding: identity=%v check=%v err=%v, want %q", identity, check, err, tc.want)
			}
		})
	}
	configuration = productionIdentityDocumentFile(t, configuration, document)
	identity, rpc, check, err := newProductionIdentity(ctx, configuration, pool, connected, newIdentityRuntimeMetrics())
	if err != nil || identity == nil || rpc == nil || check == nil {
		t.Fatalf("bound production identity: identity=%v rpc=%v check=%v err=%v", identity, rpc, check, err)
	}
	defer rpc.Close()
	if _, err := rpc.BlockNumber(ctx); err != nil {
		t.Fatalf("bound chain connection: %v", err)
	}
	measurement, err := identity.CaptureRuntimeMeasurement()
	if err != nil || measurement.ProcessID != os.Getpid() || measurement.PoolMax != pool.Stat().MaxConns() {
		t.Fatalf("owning identity runtime: measurement=%+v err=%v", measurement, err)
	}
	if err := check(ctx); err != nil {
		t.Fatalf("bound signing key: %v", err)
	}
	t.Setenv(reference, strings.Repeat("z", 48))
	if err := check(ctx); err == nil || !strings.Contains(err.Error(), "identity signing key changed") {
		t.Fatalf("changed signing key: %v", err)
	}
}
