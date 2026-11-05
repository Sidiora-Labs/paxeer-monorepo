package streams

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/livestream/internal/config"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

type repair57Session struct {
	Wallet    string `json:"wallet"`
	Expires   int64  `json:"expires"`
	Signature string `json:"signature"`
}

type repair57System struct {
	t               *testing.T
	dsn             string
	key             *ecdsa.PrivateKey
	wallet          string
	verifier        *sharedauth.WalletOperationVerifier
	store           *LifecycleStore
	database        *pgxpool.Pool
	server, private *httptest.Server
	transit         *VaultTransit
	secret          string
	bodyless        chan bool
	redis           *goredis.Client
	connectedConfig *config.Config
}

func repair57New(t *testing.T) *repair57System {
	t.Helper()
	dsn, redis := proofLivestreamStores(t)
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	f := &repair57System{t: t, dsn: dsn, redis: redis, key: key, wallet: strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex()), verifier: proofLivestreamChain(t), secret: strings.Repeat("s", 32), bodyless: make(chan bool, 8)}
	f.open()
	t.Cleanup(f.close)
	_, err = f.database.Exec(context.Background(), "CREATE SCHEMA livestream; CREATE SCHEMA chat; CREATE TABLE chat.safety_bans(wallet text,revoked_at timestamptz,expires_at timestamptz,scope text,resource text); CREATE VIEW chat.safety_bans_v1 AS SELECT * FROM chat.safety_bans; CREATE SCHEMA indexer; CREATE TABLE indexer.creations(pool text PRIMARY KEY,creator text); CREATE VIEW indexer.indexer_canonical_creations_v1 AS SELECT pool,creator FROM indexer.creations;"+f.store.SchemaSQL()+f.store.ConnectedSchemaSQL()+f.store.PlaybackPrivacySchemaSQL()+";"+sharedauth.LedgerDDL)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func (f *repair57System) close() {
	if f.server != nil {
		f.server.Close()
	}
	if f.private != nil {
		f.private.Close()
	}
	if f.database != nil {
		f.database.Close()
	}
}

