package app

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/ogrender/fonts"
	inventory "github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l16"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l18"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	sharedconfig "github.com/Sidiora-Technologies/KindleLaunch/shared/config"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	sharedprocess "github.com/Sidiora-Technologies/KindleLaunch/shared/process"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/telemetry"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nkeys"
)

func writeCompositionDocument(t *testing.T, value any) (string, string) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return writeCompositionFile(t, raw)
}
func writeCompositionFile(t *testing.T, raw []byte) (string, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "authority")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path, fmt.Sprintf("%x", sha256.Sum256(raw))
}
func compositionIdentity(t *testing.T) (config.Config, pnlJourneyDocument) {
	t.Helper()
	server := httptest.NewTLSServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	certificate := server.TLS.Certificates[0]
	key, err := x509.MarshalPKCS8PrivateKey(certificate.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Certificate[0]})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: key})
	_, signing, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{IdentityURL: server.URL, IdentitySigningKey: base64.RawStdEncoding.EncodeToString(signing)}
	d := pnlJourneyDocument{IdentityEndpoint: server.URL, IdentityIssuer: "pnld", IdentityAudience: "identity", IdentityPeerCertSHA256: fmt.Sprintf("%x", sha256.Sum256(certificate.Certificate[0]))}
	d.IdentityCAFile, d.IdentityCASHA256 = writeCompositionFile(t, certPEM)
	d.IdentityClientCertFile, d.IdentityClientCertSHA256 = writeCompositionFile(t, certPEM)
	d.IdentityClientKeyFile, d.IdentityClientKeySHA256 = writeCompositionFile(t, keyPEM)
	return cfg, d
}

