package app

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum/ethclient"
)

func lifecycleDocuments(t *testing.T) (deliveryRuntimeDocument, lifecycleRuntimeDocument, config.Config) {
	t.Helper()
	parameters := DefaultDeliveryParameters()
	parameters.MaxDeliveryLagMS = 60000
	documents, err := RenderRuntimeDocuments(renderInputs(t), parameters)
	if err != nil {
		t.Fatal(err)
	}
	var d deliveryRuntimeDocument
	var l lifecycleRuntimeDocument
	if err = json.Unmarshal(documents["delivery.json"].Bytes, &d); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(documents["lifecycle.json"].Bytes, &l); err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{IndexerMode: config.ModeLive, BackfillBatchSize: 32}
	cfg.ChainID = 125
	return d, l, cfg
}
func lifecycleRequest() store.ReplayRequest {
	return store.ReplayRequest{JobID: "history", SourceStream: "canonical", Target: "charts", Generation: "g1", ProjectionRevision: "v1", Mode: "replay", Requester: "operator", AuthorizationID: "approved", FromHeight: 101, ToHeight: 102, CheckpointHeight: 102, CheckpointHash: "0x" + strings.Repeat("1", 64)}
}
func TestLifecycleConfigurationBindsReplayArchiveAndPublications(t *testing.T) {
	cases := []struct {
		name string
		edit func(*deliveryRuntimeDocument, *lifecycleRuntimeDocument, *config.Config)
	}{
		{"chain", func(_ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument, c *config.Config) { c.ChainID = 1 }},
		{"version", func(_ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument, _ *config.Config) { l.Version = 1 }},
		{"explorer authority", func(_ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument, _ *config.Config) {
			l.Explorer = &source.CommittedPaxscanOptions{RPC: &source.CommittedSource{}}
		}},
		{"creation subject", func(d *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument, _ *config.Config) {
			d.Subjects["canonical_creation"] = "other"
		}},
		{"replay policy", func(d *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument, _ *config.Config) {
			delete(d.Policy.Subjects, "indexer.replay.v2")
		}},
		{"backfill configuration", func(_ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument, c *config.Config) {
			c.IndexerMode = config.ModeBackfill
		}},
		{"replay identity", func(_ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument, _ *config.Config) {
			l.ReplayRequests = []store.ReplayRequest{{}}
		}},
		{"replay mode", func(d *deliveryRuntimeDocument, l *lifecycleRuntimeDocument, _ *config.Config) {
			r := lifecycleRequest()
			r.SourceStream = d.CanonicalStreamID
			r.Mode = "backfill"
			l.ReplayRequests = []store.ReplayRequest{r}
		}},
		{"replay source", func(_ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument, _ *config.Config) {
			l.ReplayRequests = []store.ReplayRequest{lifecycleRequest()}
		}},
		{"backfill identity", func(_ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument, _ *config.Config) {
			l.BackfillRequest = &store.ReplayRequest{}
		}},
		{"backfill mode", func(d *deliveryRuntimeDocument, l *lifecycleRuntimeDocument, _ *config.Config) {
			r := lifecycleRequest()
			r.SourceStream = d.CanonicalStreamID
			l.BackfillRequest = &r
		}},
		{"webhook bounds", func(_ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument, _ *config.Config) {
			l.Webhooks.Targets = []delivery.WebhookTarget{{}}
		}},
		{"archive credentials", func(_ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument, _ *config.Config) {
			l.Archive.Enabled = true
		}},
		{"rebuild archive", func(_ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument, _ *config.Config) {
			l.Rebuilds = []lifecycleRebuildDocument{{}}
		}},
		{"rebuild identity", func(_ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument, _ *config.Config) {
			l.Archive = validLifecycleArchive()
			l.Rebuilds = []lifecycleRebuildDocument{{}}
		}},
		{"rebuild mode", func(d *deliveryRuntimeDocument, l *lifecycleRuntimeDocument, _ *config.Config) {
			l.Archive = validLifecycleArchive()
			r := lifecycleRequest()
			r.SourceStream = d.CanonicalStreamID
			l.Rebuilds = []lifecycleRebuildDocument{{Request: r}}
		}},
		{"promotion authorization", func(d *deliveryRuntimeDocument, l *lifecycleRuntimeDocument, _ *config.Config) {
			l.Archive = validLifecycleArchive()
			r := lifecycleRequest()
			r.SourceStream = d.CanonicalStreamID
			r.Mode = "reconstruct"
			l.Rebuilds = []lifecycleRebuildDocument{{Request: r, ConsumerID: "charts", ManifestDigest: strings.Repeat("a", 64), Promote: true}}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, l, c := lifecycleDocuments(t)
			tc.edit(&d, &l, &c)
			if err := l.validate(d, c); err == nil {
				t.Fatal("unbound lifecycle input accepted")
			}
		})
	}
	d, l, c := lifecycleDocuments(t)
	l.Committed.Endpoints = nil
	r := lifecycleRequest()
	r.SourceStream = d.CanonicalStreamID
	l.ReplayRequests = []store.ReplayRequest{r}
	r.Mode = "backfill"
	l.BackfillRequest = &r
	c.IndexerMode = config.ModeBackfill
	l.Archive = validLifecycleArchive()
	rebuildRequest := r
	rebuildRequest.Mode = "reconstruct"
	l.Rebuilds = []lifecycleRebuildDocument{{Request: rebuildRequest, ConsumerID: "charts", ManifestDigest: strings.Repeat("a", 64), Promote: true}}
	l.Operations = map[string]store.OperationScope{"replay.promote": {}}
	if err := l.validate(d, c); err != nil || l.Committed.Endpoints == nil {
		t.Fatalf("bound lifecycle input %v", err)
	}
}
func validLifecycleArchive() lifecycleArchiveDocument {
	return lifecycleArchiveDocument{Enabled: true, Endpoint: "http://localhost", Bucket: "archive", AccessKeyReference: "ARCHIVE_ACCESS", SecretKeyReference: "ARCHIVE_SECRET", ReadinessObject: "ready", ReadinessObjectBytes: 1, FromHeight: 101, BatchBlocks: 32, PollMS: 1000}
}

