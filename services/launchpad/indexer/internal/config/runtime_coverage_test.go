package config

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	dbproduction "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/nats-io/nats.go"
)

func TestArchiveTuningRetainsBoundsAndDefaults(t *testing.T) {
	for _, tc := range []struct {
		concurrency, ahead, attempt int
		valid                       bool
	}{
		{0, 0, 0, true}, {1, 1, 500, true}, {96, 1024, 15000, true}, {-1, 0, 0, false}, {97, 0, 0, false}, {1, -1, 500, false}, {1, 1025, 500, false}, {1, 1, 499, false}, {1, 1, 15001, false},
	} {
		c := Config{HistoricalConcurrency: tc.concurrency, HistoricalReadahead: tc.ahead, HistoricalAttemptTimeoutMS: tc.attempt}
		got, err := c.ArchiveTuning()
		if !tc.valid {
			if err == nil {
				t.Fatalf("unbounded archive reads: %+v", got)
			}
			continue
		}
		concurrency, attempt, ahead := tc.concurrency, tc.attempt, tc.ahead
		if concurrency == 0 {
			concurrency = 48
		}
		if attempt == 0 {
			attempt = 8000
		}
		if ahead == 0 {
			ahead = -1
		}
		if err != nil || got.Concurrency != concurrency || got.Readahead != ahead || got.AttemptTimeout != time.Duration(attempt)*time.Millisecond {
			t.Fatalf("archive tuning %+v: %v", got, err)
		}
	}
	for _, raw := range []string{"https://archive.example/rpc", "http://127.0.0.1/rpc"} {
		if err := requireHTTPURL("archive", raw); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{"ftp://archive.example", "http:///rpc", "http://archive.example/#fragment", "%"} {
		if err := requireHTTPURL("archive", raw); err == nil {
			t.Fatalf("invalid archive URL %q", raw)
		}
	}
}

func TestBoundDocumentFileAndJSONAdmission(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "document.json")
	raw := `{"name":"bound"}`
	if err := os.WriteFile(path, []byte(raw), 0600); err != nil {
		t.Fatal(err)
	}
	got, err := boundBytes(path, "", digestOf(raw))
	if err != nil || string(got) != raw {
		t.Fatalf("bound file %q %v", got, err)
	}
	link := filepath.Join(dir, "linked.json")
	if err = os.Symlink(path, link); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ path, inline, digest string }{
		{filepath.Join(dir, "missing"), "", digestOf(raw)}, {dir, "", digestOf(raw)}, {link, "", digestOf(raw)},
		{path, "", digestOf("changed")}, {"", base64.StdEncoding.EncodeToString(nil), digestOf("")}, {"", strings.Repeat("x", 1398105), digestOf(raw)},
	} {
		if _, err = boundBytes(tc.path, tc.inline, tc.digest); err == nil {
			t.Fatal("unbound document accepted")
		}
	}
	for _, body := range []string{`{`, `{"unknown":1}`, raw + ` {}`} {
		var out struct {
			Name string `json:"name"`
		}
		if err = decodeBoundDocument("", base64.StdEncoding.EncodeToString([]byte(body)), digestOf(body), &out); err == nil {
			t.Fatalf("invalid JSON accepted %q", body)
		}
	}
}

func TestProductionSecretsRejectMissingBindings(t *testing.T) {
	for _, tc := range []struct {
		name string
		c    Config
		want string
	}{
		{"environment", Config{NodeEnv: "unknown"}, "environment_invalid"},
		{"refresh", Config{NodeEnv: "production"}, "secret_refresh_bound_invalid"},
		{"provider", Config{NodeEnv: "production", SecretsRefreshSeconds: 60, SecretsProvider: "unknown"}, ""},
		{"credentials", Config{NodeEnv: "production", SecretsRefreshSeconds: 60, SecretsProvider: "env"}, "required_secret_unavailable"},
		{"fixture profile", Config{NodeEnv: "production", SecretsRefreshSeconds: 60, SecretsProvider: "env", DeploymentRealm: valuePrivateFixture, RuntimeProfile: "container"}, "fixture_nats_profile_mismatch"},
		{"fixture binding", Config{NodeEnv: "production", SecretsRefreshSeconds: 60, SecretsProvider: "env", DeploymentRealm: valuePrivateFixture, RuntimeProfile: valueOwnedLocal}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := tc.c.ResolveProductionSecrets(context.Background())
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("secret admission %v", err)
			}
		})
	}
	for _, env := range []string{"", "development", "test"} {
		c := Config{NodeEnv: env}
		check, stop, err := c.ResolveProductionSecrets(context.Background())
		if err != nil || check != nil || stop == nil {
			t.Fatalf("development binding %v", err)
		}
		stop()
	}
	for _, key := range []string{"DATABASE_URL", "REDIS_URL", "RPC_URL", "NATS_URL", "NATS_CREDENTIALS_CREDS"} {
		t.Setenv(key, "bound-value")
	}
	c := Config{NodeEnv: "production", SecretsRefreshSeconds: 1, SecretsProvider: "env", NATSCredentialsCreds: "selected"}
	check, stop, err := c.ResolveProductionSecrets(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer stop()
	if c.NATSCredentialsCreds != "bound-value" {
		t.Fatal("inline broker binding lost")
	}
	t.Setenv("RPC_URL", "")
	if err = check(context.Background()); err == nil || !strings.Contains(err.Error(), "required_secret_unavailable") {
		t.Fatalf("missing refreshed binding %v", err)
	}
	c.NATSCredentialsCreds = "selected"
	if _, _, err = c.ResolveProductionSecrets(context.Background()); err == nil {
		t.Fatal("missing startup binding accepted")
	}
	t.Setenv("RPC_URL", "bound-value")
	t.Setenv("WEBHOOK_HMAC_SECRETS", "one")
	c.WebhookHMACSecrets = []string{"selected"}
	c.WebhookURLs = []string{"a", "b"}
	if _, _, err = c.ResolveProductionSecrets(context.Background()); err == nil || !strings.Contains(err.Error(), "webhook_secret_count_mismatch") {
		t.Fatalf("webhook secret mismatch %v", err)
	}
}