func TestJourneyCompositionBindsActualRuntimeComponents(t *testing.T) {
	ctx := context.Background()
	root := coordinatortest.RepositoryRoot(t)
	chain, chainFixture := internaltest.NewAnvilFixture(t, ctx, root)
	primary := ethclient.NewClient(chain)
	db := coordinatortest.Bootstrap(t, os.Getenv("PNL_TEST_POSTGRES_DSN"), "indexer_runtime", "pnl_runtime")
	pool, err := pgxpool.New(ctx, db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	nc, err := nats.Connect(os.Getenv("PNL_TEST_NATS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	cfg, d := compositionIdentity(t)
	cfg.DeploymentRealm = "private_fixture"
	cfg.RPCURL = chainFixture.RPC
	cfg.RPCURLFallback = chainFixture.RPC + "/"
	cfg.FontDir = t.TempDir()
	if err = os.Chmod(cfg.FontDir, 0700); err != nil {
		t.Fatal(err)
	}
	cfg.PortfolioCursorKey = base64.RawURLEncoding.EncodeToString(make([]byte, 32))
	d.Version = 3
	d.Realm = cfg.DeploymentRealm
	d.FontDirectory = cfg.FontDir
	d.FontRegularSHA256 = fonts.RegularSHA256
	d.FontBoldSHA256 = fonts.BoldSHA256
	d.FontLicenseSHA256 = fonts.LicenseSHA256
	d.LiveBatch = 4
	d.ReleaseDigest = strings.Repeat("a", 64)
	d.InterfaceDigest = strings.Repeat("b", 64)
	d.NodeIDs = [2]string{"primary", "archive"}
	d.ExportsDirectory = t.TempDir()
	if err = os.Chmod(d.ExportsDirectory, 0700); err != nil {
		t.Fatal(err)
	}
	d.BrokerStream = "PNL_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	d.CDNOrigin = cfg.IdentityURL
	d.RewardManifestDigest = l18.ManifestDigest
	d.RewardRuntimeHashes = map[string]string{}
	head, err := primary.HeaderByNumber(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{l18.NFT, l18.Accumulator, l18.FeesRouter, l18.USDL} {
		code, err := primary.CodeAtHash(ctx, common.HexToAddress(target), head.Hash())
		if err != nil {
			t.Fatal(err)
		}
		d.RewardRuntimeHashes[target] = strings.ToLower(crypto.Keccak256Hash(code).Hex())
		var slot hexutil.Bytes
		if err = chain.CallContext(ctx, &slot, "eth_getStorageAt", target, "0x360894a13ba1a3210667c828492db98dca3e2076cc3735a920a3ca505d382bbc", map[string]any{"blockHash": head.Hash().Hex(), "requireCanonical": true}); err != nil {
			t.Fatal(err)
		}
		implementation := common.BytesToAddress(slot)
		if implementation != (common.Address{}) {
			code, err = primary.CodeAtHash(ctx, implementation, head.Hash())
			if err != nil {
				t.Fatal(err)
			}
			d.RewardRuntimeHashes[strings.ToLower(implementation.Hex())] = strings.ToLower(crypto.Keccak256Hash(code).Hex())
		}
	}
	d.Programs = []l18.Program{{ID: "campaign", Version: "v1", DeploymentID: "fixture", Asset: l18.USDL, ReferralRaw: "1000000", AwardCapRaw: "1000000", DailyCapRaw: "1000000", BeneficiaryCapRaw: "1000000"}}
	runtime := inventoryRuntimeDocument{Schema: "pnl", SourceStream: "composition", ConsumerID: "pnl", Generation: "g1", Owner: "pnl-1", ProjectionVersion: "inventory-v1", Epoch: coordinatortest.Epoch, Region: coordinatortest.Region}
	cfg.DeliveryConfigFile, cfg.DeliveryConfigSHA256 = writeCompositionDocument(t, runtime)
	cfg.JourneyConfigFile, cfg.JourneyConfigSHA256 = writeCompositionDocument(t, d)
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,'composition',$1,$1)`, head.Number.Int64()); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,'composition',$1,$2,0,true,true)`, head.Number.Int64(), strings.ToLower(head.Hash().Hex())); err != nil {
		t.Fatal(err)
	}
	guard, err := queue.NewEpochGuard("recovery", runtime.Epoch, runtime.Region)
	if err != nil {
		t.Fatal(err)
	}
	baseline, err := inventory.NewBaselineAuthorityWithEpochGuard(pool, primary, runtime.Schema, runtime.SourceStream, runtime.ConsumerID, runtime.Generation, runtime.Owner, runtime.ProjectionVersion, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	consumer, err := inventory.NewAccountingConsumerWithRuntime(pool, runtime.Schema, runtime.ConsumerID, runtime.Generation, runtime.Owner, runtime.ProjectionVersion, baseline, guard.Assert, delivery.StreamWriter{Schema: "api_stream"})
	if err != nil {
		t.Fatal(err)
	}
	journey, err := newPnLJourneys(ctx, cfg, pool, nil, primary, nc, baseline, consumer)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(journey.second.Close)
	if journey.rewards == nil || journey.rewardLedger == nil || journey.inventory == nil || journey.live == nil || journey.shares == nil || journey.accounting == consumer || len(journey.referrals) != 1 || journey.schema != "pnl" {
		t.Fatal("incomplete journey components")
	}
	if err = journey.settleOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err = journey.referralOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err = journey.cashbackOnce(ctx); err != nil {
		t.Fatal(err)
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err = journey.settleOnce(canceled); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, handler := range []http.HandlerFunc{journey.rebuildHTTP, journey.intervalHTTP, journey.enrollHTTP, journey.cancelLiveHTTP} {
		out := httptest.NewRecorder()
		handler(out, httptest.NewRequest("POST", "/", strings.NewReader("{}")))
		if out.Code != 401 {
			t.Fatal(out.Code, out.Body.String())
		}
	}
	principal := auth.IdentityPrincipal{Wallet: strings.ToLower(chainFixture.Alice), AccountID: uuid.NewString(), SessionID: uuid.NewString(), AuthEpoch: 1, ProofAt: time.Now().UTC(), AccessExpiresAt: time.Now().Add(time.Hour).UTC()}
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/v2/identity/request:authenticate":
			var request auth.IdentityAuthenticationRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Authorization != "Bearer owner-access" {
				http.Error(w, "authentication required", 401)
				return
			}
			if err := json.NewEncoder(w).Encode(principal); err != nil {
				t.Error(err)
			}
		case "/internal/v2/identity/session:check":
			w.WriteHeader(200)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(identityServer.Close)
	identity := auth.HTTPIdentityAuthority{Endpoint: identityServer.URL, ServiceToken: strings.Repeat("s", 32), Client: identityServer.Client()}
	journey.portfolio.Authority = identity
	journey.live, err = inventory.NewLiveTransferWorker(journey.inventory, l17.Repository{Pool: pool}, identity, 4)
	if err != nil {
		t.Fatal(err)
	}
	call := func(handler http.HandlerFunc, target, body string) *httptest.ResponseRecorder {
		request := httptest.NewRequest("POST", target, strings.NewReader(body))
		request.Header.Set("Authorization", "Bearer owner-access")
		out := httptest.NewRecorder()
		handler(out, request)
		return out
	}
	for _, body := range []string{"{", `{"unknown":true}`, "{} {}", strings.Repeat("x", 4097)} {
		out := call(journey.rebuildHTTP, "/", body)
		if out.Code != 400 {
			t.Fatal(out.Code, out.Body.String())
		}
	}
	if out := call(journey.rebuildHTTP, "/", "{}"); out.Code != 503 {
		t.Fatal(out.Code, out.Body.String())
	}
	for _, query := range []string{"unknown=1", "from=a&from=b", "from=invalid", "from=2026-01-01T00:00:00Z&to=invalid", "from=2026-01-01T00:00:00%2B01:00&to=2026-01-02T00:00:00Z", "from=2026-01-01T00:00:00Z&to=2026-01-02T00:00:00Z&limit=abc", "from=2026-01-01T00:00:00Z&to=2026-01-02T00:00:00Z&interval=invalid"} {
		out := call(journey.intervalHTTP, "/?"+query, "")
		if out.Code != 422 {
			t.Fatal(query, out.Code, out.Body.String())
		}
	}
	if out := call(journey.intervalHTTP, "/?from=2026-01-01T00:00:00Z&to=2026-01-02T00:00:00Z", ""); out.Code != 503 {
		t.Fatal(out.Code, out.Body.String())
	}
	if out := call(journey.enrollHTTP, "/", ""); out.Code != 202 {
		t.Fatal(out.Code, out.Body.String())
	}
	if out := call(journey.cancelLiveHTTP, "/", ""); out.Code != 204 {
		t.Fatal(out.Code, out.Body.String())
	}
	stopped, cancelJobs := context.WithCancel(ctx)
	cancelJobs()
	app := &App{journeys: journey, tracker: sharedprocess.NewTracker(), logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	if err = app.tracker.SetReady(); err != nil {
		t.Fatal(err)
	}
	app.startJourneyJobs(stopped)
	bounded, stopDrain := context.WithTimeout(ctx, time.Second)
	defer stopDrain()
	if err = app.tracker.Drain(bounded); err != nil {
		t.Fatal(err)
	}
	if got := shareLabel(cfg.IdentityURL, "abc"); !strings.HasSuffix(got, "/pnl/abc") || strings.HasPrefix(got, "https://") {
		t.Fatal(got)
	}
	if got := shareLabel("http://sidiora.fun", "abc"); got != "sidiora.fun/pnl/abc" {
		t.Fatal(got)
	}
	productionConfig := cfg
	productionConfig.NodeEnv = "production"
	productionConfig.DeploymentRealm = "production"
	productionConfig.SecretsProvider = "env"
	productionConfig.SecretsRefreshSeconds = 1
	productionConfig.RuntimeProfile = "owned_local"
	productionConfig.BindAddress = "127.0.0.1"
	productionConfig.RuntimeZone = "zone-a"
	productionConfig.RuntimeMetricsEnvironment = "qualification"
	productionConfig.RuntimeInstanceID = "pnl_composition"
	productionConfig.RuntimePoolID = "interactive"
	productionConfig.DatabaseURL = db.Runtimes["pnl_runtime"].DSN
	productionConfig.RedisURL = internaltest.NewRedisURL(t)
	productionConfig.NATSURL = os.Getenv("PNL_TEST_NATS_URL")
	productionConfig.WebhookHMACSecret = strings.Repeat("s", 32)
	productionConfig.ExpectedSchemaFile, productionConfig.ExpectedSchemaSHA256 = db.ExpectedSchemaFile(t, "pnl_runtime")
	database, err := url.Parse(productionConfig.DatabaseURL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.ParseUint(database.Port(), 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	budget := production.ConnectionBudget{Identity: "pnl_composition", Targets: []production.PostgresTarget{{ID: "primary", Member: "fixture", Host: database.Hostname(), Port: uint16(port), MaxConnections: 100}}, Operational: []production.OperationalBudget{{ID: "admin", Target: "primary", Category: "admin", Connections: 1}, {ID: "migration", Target: "primary", Category: "migration", Connections: 1}}}
	for _, role := range []string{"apid:rest", "apid:sockets", "indexerd", "chartsd", "statsd", "pnld", "rankingd", "mediagatewayd", "userd", "sociald", "metadatad", "livestreamd"} {
		budget.Clients = append(budget.Clients, production.ClientBudget{Role: role, PoolID: "interactive", Target: "primary", Transport: shareddb.TransportDirect, Alias: strings.TrimPrefix(database.Path, "/"), User: func() string {
			if role == "pnld" {
				return database.User.Username()
			}
			return strings.ReplaceAll(role, ":", "_")
		}(), QueryClass: shareddb.Interactive, Processes: production.LiveProcesses{Replicas: 1, Maximum: 1}, MaxConnections: 2})
	}
	productionConfig.RuntimeBudgetFile, productionConfig.RuntimeBudgetSHA256 = writeCompositionDocument(t, budget)
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
	productionConfig.NATSCredentialsCreds = "-----BEGIN NATS USER JWT-----\n" + signing + "." + enc(signature) + "\n------END NATS USER JWT------\n\n-----BEGIN USER NKEY SEED-----\n" + string(seed) + "\n------END USER NKEY SEED------\n"
	runtime.Consumer = natsdriver.ConsumerConfig{Stream: d.BrokerStream, Durable: "pnl-composition", Subject: accounting.InventorySubject, Batch: 1, MaxAckPending: 4, MaxWaiting: 1, MaxDeliver: 8, FetchWait: 100 * time.Millisecond, AckWait: time.Second}
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = js.CreateStream(ctx, jetstream.StreamConfig{Name: runtime.Consumer.Stream, Subjects: []string{runtime.Consumer.Subject}, Storage: jetstream.FileStorage}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := js.DeleteStream(context.Background(), runtime.Consumer.Stream); err != nil {
			t.Error(err)
		}
	})
	broker, err := runtime.Consumer.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = js.CreateOrUpdateConsumer(ctx, runtime.Consumer.Stream, broker); err != nil {
		t.Fatal(err)
	}
	productionConfig.DeliveryConfigFile, productionConfig.DeliveryConfigSHA256 = writeCompositionDocument(t, runtime)
	productionConfig.JourneyConfigFile = ""
	productionConfig.JourneyConfigDocument = ""
	for key, value := range map[string]string{"DATABASE_URL": productionConfig.DatabaseURL, "REDIS_URL": productionConfig.RedisURL, "RPC_URL": productionConfig.RPCURL, "RPC_URL_FALLBACK": productionConfig.RPCURLFallback, "NATS_URL": productionConfig.NATSURL, "NATS_CREDENTIALS_CREDS": productionConfig.NATSCredentialsCreds, "WEBHOOK_HMAC_SECRET": productionConfig.WebhookHMACSecret} {
		t.Setenv(key, value)
	}
	built, err := New(ctx, productionConfig, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	health := httptest.NewRecorder()
	built.Router.ServeHTTP(health, httptest.NewRequest("GET", "/health/ready", nil))
	if health.Code != 200 {
		built.Close()
		t.Fatal(health.Code, health.Body.String())
	}
	accountingContext, cancelAccounting := context.WithCancel(ctx)
	cancelAccounting()
	if err = built.runAccounting(accountingContext); !errors.Is(err, context.Canceled) {
		built.Close()
		t.Fatal(err)
	}
	built.Close()
	grantSQL, err := l18.OutboxRuntimeGrantSQL("pnl", database.User.Username())
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, grantSQL); err != nil {
		t.Fatal(err)
	}
	fullConfig := productionConfig
	var expected production.ExpectedSchema
	if err = json.Unmarshal(db.Runtimes["pnl_runtime"].ExpectedSchema, &expected); err != nil {
		t.Fatal(err)
	}
	expected.Privileges = append(expected.Privileges, production.Grant{Schema: "pnl", Object: "l18_outbox", Privileges: []string{"UPDATE"}, Columns: []string{"published_at"}, ObjectKind: "table"})
	fullConfig.ExpectedSchemaFile, fullConfig.ExpectedSchemaSHA256 = writeCompositionDocument(t, expected)
	fullConfig.RuntimeInstanceID = "pnl_composition_full"
	fullDocument := d
	fullDocument.Realm = "production"
	fullConfig.JourneyConfigFile, fullConfig.JourneyConfigSHA256 = writeCompositionDocument(t, fullDocument)
	for key, value := range map[string]string{"IDENTITY_SIGNING_KEY": fullConfig.IdentitySigningKey, "PNL_CURSOR_KEY": fullConfig.PortfolioCursorKey} {
		t.Setenv(key, value)
	}
	complete, err := New(ctx, fullConfig, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	for _, request := range []struct {
		method, path string
		want         int
	}{{"GET", "/health/ready", 200}, {"GET", "/v3/me/portfolio", 401}, {"OPTIONS", "/v3/me/portfolio", 405}, {"GET", "/v2/me/rewards", 401}, {"GET", "/v3/portfolio/shares/invalid/image.png", 404}} {
		response := httptest.NewRecorder()
		complete.Router.ServeHTTP(response, httptest.NewRequest(request.method, request.path, nil))
		if response.Code != request.want {
			complete.Close()
			t.Fatal(request, response.Code, response.Body.String())
		}
	}
	complete.Close()
	assertProductionLifecycle(t, fullConfig)
	for _, tc := range []struct {
		name   string
		change func(*config.Config, *inventoryRuntimeDocument)
	}{
		{"bound inventory document", func(c *config.Config, d *inventoryRuntimeDocument) { c.DeliveryConfigSHA256 = strings.Repeat("0", 64) }},
		{"inventory schema", func(c *config.Config, d *inventoryRuntimeDocument) { d.Schema = "other" }},
		{"source stream", func(c *config.Config, d *inventoryRuntimeDocument) { d.SourceStream = "invalid stream" }},
		{"recovery epoch", func(c *config.Config, d *inventoryRuntimeDocument) { d.Epoch = 0 }},
		{"archive independence", func(c *config.Config, d *inventoryRuntimeDocument) { c.RPCURLFallback = c.RPCURL }},
		{"archive URL", func(c *config.Config, d *inventoryRuntimeDocument) { c.RPCURLFallback = ":" }},
		{"primary URL", func(c *config.Config, d *inventoryRuntimeDocument) { c.RPCURL = ":" }},
		{"broker URL", func(c *config.Config, d *inventoryRuntimeDocument) { c.NATSURL = "nats://127.0.0.1:1" }},
		{"consumer subject", func(c *config.Config, d *inventoryRuntimeDocument) { d.Consumer.Subject = "unbound.inventory" }},
		{"trusted proxies", func(c *config.Config, d *inventoryRuntimeDocument) { c.TrustedProxyCIDRs = "invalid" }},
		{"fixture evidence", func(c *config.Config, d *inventoryRuntimeDocument) {
			c.FixtureCommitAckBarrier = filepath.Join(t.TempDir(), "barrier")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate, document := fullConfig, runtime
			candidate.RuntimeInstanceID = "pnl_invalid_" + strings.ReplaceAll(tc.name, " ", "_")
			candidate.JourneyConfigFile = ""
			candidate.JourneyConfigDocument = ""
			tc.change(&candidate, &document)
			if tc.name != "bound inventory document" {
				candidate.DeliveryConfigFile, candidate.DeliveryConfigSHA256 = writeCompositionDocument(t, document)
			}
			for key, value := range map[string]string{"RPC_URL": candidate.RPCURL, "RPC_URL_FALLBACK": candidate.RPCURLFallback, "NATS_URL": candidate.NATSURL} {
				t.Setenv(key, value)
			}
			built, err := New(ctx, candidate, slog.New(slog.NewTextHandler(io.Discard, nil)))
			if err == nil {
				built.Close()
				t.Fatal("unbound runtime configuration accepted")
			}
		})
	}
	for _, mutate := range []func(*config.Config, *pnlJourneyDocument){
		func(c *config.Config, d *pnlJourneyDocument) { d.Version = 2 },
		func(c *config.Config, d *pnlJourneyDocument) { d.ReleaseDigest = strings.Repeat("g", 64) },
		func(c *config.Config, d *pnlJourneyDocument) { d.CDNOrigin = "invalid" },
		func(c *config.Config, d *pnlJourneyDocument) { d.CDNZone = "invalid"; c.PortfolioCDNToken = "token" },
		func(c *config.Config, d *pnlJourneyDocument) { d.FontDirectory = "relative"; c.FontDir = "relative" },
		func(c *config.Config, d *pnlJourneyDocument) { c.PortfolioCursorKey = "invalid" },
		func(c *config.Config, d *pnlJourneyDocument) { c.DeliveryConfigSHA256 = strings.Repeat("0", 64) },
		func(c *config.Config, d *pnlJourneyDocument) { c.RPCURLFallback = ":" },
		func(c *config.Config, d *pnlJourneyDocument) { d.NodeIDs = [2]string{"same", "same"} },
		func(c *config.Config, d *pnlJourneyDocument) {
			d.ExportsDirectory = t.TempDir()
			if err := os.Chmod(d.ExportsDirectory, 0755); err != nil {
				t.Fatal(err)
			}
		},
		func(c *config.Config, d *pnlJourneyDocument) { d.IdentityCAFile = "relative" },
		func(c *config.Config, d *pnlJourneyDocument) { d.LiveBatch = 0 },
		func(c *config.Config, d *pnlJourneyDocument) { d.RewardManifestDigest = "invalid" },
		func(c *config.Config, d *pnlJourneyDocument) { d.Programs[0].ReferralRaw = "bad" },
		func(c *config.Config, d *pnlJourneyDocument) { d.ApprovedCommunityWallets = []string{"invalid"} },
		func(c *config.Config, d *pnlJourneyDocument) { d.BrokerStream = "invalid stream" },
	} {
		candidate, document := cfg, d
		document.Programs = append([]l18.Program(nil), d.Programs...)
		mutate(&candidate, &document)
		candidate.JourneyConfigFile, candidate.JourneyConfigSHA256 = writeCompositionDocument(t, document)
		if j, err := newPnLJourneys(ctx, candidate, pool, nil, primary, nc, baseline, consumer); err == nil {
			j.second.Close()
			t.Fatal("invalid journey configuration accepted")
		}
	}

}

func TestBoundIdentityFilesAndSigningKeys(t *testing.T) {
	cfg, d := compositionIdentity(t)
	authority, err := pnlIdentityAuthority(cfg, d)
	if err != nil || authority.Endpoint != cfg.IdentityURL || authority.Client.Timeout != 2*time.Second {
		t.Fatal(authority, err)
	}
	if authority.Client.CheckRedirect(httptest.NewRequest("GET", authority.Endpoint, nil), nil) != http.ErrUseLastResponse {
		t.Fatal("redirect followed")
	}
	transport := authority.Client.Transport.(*http.Transport)
	defer transport.CloseIdleConnections()
	if transport.TLSClientConfig.MinVersion != tls.VersionTLS13 || len(transport.TLSClientConfig.Certificates) != 1 {
		t.Fatal("identity TLS binding")
	}
	for _, mutate := range []func(*config.Config, *pnlJourneyDocument){
		func(c *config.Config, d *pnlJourneyDocument) { d.IdentityCAFile = "relative" },
		func(c *config.Config, d *pnlJourneyDocument) { d.IdentityCASHA256 = strings.Repeat("0", 64) },
		func(c *config.Config, d *pnlJourneyDocument) {
			d.IdentityCAFile, d.IdentityCASHA256 = writeCompositionFile(t, []byte("invalid"))
		},
		func(c *config.Config, d *pnlJourneyDocument) { d.IdentityClientCertFile = "relative" },
		func(c *config.Config, d *pnlJourneyDocument) { d.IdentityClientKeyFile = "relative" },
		func(c *config.Config, d *pnlJourneyDocument) {
			d.IdentityClientKeyFile, d.IdentityClientKeySHA256 = writeCompositionFile(t, []byte("invalid"))
		},
		func(c *config.Config, d *pnlJourneyDocument) { c.IdentitySigningKey = "!" },
		func(c *config.Config, d *pnlJourneyDocument) {
			c.IdentitySigningKey = base64.RawStdEncoding.EncodeToString(make([]byte, 64))
		},
		func(c *config.Config, d *pnlJourneyDocument) { d.IdentityPeerCertSHA256 = "invalid" },
	} {
		candidate, document := cfg, d
		mutate(&candidate, &document)
		if _, err := pnlIdentityAuthority(candidate, document); err == nil {
			t.Fatal("unbound identity accepted")
		}
	}
	for _, kind := range []string{"directory", "symlink", "large", "missing"} {
		t.Run(kind, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "identity")
			switch kind {
			case "directory":
				err = os.Mkdir(path, 0700)
			case "symlink":
				err = os.Symlink(d.IdentityCAFile, path)
			case "large":
				err = os.WriteFile(path, make([]byte, 1048577), 0600)
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err := pnlBoundAuthorityFile(path, strings.Repeat("0", 64)); err == nil {
				t.Fatal("nonregular identity accepted")
			}
		})
	}
}

func TestRuntimeHandlerTracksMetricsAndDrain(t *testing.T) {
	metrics, err := telemetry.New("pnld", "qualification", "zone-a")
	if err != nil {
		t.Fatal(err)
	}
	tracker := sharedprocess.NewTracker()
	if err = tracker.SetReady(); err != nil {
		t.Fatal(err)
	}
	handler := runtimeHandler(tracker, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/invalid" {
			w.WriteHeader(422)
		} else if r.URL.Path == "/explicit" {
			w.WriteHeader(204)
		}
	}), metrics)
	for _, path := range []string{"/empty", "/invalid", "/explicit", "/health", "/health/live", "/health/ready", "/metrics"} {
		request := httptest.NewRequest("GET", path, nil)
		if path == "/empty" {
			ctx, cancel := context.WithCancel(request.Context())
			cancel()
			request = request.WithContext(ctx)
		}
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, request)
		want := 200
		if path == "/invalid" {
			want = 422
		}
		if path == "/explicit" {
			want = 204
		}
		if out.Code != want {
			t.Fatal(path, out.Code)
		}
	}
	tracker.BeginDrain()
	out := httptest.NewRecorder()
	handler.ServeHTTP(out, httptest.NewRequest("GET", "/empty", nil))
	if out.Code != 503 {
		t.Fatal(out.Code)
	}
	out = httptest.NewRecorder()
	handler.ServeHTTP(out, httptest.NewRequest("GET", "/health/live", nil))
	if out.Code != 200 {
		t.Fatal(out.Code)
	}
}

func TestBackgroundLoopContinuesAfterErrorAndStopsOnCancellation(t *testing.T) {
	a := &App{logger: slog.New(slog.NewTextHandler(io.Discard, nil))}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	var calls atomic.Int64
	go func() {
		defer close(done)
		a.loop(ctx, time.Millisecond, "reconcile", func(context.Context) error {
			if calls.Add(1) == 2 {
				cancel()
			}
			return errors.New("dependency unavailable")
		})
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("loop did not stop")
	}
	if calls.Load() != 2 {
		t.Fatal(calls.Load())
	}
}

func TestDevelopmentRuntimeStartsAndShutsDown(t *testing.T) {
	dsn, developmentPool := internaltest.NewPostgresWithDSN(t)
	internaltest.EnsureIndexerSchema(t, developmentPool)
	redisURL := internaltest.NewRedisURL(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	endpoint := httptest.NewServer(http.NotFoundHandler())
	defer endpoint.Close()
	for key, role := range map[string]string{"EVENT_EMITTER_ADDRESS": "EventEmitter", "POOL_REGISTRY_ADDRESS": "PoolRegistry", "ROUTER_ADDRESS": "Router", "FACTORY_ADDRESS": "SidioraFactory", "QUOTER_ADDRESS": "Quoter", "PROTOCOL_CONFIG_ADDRESS": "ProtocolConfig", "FEE_ACCUMULATOR_ADDRESS": "FeeAccumulator", "SIDIORA_NFT_ADDRESS": "SidioraNFT", "FEES_ROUTER_ADDRESS": "FeesRouter", "POOL_BEACON_ADDRESS": "PoolBeacon"} {
		t.Setenv(key, deployment.Current().Roles[role].Address)
	}
	for key, value := range map[string]string{"NODE_ENV": "test", "DATABASE_URL": dsn, "REDIS_URL": redisURL, "REDIS_BULL_URL": redisURL, "RPC_URL": endpoint.URL, "WEBHOOK_HMAC_SECRET": strings.Repeat("s", 32), "PORT": strconv.Itoa(port), "BIND_ADDRESS": "127.0.0.1", "PNL_RECONCILE_INTERVAL_MS": "1", "LOG_LEVEL": "error"} {
		t.Setenv(key, value)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx) }()
	client := &http.Client{Timeout: 100 * time.Millisecond}
	ready := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		response, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health/live", port))
		if err == nil {
			response.Body.Close()
			if response.StatusCode == 200 {
				ready = true
				break
			}
		}
		select {
		case err := <-done:
			t.Fatal(err)
		case <-time.After(10 * time.Millisecond):
		}
	}
	if !ready {
		t.Fatal("runtime did not become live")
	}
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(sharedprocess.PropagationDelay + time.Second):
		t.Fatal("runtime did not stop")
	}

	for _, cfg := range []config.Config{{BaseEnv: sharedconfig.BaseEnv{NodeEnv: "invalid"}}, {BaseEnv: sharedconfig.BaseEnv{DatabaseURL: "invalid"}}} {
		if a, err := New(context.Background(), cfg, slog.Default()); err == nil {
			a.Close()
			t.Fatal("invalid configuration accepted")
		}
	}
}
