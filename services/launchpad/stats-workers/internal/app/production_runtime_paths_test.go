package app

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/internaltest"
	sharedconfig "github.com/Sidiora-Technologies/KindleLaunch/shared/config"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nkeys"
)

func productionRuntimeConfig(t *testing.T) (config.Config, statsRuntimeDocument) {
	t.Helper()
	dsn, pool := internaltest.NewPostgresWithDSN(t)
	internaltest.EnsureMetadataSchema(t, pool)
	redisURL := internaltest.NewRedisURL(t)
	d := readDelivery(t, filepath.Join("testdata", "delivery.production.json"))
	suffix := strings.ReplaceAll(time.Now().UTC().Format("150405.000000000"), ".", "")
	d.Consumers[0].Stream = "RUNTIME_MARKET_" + suffix
	d.Consumers[0].Durable = "runtime_market_" + suffix
	d.Phase2.OutputStream = "RUNTIME_STATS_" + suffix
	for i := range d.Phase2.SearchConsumers {
		c := &d.Phase2.SearchConsumers[i].Consumer
		c.Durable += "_" + suffix
		if d.Phase2.SearchConsumers[i].Kind == "creation" {
			c.Stream = d.Consumers[0].Stream
		} else {
			c.Stream = d.Phase2.OutputStream
		}
	}
	nc := internaltest.ConnectNATS(t, os.Getenv("L20_NATS_URL"))
	js, err := nc.JetStream()
	if err != nil {
		t.Fatal(err)
	}
	for _, stream := range []nats.StreamConfig{{Name: d.Consumers[0].Stream, Subjects: []string{"accounting.market.v1", "canonical.creation.v1"}, Storage: nats.FileStorage, MaxBytes: 1048576}, {Name: d.Phase2.OutputStream, Subjects: []string{"stats.minute.v2", "search.invalidation.v2"}, Storage: nats.FileStorage, MaxBytes: 1048576}} {
		if _, err := js.AddStream(&stream); err != nil {
			t.Fatal(err)
		}
		name := stream.Name
		t.Cleanup(func() {
			if err := js.DeleteStream(name); err != nil {
				t.Error(err)
			}
		})
	}
	broker, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	subscriptions := append([]natsdriver.ConsumerConfig(nil), d.Consumers...)
	for _, sub := range d.Phase2.SearchConsumers {
		subscriptions = append(subscriptions, sub.Consumer)
	}
	for _, sub := range subscriptions {
		expected, err := sub.BrokerConsumerConfig()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := broker.CreateConsumer(context.Background(), sub.Stream, expected); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := json.Marshal(d)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(raw)
	path := filepath.Join(t.TempDir(), "delivery.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certificate := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "statistics-client"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth}}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}))
	keyPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	account, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	user, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	accountPub, err := account.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	userPub, err := user.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	signing := enc([]byte(`{"typ":"JWT","alg":"ed25519-nkey"}`)) + "." + enc([]byte(`{"sub":"`+userPub+`","iss":"`+accountPub+`","nats":{"type":"user","version":2}}`))
	signature, err := account.Sign([]byte(signing))
	if err != nil {
		t.Fatal(err)
	}
	seed, err := user.Seed()
	if err != nil {
		t.Fatal(err)
	}
	creds := "-----BEGIN NATS USER JWT-----\n" + signing + "." + enc(signature) + "\n------END NATS USER JWT------\n\n-----BEGIN USER NKEY SEED-----\n" + string(seed) + "\n------END USER NKEY SEED------\n"
	roles := deployment.Current().Roles
	cfg := config.Config{BaseEnv: sharedconfig.BaseEnv{NodeEnv: "production", SecretsProvider: "env", SecretsRefreshSeconds: 60, DatabaseURL: dsn, RedisURL: redisURL, RPCURL: "http://127.0.0.1:8545", RouterAddress: roles["Router"].Address, EventEmitterAddress: roles["EventEmitter"].Address}, StatsCursorKey: strings.Repeat("a", 64), StatsAuditKey: strings.Repeat("b", 64), RuntimeMetricsEnvironment: "production", RuntimeZone: "zone-a", RuntimeInstanceID: "stats-runtime", IdentityURL: "https://127.0.0.1:8443", IdentityMTLSCAPEM: certPEM, IdentityMTLSClientCertPEM: certPEM, IdentityMTLSClientKeyPEM: keyPEM, IdentityMTLSPeerCertSHA256: strings.Repeat("c", 64), IdentitySigningKey: base64.StdEncoding.EncodeToString(key), NATSCredentialsCreds: creds, NATSURL: os.Getenv("L20_NATS_URL"), DeliveryConfigFile: path, DeliveryConfigSHA256: hex.EncodeToString(digest[:]), PublicStatsIntervalMS: 2000, PublicStatsBatch: 50, PublicStreamSchema: "api_stream", LegacyBridgeIntervalMS: 1000, LegacyBridgeBatch: 100, WebhookHMACSecret: strings.Repeat("h", 32)}
	for name, value := range map[string]string{"DATABASE_URL": dsn, "REDIS_URL": redisURL, "RPC_URL": cfg.RPCURL, "WEBHOOK_HMAC_SECRET": cfg.WebhookHMACSecret, "NATS_URL": cfg.NATSURL, "STATS_CURSOR_KEY": cfg.StatsCursorKey, "STATS_AUDIT_KEY": cfg.StatsAuditKey, "IDENTITY_ASSERTION_SIGNING_KEY": cfg.IdentitySigningKey, "IDENTITY_MTLS_CLIENT_KEY_PEM": keyPEM, "NATS_CREDENTIALS_CREDS": creds} {
		t.Setenv(name, value)
	}
	return cfg, d
}

func TestProductionRuntimeWiresDurableWorkers(t *testing.T) {
	cfg, d := productionRuntimeConfig(t)
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := newWithRuntimePool(ctx, cfg, logger, func(ctx context.Context, c *config.Config, opts shareddb.PoolOptions) (*pgxpool.Pool, error) {
		return shareddb.NewPool(ctx, c.DatabaseURL, opts)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	if app.accounting == nil || app.nats == nil || app.rpc == nil || len(app.transports) != len(d.Consumers)+len(d.Phase2.SearchConsumers) || len(app.jobs) < 12 {
		t.Fatalf("runtime dependencies: accounting=%t transports=%d jobs=%d", app.accounting != nil, len(app.transports), len(app.jobs))
	}
	response := httptest.NewRecorder()
	app.Router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	if response.Code != http.StatusOK {
		t.Fatalf("production liveness: %d %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	app.Router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	if response.Code != http.StatusOK || !strings.Contains(response.Body.String(), "statsd") {
		t.Fatalf("runtime telemetry: %d %s", response.Code, response.Body.String())
	}

	for _, id := range []string{"60000", "190001"} {
		if id == "190001" {
			ddl, err := delivery.PartitionSchemaDDL("indexer")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := app.pool.Exec(ctx, ddl); err != nil {
				t.Fatal(err)
			}
		}
		raw, err := os.ReadFile(filepath.Join("../../../../../migrations", id+".sql"))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := app.pool.Exec(ctx, "CREATE SCHEMA IF NOT EXISTS indexer"); err != nil {
			t.Fatal(err)
		}
		if _, err := app.pool.Exec(ctx, string(raw)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := app.pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,$1,'indexer',1,clock_timestamp()+interval '1 hour')`, d.SourceStream); err != nil {
		t.Fatal(err)
	}
	if _, err := app.pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,$1,10,10)`, d.SourceStream); err != nil {
		t.Fatal(err)
	}
	checkReady := func(source, broker string) {
		t.Helper()
		response := httptest.NewRecorder()
		app.Router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
		var body struct {
			Checks map[string]string `json:"checks"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil || response.Code != 503 || body.Checks["source"] != source || body.Checks["broker"] != broker {
			t.Fatalf("runtime readiness: %d %s %v", response.Code, response.Body.String(), err)
		}
	}
	checkReady("ok", "ok")
	if _, err := app.pool.Exec(ctx, `UPDATE indexer.indexer_ingest_watermarks SET raw_complete_height=11`); err != nil {
		t.Fatal(err)
	}
	checkReady("failed", "ok")
	jobs := map[string]bool{}
	for _, job := range app.jobs {
		if job.name == "" || job.run == nil || jobs[job.name] {
			t.Fatalf("invalid runtime job: %+v", job)
		}
		jobs[job.name] = true
		if !strings.HasPrefix(job.name, "search ") && job.name != "market accounting" {
			bounded, cancel := context.WithTimeout(ctx, 40*time.Millisecond)
			err := job.run(bounded)
			cancel()
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("%s did not stop at its deadline: %v", job.name, err)
			}
		}
	}
	for _, name := range []string{"market accounting", "stats durable publisher", "canonical window expiry", "canonical transfer holders", "analytics expiry", "public market stats", "canonical multihop", "canonical legacy read model", "legacy idle windows", "platform precompute", "bucket-trader cleanup", "search durable outbox"} {
		if !jobs[name] {
			t.Fatalf("missing durable job %s", name)
		}
	}
	app.nats.Close()
	checkReady("failed", "failed")
}

func TestProductionRuntimeRejectsInvalidBindings(t *testing.T) {
	cfg, _ := productionRuntimeConfig(t)
	for _, tc := range []struct {
		name string
		edit func(*config.Config)
		want string
	}{
		{"metrics zone", func(c *config.Config) { c.RuntimeZone = "unknown" }, ""},
		{"delivery digest", func(c *config.Config) { c.DeliveryConfigSHA256 = strings.Repeat("0", 64) }, ""},
		{"identity certificate", func(c *config.Config) { c.IdentityMTLSClientCertPEM = "invalid" }, ""},
		{"worker bounds", func(c *config.Config) { c.PublicStatsBatch = 0 }, ""},
		{"stream schema", func(c *config.Config) { c.PublicStreamSchema = "invalid-schema" }, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := cfg
			tc.edit(&c)
			app, err := newWithRuntimePool(context.Background(), c, slog.New(slog.NewTextHandler(io.Discard, nil)), func(ctx context.Context, c *config.Config, opts shareddb.PoolOptions) (*pgxpool.Pool, error) {
				return shareddb.NewPool(ctx, c.DatabaseURL, opts)
			})
			if err == nil || app != nil {
				if app != nil {
					app.Close()
				}
				t.Fatalf("invalid binding accepted: %v", err)
			}
		})
	}
}

func TestProductionRuntimeDrainsAfterRequiredWorkerFailure(t *testing.T) {
	cfg, _ := productionRuntimeConfig(t)
	cfg.BindAddress = "127.0.0.1"
	cfg.Port = 0
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	app, err := newWithRuntimePool(ctx, cfg, logger, func(ctx context.Context, c *config.Config, opts shareddb.PoolOptions) (*pgxpool.Pool, error) {
		return shareddb.NewPool(ctx, c.DatabaseURL, opts)
	})
	if err != nil {
		t.Fatal(err)
	}
	defer app.Close()
	closed := make(chan struct{})
	go func() { defer close(closed); time.Sleep(100 * time.Millisecond); app.nats.Close() }()
	err = runInitializedApp(ctx, cfg, logger, app)
	<-closed
	if err == nil || !strings.Contains(err.Error(), "stats worker ") {
		t.Fatalf("required worker failure: %v", err)
	}
	if err := app.tracker.Ready(context.Background()); err == nil {
		t.Fatal("failed worker left runtime ready")
	}
}