func (f *repair57System) open() {
	f.t.Helper()
	ctx := context.Background()
	configuration, err := pgxpool.ParseConfig(f.dsn)
	if err != nil {
		f.t.Fatal(err)
	}
	configuration.ConnConfig.RuntimeParams["search_path"] = "livestream,pg_catalog"
	f.database, err = pgxpool.NewWithConfig(ctx, configuration)
	if err != nil {
		f.t.Fatal(err)
	}
	f.store, err = NewLifecycleStore(f.database, "livestream", 3, 20)
	if err != nil {
		f.t.Fatal(err)
	}
	policy := &RestrictionReader{Pool: f.database}
	authorize := func(ctx context.Context, r *http.Request, action string, o Operation) (Authorization, error) {
		var session repair57Session
		raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
		if err != nil || json.Unmarshal(raw, &session) != nil || session.Expires <= time.Now().Unix() {
			return Authorization{}, ErrDenied
		}
		if err = f.verifier.Verify(ctx, session.Wallet, repair57SessionStatement(session), session.Signature); err != nil {
			return Authorization{}, ErrDenied
		}
		var creator string
		if err = f.database.QueryRow(ctx, "SELECT creator FROM indexer.indexer_canonical_creations_v1 WHERE pool=$1", strings.ToLower(o.Pool)).Scan(&creator); err != nil {
			return Authorization{}, err
		}
		if creator != session.Wallet || o.Wallet != "" && o.Wallet != session.Wallet {
			return Authorization{}, ErrDenied
		}
		allowed, err := policy.Allowed(ctx, session.Wallet, o.Pool)
		if err != nil {
			return Authorization{}, err
		}
		return Authorization{Wallet: session.Wallet, Pool: o.Pool, Allowed: allowed, ExpiresAt: time.Unix(session.Expires, 0)}, nil
	}
	deps := Deps{StudioWebhook: true, Lifecycle: f.store, OperationVerifier: f.verifier, OperationLedger: &sharedauth.Ledger{Pool: f.database, Environment: "test", AuditKey: []byte(strings.Repeat("a", 32))}, Restrictions: policy, Authorize: authorize, WebhookSecret: f.secret, WebhookMilliseconds: true, DecodeWebhook: DecodeStudioWebhook}
	if f.transit != nil {
		deps.IngestKeys = &EncryptedCredentials{Store: f.store, Transit: f.transit, IngestEndpoint: "rtmps://rtmp.livepeer.studio/live"}
	}
	if f.connectedConfig != nil {
		runtime, err := NewConnectedRuntime(ctx, *f.connectedConfig, f.database, f.redis, f.verifier.Backend, DecodeStudioWebhook)
		if err != nil {
			f.t.Fatal(err)
		}
		deps = runtime.Deps
		f.store = deps.Lifecycle
	}
	router := chi.NewRouter()
	New(deps).RegisterRoutes(router)

	// Every HTTP exchange crosses the shared mTLS and signed-request verifier.
	// The edge uses an actual reverse proxy, certificate and assertion issuer.
	pki := testpki.New(f.t)
	upstream := pki.Leaf("livestream", "livestream.test")
	gateway := pki.Leaf("gateway", "gateway.test")
	assertion := testpki.Ed25519(f.t)
	transport := sharedauth.AssertionTransport{AssertionHeader: sharedauth.ActorAssertionHeader, SignatureHeader: sharedauth.ActorSignatureHeader}
	issuer := sharedauth.ServiceRequestIssuer{Issuer: "gateway", Audience: "livestream", Key: assertion.Private, Transport: transport}
	verifier := sharedauth.ServiceRequestVerifier{Issuer: "gateway", Audience: "livestream", Key: assertion.Private.Public().(ed25519.PublicKey), PeerCertSHA256: gateway.SHA256, Transport: transport}
	f.private = httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
		if err != nil || verifier.Authenticate(r, raw) != nil {
			http.Error(w, "untrusted ingress", http.StatusForbidden)
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		if strings.HasSuffix(r.URL.Path, "/ingest-credential") {
			select {
			case f.bodyless <- len(raw) == 0 && r.Header.Get("Content-Type") == "":
			default:
			}
		}
		router.ServeHTTP(w, r)
	}))
	f.private.TLS = &tls.Config{MinVersion: tls.VersionTLS13, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.Roots(), Certificates: []tls.Certificate{upstream.Pair}}
	f.private.StartTLS()
	target, err := url.Parse(f.private.URL)
	if err != nil {
		f.t.Fatal(err)
	}
	proxy := httputil.NewSingleHostReverseProxy(target)
	connection := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), Certificates: []tls.Certificate{gateway.Pair}}}
	f.t.Cleanup(connection.CloseIdleConnections)
	proxy.Transport = connection
	director := proxy.Director
	proxy.Director = func(r *http.Request) {
		director(r)
		var raw []byte
		var err error
		if r.Body != nil {
			raw, err = io.ReadAll(r.Body)
			if err != nil {
				panic(err)
			}
		}
		r.Body = io.NopCloser(bytes.NewReader(raw))
		id, err := randomID()
		if err != nil {
			panic(err)
		}
		r.Header.Set("X-Request-ID", id)
		if err = issuer.SignRequest(r, raw); err != nil {
			panic(err)
		}
	}
	f.server = httptest.NewServer(proxy)
}

