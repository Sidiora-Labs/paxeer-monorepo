package app

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	v1 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v1"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
)

const (
	gatewayDNS    = "gateway.railway.internal"
	rankingDNS    = "ranking.railway.internal"
	visibilityDNS = "social.railway.internal"
	sourceStream  = "chain125-canonical"
	projection    = "accounting-market-v1"
)

// testPKI is an ephemeral CA and the leaf identities of the ranking mTLS
// perimeter, generated with crypto/x509 for this test only.
type testPKI struct {
	caKey  *ecdsa.PrivateKey
	caCert *x509.Certificate
	caPEM  []byte
}

type leaf struct {
	certPEM, keyPEM []byte
	der             []byte
}

func (l leaf) pin() string { return fmt.Sprintf("%x", sha256.Sum256(l.der)) }

func (l leaf) pair(t *testing.T) tls.Certificate {
	t.Helper()
	pair, err := tls.X509KeyPair(l.certPEM, l.keyPEM)
	if err != nil {
		t.Fatal(err)
	}
	return pair
}

func newPKI(t *testing.T) *testPKI {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: serial(t), Subject: pkix.Name{CommonName: "ranking test CA"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return &testPKI{caKey: key, caCert: cert, caPEM: pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})}
}

func serial(t *testing.T) *big.Int {
	t.Helper()
	n, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func (p *testPKI) issue(t *testing.T, dns string, usage x509.ExtKeyUsage) leaf {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: serial(t), Subject: pkix.Name{CommonName: dns}, DNSNames: []string{dns},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{usage},
	}
	der, err := x509.CreateCertificate(rand.Reader, template, p.caCert, &key.PublicKey, p.caKey)
	if err != nil {
		t.Fatal(err)
	}
	pkcs8, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	return leaf{pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: pkcs8}), der}
}

// writeBound writes a bound runtime document and returns its sha256.
func writeBound(t *testing.T, path string, raw []byte) string {
	t.Helper()
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	return production.Checksum(raw)
}

func contractEnv(t *testing.T) {
	t.Helper()
	// Contract addresses must equal the reviewed chain-125 deployment manifest
	// (shared/config.BaseEnv.Validate rejects any other address).
	roles := deployment.Current().Roles
	for k, v := range map[string]string{
		"EVENT_EMITTER_ADDRESS":   roles["EventEmitter"].Address,
		"POOL_REGISTRY_ADDRESS":   roles["PoolRegistry"].Address,
		"ROUTER_ADDRESS":          roles["Router"].Address,
		"FACTORY_ADDRESS":         roles["SidioraFactory"].Address,
		"QUOTER_ADDRESS":          roles["Quoter"].Address,
		"PROTOCOL_CONFIG_ADDRESS": roles["ProtocolConfig"].Address,
		"FEE_ACCUMULATOR_ADDRESS": roles["FeeAccumulator"].Address,
		"SIDIORA_NFT_ADDRESS":     roles["SidioraNFT"].Address,
		"FEES_ROUTER_ADDRESS":     roles["FeesRouter"].Address,
		"POOL_BEACON_ADDRESS":     roles["PoolBeacon"].Address,
	} {
		t.Setenv(k, v)
	}
}

