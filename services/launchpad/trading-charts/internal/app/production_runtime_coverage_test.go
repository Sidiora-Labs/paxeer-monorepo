package app

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
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
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	production "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nkeys"
)

func productionChartCredentials(t *testing.T) string {
	t.Helper()
	account, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	user, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	accountID, err := account.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	userID, err := user.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	encode := base64.RawURLEncoding.EncodeToString
	message := encode([]byte(`{"typ":"JWT","alg":"ed25519-nkey"}`)) + "." + encode([]byte(`{"sub":"`+userID+`","iss":"`+accountID+`","nats":{"type":"user","version":2}}`))
	signature, err := account.Sign([]byte(message))
	if err != nil {
		t.Fatal(err)
	}
	seed, err := user.Seed()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "charts.creds")
	content := "-----BEGIN NATS USER JWT-----\n" + message + "." + encode(signature) + "\n------END NATS USER JWT------\n\n-----BEGIN USER NKEY SEED-----\n" + string(seed) + "\n------END USER NKEY SEED------\n"
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
	return path
}

func productionChartDatabase(t *testing.T) (string, production.ExpectedSchema, production.ConnectionBudget) {
	t.Helper()
	ctx := context.Background()
	adminDSN := os.Getenv(internaltest.PostgresEnv)
	admin, err := pgx.Connect(ctx, adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = admin.Close(ctx) }()
	var nonce [5]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		t.Fatal(err)
	}
	role := "charts_rt_" + hex.EncodeToString(nonce[:])
	var secret [16]byte
	if _, err := rand.Read(secret[:]); err != nil {
		t.Fatal(err)
	}
	password := hex.EncodeToString(secret[:])
	if _, err := admin.Exec(ctx, "CREATE ROLE "+pgx.Identifier{role}.Sanitize()+" LOGIN PASSWORD '"+password+"'"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		connection, err := pgx.Connect(context.Background(), adminDSN)
		if err != nil {
			t.Error(err)
			return
		}
		defer func() { _ = connection.Close(context.Background()) }()
		if _, err := connection.Exec(context.Background(), "DROP ROLE IF EXISTS "+pgx.Identifier{role}.Sanitize()); err != nil {
			t.Error(err)
		}
	})
	dsn, pool := internaltest.NewPostgres(t)
	for _, sql := range []string{
		`CREATE SCHEMA migration_control`,
		`CREATE SCHEMA recovery`,
		`REVOKE CREATE ON SCHEMA public FROM PUBLIC`,
		`REVOKE EXECUTE ON ALL FUNCTIONS IN SCHEMA candles FROM PUBLIC`,
		`CREATE FUNCTION migration_control.readiness(text) RETURNS TABLE(registry_digest text,interface_digest text,reader_min_version bigint,migrations jsonb) LANGUAGE sql STABLE AS $$ SELECT repeat('a',64),repeat('b',64),1::bigint,jsonb_build_object('1',repeat('c',64)) $$`,
		`CREATE FUNCTION recovery.current_epoch() RETURNS TABLE(epoch bigint,region text) LANGUAGE sql STABLE AS $$ SELECT 1::bigint,'local'::text $$`,
		"GRANT USAGE ON SCHEMA candles,migration_control,recovery TO " + pgx.Identifier{role}.Sanitize(),
		"GRANT SELECT ON candles.candles TO " + pgx.Identifier{role}.Sanitize(),
		"GRANT EXECUTE ON FUNCTION migration_control.readiness(text),recovery.current_epoch() TO " + pgx.Identifier{role}.Sanitize(),
	} {
		if _, err := pool.Exec(ctx, sql); err != nil {
			t.Fatalf("runtime database schema: %v", err)
		}
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	fingerprint, err := production.CatalogFingerprint(ctx, tx, "candles")
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	expected := production.ExpectedSchema{
		Service: "core/trading-charts", RuntimeRole: role,
		RegistryDigest: strings.Repeat("a", 64), InterfaceDigest: strings.Repeat("b", 64), ReaderVersion: 1,
		Migrations: map[int64]string{1: strings.Repeat("c", 64)}, Schemas: map[string]string{"candles": fingerprint}, Epoch: 1, Region: "local",
		Privileges: []production.Grant{
			{Schema: "candles", Object: "candles", Privileges: []string{"SELECT"}},
			{Schema: "migration_control", Object: "readiness", ObjectKind: "function", ArgumentTypes: []string{"text"}, Privileges: []string{"EXECUTE"}},
			{Schema: "recovery", Object: "current_epoch", ObjectKind: "function", Privileges: []string{"EXECUTE"}},
		},
	}
	parsed, err := url.Parse(dsn)
	if err != nil {
		t.Fatal(err)
	}
	parsed.User = url.UserPassword(role, password)
	port64, err := strconv.ParseUint(parsed.Port(), 10, 16)
	if err != nil {
		t.Fatal(err)
	}
	var maxConnections int64
	if err := pool.QueryRow(ctx, `SELECT setting::bigint FROM pg_settings WHERE name='max_connections'`).Scan(&maxConnections); err != nil {
		t.Fatal(err)
	}
	budget := production.ConnectionBudget{Identity: "charts_runtime_fixture", Targets: []production.PostgresTarget{{ID: "fixture_pg", Member: "fixture_pg_primary", Host: parsed.Hostname(), Port: uint16(port64), MaxConnections: maxConnections}}, Operational: []production.OperationalBudget{{ID: "migration", Target: "fixture_pg", Category: "migration", Connections: 2}}}
	for i, name := range []string{"apid:rest", "apid:sockets", "indexerd", "chartsd", "statsd", "pnld", "rankingd", "mediagatewayd", "userd", "sociald", "metadatad", "livestreamd"} {
		user := fmt.Sprintf("unused_runtime_%d", i)
		poolID := fmt.Sprintf("runtime_%d", i)
		if name == "chartsd" {
			user, poolID = role, "charts_batch"
		}
		budget.Clients = append(budget.Clients, production.ClientBudget{Role: name, PoolID: poolID, Target: "fixture_pg", Transport: runtimedb.TransportDirect, Alias: strings.TrimPrefix(parsed.Path, "/"), User: user, QueryClass: runtimedb.Batch, Processes: production.LiveProcesses{Replicas: 1, Surge: 1, Maximum: 2}, MaxConnections: 1})
	}
	if err := budget.Validate(); err != nil {
		t.Fatal(err)
	}
	return parsed.String(), expected, budget
}