func (f *repair57System) useConnectedRuntime() {
	f.t.Helper()
	pki := testpki.New(f.t)
	client := pki.Leaf("livestream-identity", "livestream.test")
	identity := pki.Leaf("identity", "identity.test")
	key := testpki.Ed25519(f.t)
	policy := BudgetPolicy{VendorUserID: "repair_vendor", DeliveryPricePerMinute: "1", TranscodePricePerMinute: "1", StoragePricePerMinute: "1", MonthlyUnits: "1000", BroadcastReserveUnits: "1", Currency: "USD", PeakDurationSeconds: 60, ApprovalReference: "repair_test", DeliveryPricePerTB: "1"}
	raw, err := json.Marshal(policy)
	if err != nil {
		f.t.Fatal(err)
	}
	path := filepath.Join(f.t.TempDir(), "budget.json")
	if err = os.WriteFile(path, raw, 0600); err != nil {
		f.t.Fatal(err)
	}
	f.connectedConfig = &config.Config{
		NodeEnv: "production", LogLevel: "info", Port: 3000, DatabaseURL: "postgres://proof@localhost/postgres", RedisURL: "redis://localhost:6379",
		LivepeerAPIURL: "https://livepeer.studio/api", LivepeerAPIKey: "uncontacted_provider", LivepeerWebhookSecret: f.secret, ProviderMode: "available",
		PlaybackWebhookID: "playback_hook", PlaybackWebhookSecret: strings.Repeat("p", 32), PlaybackWebhookURL: "https://livestream.test/webhooks/livepeer/playback-access", BudgetAlertURL: "https://budget.test/alert", BudgetAlertKey: strings.Repeat("b", 32),
		BudgetPolicyFile: path, BudgetPolicySHA256: fmt.Sprintf("%x", sha256.Sum256(raw)), WebhookTimestampUnit: "milliseconds", MaxStreamsPerWallet: 3, MaxGlobalStreams: 20,
		ActionEnvironment: "test", ActionAuditKey: strings.Repeat("a", 32), ViewerTokenKey: strings.Repeat("v", 32), VaultTransitURL: "https://vault.test", VaultTransitToken: strings.Repeat("t", 32), VaultTransitKey: "repair57", IngestEndpoint: "rtmps://rtmp.livepeer.studio/live",
		IdentityURL: "https://identity.test", IdentityCAFile: pki.CAFile, IdentityClientCertFile: client.CertFile, IdentityClientKeyFile: client.KeyFile, IdentityPeerCertSHA256: identity.SHA256, IdentitySigningKey: key.StdBase64,
	}
	f.close()
	f.open()
}

func repair57SessionStatement(s repair57Session) string {
	return "livestream repair owner session\n" + s.Wallet + "\n" + strconv.FormatInt(s.Expires, 10)
}

func (f *repair57System) session(key *ecdsa.PrivateKey) string {
	f.t.Helper()
	s := repair57Session{Wallet: strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex()), Expires: time.Now().Add(5 * time.Minute).Unix()}
	signature, err := crypto.Sign(accounts.TextHash([]byte(repair57SessionStatement(s))), key)
	if err != nil {
		f.t.Fatal(err)
	}
	s.Signature = hexutil.Encode(signature)
	raw, err := json.Marshal(s)
	if err != nil {
		f.t.Fatal(err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}

func (f *repair57System) create(idempotency string) (Operation, CreateIntent) {
	f.t.Helper()
	r := CreateIntent{Wallet: f.wallet, Pool: "0x2222222222222222222222222222222222222222", Title: "Repair contract stream", IdempotencyKey: idempotency}
	if _, err := f.database.Exec(context.Background(), "INSERT INTO indexer.creations(pool,creator) VALUES($1,$2) ON CONFLICT DO NOTHING", r.Pool, r.Wallet); err != nil {
		f.t.Fatal(err)
	}
	o, err := f.store.Create(context.Background(), r, CreatorDecision{Wallet: r.Wallet, Pool: r.Pool, Allowed: true, ExpiresAt: time.Now().Add(time.Minute)}, true)
	if err != nil {
		f.t.Fatal(err)
	}
	return o, r
}

func (f *repair57System) signed(path, action string, body []byte, key *ecdsa.PrivateKey) map[string]string {
	f.t.Helper()
	digest, err := sharedauth.BodyDigest(body)
	if err != nil {
		f.t.Fatal(err)
	}
	var nonce [32]byte
	if _, err = rand.Read(nonce[:]); err != nil {
		f.t.Fatal(err)
	}
	now := time.Now().Unix()
	e := sharedauth.OperationEnvelope{Version: 1, ChainID: "125", Environment: "test", Actor: strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex()), Action: action, Method: "POST", Path: path, BodyHash: digest, Nonce: hex.EncodeToString(nonce[:]), IssuedAt: now, ExpiresAt: now + 300}
	statement, err := e.Statement()
	if err != nil {
		f.t.Fatal(err)
	}
	sig, err := crypto.Sign(accounts.TextHash([]byte(statement)), key)
	if err != nil {
		f.t.Fatal(err)
	}
	raw, err := json.Marshal(e)
	if err != nil {
		f.t.Fatal(err)
	}
	return map[string]string{"Authorization": "Bearer " + f.session(key), "X-Kindle-Operation": base64.RawURLEncoding.EncodeToString(raw), "X-Kindle-Signature": hexutil.Encode(sig), "Content-Type": "application/json"}
}

