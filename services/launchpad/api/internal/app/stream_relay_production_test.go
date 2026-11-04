package app

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// streamRelayNATSEnv names the already running JetStream server the relay test
// publishes to (the local fixture: nats://user:password@127.0.0.1:24222).
const streamRelayNATSEnv = "CORE_API_TEST_NATS_URL"

type relayFrame struct {
	topic, eventID string
	sequence       int64
	wire           []byte
}

// committedFrames builds one committed wire frame per topic family from the
// frozen v2 fixtures (two for market.rankings.trending), each admitted by
// delivery.ParseCommittedStreamFrame exactly as the relay checks it.
func committedFrames(t *testing.T) []relayFrame {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "broker", "frozen_stream_fixtures_test.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fixtures map[string]map[string]any
	if err = json.Unmarshal(raw, &fixtures); err != nil {
		t.Fatal(err)
	}
	out := []relayFrame{}
	for _, family := range []string{"MarketTradesEvent", "CandleEvent", "StatsEvent", "MarketPriceEvent", "RankingEvent", "PortfolioEvent", "RankingEvent"} {
		f := map[string]any{}
		for k, v := range fixtures[family] {
			f[k] = v
		}
		topic := f["topic"].(string)
		sequence := int64(1)
		for _, prior := range out {
			if prior.topic == topic {
				sequence = prior.sequence + 1
			}
		}
		id := uuid.NewString()
		f["eventId"], f["sequence"] = id, fmt.Sprint(sequence)
		if family == "PortfolioEvent" {
			// The committed private.portfolio.<wallet> topic names the summary wallet.
			payload := f["payload"].(map[string]any)
			if summary, ok := payload["summary"].(map[string]any); ok {
				topic = "private.portfolio." + summary["wallet"].(string)
				f["topic"] = topic
			}
			f["revision"] = payload["summaryRevision"]
		}
		wire, err := json.Marshal(f)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = delivery.ParseCommittedStreamFrame(wire); err != nil {
			t.Fatalf("%s fixture is not a committed frame: %v", family, err)
		}
		out = append(out, relayFrame{topic: topic, eventID: id, sequence: sequence, wire: wire})
	}
	return out
}

