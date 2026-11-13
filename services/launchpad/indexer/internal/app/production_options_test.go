package app

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v5"
)

func exerciseProductionOptions(t *testing.T, ctx context.Context, cfg config.Config, logger *slog.Logger, admin *pgx.Conn) {
	t.Helper()
	var r receiptRuntimeDocument
	var d deliveryRuntimeDocument
	var l lifecycleRuntimeDocument
	for _, item := range []struct {
		path, hash string
		out        any
	}{{cfg.ReceiptConfigFile, cfg.ReceiptConfigSHA256, &r}, {cfg.DeliveryConfigFile, cfg.DeliveryConfigSHA256, &d}, {cfg.LifecycleConfigFile, cfg.LifecycleConfigSHA256, &l}} {
		if err := config.ReadBoundDocument(item.path, item.hash, item.out); err != nil {
			t.Fatal(err)
		}
	}
	binary, err := runtimeBinaryDigest()
	if err != nil {
		t.Fatal(err)
	}
	archivePolicy := validLifecycleArchive()
	archivePolicy.Prefix = "canonical"
	archivePolicy.DecoderDigest = binary
	archivePolicy.CodeDigest = binary
	archivePolicy.ABIDigest = delivery.Hash(r.Production.CatalogJSON)
	for _, tc := range []struct {
		name string
		edit func(*config.Config, *receiptRuntimeDocument, *deliveryRuntimeDocument, *lifecycleRuntimeDocument)
		want string
	}{

		{"refresh bound", func(c *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
			c.SecretsRefreshSeconds = 0
		}, "secret_refresh_bound_invalid"},
		{"metrics environment", func(c *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
			c.RuntimeMetricsEnvironment = ""
		}, "telemetry_identity_outside_inventory"},
		{"pool budget", func(c *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
			c.RuntimePoolID = "unissued"
		}, "unregistered runtime pool"},
		{"redis URL", func(_ *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
			t.Setenv("REDIS_URL", ":")
		}, "redis"},
		{"RPC transport", func(_ *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
			t.Setenv("RPC_URL", ":")
		}, "requires http or https"},
		{"receipt document", func(_ *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
		}, "runtime_document_binding_mismatch"},
		{"delivery document", func(_ *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
		}, "runtime_document_binding_mismatch"},
		{"lifecycle document", func(_ *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
		}, "runtime_document_binding_mismatch"},
		{"historical endpoint", func(c *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
			c.HistoricalRPCURL = cfg.RPCURL
			t.Setenv("INDEXER_HISTORICAL_RPC_URL", cfg.RPCURL)
		}, ""},
		{"reserved endpoint", func(_ *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument) {
			l.Committed.Endpoints[historicalEndpointID] = cfg.RPCURL
		}, "reserved"},
		{"broker URL", func(_ *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
			t.Setenv("NATS_URL", ":")
		}, ""},
		{"broker credentials", func(c *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
			c.NATSCredentialsCreds = "invalid"
			t.Setenv("NATS_CREDENTIALS_CREDS", "invalid")
		}, ""},
		{"missing backfill request", func(c *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
			c.IndexerMode = config.ModeBackfill
		}, "backfill"},
		{"batch", func(c *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
			c.IndexerBatchSize = 65
		}, "batch size"},
		{"delivery owner", func(_ *config.Config, _ *receiptRuntimeDocument, d *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
			d.Owner = ""
		}, "incomplete durable"},
		{"archive concurrency", func(c *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
			c.HistoricalConcurrency = 97
		}, "CONCURRENCY"},
		{"realm", func(_ *config.Config, r *receiptRuntimeDocument, _ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
			r.Realm = "unreviewed"
		}, "explicit receipt realm"},
		{"private profile", func(c *config.Config, r *receiptRuntimeDocument, _ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
			r.Realm = valuePrivateFixture
			c.RuntimeProfile = "container"
			c.Port = 8080
			c.BindAddress = "0.0.0.0"
		}, "owned local"},
		{"private evidence", func(_ *config.Config, r *receiptRuntimeDocument, _ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
			r.Realm = valuePrivateFixture
		}, "fixture"},
		{"receipt identity", func(_ *config.Config, r *receiptRuntimeDocument, _ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
			r.Production.Anchor.Hash = common.HexToHash("0x1")
		}, "qualification"},
		{"epoch", func(_ *config.Config, _ *receiptRuntimeDocument, d *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
			d.Epoch = 0
		}, "invalid recovery authority"},
		{"delivery policy", func(_ *config.Config, _ *receiptRuntimeDocument, d *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
			d.Policy.Subjects = nil
		}, "delivery"},
		{"lifecycle version", func(_ *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument) {
			l.Version = 1
		}, "lifecycle"},
		{"endpoint secret", func(_ *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument) {
			l.EndpointSecretReferences = map[string]string{"paxeer-primary": "UNISSUED_COMMITTED_ENDPOINT"}
		}, "endpoint secret"},
		{"genesis", func(_ *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument) {
			l.Committed.GenesisHash = common.HexToHash("0x1")
		}, "genesis"},
		{"anchor", func(_ *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument) {
			l.Committed.Anchor.Hash = common.HexToHash("0x1")
		}, "anchor"},
		{"selection trust", func(_ *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument) {
			l.SourceSelectionKeys = map[string][]byte{"request": make([]byte, ed25519.PublicKeySize), "approve": make([]byte, ed25519.PublicKeySize)}
		}, "authorization"},
		{"checkpoint", func(_ *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument) {
			l.InitialCheckpointHash = common.HexToHash("0x1").Hex()
		}, "checkpoint"},
		{"webhook", func(_ *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument) {
			l.Webhooks = lifecycleWebhookDocument{Targets: []delivery.WebhookTarget{{}}, Generation: "g1", PollMS: 100}
		}, "webhook"},
		{"fixture barrier", func(c *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, _ *lifecycleRuntimeDocument) {
			c.FixturePubAckBarrier = "publication"
		}, "barrier forbidden"},
		{"archive provenance", func(_ *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument) {
			l.Archive = archivePolicy
			l.Archive.CodeDigest = strings.Repeat("0", 64)
		}, "provenance pins"},
		{"archive access", func(_ *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument) {
			l.Archive = archivePolicy
		}, "access key"},
		{"work budget", func(_ *config.Config, _ *receiptRuntimeDocument, _ *deliveryRuntimeDocument, l *lifecycleRuntimeDocument) {
			l.WorkConcurrency = 0
		}, "budget"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := cfg
			for _, key := range []string{"REDIS_URL", "RPC_URL", "NATS_URL", "NATS_CREDENTIALS_CREDS", "INDEXER_HISTORICAL_RPC_URL"} {
				t.Setenv(key, os.Getenv(key))
			}
			duplicate := func(in, out any) {
				raw, e := json.Marshal(in)
				if e != nil {
					t.Fatal(e)
				}
				if e = json.Unmarshal(raw, out); e != nil {
					t.Fatal(e)
				}
			}
			var rr receiptRuntimeDocument
			duplicate(r, &rr)
			var dd deliveryRuntimeDocument
			duplicate(d, &dd)
			var ll lifecycleRuntimeDocument
			duplicate(l, &ll)
			tc.edit(&c, &rr, &dd, &ll)
			c.ReceiptConfigFile, c.ReceiptConfigSHA256 = status1695Bound(t, rr)
			c.DeliveryConfigFile, c.DeliveryConfigSHA256 = status1695Bound(t, dd)
			c.LifecycleConfigFile, c.LifecycleConfigSHA256 = status1695Bound(t, ll)

			switch tc.name {
			case "receipt document":
				c.ReceiptConfigSHA256 = strings.Repeat("0", 64)
			case "delivery document":
				c.DeliveryConfigSHA256 = strings.Repeat("0", 64)
			case "lifecycle document":
				c.LifecycleConfigSHA256 = strings.Repeat("0", 64)
			}
			a, e := New(ctx, c, logger)
			if tc.name == "historical endpoint" {
				if e != nil {
					t.Fatal(e)
				}
				defer a.Close()
				if a.lifecycle.Committed.HistoricalEndpoints[historicalEndpointID] != cfg.RPCURL {
					t.Fatal("historical endpoint absent")
				}
				return
			}
			if e == nil {
				a.Close()
				t.Fatal("unbound production option accepted")
			}
			if !strings.Contains(strings.ToLower(e.Error()), strings.ToLower(tc.want)) {
				t.Fatalf("expected %q: %v", tc.want, e)
			}
		})
	}
	t.Run("endpoint trust and secret readiness", func(t *testing.T) {
		c := cfg
		var ll lifecycleRuntimeDocument
		body, e := json.Marshal(l)
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(body, &ll); e != nil {
			t.Fatal(e)
		}
		var endpoint string
		for id := range ll.Committed.Endpoints {
			endpoint = id
			break
		}
		if endpoint == "" {
			t.Fatal("committed endpoint absent")
		}
		t.Setenv("INDEXER_COMMITTED_ENDPOINT", cfg.RPCURL)
		ll.EndpointSecretReferences = map[string]string{endpoint: "INDEXER_COMMITTED_ENDPOINT"}
		first := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{51}, ed25519.SeedSize))
		second := ed25519.NewKeyFromSeed(bytes.Repeat([]byte{52}, ed25519.SeedSize))
		ll.SourceSelectionKeys = map[string][]byte{"requester": first.Public().(ed25519.PublicKey), "approver": second.Public().(ed25519.PublicKey)}
		c.LifecycleConfigFile, c.LifecycleConfigSHA256 = status1695Bound(t, ll)
		a, e := New(ctx, c, logger)
		if e != nil {
			t.Fatal(e)
		}
		defer a.Close()
		rr := httptest.NewRecorder()
		a.Router.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/internal/source-selection", strings.NewReader(`{}`)))
		if rr.Code != http.StatusForbidden {
			t.Fatalf("selection admission %d %s", rr.Code, rr.Body.String())
		}
		for _, value := range []string{"", "http://127.0.0.1:1"} {
			t.Setenv("INDEXER_COMMITTED_ENDPOINT", value)
			rr := httptest.NewRecorder()
			a.Router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/health/ready", nil))
			var state struct {
				Checks map[string]string `json:"checks"`
				Status string            `json:"status"`
			}
			if e := json.Unmarshal(rr.Body.Bytes(), &state); e != nil {
				t.Fatal(e)
			}
			if rr.Code != http.StatusServiceUnavailable || state.Status != "degraded" || state.Checks["lifecycle_secrets"] != "failed" {
				t.Fatalf("secret readiness %d %s", rr.Code, rr.Body.String())
			}
		}
	})
	for _, mode := range []string{"live", "backfill only", "backfill live", "backfill admission", "occupied listener"} {
		t.Run("production service "+mode, func(t *testing.T) {
			c := cfg
			var request store.ReplayRequest
			if strings.HasPrefix(mode, "backfill") {
				var hash string
				reference, e := New(ctx, cfg, logger)
				if e != nil {
					t.Fatal(e)
				}
				checkpoint, e := reference.history.RPC.BlockAt(ctx, 33963717)
				reference.Close()
				if e != nil {
					t.Fatal(e)
				}
				hash = strings.ToLower(checkpoint.Hash.Hex())
				request = store.ReplayRequest{JobID: strings.ReplaceAll(mode, " ", "-"), SourceStream: d.CanonicalStreamID, Target: "charts", Generation: "service-runtime", ProjectionRevision: "revision2", Mode: "backfill", Requester: "operator", AuthorizationID: strings.ReplaceAll(mode, " ", "-"), FromHeight: 33963717, ToHeight: 33963717, CheckpointHeight: 33963717, CheckpointHash: hash}
				ll := l
				ll.BackfillRequest = &request
				c.IndexerMode = config.ModeBackfill
				c.BackfillOnly = mode == "backfill only"
				c.LifecycleConfigFile, c.LifecycleConfigSHA256 = status1695Bound(t, ll)
				if mode != "backfill admission" {
					if _, e := admin.Exec(ctx, `INSERT INTO indexer.indexer_replay_authorizations VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,1,clock_timestamp()+interval '1 hour',NULL)`, request.AuthorizationID, request.Requester, strings.Repeat("a", 64), request.Mode, request.Target, request.Generation, request.ProjectionRevision, request.FromHeight, request.ToHeight); e != nil {
						t.Fatal(e)
					}
				}
			}
			listener, e := net.Listen("tcp", "127.0.0.1:0")
			if e != nil {
				t.Fatal(e)
			}
			c.Port = listener.Addr().(*net.TCPAddr).Port
			if mode == "occupied listener" {
				defer listener.Close()
			} else if e = listener.Close(); e != nil {
				t.Fatal(e)
			}
			var bind func(reflect.Value)
			bind = func(v reflect.Value) {
				typ := v.Type()
				for i := 0; i < v.NumField(); i++ {
					field := v.Field(i)
					definition := typ.Field(i)
					if definition.Anonymous && field.Kind() == reflect.Struct {
						bind(field)
						continue
					}
					key := definition.Tag.Get("env")
					if key == "" || field.IsZero() {
						continue
					}
					var value string
					switch field.Kind() {
					case reflect.String:
						value = field.String()
					case reflect.Int, reflect.Int64:
						value = strconv.FormatInt(field.Int(), 10)
					case reflect.Bool:
						value = strconv.FormatBool(field.Bool())
					default:
						continue
					}
					t.Setenv(key, value)
				}
			}
			bind(reflect.ValueOf(c))
			for key, value := range map[string]string{"INDEXER_LOG_SOURCE": "rpc", "INDEXER_HEAD_SOURCE": "rpc", "REDIS_BULL_URL": c.RedisURL, "WEBHOOK_URLS": "", "WEBHOOK_HMAC_SECRET": "", "WEBHOOK_HMAC_SECRETS": "", "LOG_LEVEL": "error", "INDEXER_LIFECYCLE_CONFIG_DOCUMENT": "", "INDEXER_RECEIPT_CONFIG_DOCUMENT": "", "DELIVERY_CONFIG_DOCUMENT": "", "DB_BUDGET_DOCUMENT": "", "EXPECTED_SCHEMA_DOCUMENT": ""} {
				t.Setenv(key, value)
			}
			for role, key := range map[string]string{"EventEmitter": "EVENT_EMITTER_ADDRESS", "PoolRegistry": "POOL_REGISTRY_ADDRESS", "Router": "ROUTER_ADDRESS", "SidioraFactory": "FACTORY_ADDRESS", "Quoter": "QUOTER_ADDRESS", "ProtocolConfig": "PROTOCOL_CONFIG_ADDRESS", "FeeAccumulator": "FEE_ACCUMULATOR_ADDRESS", "SidioraNFT": "SIDIORA_NFT_ADDRESS", "FeesRouter": "FEES_ROUTER_ADDRESS", "PoolBeacon": "POOL_BEACON_ADDRESS"} {
				t.Setenv(key, r.Production.Manifest.Roles[role].Address)
			}
			runCtx, stop := context.WithCancel(ctx)
			defer stop()
			done := make(chan error, 1)
			go func() { done <- Run(runCtx) }()
			if mode == "backfill only" || mode == "backfill admission" || mode == "occupied listener" {
				select {
				case e := <-done:
					switch mode {
					case "backfill only":
						if e != nil {
							t.Fatal(e)
						}
						var status string
						if e = admin.QueryRow(ctx, `SELECT status FROM indexer.indexer_replay_jobs WHERE job_id=$1`, request.JobID).Scan(&status); e != nil || status != "completed" {
							t.Fatalf("backfill completion %s %v", status, e)
						}
					case "backfill admission":
						if e == nil || !strings.Contains(e.Error(), "authorization") {
							t.Fatalf("unissued backfill: %v", e)
						}
					case "occupied listener":
						if e == nil || !strings.Contains(e.Error(), "address already in use") {
							t.Fatalf("occupied listener: %v", e)
						}
					}
				case <-time.After(15 * time.Second):
					t.Fatal("service did not finish bounded startup")
				}
				return
			}
			client := &http.Client{Timeout: time.Second}
			deadline := time.After(10 * time.Second)
			for {
				response, e := client.Get(fmt.Sprintf("http://127.0.0.1:%d/health/live", c.Port))
				if e == nil {
					if e = response.Body.Close(); e != nil {
						t.Fatal(e)
					}
					if response.StatusCode != http.StatusOK {
						t.Fatalf("production liveness %d", response.StatusCode)
					}
					break
				}
				select {
				case e := <-done:
					t.Fatalf("production service stopped before listening %v", e)
				case <-deadline:
					t.Fatal("production service did not listen")
				case <-time.After(10 * time.Millisecond):
				}
			}
			if mode == "backfill live" {
				for {
					var state string
					err := admin.QueryRow(ctx, `SELECT status FROM indexer.indexer_replay_jobs WHERE job_id=$1`, request.JobID).Scan(&state)
					if err == nil && state == "completed" {
						break
					}
					select {
					case e := <-done:
						t.Fatalf("backfill stopped before live transition: %v", e)
					case <-deadline:
						t.Fatalf("backfill did not complete before live transition: state=%s err=%v", state, err)
					case <-time.After(10 * time.Millisecond):
					}
				}
			}
			// Let the persistent relay observe the actual empty delivery queue.
			timer := time.NewTimer(250 * time.Millisecond)
			select {
			case e := <-done:
				timer.Stop()
				t.Fatalf("production service stopped while serving %v", e)
			case <-timer.C:
			}
			stop()
			select {
			case e := <-done:
				if e != nil && !errors.Is(e, context.Canceled) {
					t.Fatal(e)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("production service did not drain")
			}
		})

	}
	for _, mode := range []string{"backfill", "replay", "webhook", "rebuild"} {
		t.Run("configured "+mode, func(t *testing.T) {
			c := cfg
			var ll lifecycleRuntimeDocument
			body, e := json.Marshal(l)
			if e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(body, &ll); e != nil {
				t.Fatal(e)
			}
			request := lifecycleRequest()
			request.SourceStream = d.CanonicalStreamID
			request.JobID = "configured-" + mode
			switch mode {
			case "backfill":
				c.IndexerMode = config.ModeBackfill
				request.Mode = "backfill"
				ll.BackfillRequest = &request
			case "replay":
				ll.ReplayRequests = []store.ReplayRequest{request}
			case "webhook":
				ll.Webhooks = lifecycleWebhookDocument{Targets: []delivery.WebhookTarget{{ID: "consumer", URL: "https://localhost/events", SecretReference: "WEBHOOK_SIGNING", Subjects: map[string]bool{"indexer.replay.v2": true}}}, Generation: "g1", PollMS: 100, LeaseMS: 30000, TimeoutMS: 1000, RetryMS: 1000, MaxAttempts: 3}
			case "rebuild":
				objects := httptest.NewServer(http.HandlerFunc(http.NotFound))
				defer objects.Close()
				t.Setenv("ARCHIVE_ACCESS", "local-access")
				t.Setenv("ARCHIVE_SECRET", "local-secret")
				ll.Archive = archivePolicy
				ll.Archive.Endpoint = objects.URL
				ll.Archive.FromHeight = ll.InitialCheckpointHeight + 1
				request.Mode = "reconstruct"
				ll.Rebuilds = []lifecycleRebuildDocument{{Request: request, ConsumerID: "charts", ManifestDigest: strings.Repeat("a", 64)}}
			}
			c.LifecycleConfigFile, c.LifecycleConfigSHA256 = status1695Bound(t, ll)
			a, e := New(ctx, c, logger)
			if e != nil {
				t.Fatal(e)
			}
			defer a.Close()
			if mode == "backfill" {
				if e = a.Backfill.Run(ctx); e == nil {
					t.Fatal("unissued historical request executed")
				}
				return
			}
			if e = a.workers.Start(ctx); e != nil {
				t.Fatal(e)
			}
			deadline := time.After(5 * time.Second)
			worker := map[string]string{"replay": "replay_0", "rebuild": "rebuild_0", "webhook": "webhooks"}[mode]
			for {
				a.workers.mu.Lock()
				state := a.workers.states[worker]
				a.workers.mu.Unlock()
				if state.started {
					if mode == "webhook" && state.failure != nil {
						t.Fatalf("empty webhook queue %v", state.failure)
					}
					if mode != "webhook" && state.failure == nil {
						t.Fatal("unissued reconstruction or replay executed")
					}
					break
				}
				select {
				case <-deadline:
					t.Fatal("configured worker did not run")
				case <-time.After(5 * time.Millisecond):
				}
			}
		})
	}

	t.Run("archive resources", func(t *testing.T) {
		t.Setenv("ARCHIVE_ACCESS", "local-access")
		t.Setenv("ARCHIVE_SECRET", "local-secret")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "1")
			w.Header().Set("ETag", `"ready"`)
			if r.Method != http.MethodHead {
				_, _ = io.WriteString(w, "x")
			}
		}))
		defer server.Close()
		policy := archivePolicy
		policy.Endpoint = server.URL
		l.Archive = policy
		c := cfg
		c.LifecycleConfigFile, c.LifecycleConfigSHA256 = status1695Bound(t, l)
		a, e := New(ctx, c, logger)
		if e != nil {
			t.Fatal(e)
		}
		defer a.Close()
		providers := a.SourceCapacityProviders()
		if providers.Live != a.history.RPC || providers.Processor != a.Processor || providers.Pool != a.pool || a.archive == nil || a.rawArchive == nil || a.replay.Archive != a.archive {
			t.Fatal("archive providers absent")
		}
		for _, path := range []string{"/health/ready", "/ready"} {
			rr := httptest.NewRecorder()
			a.Router.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, path, nil))
			if rr.Code == http.StatusInternalServerError {
				t.Fatalf("readiness failed: %s", rr.Body.String())
			}
		}
		if e = authorizeRuntimeOperation(ctx, a.pool, store.OperationScope{}); e == nil {
			t.Fatal("unissued operation authorized")
		}
		if e = authorizeRuntimeOperation(ctx, a.pool, store.OperationScope{AuthorizationID: "unissued", Operator: "operator", From: 1, To: 2}); e == nil {
			t.Fatal("missing operation authorized")
		}
	})
}