func TestRuntimeWorkersExposeFirstRunFailuresAndCancellation(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	workers := newRuntimeWorkers(logger)
	var calls atomic.Int64
	workers.AddPeriodic("poll", time.Millisecond, false, func(context.Context) error { calls.Add(1); return nil })
	workers.AddOnce("history", func(context.Context) error { return nil })
	if err := workers.Ready(context.Background()); err == nil {
		t.Fatal("unstarted workers ready")
	}
	if err := workers.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer workers.Close()
	deadline := time.After(time.Second)
	for workers.Ready(context.Background()) != nil {
		select {
		case <-deadline:
			t.Fatal("workers did not become ready")
		case <-time.After(time.Millisecond):
		}
	}
	if err := workers.Start(context.Background()); err == nil {
		t.Fatal("workers started twice")
	}
	if calls.Load() == 0 {
		t.Fatal("periodic work did not execute")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := workers.Ready(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled readiness %v", err)
	}
	workers.Close()
	failed := newRuntimeWorkers(logger)
	failed.AddPeriodic("archive", time.Second, true, func(context.Context) error { return errors.New("archive unavailable") })
	if err := failed.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	defer failed.Close()
	failed.wg.Wait()
	if err := failed.Ready(context.Background()); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("failed worker readiness %v", err)
	}
	blocked := newRuntimeWorkers(logger)
	entered := make(chan struct{})
	release := make(chan struct{})
	blocked.AddOnce("drain", func(context.Context) error { close(entered); <-release; return nil })
	if err := blocked.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	<-entered
	blocked.CloseContext(ctx)
	close(release)
	blocked.wg.Wait()
	slots := make(chan struct{}, 1)
	sentinel := errors.New("work rejected")
	if err := withRuntimeSlot(context.Background(), slots, func() error { return sentinel }); !errors.Is(err, sentinel) || len(slots) != 0 {
		t.Fatalf("slot result %v size %d", err, len(slots))
	}
	slots <- struct{}{}
	if err := withRuntimeSlot(ctx, slots, func() error { t.Fatal("cancelled slot acquired"); return nil }); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled slot %v", err)
	}
}