// TestProductionStreamRelayPublishesCommittedFrames runs the API_ROLE=rest
// api_stream relay (bound document -> newStreamRelay -> streamRelay.Run) as the
// least-privilege api_rest_runtime role against a database built from the
// registered api_stream/recovery migrations and the reviewed grants, publishing
// to the fixture JetStream; the API_ROLE=sockets committed consumer
// (broker.PostgresStream.ConsumeCommitted over a natsdriver durable) then
// accepts the relayed frames byte-for-byte against api_stream.
func TestProductionStreamRelayPublishesCommittedFrames(t *testing.T) {
	natsURL := os.Getenv(streamRelayNATSEnv)
	if natsURL == "" {
		t.Fatalf("%s is not set: point it at a running JetStream server; tests never start containers", streamRelayNATSEnv)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db := internaltest.NewAPIRuntimeDatabase(t, "api_rest_runtime", "api_stream", "recovery")
	admin, err := pgxpool.New(ctx, db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close()
	runtimePool, err := pgxpool.New(ctx, db.RuntimeDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer runtimePool.Close()

	nc, err := nats.Connect(natsURL, nats.Timeout(3*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	// Registered before the stream cleanups so it closes after them.
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	suffix := strings.ToUpper(strings.ReplaceAll(uuid.NewString()[:8], "-", ""))
	live, rankings := "LIVE_STREAM_T"+suffix, "RANKINGS_T"+suffix
	routes := []delivery.StreamRoute{
		{Stream: live, Subjects: []string{"market.trades.*", "market.candles.*.*", "market.stats.*", "market.price.*", "private.portfolio.*"}},
		{Stream: rankings, Subjects: []string{"market.rankings.*"}},
	}
	// The relay publishes to the fixed topic subjects, so only one run can hold
	// them; streams this test leaked from an interrupted run (older than any
	// run's 2 minute budget) are removed first.
	leaked := js.ListStreams(ctx)
	for info := range leaked.Info() {
		name := info.Config.Name
		if (strings.HasPrefix(name, "LIVE_STREAM_T") || strings.HasPrefix(name, "RANKINGS_T")) && time.Since(info.Created) > 10*time.Minute {
			_ = js.DeleteStream(ctx, name)
		}
	}
	for _, route := range routes {
		if _, err = js.CreateStream(ctx, jetstream.StreamConfig{Name: route.Stream, Subjects: route.Subjects, Storage: jetstream.FileStorage, Retention: jetstream.LimitsPolicy, Discard: jetstream.DiscardNew, MaxAge: 72 * time.Hour, MaxBytes: 32 << 20, Duplicates: 2 * time.Minute, Replicas: 1}); err != nil {
			t.Fatalf("create %s: %v", route.Stream, err)
		}
		name := route.Stream
		t.Cleanup(func() { _ = js.DeleteStream(context.Background(), name) })
	}

	// Startup broker verification fails closed on the single-node fixture:
	// production requires the R3 file streams RunCommitted requires.
	if err = verifyRelayStreams(ctx, nc, routes); err == nil || !strings.Contains(err.Error(), "R3") {
		t.Fatalf("non-R3 stream admitted at startup: %v", err)
	}
	if err = verifyRelayStreams(ctx, nc, []delivery.StreamRoute{{Stream: "ABSENT_T" + suffix, Subjects: []string{"market.price.*"}}}); err == nil {
		t.Fatal("absent stream admitted at startup")
	}

	// The bound STREAM_RELAY_CONFIG document, read through config.Config.
	document := streamRelayDocument{Routes: routes, Epoch: db.Epoch, Region: db.Region, Batch: 50, MaxAttempts: 5, LeaseDuration: 10 * time.Second, PublishTimeout: 2 * time.Second, RetryDelay: time.Second, IdleInterval: 100 * time.Millisecond, MaxLag: time.Minute, Concurrency: 1, EventsPerSecond: 500, Burst: 50}
	rawDocument, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "stream-relay.json")
	if err = os.WriteFile(path, rawDocument, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{StreamRelayFile: path, StreamRelaySHA256: fmt.Sprintf("%x", sha256.Sum256(rawDocument)), RuntimeInstanceID: "api-1"}
	loaded, err := readStreamRelayDocument(cfg)
	if err != nil || loaded == nil {
		t.Fatalf("bound relay document: %v", err)
	}
	if _, err = readStreamRelayDocument(&config.Config{StreamRelayFile: path, StreamRelaySHA256: strings.Repeat("0", 64)}); err == nil {
		t.Fatal("relay document admitted under a different digest")
	}
	if _, err = readStreamRelayDocument(&config.Config{StreamRelayFile: path}); err == nil {
		t.Fatal("partially configured relay admitted")
	}
	unrouted := *loaded
	unrouted.Routes = []delivery.StreamRoute{{Stream: rankings, Subjects: []string{"market.rankings.*"}}}
	if _, err = newStreamRelay(ctx, runtimePool, nc, cfg.RuntimeInstanceID, unrouted); err == nil {
		t.Fatal("relay admitted routes that leave committed families unpublished")
	}

	// The reviewed role already owns the fence grant. Start without changing it.
	var fenceGranted bool
	if err = runtimePool.QueryRow(ctx, "SELECT has_function_privilege(current_user, 'recovery.assert_epoch(bigint,text)', 'EXECUTE')").Scan(&fenceGranted); err != nil || !fenceGranted {
		t.Fatalf("reviewed runtime fence privilege: %t %v", fenceGranted, err)
	}
	relay, err := newStreamRelay(ctx, runtimePool, nc, cfg.RuntimeInstanceID, *loaded)
	if err != nil {
		t.Fatalf("relay startup with unchanged reviewed grants: %v", err)
	}

	// An independent role receives the same planned grants except the fence.
	// It has no membership in the reviewed role and cannot inherit its EXECUTE.
	restrictedRole := "relay_no_fence_" + strings.ToLower(suffix)
	for _, statement := range relayRestrictedRoleStatements(t, restrictedRole) {
		if _, err = admin.Exec(ctx, statement); err != nil {
			t.Fatal(err)
		}
	}
	defer func() {
		if _, err := admin.Exec(context.Background(), "DROP OWNED BY "+pgx.Identifier{restrictedRole}.Sanitize()); err != nil {
			t.Error(err)
		}
		if _, err := admin.Exec(context.Background(), "DROP ROLE "+pgx.Identifier{restrictedRole}.Sanitize()); err != nil {
			t.Error(err)
		}
	}()
	restrictedConfig, err := pgxpool.ParseConfig(db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	restrictedConfig.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE "+pgx.Identifier{restrictedRole}.Sanitize())
		return err
	}
	restrictedPool, err := pgxpool.NewWithConfig(ctx, restrictedConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer restrictedPool.Close()
	var permissionError *pgconn.PgError
	if _, err = newStreamRelay(ctx, restrictedPool, nc, cfg.RuntimeInstanceID, *loaded); err == nil || !strings.Contains(err.Error(), "restoration epoch fence rejected") || !errors.As(err, &permissionError) || permissionError.Code != "42501" {
		t.Fatalf("relay started without the restoration fence grant: %v", err)
	}
	stale := *loaded
	stale.Epoch = db.Epoch + 1
	if _, err = newStreamRelay(ctx, runtimePool, nc, cfg.RuntimeInstanceID, stale); err == nil {
		t.Fatal("relay started under a stale restoration epoch")
	}

	// Committed frames, as the api_stream.append_* functions leave them.
	frames := committedFrames(t)
	for _, f := range frames {
		var partition int64
		if err = admin.QueryRow(ctx, `INSERT INTO api_stream.stream_projection(topic,sequence,frame) VALUES($1,$2,$3) ON CONFLICT(topic) DO UPDATE SET sequence=EXCLUDED.sequence,frame=EXCLUDED.frame RETURNING partition_id`, f.topic, f.sequence, f.wire).Scan(&partition); err != nil {
			t.Fatal(err)
		}
		if _, err = admin.Exec(ctx, `INSERT INTO api_stream.stream_delivery_outbox(partition_id,sequence,event_id,frame) VALUES($1,$2,$3,$4)`, partition, f.sequence, f.eventID, f.wire); err != nil {
			t.Fatal(err)
		}
	}
	if err = relay.Ready(ctx); err != nil {
		t.Fatalf("fresh backlog within MaxLag must stay ready: %v", err)
	}

	runCtx, stop := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() { relay.Run(runCtx, nil); close(done) }()
	defer func() { stop(); <-done }()
	deadline := time.Now().Add(30 * time.Second)
	for {
		stats, err := relay.relay.Stats(ctx)
		if err == nil && stats.PendingRows == 0 && stats.QuarantinedRows == 0 && stats.GapPartitions == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("relay did not drain the outbox: %+v %v", stats, err)
		}
		timer := time.NewTimer(50 * time.Millisecond)
		<-timer.C
	}
	if err = relay.Ready(ctx); err != nil {
		t.Fatalf("relay readiness after drain: %v", err)
	}

	// Reopen the production relay over the persisted publication ledger. A
	// completed batch must not publish duplicate messages after restart.
	stop()
	<-done
	recovered, err := newStreamRelay(ctx, runtimePool, nc, cfg.RuntimeInstanceID, *loaded)
	if err != nil {
		t.Fatalf("relay restart: %v", err)
	}
	if err = recovered.relay.RunOnce(ctx); err != nil {
		t.Fatalf("publication recovery: %v", err)
	}
	if err = recovered.Ready(ctx); err != nil {
		t.Fatalf("recovered relay readiness: %v", err)
	}

	// Every frame is on its topic subject in the routed stream, byte-identical,
	// deduplicated by Nats-Msg-Id = Hash(eventId).
	for _, route := range routes {
		stream, err := js.Stream(ctx, route.Stream)
		if err != nil {
			t.Fatal(err)
		}
		want := 0
		for _, f := range frames {
			for _, filter := range route.Subjects {
				if delivery.SubjectMatches(filter, f.topic) {
					want++
				}
			}
		}
		info, err := stream.Info(ctx)
		if err != nil || int(info.State.Msgs) != want {
			t.Fatalf("%s holds %v messages, want %d: %v", route.Stream, info.State.Msgs, want, err)
		}
	}
	for _, f := range frames {
		stream := live
		if strings.HasPrefix(f.topic, "market.rankings.") {
			stream = rankings
		}
		var brokerStream string
		var brokerSequence int64
		var state string
		if err = admin.QueryRow(ctx, `SELECT p.state,p.broker_stream,p.broker_sequence FROM api_stream.stream_delivery_publications p JOIN api_stream.stream_delivery_outbox o USING(partition_id,sequence) WHERE o.event_id=$1`, f.eventID).Scan(&state, &brokerStream, &brokerSequence); err != nil || state != "published" || brokerStream != stream {
			t.Fatalf("%s publication %s %s: %v", f.topic, state, brokerStream, err)
		}
		s, err := js.Stream(ctx, stream)
		if err != nil {
			t.Fatal(err)
		}
		message, err := s.GetMsg(ctx, uint64(brokerSequence))
		if err != nil {
			t.Fatal(err)
		}
		if message.Subject != f.topic || string(message.Data) != string(f.wire) || message.Header.Get(jetstream.MsgIDHeader) != delivery.Hash([]byte(f.eventID)) {
			t.Fatalf("relayed %s differs from the committed outbox frame", f.topic)
		}
	}

	// API_ROLE=sockets: the committed consumer binds the relayed subjects and
	// verifies each message against api_stream before waking topic watchers.
	stream, err := broker.NewPostgresStream(admin, "api_stream", []byte(strings.Repeat("k", 32)), sharedauth.HTTPStreamAuthority{})
	if err != nil {
		t.Fatal(err)
	}
	fanout := broker.NewTopicFanout()
	for _, f := range frames {
		if f.sequence != 1 {
			continue
		}
		consumerStream := live
		if strings.HasPrefix(f.topic, "market.rankings.") {
			consumerStream = rankings
		}
		durable := "api_t_" + strings.ReplaceAll(strings.SplitN(f.topic, ".", 3)[1], "-", "_")
		assignment := natsdriver.ConsumerConfig{Stream: consumerStream, Durable: durable, Subject: f.topic, Batch: 32, MaxAckPending: 128, MaxWaiting: 4, MaxDeliver: 10, FetchWait: 200 * time.Millisecond, AckWait: 30 * time.Second}
		brokerConfig, err := assignment.BrokerConsumerConfig()
		if err != nil {
			t.Fatal(err)
		}
		if _, err = js.CreateOrUpdateConsumer(ctx, consumerStream, brokerConfig); err != nil {
			t.Fatalf("consumer %s: %v", durable, err)
		}
		watch, unwatch, err := fanout.Watch([]string{f.topic})
		if err != nil {
			t.Fatal(err)
		}
		consumer, err := natsdriver.NewConsumer(ctx, nc, assignment)
		if err != nil {
			t.Fatalf("natsdriver consumer %s: %v", durable, err)
		}
		consumeCtx, stopConsume := context.WithCancel(ctx)
		consumed := make(chan error, 1)
		go func() { consumed <- stream.ConsumeCommitted(consumeCtx, consumer, fanout) }()
		select {
		case <-watch:
		case err = <-consumed:
			t.Fatalf("committed consumer for %s stopped: %v", f.topic, err)
		case <-time.After(10 * time.Second):
			t.Fatalf("committed consumer never accepted %s", f.topic)
		}
		var expected uint64 = 1
		if strings.HasPrefix(f.topic, "market.rankings.") {
			expected = 2
		}
		ackDeadline := time.Now().Add(10 * time.Second)
		for {
			c, err := js.Consumer(ctx, consumerStream, durable)
			if err != nil {
				t.Fatal(err)
			}
			info, err := c.Info(ctx)
			if err == nil && info.AckFloor.Consumer == expected && info.NumAckPending == 0 {
				break
			}
			if time.Now().After(ackDeadline) {
				t.Fatalf("%s: committed consumer acknowledged %+v, want %d: %v", f.topic, info, expected, err)
			}
			timer := time.NewTimer(50 * time.Millisecond)
			<-timer.C
		}
		stopConsume()
		<-consumed
		consumer.Close()
		unwatch()
	}
}

func TestStreamRelayDocumentBounds(t *testing.T) {
	good := streamRelayDocument{Routes: []delivery.StreamRoute{{Stream: "LIVE_STREAM", Subjects: []string{"market.>", "private.portfolio.*"}}}, Epoch: 1, Region: "railway-prod", Batch: 100, MaxAttempts: 10, LeaseDuration: 30 * time.Second, PublishTimeout: 5 * time.Second, RetryDelay: 2 * time.Second, IdleInterval: 250 * time.Millisecond, MaxLag: 2 * time.Minute, Concurrency: 1, EventsPerSecond: 500, Burst: 100}
	if err := good.validate(); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(*streamRelayDocument){
		"epoch":   func(d *streamRelayDocument) { d.Epoch = 0 },
		"region":  func(d *streamRelayDocument) { d.Region = "" },
		"publish": func(d *streamRelayDocument) { d.PublishTimeout = d.LeaseDuration },
		"idle":    func(d *streamRelayDocument) { d.IdleInterval = 0 },
		"lag":     func(d *streamRelayDocument) { d.MaxLag = 0 },
		"rate":    func(d *streamRelayDocument) { d.EventsPerSecond = 0 },
	} {
		d := good
		mutate(&d)
		if d.validate() == nil {
			t.Fatalf("%s out of bounds admitted", name)
		}
	}
	for _, c := range []struct {
		inner, outer string
		want         bool
	}{
		{"market.rankings.*", "market.rankings.*", true},
		{"market.rankings.*", "market.>", true},
		{"market.candles.*.*", "market.candles.>", true},
		{"market.rankings.trending", "market.rankings.*", true},
		{"market.rankings.*", "market.rankings.trending", false},
		{"market.>", "market.*", false},
		{"market.stats.*", "market.trades.>", false},
		{"market.candles.*.*", "market.candles.*", false},
	} {
		if filterWithin(c.inner, c.outer) != c.want {
			t.Fatalf("filterWithin(%s, %s) != %v", c.inner, c.outer, c.want)
		}
	}
}

// TestRuntimeIdentityInlineMaterial builds the production identity channel
// from the inline IDENTITY_CA_PEM, IDENTITY_CLIENT_CERT_PEM and
// IDENTITY_CLIENT_KEY_PEM values and completes an mTLS round trip with a
// listener that requires the client certificate.
func TestRuntimeIdentityInlineMaterial(t *testing.T) {
	pki := testpki.New(t)
	server := pki.Leaf("gateway-server", "gateway.railway.internal")
	client := pki.Leaf("api-client", "api.railway.internal")
	caPEM, err := os.ReadFile(pki.CAFile)
	if err != nil {
		t.Fatal(err)
	}
	presented := make(chan []byte, 1)
	listener := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		presented <- r.TLS.PeerCertificates[0].Raw
	}))
	listener.TLS = &tls.Config{MinVersion: tls.VersionTLS13, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.Roots(), Certificates: []tls.Certificate{server.Pair}}
	listener.StartTLS()
	defer listener.Close()
	cfg := &config.Config{IdentityURL: listener.URL, IdentityPeerCertSHA256: server.SHA256, IdentityCAPEM: string(caPEM), IdentityClientCertPEM: string(client.CertPEM), IdentityClientKeyPEM: string(client.KeyPEM), IdentitySigningKey: base64.RawStdEncoding.EncodeToString(testpki.Ed25519(t).Private), IdentityIssuer: "api", IdentityAudience: "identity"}
	authority, err := runtimeIdentity(cfg)
	if err != nil {
		t.Fatalf("inline identity material rejected: %v", err)
	}
	response, err := authority.Client.Get(listener.URL)
	if err != nil {
		t.Fatalf("inline mTLS round trip: %v", err)
	}
	response.Body.Close()
	if got := <-presented; string(got) != string(client.Pair.Certificate[0]) {
		t.Fatal("listener saw a different client certificate than IDENTITY_CLIENT_CERT_PEM")
	}
	cfg.IdentityCAFile = pki.CAFile
	if _, err = runtimeIdentity(cfg); err == nil || !strings.Contains(err.Error(), "IDENTITY_CA_FILE and IDENTITY_CA_PEM") {
		t.Fatalf("both identity CA sources accepted: %v", err)
	}
}

// loadInlineDocument loads the config with one inline bound document whose
// declared digest is wrong for its bytes.
func loadInlineDocument(t *testing.T, name string) config.Config {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://kl:kl@localhost:5432/kl")
	t.Setenv("REDIS_URL", "redis://localhost:6379/0")
	t.Setenv(name+"_DOCUMENT", base64.StdEncoding.EncodeToString([]byte(`{"schemaVersion":2}`)))
	t.Setenv(name+"_SHA256", fmt.Sprintf("%x", sha256.Sum256([]byte("another document"))))
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("inline %s_DOCUMENT rejected at load: %v", name, err)
	}
	return cfg
}

func TestNetworkAuthorityInlineDocumentGate(t *testing.T) {
	cfg := loadInlineDocument(t, "NETWORK_AUTHORITY_CONFIG")
	cfg.NetworkRPCURL = "https://rpc.example"
	if _, err := newNetworkAuthority(context.Background(), nil, &cfg); err == nil || err.Error() != "runtime_document_binding_mismatch" {
		t.Fatalf("inline network authority document with a wrong digest did not fail startup: %v", err)
	}
}

func TestDiscoveryRuntimeInlineDocumentGate(t *testing.T) {
	cfg := loadInlineDocument(t, "DISCOVERY_RUNTIME_CONFIG")
	admission, err := rest.NewAdmission(10, 1, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	a := &App{cfg: cfg}
	if err = a.registerDiscovery(context.Background(), chi.NewRouter(), nil, admission, map[string]sharedhttp.Check{}); err == nil || err.Error() != "runtime_document_binding_mismatch" {
		t.Fatalf("inline discovery runtime document with a wrong digest did not fail startup: %v", err)
	}
}

func TestStreamRelayInlineDocumentGate(t *testing.T) {
	cfg := loadInlineDocument(t, "STREAM_RELAY_CONFIG")
	if _, err := readStreamRelayDocument(&cfg); err == nil || err.Error() != "runtime_document_binding_mismatch" {
		t.Fatalf("inline stream relay document with a wrong digest did not fail startup: %v", err)
	}
	t.Setenv("STREAM_RELAY_CONFIG_SHA256", "")
	if _, err := config.Load(); err == nil || !strings.Contains(err.Error(), "STREAM_RELAY_CONFIG_DOCUMENT is set without STREAM_RELAY_CONFIG_SHA256") {
		t.Fatalf("inline stream relay document without a digest accepted: %v", err)
	}
}

// reviewedRelayRole is shared by the service-backed qualification and the
// hermetic repair gate, so neither maintains a second privilege inventory.
func reviewedRelayRole(t *testing.T) production.RoleManifest {
	t.Helper()
	path := filepath.Join(internaltest.RepositoryRoot(t), "launchpad/spec/production-system/integration/migrations/grants.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var document struct {
		Roles []production.RoleManifest `json:"roles"`
	}
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	for _, role := range document.Roles {
		if role.Service == "core/api" && role.RuntimeRole == "api_rest_runtime" {
			if err := role.Validate(); err != nil {
				t.Fatal(err)
			}
			return role
		}
	}
	t.Fatal("reviewed api_rest_runtime role missing")
	return production.RoleManifest{}
}

func relayRestrictedRoleStatements(t *testing.T, name string) []string {
	t.Helper()
	role := reviewedRelayRole(t)
	role.RuntimeRole = name
	role.OwnerReadGrants = nil
	role.SchemaCapabilities = nil
	grants := make([]production.Grant, 0, len(role.Grants))
	removed := 0
	for _, grant := range role.Grants {
		if grant.Schema != "api_stream" && grant.Schema != "recovery" {
			continue
		}
		if grant.Schema == "recovery" && grant.Object == "assert_epoch" {
			removed++
			continue
		}
		grants = append(grants, grant)
	}
	if removed != 1 {
		t.Fatalf("reviewed fence grant count = %d, want 1", removed)
	}
	role.Grants = grants
	provision, err := role.ProvisionSQL()
	if err != nil {
		t.Fatal(err)
	}
	statements, err := role.GrantSQL()
	if err != nil {
		t.Fatal(err)
	}
	// ProvisionSQL's runtime CREATE ROLE has all reviewed privilege attributes.
	// The owner and schema already exist; create only the independent runtime.
	return append([]string{provision[1]}, statements...)
}