func TestRuntimeBrokerOptionsRejectMissingCredentials(t *testing.T) {
	for _, c := range []Config{{}, {DeploymentRealm: valuePrivateFixture}, {DeploymentRealm: valuePrivateFixture, RuntimeProfile: valueOwnedLocal, NATSUsername: "user", NATSPassword: "password"}, {NATSCredentialsCreds: "invalid"}} {
		if _, err := c.RuntimeNATSOptions(); err == nil {
			t.Fatal("unbound broker credentials accepted")
		}
	}
	_, creds := inlineNATSCreds(t)
	for _, raw := range []string{strings.Repeat("x", 1048577), "-----BEGIN NATS USER JWT-----\na.b.c\n------END NATS USER JWT------\n", creds[:strings.Index(creds, "-----BEGIN USER NKEY SEED-----")]} {
		if _, err := inlineNATSCredentials(raw); err == nil {
			t.Fatal("invalid broker seed admitted")
		}
	}
	path := filepath.Join(t.TempDir(), "user.creds")
	if err := os.WriteFile(path, []byte(creds), 0600); err != nil {
		t.Fatal(err)
	}
	c := Config{NATSCredentials: path}
	options, err := c.RuntimeNATSOptions()
	if err != nil {
		t.Fatal(err)
	}
	var applied nats.Options
	for _, option := range options {
		if err = option(&applied); err != nil {
			t.Fatal(err)
		}
	}
	if applied.UserJWT == nil || applied.SignatureCB == nil {
		t.Fatal("file credentials lack authentication")
	}
}

func TestRuntimePoolAdmissionAndSchemaBinding(t *testing.T) {
	ctx := context.Background()
	dsn, pool := canonicaltest.New(t)
	development := Config{}
	development.DatabaseURL = dsn
	opened, err := development.OpenRuntimePool(ctx, "indexerd", runtimedb.PoolOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if err = development.CheckRuntimeSchema(ctx, opened); err != nil {
		t.Fatal(err)
	}
	opened.Close()
	for _, tc := range []struct {
		name string
		edit func(*Config)
		want string
	}{
		{"profile", func(c *Config) { c.RuntimeProfile = "other" }, "runtime_profile_required"},
		{"container", func(c *Config) { c.RuntimeProfile = "container" }, "container_bind_mismatch"},
		{"loopback", func(c *Config) { c.BindAddress = "0.0.0.0" }, "owned_loopback_required"},
		{"budget selection", func(c *Config) {}, "runtime_pool_configuration_missing"},
		{"budget binding", func(c *Config) {
			c.RuntimeBudgetDocument = "e30="
			c.RuntimePoolID = "batch"
			c.RuntimeInstanceID = "indexerd-1"
		}, "runtime_budget_binding_mismatch"},
		{"budget JSON", func(c *Config) {
			c.RuntimeBudgetDocument = "e30="
			c.RuntimeBudgetSHA256 = digestOf("{}")
			c.RuntimePoolID = "batch"
			c.RuntimeInstanceID = "indexerd-1"
		}, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := Config{NodeEnv: "production", RuntimeProfile: valueOwnedLocal, BindAddress: "127.0.0.1"}
			c.DatabaseURL = dsn
			tc.edit(&c)
			if got, err := c.OpenRuntimePool(ctx, "indexerd", runtimedb.PoolOptions{}); err == nil || !strings.Contains(err.Error(), tc.want) {
				if got != nil {
					got.Close()
				}
				t.Fatalf("pool admission %v", err)
			}
		})
	}
	prod := Config{NodeEnv: "production"}
	if err = prod.CheckRuntimeSchema(ctx, pool); err == nil {
		t.Fatal("missing schema binding accepted")
	}
	prod.ExpectedSchemaDocument = "e30="
	prod.ExpectedSchemaSHA256 = digestOf("changed")
	if err = prod.CheckRuntimeSchema(ctx, pool); err == nil {
		t.Fatal("changed schema bytes accepted")
	}
	prod.ExpectedSchemaSHA256 = digestOf("{}")
	if err = prod.CheckRuntimeSchema(ctx, pool); err == nil {
		t.Fatal("invalid schema manifest accepted")
	}
	want := dbproduction.ExpectedSchema{Service: "core/indexer", RuntimeRole: "indexer_runtime", RegistryDigest: strings.Repeat("a", 64), InterfaceDigest: strings.Repeat("b", 64), ReaderVersion: 1, Migrations: map[int64]string{60000: strings.Repeat("c", 64)}, Schemas: map[string]string{"indexer": strings.Repeat("d", 64)}, Epoch: 1, Region: "fixture"}
	body, err := json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	prod.ExpectedSchemaDocument = base64.StdEncoding.EncodeToString(body)
	prod.ExpectedSchemaSHA256 = digestOf(string(body))
	if err = prod.CheckRuntimeSchema(ctx, pool); err == nil {
		t.Fatal("unissued database schema admitted")
	}
	want.Service = "core/pnl-tracker"
	body, err = json.Marshal(want)
	if err != nil {
		t.Fatal(err)
	}
	prod.ExpectedSchemaDocument = base64.StdEncoding.EncodeToString(body)
	prod.ExpectedSchemaSHA256 = digestOf(string(body))
	if err = prod.CheckRuntimeSchema(ctx, pool); err == nil {
		t.Fatal("different service schema admitted")
	}
}