func (f *repair57System) exchange(method, path string, body []byte, headers map[string]string, status int) []byte {
	f.t.Helper()
	req, err := http.NewRequest(method, f.server.URL+path, bytes.NewReader(body))
	if err != nil {
		f.t.Fatal(err)
	}
	for name, value := range headers {
		req.Header.Set(name, value)
	}
	response, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		f.t.Fatal(err)
	}
	if response.StatusCode != status || response.Header.Get("Cache-Control") != "no-store" || response.Header.Get("Content-Type") != "application/json" {
		f.t.Fatalf("%s %s: status=%d headers=%v body=%s", method, path, response.StatusCode, response.Header, raw)
	}
	if status >= 400 {
		if err = v2.ValidateJSON("Error", raw); err != nil {
			f.t.Fatal(err)
		}
	}
	return raw
}

type repair57SDK struct {
	t               *testing.T
	repo, directory string
}

func repair57Consumer(t *testing.T) repair57SDK {
	t.Helper()
	repo, err := filepath.Abs("../../../../..")
	if err != nil {
		t.Fatal(err)
	}
	consumer := repair57SDK{t: t, repo: repo, directory: t.TempDir()}
	consumer.run("prepare", nil)
	return consumer
}

func (s repair57SDK) run(mode string, value any) []byte {
	s.t.Helper()
	input, err := json.Marshal(value)
	if err != nil {
		s.t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "node", "--no-warnings", "testdata/repair57-sdk/consumer.mjs", mode, s.repo, s.directory)
	cmd.Stdin = bytes.NewReader(input)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	raw, err := cmd.Output()
	if err != nil {
		s.t.Fatalf("real SDK/browser consumer %s: %v: %s", mode, err, stderr.Bytes())
	}
	return raw
}

func (s repair57SDK) decode(schema string, raw []byte, dst any) {
	s.t.Helper()
	decoded := s.run("decode", map[string]any{"schema": schema, "wire": string(raw)})
	if err := json.Unmarshal(decoded, dst); err != nil {
		s.t.Fatal(err)
	}
	if err := v2.ValidateJSON(schema, raw); err != nil {
		s.t.Fatal(err)
	}
}