func TestWebhookTransportBindsTrustedCertificateBytes(t *testing.T) {
	pair, _, _ := status1695TLS(t)
	body := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: pair.Certificate[0]})
	path := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(path, body, 0600); err != nil {
		t.Fatal(err)
	}
	doc := lifecycleWebhookDocument{CAFile: path, CASHA256: fmt.Sprintf("%x", sha256.Sum256(body))}
	transport, err := runtimeWebhookTransport(doc)
	if err != nil || transport.TLSClientConfig.MinVersion != tls.VersionTLS12 || transport.TLSClientConfig.RootCAs == nil {
		t.Fatalf("trusted webhook transport %v", err)
	}
	transport.CloseIdleConnections()
	for _, tc := range []lifecycleWebhookDocument{{CAFile: path + ".absent", CASHA256: doc.CASHA256}, {CAFile: filepath.Dir(path), CASHA256: doc.CASHA256}, {CAFile: path, CASHA256: strings.Repeat("0", 64)}, {CASHA256: doc.CASHA256}} {
		if got, err := runtimeWebhookTransport(tc); err == nil {
			got.CloseIdleConnections()
			t.Fatal("unbound trust roots accepted")
		}
	}
	if err = os.WriteFile(path, []byte("invalid pem"), 0600); err != nil {
		t.Fatal(err)
	}
	doc.CASHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte("invalid pem")))
	if _, err = runtimeWebhookTransport(doc); err == nil {
		t.Fatal("invalid certificate accepted")
	}
	if transport, err = runtimeWebhookTransport(lifecycleWebhookDocument{}); err != nil || transport.TLSClientConfig.RootCAs != nil {
		t.Fatalf("system trust roots %v", err)
	} else {
		transport.CloseIdleConnections()
	}
}