func TestProductionAppConnectsCanonicalProvidersAndServesRuntimeRoutes(t *testing.T) {
	ctx := context.Background()
	dsn, expected, budget := productionChartDatabase(t)
	redisURL := internaltest.NewRedisURL(t)
	_, anchor, rpcURL := internaltest.NewChainSpanAtWithURL(t, 0, 0)
	nc, stream := internaltest.NewBroker(t, "CHART_RUNTIME", []string{accounting.MarketSubject, "candle.updated.v2"})
	consumerConfig := natsdriver.ConsumerConfig{Stream: "CHART_RUNTIME", Durable: "charts_runtime", Subject: accounting.MarketSubject, Batch: 1, MaxAckPending: 100, MaxWaiting: 1, MaxDeliver: 20, FetchWait: 100 * time.Millisecond, AckWait: time.Second}
	brokerConfig, err := consumerConfig.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := stream.CreateConsumer(ctx, brokerConfig); err != nil {
		t.Fatal(err)
	}
	policy := delivery.Policy{Subjects: map[string]bool{accounting.MarketSubject: true, "candle.updated.v2": true}, EventTypes: map[string]bool{"market": true}}
	document := chartRuntimeDocument{Schema: "candles", SourceStream: "canonical", ConsumerID: "charts_runtime", Generation: "runtime_1", Owner: "runtime_owner", ProjectionVersion: "market-v1", Epoch: 1, Region: "local", MaxReplayBlocks: 4, MaxReplayFills: 10, Policy: policy, Consumer: consumerConfig, ChartOutbox: chartOutboxConfig{Stream: "CHART_RUNTIME", Subject: "candle.updated.v2", Batch: 1, IntervalMillis: 1000}}
	deliveryWire, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	deliveryPath := filepath.Join(t.TempDir(), "delivery.json")
	if err := os.WriteFile(deliveryPath, deliveryWire, 0600); err != nil {
		t.Fatal(err)
	}
	features := config.ChartRuntimeFeatures{SchemaVersion: 2, EndpointSetVersion: "local", DeploymentVersion: deployment.Current().DeploymentVersion, AnchorHeight: 1, AnchorHash: anchor.Hash, CanonicalAPIOrigin: rpcURL, ReplayGatewayOrigin: rpcURL, CanonicalSeriesModel: "qualified-exact-v2", DisplayPrecision: 6, MaxHistoryOrigins: 2, RollupBatch: 4, RollupIntervalMillis: 1000}
	featureWire, err := json.Marshal(features)
	if err != nil {
		t.Fatal(err)
	}
	expectedWire, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	budgetWire, err := json.Marshal(budget)
	if err != nil {
		t.Fatal(err)
	}
	credentials := productionChartCredentials(t)
	for key, value := range map[string]string{"DATABASE_URL": dsn, "REDIS_URL": redisURL, "RPC_URL": rpcURL, "WEBHOOK_HMAC_SECRET": strings.Repeat("s", 32), "NATS_URL": nc.ConnectedUrl(), "NATS_CREDENTIALS": credentials} {
		t.Setenv(key, value)
	}
	cfg := config.Config{
		RuntimeMetricsEnvironment: "qualification", RuntimeZone: "zone-a", RuntimeProfile: "owned_local", BindAddress: "127.0.0.1",
		RuntimeBudgetDocument: base64.StdEncoding.EncodeToString(budgetWire), RuntimeBudgetSHA256: production.Checksum(budgetWire), RuntimePoolID: "charts_batch", RuntimeInstanceID: "charts_fixture",
		ExpectedSchemaDocument: base64.StdEncoding.EncodeToString(expectedWire), ExpectedSchemaSHA256: production.Checksum(expectedWire),
		FeaturesConfigDocument: base64.StdEncoding.EncodeToString(featureWire), FeaturesConfigSHA256: production.Checksum(featureWire),
		DeliveryConfigFile: deliveryPath, DeliveryConfigSHA256: production.Checksum(deliveryWire), NATSCredentials: credentials,
	}
	cfg.NodeEnv, cfg.SecretsProvider, cfg.SecretsRefreshSeconds, cfg.Port = "production", "env", 1, 3000
	application, err := New(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(application.Close)
	baseConfig := cfg
	if application.accounting == nil || application.exact == nil || application.transport == nil || application.chartPublisher == nil || application.verifiedRPC == nil || application.features.AnchorHash != anchor.Hash {
		t.Fatalf("production provider wiring: %+v", application)
	}
	if err := verifyArchiveAnchor(ctx, application.verifiedRPC, features.AnchorHeight, features.AnchorHash); err != nil {
		t.Fatalf("canonical runtime anchor: %v", err)
	}
	if err := verifyArchiveAnchor(ctx, application.verifiedRPC, features.AnchorHeight, "0x"+strings.Repeat("f", 64)); err == nil || !strings.Contains(err.Error(), "anchor changed") {
		t.Fatalf("different runtime anchor: %v", err)
	}
	if err := verifyArchiveAnchor(ctx, application.verifiedRPC, features.AnchorHeight+100000, features.AnchorHash); err == nil {
		t.Fatal("unavailable runtime anchor was admitted")
	}
	for _, path := range []string{"/health/live", "/metrics"} {
		response := httptest.NewRecorder()
		application.Router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != http.StatusOK || response.Body.Len() == 0 {
			t.Fatalf("runtime route %s: %d %s", path, response.Code, response.Body.String())
		}
	}
	info, err := stream.Info(ctx)
	if err != nil || info.Config.Storage != jetstream.FileStorage {
		t.Fatalf("owned file-backed broker: %+v, %v", info, err)
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	boundedOutbox := application.chartOutbox
	application.chartOutbox.Batch = 0
	if err := application.startChartOutbox(ctx, logger); err == nil || !strings.Contains(err.Error(), "bounded chart outbox configuration") || application.chartRelayDone != nil {
		t.Fatalf("outbox batch admission: %v", err)
	}
	application.chartOutbox = boundedOutbox
	application.tracker.BeginDrain()
	if err := application.startChartOutbox(ctx, logger); err == nil || application.chartRelayDone != nil {
		t.Fatalf("outbox after runtime drain: %v", err)
	}
	application.Close()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"NODE_ENV": "production", "PORT": strconv.Itoa(port), "BIND_ADDRESS": "127.0.0.1", "REDIS_BULL_URL": redisURL,
		"RUNTIME_PROFILE": "owned_local", "ZONE": "zone-a", "METRICS_ENVIRONMENT": "qualification",
		"DB_BUDGET_DOCUMENT": cfg.RuntimeBudgetDocument, "DB_BUDGET_SHA256": cfg.RuntimeBudgetSHA256, "DB_POOL_ID": cfg.RuntimePoolID, "INSTANCE_ID": cfg.RuntimeInstanceID,
		"EXPECTED_SCHEMA_DOCUMENT": cfg.ExpectedSchemaDocument, "EXPECTED_SCHEMA_SHA256": cfg.ExpectedSchemaSHA256,
		"CHART_FEATURES_CONFIG_DOCUMENT": cfg.FeaturesConfigDocument, "CHART_FEATURES_CONFIG_SHA256": cfg.FeaturesConfigSHA256,
		"DELIVERY_CONFIG_FILE": cfg.DeliveryConfigFile, "DELIVERY_CONFIG_SHA256": cfg.DeliveryConfigSHA256,
		"SECRETS_PROVIDER": "env", "SECRETS_REFRESH_SECONDS": "1", "LOG_LEVEL": "error",
	} {
		t.Setenv(key, value)
	}
	for key, role := range map[string]string{
		"EVENT_EMITTER_ADDRESS": "EventEmitter", "POOL_REGISTRY_ADDRESS": "PoolRegistry", "ROUTER_ADDRESS": "Router", "FACTORY_ADDRESS": "SidioraFactory", "QUOTER_ADDRESS": "Quoter", "PROTOCOL_CONFIG_ADDRESS": "ProtocolConfig", "FEE_ACCUMULATOR_ADDRESS": "FeeAccumulator", "SIDIORA_NFT_ADDRESS": "SidioraNFT", "FEES_ROUTER_ADDRESS": "FeesRouter", "POOL_BEACON_ADDRESS": "PoolBeacon",
	} {
		t.Setenv(key, deployment.Current().Roles[role].Address)
	}
	runCtx, stop := context.WithCancel(ctx)
	done := make(chan error, 1)
	go func() { done <- Run(runCtx) }()
	ready := false
	probe := http.Client{Timeout: 200 * time.Millisecond}
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		response, probeErr := probe.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/health/live")
		if probeErr == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ready {
		stop()
		t.Fatal("production runtime did not become ready before shutdown")
	}
	stop()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("production runtime shutdown: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("production runtime did not shut down")
	}
	rollupCtx, stopRollup := context.WithCancel(ctx)
	rollupDone := make(chan error, 1)
	go func() { rollupDone <- Run(rollupCtx) }()
	ready = false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		response, probeErr := probe.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/health/live")
		if probeErr == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ready {
		stopRollup()
		t.Fatal("runtime did not become ready")
	}
	select {
	case err := <-rollupDone:
		if err != nil {
			t.Fatalf("runtime with unavailable rollup authority: %v", err)
		}
	case <-time.After(5 * time.Second):
		stopRollup()
		t.Fatal("runtime did not drain when rollup authority was unavailable")
	}
	stopRollup()
	consumerCtx, stopConsumer := context.WithCancel(ctx)
	consumerDone := make(chan error, 1)
	go func() { consumerDone <- Run(consumerCtx) }()
	ready = false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		response, probeErr := probe.Get("http://127.0.0.1:" + strconv.Itoa(port) + "/health/live")
		if probeErr == nil {
			_, _ = io.Copy(io.Discard, response.Body)
			_ = response.Body.Close()
			if response.StatusCode == http.StatusOK {
				ready = true
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ready {
		stopConsumer()
		t.Fatal("runtime health did not become ready")
	}
	if err := stream.DeleteConsumer(ctx, consumerConfig.Durable); err != nil {
		stopConsumer()
		t.Fatal(err)
	}
	select {
	case err := <-consumerDone:
		if err != nil {
			t.Fatalf("runtime after retained consumer removal: %v", err)
		}
	case <-time.After(5 * time.Second):
		stopConsumer()
		t.Fatal("runtime did not drain after broker consumer removal")
	}
	stopConsumer()
	if _, err := stream.CreateConsumer(ctx, brokerConfig); err != nil {
		t.Fatal(err)
	}
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("PORT", strconv.Itoa(occupied.Addr().(*net.TCPAddr).Port))
	serveCtx, stopServe := context.WithTimeout(ctx, 10*time.Second)
	serveErr := Run(serveCtx)
	stopServe()
	if err := occupied.Close(); err != nil {
		t.Fatal(err)
	}
	if serveErr == nil || !strings.Contains(serveErr.Error(), "address already in use") {
		t.Fatalf("occupied runtime listener: %v", serveErr)
	}
	_, objectConfig := internaltest.NewObjectStoreWithConfig(t)
	features.Archive = &config.ChartArchiveFeatures{Endpoint: objectConfig.Endpoint, Region: objectConfig.Region, Bucket: objectConfig.Bucket}
	featureWire, err = json.Marshal(features)
	if err != nil {
		t.Fatal(err)
	}
	cfg.FeaturesConfigDocument = base64.StdEncoding.EncodeToString(featureWire)
	cfg.FeaturesConfigSHA256 = production.Checksum(featureWire)
	var token [24]byte
	if _, err := rand.Read(token[:]); err != nil {
		t.Fatal(err)
	}
	t.Setenv("S3_ACCESS_KEY_ID", objectConfig.AccessKeyID)
	t.Setenv("S3_SECRET_ACCESS_KEY", objectConfig.SecretAccessKey)
	operator := hex.EncodeToString(token[:])
	t.Setenv("CHART_OPERATOR_TOKEN", operator)
	archiveApp, err := New(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(archiveApp.Close)
	request := httptest.NewRequest(http.MethodPost, "/internal/v2/charts/retention:apply", strings.NewReader(`{}`))
	response := httptest.NewRecorder()
	archiveApp.Router.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("archive operator admission: %d %s", response.Code, response.Body.String())
	}
	for _, tc := range []struct {
		name, path, authorization, body string
		status                          int
	}{
		{"retention rejects unknown pool", "/internal/v2/charts/retention:apply", "Bearer " + operator, `{}`, http.StatusUnprocessableEntity},
		{"rebuild requires bearer", "/internal/v2/charts/archive:rebuild", "", `{}`, http.StatusUnauthorized},
		{"rebuild rejects other operator", "/internal/v2/charts/archive:rebuild", "Bearer other", `{}`, http.StatusUnauthorized},
		{"rebuild rejects unbounded request", "/internal/v2/charts/archive:rebuild", "Bearer " + operator, `{}`, http.StatusUnprocessableEntity},
		{"rebuild rejects missing archive", "/internal/v2/charts/archive:rebuild", "Bearer " + operator, `{"runId":"local_rebuild","pool":"0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa","from":1704067200,"to":1704067260,"manifests":[]}`, http.StatusServiceUnavailable},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, tc.path, strings.NewReader(tc.body))
			if tc.authorization != "" {
				req.Header.Set("Authorization", tc.authorization)
			}
			response := httptest.NewRecorder()
			archiveApp.Router.ServeHTTP(response, req)
			if response.Code != tc.status {
				t.Fatalf("archive route %s: %d %s", tc.path, response.Code, response.Body.String())
			}
		})
	}
	archiveApp.Close()
	bindDelivery := func(value chartRuntimeDocument) (string, string) {
		t.Helper()
		wire, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "runtime.json")
		if err := os.WriteFile(path, wire, 0600); err != nil {
			t.Fatal(err)
		}
		return path, production.Checksum(wire)
	}
	bindFeatures := func(c *config.Config, value config.ChartRuntimeFeatures) {
		t.Helper()
		wire, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		c.FeaturesConfigDocument = base64.StdEncoding.EncodeToString(wire)
		c.FeaturesConfigSHA256 = production.Checksum(wire)
	}
	for _, tc := range []struct {
		name, want string
		change     func(*config.Config)
	}{
		{"unbound production secrets", "binding", func(c *config.Config) { c.FeaturesConfigSHA256 = strings.Repeat("0", 64) }},
		{"invalid telemetry identity", "telemetry_identity_outside_inventory", func(c *config.Config) { c.RuntimeZone = "unallocated" }},
		{"unbound delivery document", "binding", func(c *config.Config) { c.DeliveryConfigFile = "" }},
		{"incomplete runtime document", "incomplete runtime configuration", func(c *config.Config) {
			d := document
			d.Owner = ""
			c.DeliveryConfigFile, c.DeliveryConfigSHA256 = bindDelivery(d)
		}},
		{"unadmitted outbox subject", "not admitted", func(c *config.Config) {
			d := document
			d.Policy.Subjects = map[string]bool{accounting.MarketSubject: true}
			c.DeliveryConfigFile, c.DeliveryConfigSHA256 = bindDelivery(d)
		}},
		{"outbox batch outside bound", "bounded chart outbox configuration", func(c *config.Config) {
			d := document
			d.ChartOutbox.Batch = 0
			c.DeliveryConfigFile, c.DeliveryConfigSHA256 = bindDelivery(d)
		}},
		{"invalid chart runtime features", "charts_features_invalid", func(c *config.Config) {
			f := features
			f.SchemaVersion = 0
			bindFeatures(c, f)
		}},
		{"unavailable chart anchor", "anchor", func(c *config.Config) {
			f := features
			f.AnchorHash = "0x" + strings.Repeat("f", 64)
			bindFeatures(c, f)
		}},
		{"invalid recovery epoch", "invalid recovery authority", func(c *config.Config) {
			d := document
			d.Epoch = 0
			c.DeliveryConfigFile, c.DeliveryConfigSHA256 = bindDelivery(d)
		}},
		{"incomplete delivery policy", "invalid store configuration", func(c *config.Config) {
			d := document
			d.Policy.EventTypes = map[string]bool{}
			c.DeliveryConfigFile, c.DeliveryConfigSHA256 = bindDelivery(d)
		}},
		{"unbounded replay", "bounded replay configuration", func(c *config.Config) {
			d := document
			d.MaxReplayBlocks = 0
			c.DeliveryConfigFile, c.DeliveryConfigSHA256 = bindDelivery(d)
		}},
		{"unknown durable", "consumer not found", func(c *config.Config) {
			d := document
			d.Consumer.Durable = "unprovisioned"
			c.DeliveryConfigFile, c.DeliveryConfigSHA256 = bindDelivery(d)
		}},
		{"unavailable inbound subject", "durable configuration differs", func(c *config.Config) {
			d := document
			d.Consumer.Subject = "chart.unavailable"
			c.DeliveryConfigFile, c.DeliveryConfigSHA256 = bindDelivery(d)
		}},
		{"unavailable broker", "no servers available", func(_ *config.Config) { t.Setenv("NATS_URL", "nats://127.0.0.1:1") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			variant := baseConfig
			tc.change(&variant)
			opened, err := New(ctx, variant, slog.New(slog.NewTextHandler(io.Discard, nil)))
			if opened != nil {
				opened.Close()
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("runtime admission: %v", err)
			}
		})
	}
	t.Run("archive operator token", func(t *testing.T) {
		t.Setenv("CHART_OPERATOR_TOKEN", "")
		opened, err := New(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)))
		if opened != nil {
			opened.Close()
		}
		if err == nil || !strings.Contains(err.Error(), "required_secret_unavailable: CHART_OPERATOR_TOKEN") {
			t.Fatalf("archive without operator authority: %v", err)
		}
	})
	t.Setenv("REDIS_URL", "not a redis URL")
	opened, err := New(ctx, baseConfig, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if opened != nil {
		opened.Close()
	}
	if err == nil || !strings.Contains(err.Error(), "redis: parse url") {
		t.Fatalf("invalid Redis authority: %v", err)
	}
}