func TestRepairIntegration_57_002_CreateEnvelopeReplay(t *testing.T) {
	f := repair57New(t)
	consumer := repair57Consumer(t)
	operation, intent := f.create("repair57-create")
	body, err := json.Marshal(v2.StreamCreateBody{PoolAddress: intent.Pool, Title: intent.Title})
	if err != nil {
		t.Fatal(err)
	}
	headers := f.signed("/v2/streams", "stream.create", body, f.key)
	headers["Idempotency-Key"] = intent.IdempotencyKey
	first := f.exchange("POST", "/v2/streams", body, headers, http.StatusAccepted)
	var response v2.StreamCreateResponse
	consumer.decode("StreamCreateResponse", first, &response)
	if response.Data.Operation.OperationId != operation.ID || response.Data.Operation.Status != "requested" || response.Revision == nil || *response.Revision != response.Data.Operation.Revision || response.Watermark != nil || response.NextCursor != nil || response.Quality != "complete" {
		t.Fatalf("wrong create envelope: %s", first)
	}
	var stored []byte
	var requestID string
	if err = f.database.QueryRow(context.Background(), "SELECT response,request_id FROM operation_ledger WHERE action='stream.create'").Scan(&stored, &requestID); err != nil || !bytes.Equal(stored, first) || requestID != response.RequestId {
		t.Fatalf("response not frozen inside ledger: err=%v", err)
	}
	f.close()
	f.open()
	if _, err = f.store.BeginProvision(context.Background(), operation.ID); err != nil {
		t.Fatal(err)
	}
	replay := f.exchange("POST", "/v2/streams", body, headers, http.StatusAccepted)
	if !bytes.Equal(first, replay) {
		t.Fatal("restart replay changed original response bytes")
	}
	consumer.decode("StreamCreateResponse", replay, &response)
	var count int
	if err = f.database.QueryRow(context.Background(), "SELECT count(*) FROM livestream.stream_operations").Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate admission: %d %v", count, err)
	}
	changed := bytes.Replace(body, []byte(intent.Title), []byte("Tampered stream title"), 1)
	f.exchange("POST", "/v2/streams", changed, headers, http.StatusServiceUnavailable)
}

func TestRepairIntegration_57_003_OperationPollingRecovery(t *testing.T) {
	f := repair57New(t)
	consumer := repair57Consumer(t)
	o, _ := f.create("repair57-poll")
	path := "/v2/streams/operations/" + o.ID
	headers := map[string]string{"Authorization": "Bearer " + f.session(f.key)}
	first := f.exchange("GET", path, nil, headers, http.StatusOK)
	var response v2.StreamOperationGetResponse
	consumer.decode("StreamOperationGetResponse", first, &response)
	if response.Data.Operation.OperationId != o.ID || response.Data.Operation.Status != "requested" {
		t.Fatal(response)
	}
	firstID := response.RequestId
	if _, err := f.store.BeginProvision(context.Background(), o.ID); err != nil {
		t.Fatal(err)
	}
	f.close()
	f.open()
	o, err := f.store.BindProvider(context.Background(), o.ID, "recovered_provider")
	if err != nil {
		t.Fatal(err)
	}
	recovered := f.exchange("GET", path, nil, headers, http.StatusOK)
	consumer.decode("StreamOperationGetResponse", recovered, &response)
	if response.RequestId == firstID || response.Data.Operation.OperationId != o.ID || response.Data.Operation.Status != "ready" || response.Revision == nil || string(*response.Revision) != strconv.FormatInt(o.Revision, 10) || *response.Revision != response.Data.Operation.Revision {
		t.Fatalf("poll lost recovered provider binding: %s", recovered)
	}
	f.exchange("GET", path, nil, nil, http.StatusForbidden)
	other, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	f.exchange("GET", path, nil, map[string]string{"Authorization": "Bearer " + f.session(other)}, http.StatusForbidden)
	if _, err = f.database.Exec(context.Background(), "INSERT INTO chat.safety_bans(wallet,scope)VALUES($1,'global')", f.wallet); err != nil {
		t.Fatal(err)
	}
	f.exchange("GET", path, nil, headers, http.StatusForbidden)
}

func TestRepairIntegration_57_004_BodylessRetirement(t *testing.T) {
	f := repair57New(t)
	consumer := repair57Consumer(t)
	o, _ := f.create("repair57-retired")
	f.useConnectedRuntime()
	raw := consumer.run("retired", map[string]string{"origin": f.server.URL, "id": o.ID, "session": f.session(f.key)})
	var response v2.Error
	if err := json.Unmarshal(raw, &response); err != nil || response.Code != "INGEST_CREDENTIAL_RETIRED" || response.Retryable {
		t.Fatal(response, err)
	}
	select {
	case correct := <-f.bodyless:
		if !correct {
			t.Fatal("SDK request acquired an undocumented JSON body or content type")
		}
	case <-time.After(time.Second):
		t.Fatal("SDK request did not cross trusted ingress")
	}
	var count int
	if err := f.database.QueryRow(context.Background(), "SELECT count(*) FROM operation_ledger").Scan(&count); err != nil || count != 0 {
		t.Fatalf("retired request consumed signed authorization: %d %v", count, err)
	}
}