func legacyLifecycleConfig(t *testing.T) config.Config {
	t.Helper()
	dsn, _ := internaltest.NewPostgres(t)
	cfg := config.Config{IndexerMode: config.ModeLive, IndexerLogSource: config.SourceRPC, IndexerHeadSource: config.SourceRPC, IndexerBatchSize: 32, IndexerConcurrency: 1, IndexerPollIntervalMS: 10, BackfillBatchSize: 32}
	cfg.NodeEnv = "test"
	cfg.ChainID = 125
	cfg.DatabaseURL = dsn
	cfg.RedisURL = os.Getenv(internaltest.ExternalRedisEnv)
	cfg.RPCURL = lifecycleNode(t)
	cfg.EventEmitterAddress = deployment.Current().Roles["EventEmitter"].Address
	cfg.PoolRegistryAddress = deployment.Current().Roles["PoolRegistry"].Address
	return cfg
}
func TestLegacyApplicationBuildsProvidersAndReleasesResources(t *testing.T) {
	cfg := legacyLifecycleConfig(t)
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	app, err := New(context.Background(), cfg, logger)
	if err != nil {
		t.Fatal(err)
	}
	if app.SourceCapacityProviders().Live != nil || (*App)(nil).SourceCapacityProviders().Live != nil {
		t.Fatal("legacy application offered canonical measurement")
	}
	rr := httptest.NewRecorder()
	app.Router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/status", nil))
	if rr.Code != http.StatusOK || !strings.Contains(rr.Body.String(), `"mode":"legacy"`) {
		t.Fatalf("legacy status %d %s", rr.Code, rr.Body.String())
	}
	app.Close()
	for _, edit := range []func(*config.Config){func(c *config.Config) { c.DatabaseURL = "invalid" }, func(c *config.Config) { c.RedisURL = "invalid" }, func(c *config.Config) { c.RPCURL = ":" }, func(c *config.Config) { c.IndexerLogSource = "unavailable" }, func(c *config.Config) { c.WebhookURLs = []string{"https://localhost"} }} {
		changed := cfg
		edit(&changed)
		if app, err := New(context.Background(), changed, logger); err == nil {
			app.Close()
			t.Fatal("invalid application configuration booted")
		}
	}
}
func TestLegacyServiceRunServesAndShutsDown(t *testing.T) {
	cfg := legacyLifecycleConfig(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{"NODE_ENV": "test", "DATABASE_URL": cfg.DatabaseURL, "REDIS_URL": cfg.RedisURL, "REDIS_BULL_URL": cfg.RedisURL, "RPC_URL": cfg.RPCURL, "RPC_URL_FALLBACK": "", "PAXSCAN_DATABASE_URL": "", "INDEXER_LOG_SOURCE": "rpc", "INDEXER_HEAD_SOURCE": "rpc", "INDEXER_MODE": "live", "INDEXER_START_BLOCK": "0", "INDEXER_POLL_INTERVAL_MS": "10", "BIND_ADDRESS": "127.0.0.1", "PORT": fmt.Sprint(port), "WEBHOOK_URLS": "", "WEBHOOK_HMAC_SECRET": "", "WEBHOOK_HMAC_SECRETS": "", "LOG_LEVEL": "error"} {
		t.Setenv(key, value)
	}
	for role, key := range map[string]string{"EventEmitter": "EVENT_EMITTER_ADDRESS", "PoolRegistry": "POOL_REGISTRY_ADDRESS", "Router": "ROUTER_ADDRESS", "SidioraFactory": "FACTORY_ADDRESS", "Quoter": "QUOTER_ADDRESS", "ProtocolConfig": "PROTOCOL_CONFIG_ADDRESS", "FeeAccumulator": "FEE_ACCUMULATOR_ADDRESS", "SidioraNFT": "SIDIORA_NFT_ADDRESS", "FeesRouter": "FEES_ROUTER_ADDRESS", "PoolBeacon": "POOL_BEACON_ADDRESS"} {
		t.Setenv(key, deployment.Current().Roles[role].Address)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- Run(ctx) }()
	client := &http.Client{Timeout: time.Second}
	url := fmt.Sprintf("http://127.0.0.1:%d/health/live", port)
	deadline := time.After(10 * time.Second)
	for {
		response, err := client.Get(url)
		if err == nil {
			if err = response.Body.Close(); err != nil {
				t.Fatal(err)
			}
			if response.StatusCode != 200 {
				t.Fatalf("liveness %d", response.StatusCode)
			}
			break
		}
		select {
		case err := <-done:
			t.Fatalf("service stopped before ready %v", err)
		case <-deadline:
			t.Fatal("service did not listen")
		case <-time.After(10 * time.Millisecond):
		}
	}
	cancel()
	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("service did not stop")
	}
	ch := make(chan error, 1)
	if err := firstNonShutdownErr(ch); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("listen rejected")
	ch <- sentinel
	if err := firstNonShutdownErr(ch); !errors.Is(err, sentinel) {
		t.Fatal(err)
	}
	if err := shutdownServer(context.Background(), &http.Server{}); err != nil {
		t.Fatal(err)
	}
}

func lifecycleNode(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	command := exec.CommandContext(ctx, "anvil", "--host", "127.0.0.1", "--port", fmt.Sprint(port), "--chain-id", "125", "--threads", "2", "--silent")
	if err = command.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); _ = command.Wait() })
	endpoint := fmt.Sprintf("http://127.0.0.1:%d", port)
	deadline := time.After(10 * time.Second)
	for {
		client, err := ethclient.DialContext(ctx, endpoint)
		if err == nil {
			chain, readErr := client.ChainID(ctx)
			client.Close()
			if readErr == nil && chain.Uint64() == 125 {
				return endpoint
			}
		}
		select {
		case <-deadline:
			t.Fatal("chain node did not start")
		case <-time.After(10 * time.Millisecond):
		}
	}
}