// TestNewServesAndRanks boots rankingd on its production native path
// (NODE_ENV=production, container profile): the bound connection budget
// admits the pool, the bound ExpectedSchema is checked against the actual
// catalog as the least-privilege runtime role, the bound runtime document
// supplies the mTLS perimeter, and the HTTPS listener enforces the pinned
// gateway identity. Publication uses a loopback social visibility transcript
// served over mutually authenticated TLS.
func TestNewServesAndRanks(t *testing.T) {
	internaltest.RankingBroker(t)
	db := internaltest.NewRankingRuntimeDatabase(t)
	redisURL := internaltest.NewRedisURL(t)
	ctx := context.Background()
	dir := t.TempDir()

	pki := newPKI(t)
	server := pki.issue(t, rankingDNS, x509.ExtKeyUsageServerAuth)
	client := pki.issue(t, rankingDNS, x509.ExtKeyUsageClientAuth)
	gateway := pki.issue(t, gatewayDNS, x509.ExtKeyUsageClientAuth)
	observer := pki.issue(t, "observer.railway.internal", x509.ExtKeyUsageClientAuth)
	social := pki.issue(t, visibilityDNS, x509.ExtKeyUsageServerAuth)

	peer, authority := publicationRecoveryVisibilityPeer(t, pki, social, client, map[string]bool{"0x" + strings.Repeat("a", 40): true})
	authority.Store(false)
	caBundle := pki.caPEM
	clientCert, clientKey := client.certPEM, client.keyPEM
	endpoint := peer.URL + "/internal/v2/social/ranking-visibility"
	peerDNS, peerPin := visibilityDNS, social.pin()

	files := map[string][]byte{"ca.pem": caBundle, "client.pem": clientCert, "server.pem": server.certPEM}
	pins := map[string]string{}
	for name, raw := range files {
		pins[name] = writeBound(t, filepath.Join(dir, name), raw)
	}
	cursor := make([]byte, 32)
	if _, err := rand.Read(cursor); err != nil {
		t.Fatal(err)
	}
	binding := config.RankingRuntimeBinding{
		Version: 2, RankingSchema: "ranking", StatsSchema: "stats", SourceSchema: "indexer",
		SourceStream: sourceStream, ProjectionVersion: projection,
		CacheNamespace: "snapshots", CursorKeyReference: "TEST_RANKING_CURSOR_KEY",
		RecoverySchema: "recovery", RecoveryEpoch: db.Epoch, RecoveryRegion: db.Region,
		VisibilityEndpoint: endpoint, VisibilityDNS: peerDNS, VisibilityCertSHA256: peerPin,
		GatewayDNS: gatewayDNS, GatewayCertSHA256: gateway.pin(),
		CAFile: filepath.Join(dir, "ca.pem"), CASHA256: pins["ca.pem"],
		ClientCertFile: filepath.Join(dir, "client.pem"), ClientCertSHA256: pins["client.pem"], ClientKeyReference: "TEST_RANKING_CLIENT_KEY_PEM",
		ServerCertFile: filepath.Join(dir, "server.pem"), ServerCertSHA256: pins["server.pem"], ServerKeyReference: "TEST_RANKING_SERVER_KEY_PEM",
		MaxAttempts: 10, MaximumReads: 16,
	}
	admin, err := pgx.Connect(ctx, db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer admin.Close(ctx)
	var maxConnections int64
	if err = admin.QueryRow(ctx, "SELECT setting::bigint FROM pg_settings WHERE name='max_connections'").Scan(&maxConnections); err != nil {
		t.Fatal(err)
	}

	port, privatePort := 8080, 8443
	contractEnv(t)
	for k, v := range map[string]string{
		"NODE_ENV":                       "production",
		"RUNTIME_PROFILE":                "owned_local",
		"BIND_ADDRESS":                   "127.0.0.1",
		"PORT":                           strconv.Itoa(port),
		"RANKING_PRIVATE_LISTEN_ADDRESS": "127.0.0.1:" + strconv.Itoa(privatePort),
		"ZONE":                           "zone-a",
		"METRICS_ENVIRONMENT":            "production",
		"LOG_LEVEL":                      "error",
		"SECRETS_PROVIDER":               "env",
		"SECRETS_REFRESH_SECONDS":        "30",
		"DATABASE_URL":                   db.RuntimeDSN,
		"REDIS_URL":                      redisURL,
		"REDIS_BULL_URL":                 redisURL,
		"RPC_URL":                        "https://rpc.example.com/rpc",
		"DB_POOL_ID":                     "ranking_batch",
		"INSTANCE_ID":                    "ranking-test-1",
		"RANKING_TICK_INTERVAL_MS":       "30000",
		"RANKING_NEW_TICK_INTERVAL_MS":   "10000",
		"TEST_RANKING_CLIENT_KEY_PEM":    string(clientKey),
		"TEST_RANKING_SERVER_KEY_PEM":    string(server.keyPEM),
		"TEST_RANKING_CURSOR_KEY":        base64.RawURLEncoding.EncodeToString(cursor),
	} {
		t.Setenv(k, v)
	}

	composeStartup(t, db, binding, maxConnections)
	cfg, err := config.Load()
	if err != nil {
		t.Fatal(err)
	}
	var loaded config.RankingRuntimeBinding
	if err := config.ReadBoundDocument(cfg.RankingRuntimeFile, cfg.RankingRuntimeSHA256, &loaded); err != nil {
		t.Fatal(err)
	}
	if loaded.CursorKeyReference != "TEST_RANKING_CURSOR_KEY" {
		t.Fatal("loaded ranking binding must reference the deployment cursor key")
	}
	provider, err := secrets.NewChecked(secrets.Options{Provider: secrets.ProviderType(cfg.SecretsProvider)})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := provider.GetRequired(ctx, loaded.CursorKeyReference)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(decoded) < 32 || !bytes.Equal(decoded, cursor) {
		t.Fatal("loaded ranking binding must resolve the generated cursor key")
	}
	provisionStartupBroker(t)
	binary := startupBinary(t)

	// The bound documents are refused when their pins differ.
	t.Run("bound documents", func(t *testing.T) {
		cfg, err := config.Load()
		if err != nil {
			t.Fatalf("config.Load: %v", err)
		}
		cfg.ExpectedSchemaSHA256 = strings.Repeat("0", 64)
		if _, err = New(ctx, cfg, quietLogger()); err == nil {
			t.Fatal("expected-schema pin mismatch admitted")
		}
		cfg, _ = config.Load()
		cfg.RankingRuntimeSHA256 = strings.Repeat("0", 64)
		if _, err = New(ctx, cfg, quietLogger()); err == nil {
			t.Fatal("runtime document pin mismatch admitted")
		}
	})
	t.Run("runtime binding fields", func(t *testing.T) {
		for _, tc := range []struct {
			name   string
			change func(*config.RankingRuntimeBinding)
		}{
			{"gateway identity", func(b *config.RankingRuntimeBinding) { b.GatewayDNS = "" }},
			{"CA document", func(b *config.RankingRuntimeBinding) { b.CASHA256 = strings.Repeat("0", 64) }},
			{"client certificate", func(b *config.RankingRuntimeBinding) { b.ClientCertSHA256 = strings.Repeat("0", 64) }},
			{"client key", func(b *config.RankingRuntimeBinding) { b.ClientKeyReference = "RANKING_ABSENT_CLIENT_KEY" }},
			{"visibility endpoint", func(b *config.RankingRuntimeBinding) {
				b.VisibilityEndpoint = strings.Replace(b.VisibilityEndpoint, "https://", "http://", 1)
			}},
			{"server certificate", func(b *config.RankingRuntimeBinding) { b.ServerCertSHA256 = strings.Repeat("0", 64) }},
			{"server key", func(b *config.RankingRuntimeBinding) { b.ServerKeyReference = "RANKING_ABSENT_SERVER_KEY" }},
			{"recovery schema", func(b *config.RankingRuntimeBinding) { b.RecoverySchema = "invalid-schema" }},
			{"source schema", func(b *config.RankingRuntimeBinding) { b.SourceSchema = "invalid-schema" }},
			{"ranking schema", func(b *config.RankingRuntimeBinding) { b.RankingSchema = "invalid-schema" }},
			{"cache namespace", func(b *config.RankingRuntimeBinding) { b.CacheNamespace = "!" }},
			{"cursor secret", func(b *config.RankingRuntimeBinding) { b.CursorKeyReference = "RANKING_ABSENT_CURSOR_KEY" }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				changed := binding
				tc.change(&changed)
				raw, err := json.Marshal(changed)
				if err != nil {
					t.Fatal(err)
				}
				cfg, err := config.Load()
				if err != nil {
					t.Fatal(err)
				}
				cfg.RankingRuntimeFile = filepath.Join(t.TempDir(), "ranking-runtime.json")
				cfg.RankingRuntimeSHA256 = writeBound(t, cfg.RankingRuntimeFile, raw)
				if instance, err := New(ctx, cfg, quietLogger()); err == nil {
					instance.Close()
					t.Fatal("invalid runtime binding accepted")
				} else {
					reference := map[string]string{"client key": changed.ClientKeyReference, "server key": changed.ServerKeyReference, "cursor secret": changed.CursorKeyReference}[tc.name]
					if reference != "" && !strings.Contains(err.Error(), reference) {
						t.Fatalf("missing secret diagnostic does not name its reference: %v", err)
					}
				}
			})
		}
	})

	process := startRankingProcess(t, binary)
	shutdown := process.stop

	roots := x509.NewCertPool()
	roots.AddCert(pki.caCert)
	base := fmt.Sprintf("https://127.0.0.1:%d", privatePort)
	healthBase := fmt.Sprintf("http://127.0.0.1:%d", port)
	httpsClient := func(identity *leaf) *http.Client {
		cfg := &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, ServerName: rankingDNS}
		if identity != nil {
			cfg.Certificates = []tls.Certificate{identity.pair(t)}
		}
		return &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: cfg}}
	}
	gatewayClient, observerClient := httpsClient(&gateway), httpsClient(&observer)
	get := func(c *http.Client, path string) (int, map[string]any) {
		t.Helper()
		resp, err := c.Get(base + path) //nolint:noctx // bounded client timeout
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		defer resp.Body.Close()
		raw, _ := io.ReadAll(resp.Body)
		body := map[string]any{}
		_ = json.Unmarshal(raw, &body)
		return resp.StatusCode, body
	}

	// The native listener comes up (New succeeded: budget-admitted pool,
	// runtime-role schema readiness, secrets, mTLS material, epoch guard).
	deadline := time.Now().Add(30 * time.Second)
	for {
		select {
		case <-process.done:
			t.Fatalf("rankingd exited during startup: %v", process.err)
		default:
		}
		resp, err := observerClient.Get(base + "/health/live") //nolint:noctx // bounded client timeout
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("rankingd listener not live: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	healthClient := &http.Client{Timeout: 5 * time.Second}
	defer healthClient.CloseIdleConnections()
	for _, path := range []string{"/health/live", "/health/ready", "/rankings/trending"} {
		response, err := healthClient.Get(healthBase + path)
		if err != nil {
			t.Fatalf("plain health listener GET %s: %v", path, err)
		}
		response.Body.Close()
		want := http.StatusOK
		if path == "/health/ready" {
			want = http.StatusServiceUnavailable
		} else if path == "/rankings/trending" {
			want = http.StatusNotFound
		}
		if response.StatusCode != want {
			t.Fatalf("plain health listener %s = %d, want %d", path, response.StatusCode, want)
		}
	}

	// mTLS is mandatory: no client certificate, no connection.
	if resp, err := httpsClient(nil).Get(base + "/health/live"); err == nil { //nolint:noctx // bounded client timeout
		resp.Body.Close()
		t.Fatal("TLS handshake without a client certificate succeeded")
	}
	// A CA-issued identity that is not the pinned gateway is refused.
	if code, body := get(observerClient, "/rankings/trending"); code != http.StatusForbidden || body["code"] != "TRUSTED_INGRESS_REQUIRED" {
		t.Fatalf("non-gateway ingress = %d %v", code, body)
	}
	// Through the pinned gateway the snapshot API is reached.
	if code, body := get(gatewayClient, "/rankings/not-a-category"); code != http.StatusBadRequest || !strings.Contains(fmt.Sprint(body), "INVALID_CATEGORY") {
		t.Fatalf("invalid category = %d %v", code, body)
	}
	if code, _ := get(observerClient, "/metrics"); code != http.StatusOK {
		t.Fatalf("metrics status = %d", code)
	}

	t.Run("refuses without visibility authority", func(t *testing.T) {
		// Fail closed: without a moderation revision no snapshot is published
		// or served, and readiness reports unavailable.
		if code, body := get(gatewayClient, "/rankings/trending"); code != http.StatusServiceUnavailable || !strings.Contains(fmt.Sprint(body), "RANKING_UNAVAILABLE") {
			t.Fatalf("trending without visibility = %d %v", code, body)
		}
		if code, _ := get(observerClient, "/health/ready"); code != http.StatusServiceUnavailable {
			t.Fatalf("health/ready without visibility = %d, want 503", code)
		}
		if code, body := get(gatewayClient, "/internal/v2/rankings/inputs"); code != http.StatusServiceUnavailable || body["code"] != "RANKING_INPUT_UNAVAILABLE" {
			t.Fatalf("ranking inputs without visibility = %d %v", code, body)
		}
	})

	var retained v1.RankingsResponse
	t.Run("publishes and becomes ready", func(t *testing.T) {
		if peer == nil || peer.URL == "" {
			t.Fatal("publication qualification requires the loopback social mTLS peer")
		}
		authority.Store(true)
		// Seed a complete canonical source (lease, watermark, completed block,
		// one canonical creation) and wait for every category to publish and
		// the instance to report ready.
		now := time.Now().UTC()
		for _, step := range []struct {
			sql  string
			args []any
		}{
			{`INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,$1,'indexer-1',1,clock_timestamp()+interval '10 minutes')`, []any{sourceStream}},
			{`INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,$1,200,200)`, []any{sourceStream}},
			{`INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete,completed_at) VALUES(125,$1,200,$2,0,true,true,$3)`, []any{sourceStream, "0x" + strings.Repeat("c8", 32), now}},
			{`INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$3,$3,$3,1,150,$4,$5,0,0,$6,'{}','{}')`, []any{"0x" + strings.Repeat("7", 40), "0x" + strings.Repeat("a", 40), "0x" + strings.Repeat("1", 40), "0x" + strings.Repeat("96", 32), "0x" + hex.EncodeToString(make([]byte, 32)), now.Unix() - 600}},
		} {
			if _, err := admin.Exec(ctx, step.sql, step.args...); err != nil {
				t.Fatalf("seed: %v", err)
			}
		}
		deadline := time.Now().Add(90 * time.Second)
		for {
			code, body := get(observerClient, "/health/ready")
			if code == http.StatusOK {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("health/ready = %d after publication window: %v", code, body)
			}
			time.Sleep(time.Second)
		}
		for _, category := range ranker.Categories {
			if code, body := get(gatewayClient, "/rankings/"+category); code != http.StatusOK {
				t.Fatalf("%s = %d %v", category, code, body)
			}
		}
		response, err := gatewayClient.Get(base + "/rankings/new")
		if err != nil {
			t.Fatal(err)
		}
		decodeErr := json.NewDecoder(response.Body).Decode(&retained)
		closeErr := response.Body.Close()
		if decodeErr != nil || closeErr != nil || response.StatusCode != http.StatusOK || retained.Data.Snapshot.SnapshotId == "" || retained.Revision == nil || retained.Quality != "complete" {
			t.Fatalf("new snapshot: status %d, decode %v, close %v", response.StatusCode, decodeErr, closeErr)
		}
		for _, category := range ranker.Categories {
			deadline := time.Now().Add(5 * time.Second)
			for {
				var raw []byte
				err := admin.QueryRow(ctx, `SELECT frame FROM api_stream.stream_projection WHERE topic=$1`, "market.rankings."+category).Scan(&raw)
				if err == nil {
					frame, err := delivery.ParseCommittedStreamFrame(raw)
					if err != nil || frame.Topic != "market.rankings."+category {
						t.Fatalf("committed %s frame: %v", category, err)
					}
					var snapshot v1.RankingSnapshot
					if err = json.Unmarshal(frame.Payload, &snapshot); err != nil || snapshot.Category != category || snapshot.Items == nil {
						t.Fatalf("committed %s snapshot: %v", category, err)
					}
					if category == "new" && (snapshot.SnapshotId != retained.Data.Snapshot.SnapshotId || snapshot.TotalEligible != retained.Data.Snapshot.TotalEligible || frame.Revision != string(*retained.Revision)) {
						t.Fatal("committed new snapshot differs from the HTTP response")
					}
					break
				}
				if err != pgx.ErrNoRows || time.Now().After(deadline) {
					t.Fatalf("committed %s frame not available: %v", category, err)
				}
				time.Sleep(20 * time.Millisecond)
			}
		}
		response, err = gatewayClient.Get(base + "/internal/v2/rankings/inputs") //nolint:noctx // bounded client timeout
		if err != nil {
			t.Fatal(err)
		}
		var input ranker.InputSnapshot
		decodeErr = json.NewDecoder(response.Body).Decode(&input)
		closeErr = response.Body.Close()
		if decodeErr != nil || closeErr != nil || response.StatusCode != http.StatusOK || response.Header.Get("Cache-Control") != "no-store" || !input.Complete || len(input.Candidates) != 1 {
			t.Fatalf("complete ranking inputs: status %d, input %+v, decode %v, close %v", response.StatusCode, input, decodeErr, closeErr)
		}
	})

	if err := shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	gatewayClient.CloseIdleConnections()
	observerClient.CloseIdleConnections()
	process = startRankingProcess(t, binary)
	deadline = time.Now().Add(30 * time.Second)
	for {
		select {
		case <-process.done:
			t.Fatalf("rankingd exited during restart: %v", process.err)
		default:
		}
		response, err := healthClient.Get(healthBase + "/health/ready")
		if err == nil {
			response.Body.Close()
			if response.StatusCode == http.StatusOK {
				break
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("rankingd not ready after restart: %v", err)
		}
		time.Sleep(100 * time.Millisecond)
	}
	if code, body := get(gatewayClient, "/rankings/runs/"+retained.Data.Snapshot.SnapshotId+"/new"); code != http.StatusOK || body["runId"] != retained.Data.Snapshot.SnapshotId {
		t.Fatalf("retained snapshot after restart = %d %v", code, body)
	}
	if code, body := get(gatewayClient, "/rankings/new"); code != http.StatusOK {
		t.Fatalf("current snapshot after restart = %d %v", code, body)
	}
	if err := process.stop(); err != nil {
		t.Fatalf("shutdown restarted process: %v", err)
	}
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestNewFailsOnBadRedisURL(t *testing.T) {
	dsn, _ := internaltest.NewPostgresWithDSN(t)
	contractEnv(t)
	for k, v := range map[string]string{
		"DATABASE_URL":   dsn,
		"REDIS_URL":      "not-a-valid-redis-url",
		"REDIS_BULL_URL": "redis://localhost:6379",
		"RPC_URL":        "https://rpc.example.com/rpc",
		"ZONE":           "zone-a",
	} {
		t.Setenv(k, v)
	}
	// The shared base environment rejects a malformed REDIS_URL at startup
	// validation, before app.New could ever dial it.
	cfg, err := config.Load()
	if err != nil {
		if !strings.Contains(err.Error(), "REDIS_URL: url_invalid") {
			t.Fatalf("config.Load error = %v, want REDIS_URL: url_invalid", err)
		}
		return
	}
	if _, err := New(context.Background(), cfg, quietLogger()); err == nil {
		t.Fatal("app.New: expected error for invalid REDIS_URL")
	}
}