func repair57Vault(t *testing.T) *VaultTransit {
	t.Helper()
	if _, err := exec.LookPath("vault"); err != nil {
		t.Fatal("real Vault executable required for the encrypted-key path", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	address := listener.Addr().String()
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	var entropy [32]byte
	if _, err = rand.Read(entropy[:]); err != nil {
		t.Fatal(err)
	}
	token := base64.RawURLEncoding.EncodeToString(entropy[:])
	proofLivestreamProcess(t, nil, "vault", "server", "-dev", "-dev-tls", "-dev-no-store-token", "-dev-tls-cert-dir="+dir, "-dev-listen-address="+address, "-dev-root-token-id="+token)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var client *http.Client
	for {
		pem, err := os.ReadFile(filepath.Join(dir, "vault-ca.pem"))
		if err == nil {
			roots := x509.NewCertPool()
			if roots.AppendCertsFromPEM(pem) {
				client = &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: roots}}}
			}
		}
		if client != nil {
			request, err := http.NewRequestWithContext(ctx, "GET", "https://"+address+"/v1/sys/health", nil)
			if err != nil {
				t.Fatal(err)
			}
			response, err := client.Do(request)
			if err == nil {
				response.Body.Close()
				if response.StatusCode == 200 {
					break
				}
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal("real TLS Vault did not become ready", ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
	t.Cleanup(client.CloseIdleConnections)
	for _, entry := range []struct{ path, payload string }{{"/v1/sys/mounts/transit", `{"type":"transit"}`}, {"/v1/transit/keys/repair57", `{"type":"aes256-gcm96"}`}} {
		request, err := http.NewRequestWithContext(ctx, "POST", "https://"+address+entry.path, strings.NewReader(entry.payload))
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("X-Vault-Token", token)
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.Copy(io.Discard, response.Body)
		response.Body.Close()
		if response.StatusCode != 200 && response.StatusCode != 204 {
			t.Fatalf("Vault %s: status=%d", entry.path, response.StatusCode)
		}
	}
	return &VaultTransit{Endpoint: "https://" + address, Token: token, Key: "repair57", Client: client}
}

func TestRepairIntegration_57_001_PrivateIngestKeyRecovery(t *testing.T) {
	f := repair57New(t)
	consumer := repair57Consumer(t)
	f.transit = repair57Vault(t)
	o, _ := f.create("repair57-private-key")
	o, err := f.store.BeginProvision(context.Background(), o.ID)
	if err != nil {
		t.Fatal(err)
	}
	var entropy [32]byte
	if _, err = rand.Read(entropy[:]); err != nil {
		t.Fatal(err)
	}
	key := base64.RawURLEncoding.EncodeToString(entropy[:])
	ciphertext, err := f.transit.Seal(context.Background(), o.ID, "private_provider", key)
	if err != nil {
		t.Fatal(err)
	}
	o, err = f.store.BindEncryptedProvider(context.Background(), o.ID, "private_provider", ciphertext, f.transit.Key)
	if err != nil {
		t.Fatal(err)
	}
	f.close()
	f.open()
	signer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		message, err := io.ReadAll(io.LimitReader(r.Body, 8193))
		if err != nil || len(message) > 8192 {
			http.Error(w, "invalid signing request", 400)
			return
		}
		signature, err := crypto.Sign(accounts.TextHash(message), f.key)
		if err != nil {
			http.Error(w, "signing failed", 500)
			return
		}
		_, _ = io.WriteString(w, hexutil.Encode(signature))
	}))
	defer signer.Close()
	consumer.run("browser-key", map[string]string{"origin": f.server.URL, "signer": signer.URL, "id": o.ID, "wallet": f.wallet, "session": f.session(f.key), "expectedKey": key, "expectedEndpoint": "rtmps://rtmp.livepeer.studio/live"})
	path := "/v2/streams/" + o.ID + "/ingest-key"
	body := []byte(`{}`)
	f.exchange("POST", path, body, map[string]string{"Content-Type": "application/json"}, http.StatusForbidden)
	other, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	f.exchange("POST", path, body, f.signed(path, "stream.credential", body, other), http.StatusForbidden)
	bound := o
	bound.ProviderID = "other_provider"
	keys := &EncryptedCredentials{Store: f.store, Transit: f.transit, IngestEndpoint: "rtmps://rtmp.livepeer.studio/live"}
	if _, err = keys.LoadKey(context.Background(), bound); !errors.Is(err, ErrUnavailable) {
		t.Fatal("provider substitution disclosed a key", err)
	}
	if _, err = f.transit.Open(context.Background(), o.ID, "other_provider", ciphertext); err == nil {
		t.Fatal("Vault accepted substituted provider binding")
	}
	var stored string
	if err = f.database.QueryRow(context.Background(), "SELECT ciphertext FROM livestream.stream_credentials WHERE operation_id=$1", o.ID).Scan(&stored); err != nil || stored == key || strings.Contains(stored, key) {
		t.Fatal("plaintext retained in credential row", err)
	}
	if _, err = f.store.Transition(context.Background(), o.ID, f.wallet, "end", o.Revision, ActionDecision{Wallet: f.wallet, OperationID: o.ID, Action: "end", Revision: o.Revision, Allowed: true, ExpiresAt: time.Now().Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	f.exchange("POST", path, body, f.signed(path, "stream.credential", body, f.key), http.StatusConflict)
}

func TestRepairIntegration_57_005_NormalizedStudioWebhookRecovery(t *testing.T) {
	f := repair57New(t)
	consumer := repair57Consumer(t)
	o, _ := f.create("repair57-webhook")
	o, err := f.store.BindProvider(context.Background(), o.ID, "studio_provider")
	if err != nil {
		t.Fatal(err)
	}
	f.useConnectedRuntime()
	now := time.Now().UTC().Truncate(time.Millisecond)
	studio := func(id, event string, eventAt, deliveredAt time.Time) ([]byte, string) {
		raw, err := json.Marshal(struct {
			ID        string `json:"id"`
			WebhookID string `json:"webhookId"`
			CreatedAt int64  `json:"createdAt"`
			Timestamp int64  `json:"timestamp"`
			Event     string `json:"event"`
			Stream    struct {
				ID string `json:"id"`
			} `json:"stream"`
		}{ID: id, WebhookID: "studio_callback", CreatedAt: eventAt.UnixMilli(), Timestamp: deliveredAt.UnixMilli(), Event: event, Stream: struct {
			ID string `json:"id"`
		}{ID: o.ProviderID}})
		if err != nil {
			t.Fatal(err)
		}
		mac := hmac.New(sha256.New, []byte(f.secret))
		_, _ = mac.Write(raw)
		return raw, fmt.Sprintf("t=%d,v1=%s", deliveredAt.UnixMilli(), hex.EncodeToString(mac.Sum(nil)))
	}
	normalize := func(raw []byte, signature string, deliveredAt time.Time, accepted bool) NormalizedProviderWebhook {
		req := httptest.NewRequest("POST", "/webhooks/livepeer", bytes.NewReader(raw))
		req.Header.Set("Livepeer-Signature", signature)
		recorder := httptest.NewRecorder()
		normalized, ok := NormalizeStudioWebhook(recorder, req, f.secret, deliveredAt)
		if ok != accepted {
			t.Fatalf("normalizer accepted=%t want=%t status=%d", ok, accepted, recorder.Code)
		}
		return normalized
	}
	raw, signature := studio("studio_live_event", "stream.started", now.Add(-time.Minute), now)
	// Both public wire shapes must authenticate the same producer bytes. The raw
	// Studio handler applies first; the normalized frozen delivery deduplicates it.
	var delivered v2.LiveWebhookResponse
	consumer.decode("LiveWebhookResponse", f.exchange("POST", "/webhooks/livepeer", raw, map[string]string{"Content-Type": "application/json", "Livepeer-Signature": signature}, http.StatusOK), &delivered)
	if !delivered.Data.Accepted {
		t.Fatal("connected raw callback did not reach normalized application")
	}
	wire := normalize(raw, signature, now, true)
	var event v2.LiveWebhookBody
	consumer.decode("LiveWebhookBody", wire.Body, &event)
	if event.Event != "active" || event.StreamId != o.ProviderID || event.EventId != "studio_live_event" || event.OccurredAt != now.Add(-time.Minute).Format(v2LifecycleTimeLayout) {
		t.Fatal(event)
	}
	headers := map[string]string{"Content-Type": "application/json", NormalizedWebhookTimestampHeader: wire.Timestamp, NormalizedWebhookSignatureHeader: wire.Signature}
	path := "/v2/provider-webhooks/livepeer"
	f.exchange("POST", path, raw, map[string]string{"Content-Type": "application/json", "Livepeer-Signature": signature}, http.StatusUnauthorized)
	tampered := bytes.Replace(wire.Body, []byte(`"active"`), []byte(`"inactive"`), 1)
	f.exchange("POST", path, tampered, headers, http.StatusUnauthorized)
	for _, skew := range []time.Duration{-301 * time.Second, 301 * time.Second} {
		// Anchor each signed delivery immediately before HTTP, independently of
		// time spent compiling or running the SDK consumer. Future timestamps
		// round up so truncation cannot pull the 301-second case into the window.
		at := time.Now().UTC().Truncate(time.Second).Add(skew)
		if skew > 0 {
			at = at.Add(time.Second)
		}
		expiredRaw, expiredSignature := studio("expired", "stream.started", at.Add(-time.Second), at)
		expiredWire := normalize(expiredRaw, expiredSignature, at, true)
		f.exchange("POST", path, expiredWire.Body, map[string]string{"Content-Type": "application/json", NormalizedWebhookTimestampHeader: expiredWire.Timestamp, NormalizedWebhookSignatureHeader: expiredWire.Signature}, http.StatusUnauthorized)
	}
	normalize(bytes.Replace(raw, []byte("stream.started"), []byte("stream.idle"), 1), signature, now, false)
	normalize(raw, signature, now.Add(301*time.Second), false)
	for i := 0; i < 2; i++ {
		var response v2.LiveWebhookResponse
		consumer.decode("LiveWebhookResponse", f.exchange("POST", path, wire.Body, headers, http.StatusOK), &response)
		if !response.Data.Accepted || response.Quality != "complete" {
			t.Fatal(response)
		}
		if i == 0 {
			f.close()
			f.open()
		}
	}
	current, err := f.store.Get(context.Background(), o.ID)
	if err != nil || current.State != "LIVE" || current.Revision != o.Revision+1 || current.ProviderTime == nil || !current.ProviderTime.Equal(now.Add(-time.Minute)) {
		t.Fatalf("durable callback binding lost: %+v %v", current, err)
	}
	var count int
	if err = f.database.QueryRow(context.Background(), "SELECT count(*) FROM livestream.stream_inbox").Scan(&count); err != nil || count != 1 {
		t.Fatalf("callback was not applied once: count=%d err=%v", count, err)
	}
	raw, signature = studio("studio_idle_event", "stream.idle", now, now)
	wire = normalize(raw, signature, now, true)
	consumer.decode("LiveWebhookBody", wire.Body, &event)
	if event.Event != "inactive" {
		t.Fatal(event)
	}
	headers[NormalizedWebhookTimestampHeader], headers[NormalizedWebhookSignatureHeader] = wire.Timestamp, wire.Signature
	var response v2.LiveWebhookResponse
	consumer.decode("LiveWebhookResponse", f.exchange("POST", path, wire.Body, headers, http.StatusOK), &response)
	current, err = f.store.Get(context.Background(), o.ID)
	if err != nil || current.State != "ENDED" || current.Revision != o.Revision+2 {
		t.Fatalf("normalized idle event not applied: %+v %v", current, err)
	}
}
