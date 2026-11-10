package app

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"errors"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/internaltest/chainfixture"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	shareddb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nkeys"
)

func status1695Bound(t *testing.T, value any) (string, string) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "document.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	return path, production.Checksum(raw)
}

func status1695Broker(t *testing.T) (string, string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	log, err := os.Create(filepath.Join(t.TempDir(), "broker.log"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("nats-server", "-js", "-a", "127.0.0.1", "-p", strconv.Itoa(port), "-sd", t.TempDir())
	cmd.Stdout = log
	cmd.Stderr = log
	if err = cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait(); _ = log.Close() })
	endpoint := fmt.Sprintf("nats://127.0.0.1:%d", port)
	deadline := time.Now().Add(5 * time.Second)
	for {
		c, e := nats.Connect(endpoint, nats.Timeout(100*time.Millisecond))
		if e == nil {
			c.Close()
			break
		}
		if time.Now().After(deadline) {
			t.Fatal(e)
		}
		time.Sleep(20 * time.Millisecond)
	}
	user, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	defer user.Wipe()
	pub, err := user.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	seed, err := user.Seed()
	if err != nil {
		t.Fatal(err)
	}
	claims, _ := json.Marshal(map[string]any{"sub": pub, "iss": pub, "iat": time.Now().Unix(), "nats": map[string]any{"type": "user", "version": 2}})
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"JWT","alg":"ed25519-nkey"}`))
	token := header + "." + base64.RawURLEncoding.EncodeToString(claims)
	sig, err := user.Sign([]byte(token))
	if err != nil {
		t.Fatal(err)
	}
	creds := "-----BEGIN NATS USER JWT-----\n" + token + "." + base64.RawURLEncoding.EncodeToString(sig) + "\n------END NATS USER JWT------\n\n-----BEGIN USER NKEY SEED-----\n" + string(seed) + "\n------END USER NKEY SEED------\n"
	return endpoint, creds
}

func status1695TLS(t *testing.T) (tls.Certificate, *x509.CertPool, string) {
	t.Helper()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(1695), Subject: pkix.Name{CommonName: "status-operator"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	hash := sha256.Sum256(der)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}, roots, hex.EncodeToString(hash[:])
}

func status1695RPC(t *testing.T, ctx context.Context) string {
	t.Helper()
	t.Setenv(chainfixture.RecordEnv, "")
	recording := chainfixture.New(t)
	client, err := ethclient.DialContext(ctx, recording.Serve(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	// The recorded latest block is empty, so its header response is also its
	// full-transaction response. Prove that before translating this one call.
	var head struct{ Transactions []json.RawMessage }
	if err = client.Client().CallContext(ctx, &head, "eth_getBlockByNumber", "latest", false); err != nil || head.Transactions == nil || len(head.Transactions) != 0 {
		t.Fatalf("recorded empty head required: %v", err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
		if err != nil {
			t.Error(err)
			http.Error(w, "request", http.StatusBadRequest)
			return
		}
		var call struct {
			ID     json.RawMessage   `json:"id"`
			Method string            `json:"method"`
			Params []json.RawMessage `json:"params"`
		}
		if err = json.Unmarshal(body, &call); err != nil {
			t.Error(err)
			http.Error(w, "request", http.StatusBadRequest)
			return
		}
		if call.Method == "eth_getBlockByNumber" && len(call.Params) == 2 && string(call.Params[0]) == `"latest"` && string(call.Params[1]) == "true" {
			call.Params[1] = json.RawMessage("false")
			body, err = json.Marshal(call)
			if err != nil {
				t.Error(err)
				http.Error(w, "request", http.StatusBadRequest)
				return
			}
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
		recording.ServeHTTP(w, r)
	}))
	t.Cleanup(server.Close)
	return server.URL
}

func TestIndexerStatus1695(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	db := coordinatortest.Bootstrap(t, os.Getenv("L06_POSTGRES_DSN"), "indexer_runtime")
	admin, err := pgx.Connect(ctx, db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(context.Background())
	execute := func(sql string, args ...any) {
		t.Helper()
		if _, err := admin.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	rt := db.Runtimes["indexer_runtime"]
	// The coordinator installs the security journal; grant session reads and audit appends.
	role := pgx.Identifier{rt.Role}.Sanitize()
	execute("GRANT USAGE ON SCHEMA security TO " + role)
	execute("GRANT SELECT ON security.operator_sessions TO " + role)
	execute("GRANT INSERT ON security.authorization_decisions TO " + role)
	execute("GRANT USAGE ON SEQUENCE security.authorization_decisions_seq_seq TO " + role)
	var expected production.ExpectedSchema
	if err = json.Unmarshal(rt.ExpectedSchema, &expected); err != nil {
		t.Fatal(err)
	}
	expected.Privileges = append(expected.Privileges,
		production.Grant{Schema: "security", Object: "operator_sessions", ObjectKind: "table", Privileges: []string{"SELECT"}},
		production.Grant{Schema: "security", Object: "authorization_decisions", ObjectKind: "table", Privileges: []string{"INSERT"}},
		production.Grant{Schema: "security", Object: "authorization_decisions_seq_seq", ObjectKind: "sequence", Privileges: []string{"USAGE"}})
	tx, err := admin.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	expected.Schemas["security"], err = production.CatalogFingerprint(ctx, tx, "security")
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = production.RecordRuntimeReadiness(ctx, admin, expected); err != nil {
		t.Fatal(err)
	}

	endpoint, creds := status1695Broker(t)
	rpcURL := status1695RPC(t, ctx)
	cfg := config.Config{RuntimeProfile: "owned_local", BindAddress: "127.0.0.1", RuntimePoolID: "status", RuntimeInstanceID: "status1695", IndexerMode: config.ModeLive, IndexerBatchSize: 32, IndexerConcurrency: 1, BackfillBatchSize: 32, RuntimeMetricsEnvironment: "qualification", RuntimeZone: "zone-a", NATSCredentialsCreds: creds}
	cfg.NodeEnv = "production"
	cfg.SecretsProvider = "env"
	cfg.SecretsRefreshSeconds = 60
	cfg.ChainID = 125
	cfg.DatabaseURL = rt.DSN
	cfg.RedisURL = os.Getenv("INDEXER_TEST_REDIS_URL")
	cfg.RPCURL = rpcURL
	cfg.NATSURL = endpoint
	for k, v := range map[string]string{"DATABASE_URL": cfg.DatabaseURL, "REDIS_URL": cfg.RedisURL, "RPC_URL": rpcURL, "NATS_URL": endpoint, "NATS_CREDENTIALS_CREDS": creds} {
		t.Setenv(k, v)
	}
	cfg.ExpectedSchemaFile, cfg.ExpectedSchemaSHA256 = status1695Bound(t, expected)
	parsed, err := url.Parse(rt.DSN)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatal(err)
	}
	budget := production.ConnectionBudget{Identity: "status1695", Targets: []production.PostgresTarget{{ID: "local", Member: "primary", Host: parsed.Hostname(), Port: uint16(port), MaxConnections: 1000}}, Operational: []production.OperationalBudget{{ID: "migrations", Target: "local", Category: "migration", Connections: 1}}}
	for i, r := range []string{"apid:rest", "apid:sockets", "indexerd", "chartsd", "statsd", "pnld", "rankingd", "mediagatewayd", "userd", "sociald", "metadatad", "livestreamd"} {
		user := fmt.Sprintf("status_user_%d", i)
		if r == "indexerd" {
			user = rt.Role
		}
		budget.Clients = append(budget.Clients, production.ClientBudget{Role: r, PoolID: "status", Target: "local", Transport: shareddb.TransportDirect, Alias: db.Name, User: user, QueryClass: shareddb.Batch, Processes: production.LiveProcesses{Replicas: 1, Maximum: 1}, MaxConnections: 3})
	}
	cfg.RuntimeBudgetFile, cfg.RuntimeBudgetSHA256 = status1695Bound(t, budget)
	parameters := DefaultDeliveryParameters()
	parameters.Region = coordinatortest.Region
	parameters.MaxDeliveryLagMS = 1
	documents, err := RenderRuntimeDocuments(renderInputs(t), parameters)
	if err != nil {
		t.Fatal(err)
	}
	bindRendered := func(name string) (string, string) {
		t.Helper()
		d := documents[name]
		p := filepath.Join(t.TempDir(), name)
		if e := os.WriteFile(p, d.Bytes, 0600); e != nil {
			t.Fatal(e)
		}
		return p, d.SHA256
	}
	cfg.ReceiptConfigFile, cfg.ReceiptConfigSHA256 = bindRendered("receipt.json")
	cfg.DeliveryConfigFile, cfg.DeliveryConfigSHA256 = bindRendered("delivery.json")
	cfg.LifecycleConfigFile, cfg.LifecycleConfigSHA256 = bindRendered("lifecycle.json")
	var doc deliveryRuntimeDocument
	if err = config.ReadBoundDocument(cfg.DeliveryConfigFile, cfg.DeliveryConfigSHA256, &doc); err != nil {
		t.Fatal(err)
	}
	// Resume an existing canonical stream: the recording has the anchor header
	// and full blocks from 33963716 onward. Boot validates the persisted full
	// checkpoint against that same source, without synthesizing anchor traffic.
	var fence int64
	func() {
		var lifecycle lifecycleRuntimeDocument
		if err := config.ReadBoundDocument(cfg.LifecycleConfigFile, cfg.LifecycleConfigSHA256, &lifecycle); err != nil {
			t.Fatal(err)
		}
		lifecycle.Committed.Endpoints["paxeer-primary"] = rpcURL
		provider, err := source.NewCommittedSource(ctx, lifecycle.Committed)
		if err != nil {
			t.Fatal(err)
		}
		defer provider.Close()
		block, err := provider.BlockAt(ctx, 33963716)
		if err != nil {
			t.Fatal(err)
		}
		pool, err := pgxpool.New(ctx, rt.DSN)
		if err != nil {
			t.Fatal(err)
		}
		defer pool.Close()
		if err = seedCanonicalWatermark(ctx, pool, doc.CanonicalStreamID, lifecycle); err != nil {
			t.Fatal(err)
		}
		canonical := store.New(pool)
		fence, err = canonical.AcquireLease(ctx, 125, doc.CanonicalStreamID, doc.Owner, 30*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		if err = canonical.CommitCanonical(ctx, store.CanonicalBatch{ChainID: 125, StreamID: doc.CanonicalStreamID, Owner: doc.Owner, BatchID: "status1695-bootstrap", Fence: fence, PreviousHeight: 33963715, Blocks: []store.BlockCompleteness{{Height: 33963716, Hash: strings.ToLower(block.Hash.Hex()), FetchComplete: true, RequiredDecodeComplete: true}}}, nil); err != nil {
			t.Fatal(err)
		}
	}()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	boot := func() *App {
		t.Helper()
		a, e := New(ctx, cfg, logger)
		if e != nil {
			t.Fatal(e)
		}
		return a
	}
	a := boot()
	status := func(runtime *App, code int, state string) map[string]any {
		t.Helper()
		rr := httptest.NewRecorder()
		runtime.Router.ServeHTTP(rr, httptest.NewRequest("GET", "/status", nil))
		if rr.Code != code {
			t.Fatalf("status %d want %d: %s", rr.Code, code, rr.Body.String())
		}
		var body map[string]any
		if e := json.Unmarshal(rr.Body.Bytes(), &body); e != nil {
			t.Fatal(e)
		}
		if body["status"] != state {
			t.Fatalf("status body %v", body)
		}
		return body
	}
	body := status(a, 200, "ok")
	if body["mode"] != "canonical" || body["canonicalStreamId"] != parameters.CanonicalStreamID || body["lastProcessedBlock"] != float64(33963716) {
		t.Fatal(body)
	}
	for _, path := range []string{"/status/dead-letter", "/status/dead-letter/drain"} {
		method := "GET"
		if strings.HasSuffix(path, "drain") {
			method = "POST"
		}
		rr := httptest.NewRecorder()
		a.Router.ServeHTTP(rr, httptest.NewRequest(method, path, nil))
		if rr.Code != 503 {
			t.Fatalf("absent policy: %s %d", path, rr.Code)
		}
	}
	payload := json.RawMessage(`{"private":"status1695-payload-canary"}`)
	event := delivery.Envelope{EventID: "status1695-event", EventType: "canonical_creation", SchemaVersion: 1, ChainID: 125, SourceOrder: delivery.SourceOrder{Height: "33963716", TransactionIndex: "0", LogIndex: "0"}, EmittedAt: time.Now().UTC(), TraceID: "status1695", PayloadHash: delivery.Hash(payload), Payload: payload}
	if err = a.relay.Store.Produce(ctx, []delivery.Pending{{Subject: "canonical.creation.v1", Envelope: event}}, func(context.Context, pgx.Tx) error { return nil }); err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	body = status(a, 503, "degraded")
	metrics := body["delivery"].(map[string]any)
	if metrics["pendingRows"] != float64(1) || metrics["pendingBytes"].(float64) <= 0 || metrics["oldestUnpublishedAgeSeconds"].(float64) <= 0 {
		t.Fatal(metrics)
	}
	a.Close()
	parameters.MaxDeliveryLagMS = 60000
	doc.MaxDeliveryLagMS = parameters.MaxDeliveryLagMS
	cfg.DeliveryConfigFile, cfg.DeliveryConfigSHA256 = status1695Bound(t, doc)
	a = boot()
	defer func() {
		if a != nil {
			a.Close()
		}
	}()
	status(a, 200, "ok")
	committedBlock, err := a.history.RPC.BlockAt(ctx, 33963717)
	if err != nil {
		t.Fatal(err)
	}
	// Commit progress through the canonical store after boot, independently of the legacy cursor.
	canonical := store.New(a.pool)
	if err = canonical.RenewLease(ctx, 125, doc.CanonicalStreamID, doc.Owner, fence, 30*time.Second); err != nil {
		t.Fatal(err)
	}
	if err = canonical.CommitCanonical(ctx, store.CanonicalBatch{ChainID: 125, StreamID: doc.CanonicalStreamID, Owner: doc.Owner, BatchID: "status1695-commit", Fence: fence, PreviousHeight: 33963716, Blocks: []store.BlockCompleteness{{Height: 33963717, Hash: strings.ToLower(committedBlock.Hash.Hex()), FetchComplete: true, RequiredDecodeComplete: true}}}, nil); err != nil {
		t.Fatal(err)
	}
	body = status(a, 200, "ok")
	if body["lastProcessedBlock"] != float64(33963717) {
		t.Fatal(body)
	}
	leases, err := a.relay.Store.Claim(ctx, doc.Owner, 1, 30*time.Second)
	if err != nil || len(leases) != 1 {
		t.Fatalf("claim %v %v", leases, err)
	}
	if err = a.relay.Store.Retry(ctx, leases[0], delivery.SchemaViolation, 1, time.Second); err != nil {
		t.Fatal(err)
	}
	body = status(a, 503, "degraded")
	if body["delivery"].(map[string]any)["quarantinedRows"] != float64(1) {
		t.Fatal(body)
	}
	wire, err := event.Encode(doc.Policy, "canonical.creation.v1")
	if err != nil {
		t.Fatal(err)
	}
	js, err := jetstream.New(a.nats)
	if err != nil {
		t.Fatal(err)
	}
	stream, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: doc.Stream, Subjects: []string{"canonical.creation.v1"}, Storage: jetstream.FileStorage})
	if err != nil {
		t.Fatal(err)
	}
	consumerConfig := natsdriver.ConsumerConfig{Stream: doc.Stream, Durable: "status1695", Subject: "canonical.creation.v1", Batch: 1, MaxAckPending: 1, MaxWaiting: 1, MaxDeliver: 3, FetchWait: time.Second, AckWait: 30 * time.Second}
	brokerConfig, err := consumerConfig.BrokerConsumerConfig()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = stream.CreateConsumer(ctx, brokerConfig); err != nil {
		t.Fatal(err)
	}
	producer, err := natsdriver.NewPublisher(a.nats, doc.Stream)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = producer.Publish(ctx, "canonical.creation.v1", event.EventID, wire); err != nil {
		t.Fatal(err)
	}
	consumer, err := natsdriver.NewConsumer(ctx, a.nats, consumerConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer consumer.Close()
	messages, err := consumer.Fetch(ctx)
	if err != nil || len(messages) != 1 {
		t.Fatalf("actual durable message %v %v", messages, err)
	}
	restricted, err := delivery.NewStore(a.pool, "indexer", delivery.Policy{Subjects: map[string]bool{"indexer.replay.v2": true}, EventTypes: doc.Policy.EventTypes})
	if err != nil {
		t.Fatal(err)
	}
	if err = restricted.Consume(ctx, "stats", "g1", messages[0].Subject, messages[0].Wire, messages[0], func(context.Context, pgx.Tx, delivery.Envelope) error {
		return errors.New("unexpected unapproved effect")
	}); !errors.Is(err, delivery.ErrAuthorization) {
		t.Fatalf("unauthorized delivery %v", err)
	}
	a.Close()
	a = nil
	pair, roots, pin := status1695TLS(t)
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	doc.DeadLetters = &DeadLetterParameters{SecuritySchema: "security", Audience: "indexer", PolicyVersion: "status1695-v1", Issuer: "gateway", PeerCertSHA256: pin, PublicKey: public, AssertionHeader: auth.ActorAssertionHeader, SignatureHeader: auth.ActorSignatureHeader}
	cfg.DeliveryConfigFile, cfg.DeliveryConfigSHA256 = status1695Bound(t, doc)
	a = boot()
	server := httptest.NewUnstartedServer(a.Router)
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{pair}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	server.StartTLS()
	defer server.Close()
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, Certificates: []tls.Certificate{pair}}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	wallet, err := auth.NormalizeWallet("0x52908400098527886e0f7030069857d2e4169ee7")
	if err != nil {
		t.Fatal(err)
	}
	session := uuid.NewString()
	account := uuid.NewString()
	execute(`INSERT INTO security.operator_sessions(session_id,wallet,actor,audience,issued_at,expires_at,mfa_expires_at,scopes) VALUES($1,$2,'operator','indexer',clock_timestamp()-interval '1 second',clock_timestamp()+interval '2 minutes',clock_timestamp()+interval '1 minute',ARRAY['inspect','retention-delete'])`, session, wallet)
	issuer := auth.ServiceAssertionIssuer{Issuer: "gateway", Audience: "indexer", Key: key, Transport: auth.AssertionTransport{AssertionHeader: auth.ActorAssertionHeader, SignatureHeader: auth.ActorSignatureHeader}}
	send := func(method, path string, signed bool, want int) string {
		t.Helper()
		req, e := http.NewRequest(method, server.URL+path, nil)
		if e != nil {
			t.Fatal(e)
		}
		id := uuid.NewString()
		req.Header.Set("X-Request-ID", id)
		if signed {
			e = issuer.SignRequest(req, auth.IdentityPrincipal{AccountID: account, SessionID: session, Wallet: wallet, ProofAt: time.Now(), AccessExpiresAt: time.Now().Add(time.Minute)}, []string{"operator", "inspect", "retention-delete"}, nil)
			if e != nil {
				t.Fatal(e)
			}
		}
		res, e := client.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		raw, e := io.ReadAll(res.Body)
		if e != nil {
			t.Fatal(e)
		}
		if res.StatusCode != want {
			t.Fatalf("%s %s: %d want %d: %s", method, path, res.StatusCode, want, raw)
		}
		if strings.Contains(string(raw), "status1695-payload-canary") || strings.Contains(string(raw), "schema_violation") {
			t.Fatalf("private data: %s", raw)
		}
		var recordedPolicy, scope string
		var allowed bool
		if e = admin.QueryRow(ctx, `SELECT policy_version,scope,allowed FROM security.authorization_decisions WHERE request_id=$1`, id).Scan(&recordedPolicy, &scope, &allowed); e != nil {
			t.Fatal(e)
		}
		if recordedPolicy != doc.DeadLetters.PolicyVersion || allowed != (signed && want != 403 || signed && strings.HasSuffix(path, "drain")) {
			t.Fatalf("audit policy=%s scope=%s allowed=%v", recordedPolicy, scope, allowed)
		}
		return string(raw)
	}
	inspected := send("GET", "/status/dead-letter", true, 200)
	var inspection struct {
		Entries []map[string]any `json:"entries"`
	}
	if err = json.Unmarshal([]byte(inspected), &inspection); err != nil || len(inspection.Entries) != 1 {
		t.Fatalf("inspection %s %v", inspected, err)
	}
	entry := inspection.Entries[0]
	if len(entry) != 5 || entry["consumer"] != "stats" || entry["generation"] != "g1" || entry["envelopeHash"] != delivery.Hash(wire) || entry["failureClass"] != "unclassified" || entry["created"] == nil {
		t.Fatal(entry)
	}
	send("GET", "/status/dead-letter", false, 401)
	send("POST", "/status/dead-letter/drain", true, 403)
	execute(`UPDATE security.operator_sessions SET scopes=ARRAY['retention-delete'] WHERE session_id=$1`, session)
	send("GET", "/status/dead-letter", true, 403)
	execute(`UPDATE security.operator_sessions SET scopes=ARRAY['inspect'],mfa_expires_at=clock_timestamp()-interval '1 second' WHERE session_id=$1`, session)
	send("GET", "/status/dead-letter", true, 403)
	var records int
	if err = admin.QueryRow(ctx, `SELECT count(*) FROM indexer.delivery_quarantine`).Scan(&records); err != nil || records != 1 {
		t.Fatalf("retained %d %v", records, err)
	}
	var canDelete bool
	if err = admin.QueryRow(ctx, `SELECT has_table_privilege($1,'indexer.delivery_quarantine','DELETE')`, rt.Role).Scan(&canDelete); err != nil || canDelete {
		t.Fatalf("least privilege delete=%v err=%v", canDelete, err)
	}
	execute(`DELETE FROM indexer.indexer_ingest_watermarks WHERE chain_id=125 AND stream_id=$1`, doc.CanonicalStreamID)
	body = status(a, 503, "unavailable")
	if body["lastProcessedBlock"] != nil || !strings.Contains(fmt.Sprint(body["unavailableComponents"]), "canonicalWatermark") {
		t.Fatal(body)
	}
	execute("REVOKE SELECT ON indexer.indexer_ingest_watermarks FROM " + role)
	status(a, 503, "unavailable")
	execute("GRANT SELECT ON indexer.indexer_ingest_watermarks TO " + role)
	execute("REVOKE SELECT ON indexer.delivery_outbox FROM " + role)
	body = status(a, 503, "unavailable")
	if body["delivery"] != nil || !strings.Contains(fmt.Sprint(body["unavailableComponents"]), "delivery") {
		t.Fatal(body)
	}
	execute("GRANT SELECT ON indexer.delivery_outbox TO " + role)
	a.Close()
	a = nil
	for name, mutate := range map[string]func(*deliveryRuntimeDocument){"zero lag": func(d *deliveryRuntimeDocument) { d.MaxDeliveryLagMS = 0 }, "negative lag": func(d *deliveryRuntimeDocument) { d.MaxDeliveryLagMS = -1 }, "overflow lag": func(d *deliveryRuntimeDocument) { d.MaxDeliveryLagMS = math.MaxInt64 }, "empty policy": func(d *deliveryRuntimeDocument) { d.DeadLetters = &DeadLetterParameters{} }, "key": func(d *deliveryRuntimeDocument) { d.DeadLetters.PublicKey = nil }, "pin": func(d *deliveryRuntimeDocument) { d.DeadLetters.PeerCertSHA256 = "bad" }, "schema": func(d *deliveryRuntimeDocument) { d.DeadLetters.SecuritySchema = "security;drop" }, "audience": func(d *deliveryRuntimeDocument) { d.DeadLetters.Audience = "" }, "version": func(d *deliveryRuntimeDocument) { d.DeadLetters.PolicyVersion = "" }, "headers": func(d *deliveryRuntimeDocument) { d.DeadLetters.SignatureHeader = d.DeadLetters.AssertionHeader }} {
		t.Run(name, func(t *testing.T) {
			invalid := doc
			copyPolicy := *doc.DeadLetters
			invalid.DeadLetters = &copyPolicy
			mutate(&invalid)
			cfg.DeliveryConfigFile, cfg.DeliveryConfigSHA256 = status1695Bound(t, invalid)
			bad, e := New(ctx, cfg, logger)
			if bad != nil {
				bad.Close()
			}
			if e == nil || (!strings.Contains(e.Error(), "lag budget") && !strings.Contains(e.Error(), "dead-letter policy")) {
				t.Fatalf("invalid configuration accepted or wrong error: %v", e)
			}
		})
	}
	cfg.NodeEnv = "test"
	cfg.DatabaseURL, _ = internaltest.NewPostgres(t)
	cfg.IndexerLogSource = config.SourcePaxscan
	cfg.PaxscanDatabaseURL = cfg.DatabaseURL
	legacy := boot()
	defer legacy.Close()
	if err = store.New(legacy.pool).UpsertCursor(ctx, 125, 77); err != nil {
		t.Fatal(err)
	}
	body = status(legacy, 200, "ok")
	if body["mode"] != "legacy" || body["lastProcessedBlock"] != float64(77) {
		t.Fatal(body)
	}
}
