// Proves: LivestreamConnectedRoutes/SocialConnectedRoutes and streams.Handlers/social.RegisterSafetyRoutes -> Connected.ServeHTTP over HTTP and mTLS -> v2 decoders and livestream-api.getStream/parseStreamOperation.
//
// Repairs connected.go dropping X-Kindle-Signature, the web lifecycle client's
// legacy action paths/response decoding, the SDK's missing end revision body,
// and the measurement consumer accepting malformed heartbeat responses.
// Operation vectors now declare the producer's real signature header.
// Repairs livestreamd/main.go discarding the startup error; the first refusal
// is "exactly one of LIVESTREAM_TLS_CERT_FILE and LIVESTREAM_TLS_CERT_PEM is required".
// The proof supplies one TLS material source and a digest-bound schema file.
// The catalog proof includes the frozen binding routes accepted by
// allowedConnectedRoute as well as the routes appended by the application.
//
// The gateway owns no persistent state. Consecutive reads change the upstream
// row and must observe the new revision, with no cached shortcut. Malformed
// copies of real producer responses are rejected at the consumer decoder.
//
// The proof compiles sociald and livestreamd from their owning modules and
// owns loopback PostgreSQL and Redis processes. The real migration coordinator
// primitives provision the reviewed schema and least-privilege runtime roles.
// No endpoint or service executable is supplied by the operator.
// services/launchpad/social/internal/app/app.go's platform voice.Control has no
// provider: tickets/moderation must retain DEPENDENCY_UNAVAILABLE.
// services/launchpad/social/internal/social/voice_v2_wire.go writeV2VoiceError
// refuses malformed moderation/ticket input with HTTP 400 and INVALID_REQUEST;
// apps/launchpad/web/src/core/user-facing-error.ts's invalidCodes set classifies
// that code as invalid input through copyKeyForCode.
// services/launchpad/livestream/internal/streams/public_reads.go NewPublicReads
// has no broadcast provider: lifecycle writes and operation status must retain
// BROADCAST_NOT_PROVISIONED, including when input is malformed.
package proxy

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/devidentity"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/internaltest"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	goredis "github.com/redis/go-redis/v9"
)

const e2eOrigin = "https://kindle.test"

type e2ePeer struct {
	issuer string
	leaf   testpki.Leaf
	key    testpki.AssertionKey
}

type identityE2E struct {
	t                   *testing.T
	pki                 *testpki.PKI
	gatewayServer       testpki.Leaf
	gatewayClient       testpki.Leaf
	gatewayKey          testpki.AssertionKey
	peers               map[string]e2ePeer
	public              *httptest.Server
	private             *httptest.Server
	identityURL         string
	serviceToken        string
	adminDSN            string
	redisURL            string
	database            proofDatabase
	identity            *devidentity.Runtime
	authorityForGateway sharedauth.MTLSIdentityAuthority
}

// throwawayDatabase creates a database on the fixture server and drops it at
// cleanup.
func (e *identityE2E) throwawayDatabase(prefix string) string {
	t := e.t
	t.Helper()
	ctx := context.Background()
	admin, err := pgx.Connect(ctx, e.adminDSN)
	if err != nil {
		t.Fatalf("fixture postgres: %v", err)
	}
	defer admin.Close(ctx)
	name := prefix + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	if _, err = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		c, err := pgx.Connect(context.Background(), e.adminDSN)
		if err != nil {
			t.Error(err)
			return
		}
		defer c.Close(context.Background())
		if _, err = c.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
	})
	endpoint, err := url.Parse(e.adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	endpoint.Path = "/" + name
	return endpoint.String()
}

// e2eChain is a chain-125 go-ethereum node with HTTP JSON-RPC, so the
// identity's wallet-proof verifier runs its real RPC calls.
func e2eChain(t *testing.T) string {
	t.Helper()
	probe, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := probe.Addr().(*net.TCPAddr).Port
	_ = probe.Close()
	chainConfig := *params.AllDevChainProtocolChanges
	chainConfig.ChainID = big.NewInt(125)
	backend := simulated.NewBackend(types.GenesisAlloc{}, func(nodeConf *node.Config, ethConf *ethconfig.Config) {
		nodeConf.HTTPHost = "127.0.0.1"
		nodeConf.HTTPPort = port
		nodeConf.HTTPModules = []string{"eth", "net", "web3"}
		ethConf.Genesis.Config = &chainConfig
		ethConf.NetworkId = 125
	})
	t.Cleanup(func() { _ = backend.Close() })
	backend.Commit()
	rpc := fmt.Sprintf("http://127.0.0.1:%d", port)
	client, err := ethclient.Dial(rpc)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if id, err := client.ChainID(context.Background()); err != nil || id.Cmp(big.NewInt(125)) != 0 {
		t.Fatalf("chain-125 node: %v %v", id, err)
	}
	return rpc
}

func newIdentityE2E(t *testing.T) *identityE2E {
	t.Helper()
	e := &identityE2E{t: t, peers: map[string]e2ePeer{}}
	e.adminDSN, e.redisURL = proofStores(t)
	e.database = proofBootstrap(t, e.adminDSN, "chat_runtime", "livestream_runtime")
	e.pki = testpki.New(t)
	e.gatewayServer = e.pki.Leaf("gateway-server", "gateway.railway.internal")
	e.gatewayClient = e.pki.Leaf("gateway-client", "gateway.railway.internal")
	e.gatewayKey = testpki.Ed25519(t)
	identityTransport := sharedauth.AssertionTransport{AssertionHeader: sharedauth.IdentityAssertionHeader, SignatureHeader: sharedauth.IdentitySignatureHeader}
	verifiers := []sharedauth.ServiceRequestVerifier{{Transport: identityTransport, Issuer: "gateway", Audience: "identity", PeerCertSHA256: e.gatewayClient.SHA256, Key: e.gatewayKey.Private.Public().(ed25519.PublicKey)}}
	for _, issuer := range []string{"social", "livestream"} {
		peer := e2ePeer{issuer: issuer, leaf: e.pki.Leaf(issuer+"-client", issuer+".railway.internal"), key: testpki.Ed25519(t)}
		e.peers[issuer] = peer
		verifiers = append(verifiers, sharedauth.ServiceRequestVerifier{Transport: identityTransport, Issuer: issuer, Audience: "identity", PeerCertSHA256: peer.leaf.SHA256, Key: peer.key.Private.Public().(ed25519.PublicKey)})
	}
	e.peers["ranking"] = e2ePeer{leaf: e.pki.Leaf("ranking-client", "ranking.railway.internal")}

	secret := strings.Repeat("a1", 32)
	t.Setenv(devidentity.SigningSecretReference, secret)
	e.serviceToken = strings.Repeat("s7", 32)
	identity, err := devidentity.Open(context.Background(), devidentity.Options{DatabaseURL: e.throwawayDatabase("gw_identity_e2e_"), RPCURL: e2eChain(t), PublicURL: e2eOrigin, CORSAllowedOrigins: e2eOrigin, SigningSecret: secret, LimiterKey: strings.Repeat("l", 32), ServiceToken: e.serviceToken})
	if err != nil {
		t.Fatalf("identity: %v", err)
	}
	e.identity = identity
	t.Cleanup(identity.Close)
	mux := chi.NewRouter()
	// The production registration: the verifiers are configured, so the
	// IDENTITY_SERVICE_TOKEN bearer is never accepted by the internal routes.
	if err = identity.Identity.RegisterV2Routes(mux, auth.IdentityHTTPConfig{ServiceToken: e.serviceToken, ServiceVerifiers: verifiers}); err != nil {
		t.Fatal(err)
	}
	e.public = httptest.NewServer(mux)
	t.Cleanup(e.public.Close)
	e.private = httptest.NewUnstartedServer(mux)
	e.private.TLS = &tls.Config{MinVersion: tls.VersionTLS13, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: e.pki.Roots(), Certificates: []tls.Certificate{e.gatewayServer.Pair}}
	e.private.StartTLS()
	t.Cleanup(e.private.Close)
	e.identityURL = e.private.URL

	// The gateway's own identity client (issuer "gateway"), as
	// newGatewayConnectedRuntime builds it.
	transport := &http.Transport{TLSClientConfig: e.gatewayClientTLS(), Proxy: nil, MaxIdleConns: 64, MaxIdleConnsPerHost: 32, IdleConnTimeout: 30 * time.Second, ResponseHeaderTimeout: 2 * time.Second}
	e.authorityForGateway, err = sharedauth.NewMTLSIdentityAuthority(e.identityURL, e.gatewayServer.SHA256, &http.Client{Transport: transport, Timeout: 2 * time.Second}, sharedauth.ServiceRequestIssuer{Issuer: "gateway", Audience: "identity", Key: e.gatewayKey.Private, Transport: sharedauth.AssertionTransport{AssertionHeader: sharedauth.IdentityAssertionHeader, SignatureHeader: sharedauth.IdentitySignatureHeader}})
	if err != nil {
		t.Fatal(err)
	}
	return e
}

func (e *identityE2E) gatewayClientTLS() *tls.Config {
	return &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: e.pki.Roots(), Certificates: []tls.Certificate{e.gatewayClient.Pair}}
}

// peerClient is the shared identity client a platform peer builds from its
// IDENTITY_MTLS_* / IDENTITY_ASSERTION_SIGNING_KEY environment.
func (e *identityE2E) peerClient(issuer string, leaf testpki.Leaf, key testpki.AssertionKey) (sharedauth.MTLSIdentityAuthority, error) {
	return sharedauth.NewMTLSIdentityClient(sharedauth.MTLSIdentityClientConfig{Endpoint: e.identityURL, PeerCertSHA256: e.gatewayServer.SHA256, CAFile: e.pki.CAFile, ClientCertFile: leaf.CertFile, ClientKeyFile: leaf.KeyFile, Issuer: issuer, SigningKey: key.StdBase64})
}

type e2eSession struct {
	wallet, token, csrf, cookie string
	key                         *ecdsa.PrivateKey
}

func e2eSend(t *testing.T, client *http.Client, method, url string, body any, headers map[string]string) (*http.Response, []byte) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, reader)
	if err != nil {
		t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return resp, raw
}

// signIn runs the browser sign-in (challenge, personal_sign, session) on the
// public identity surface.
func (e *identityE2E) signIn() e2eSession {
	t := e.t
	t.Helper()
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	wallet := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	resp, raw := e2eSend(t, nil, http.MethodPost, e.public.URL+"/v2/auth/challenges", map[string]string{"wallet": wallet, "origin": e2eOrigin}, map[string]string{"Origin": e2eOrigin, "Idempotency-Key": uuid.NewString()})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("challenge %d: %s", resp.StatusCode, raw)
	}
	var challenge struct {
		Data struct {
			Challenge struct{ ChallengeID, Message string }
		}
	}
	if err = json.Unmarshal(raw, &challenge); err != nil {
		t.Fatal(err)
	}
	sig, err := crypto.Sign(accounts.TextHash([]byte(challenge.Data.Challenge.Message)), key)
	if err != nil {
		t.Fatal(err)
	}
	sig[64] += 27
	resp, raw = e2eSend(t, nil, http.MethodPost, e.public.URL+"/v2/auth/sessions", map[string]any{"challengeId": challenge.Data.Challenge.ChallengeID, "proof": map[string]string{"kind": "eoa_eip191", "wallet": wallet, "signature": "0x" + hex.EncodeToString(sig)}, "deviceLabel": "Web browser"}, map[string]string{"Origin": e2eOrigin})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("session %d: %s", resp.StatusCode, raw)
	}
	var login struct {
		Data struct{ AccessToken string }
	}
	if err = json.Unmarshal(raw, &login); err != nil || login.Data.AccessToken == "" {
		t.Fatalf("login %s", raw)
	}
	s := e2eSession{key: key, wallet: wallet, token: login.Data.AccessToken, csrf: resp.Header.Get("X-CSRF-Token")}
	for _, c := range resp.Cookies() {
		if c.Name == "__Host-kindle_access" {
			s.cookie = c.Value
		}
	}
	if len(s.csrf) != 64 || s.cookie == "" {
		t.Fatalf("session cookie/csrf missing: %v", resp.Header)
	}
	return s
}

// prepareSocialAccount supplies the users-owned read interfaces social needs.
// Account and verified-wallet rows are copied from the real identity producer
// after browser sign-in; neither authority identifiers nor proofs are invented.
func (e *identityE2E) prepareSocialAccount(wallet string) {
	t := e.t
	t.Helper()
	ctx := context.Background()
	var account, link []byte
	if err := e.identity.Pool.QueryRow(ctx, `SELECT to_jsonb(a),to_jsonb(w) FROM identity_accounts a JOIN identity_wallet_links w USING(account_id) WHERE w.wallet=$1 AND w.chain_id=125 AND w.state='verified'`, wallet).Scan(&account, &link); err != nil {
		t.Fatalf("services/launchpad/gateway/internal/devidentity signed-in account: %v", err)
	}
	db, err := pgx.Connect(ctx, e.database.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	tx, err := db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, row := range []struct {
		table string
		value []byte
	}{{"identity_accounts", account}, {"identity_wallet_links", link}} {
		table := pgx.Identifier{"users", row.table}.Sanitize()
		result, err := tx.Exec(ctx, "INSERT INTO "+table+" SELECT * FROM jsonb_populate_record(NULL::"+table+",$1::jsonb)", string(row.value))
		if err != nil || result.RowsAffected() != 1 {
			t.Fatalf("users-owned identity fixture %s: %v (%d rows)", row.table, err, result.RowsAffected())
		}
	}
	result, err := tx.Exec(ctx, `INSERT INTO users.account_state(account_id,wallet,profile,preferences) SELECT account_id,wallet,'{}','{}' FROM users.identity_wallet_links WHERE wallet=$1 AND chain_id=125 AND state='verified'`, wallet)
	if err != nil || result.RowsAffected() != 1 {
		t.Fatalf("users-owned account state fixture: %v (%d rows)", err, result.RowsAffected())
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

// proofStores owns real, disposable storage servers on loopback. Nothing is
// borrowed from a developer's database or Redis configuration.
func proofStores(t *testing.T) (string, string) {
	t.Helper()
	bindir, err := exec.Command("pg_config", "--bindir").Output()
	if err != nil {
		t.Fatalf("PostgreSQL pg_config: %v", err)
	}
	data, err := os.MkdirTemp("", "launchpad-gateway-postgres-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(data); err != nil {
			t.Error(err)
		}
	})
	var credential *syscall.Credential
	if os.Geteuid() == 0 {
		account, err := user.Lookup("postgres")
		if err != nil {
			t.Fatalf("PostgreSQL operating-system account: %v", err)
		}
		uid, err := strconv.ParseUint(account.Uid, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		gid, err := strconv.ParseUint(account.Gid, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
		if err = os.Chown(data, int(uid), int(gid)); err != nil {
			t.Fatal(err)
		}
	}
	pgCommand := func(name string, args ...string) *exec.Cmd {
		cmd := exec.Command(filepath.Join(strings.TrimSpace(string(bindir)), name), args...)
		cmd.Env = []string{"LANG=C", "LC_ALL=C"}
		if credential != nil {
			cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
		}
		return cmd
	}
	if out, err := pgCommand("initdb", "-D", data, "-U", "postgres", "-A", "trust", "--no-locale", "--encoding=UTF8").CombinedOutput(); err != nil {
		t.Fatalf("PostgreSQL initdb: %v\n%s", err, out)
	}
	start := func(cmd *exec.Cmd) {
		t.Helper()
		output := &lockedBuffer{}
		cmd.Stdout, cmd.Stderr = output, output
		if err := cmd.Start(); err != nil {
			t.Fatalf("start %s: %v", cmd.Path, err)
		}
		done := make(chan struct{})
		go func() { _ = cmd.Wait(); close(done) }()
		t.Cleanup(func() {
			_ = cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				_ = cmd.Process.Kill()
				<-done
			}
			if t.Failed() {
				t.Logf("%s:\n%s", cmd.Path, output.String())
			}
		})
	}
	ports := internaltest.FreePorts(t, 2)
	pgPort, redisPort := ports[0], ports[1]
	start(pgCommand("postgres", "-D", data, "-h", "127.0.0.1", "-p", strconv.Itoa(pgPort), "-k", data, "-c", "max_connections=40"))
	redisPassword := strings.ReplaceAll(uuid.NewString(), "-", "")
	redis := exec.Command("redis-server", "--bind", "127.0.0.1", "--port", strconv.Itoa(redisPort), "--save", "", "--appendonly", "no", "--requirepass", redisPassword)
	redis.Env = []string{"LANG=C"}
	start(redis)
	dsn := fmt.Sprintf("postgres://postgres@127.0.0.1:%d/postgres?sslmode=disable", pgPort)
	redisURL := fmt.Sprintf("redis://:%s@127.0.0.1:%d/0", redisPassword, redisPort)
	client := goredis.NewClient(&goredis.Options{Addr: fmt.Sprintf("127.0.0.1:%d", redisPort), Password: redisPassword})
	defer client.Close()
	deadline := time.Now().Add(15 * time.Second)
	for {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		conn, pgErr := pgx.Connect(ctx, dsn)
		if conn != nil {
			_ = conn.Close(ctx)
		}
		redisErr := client.Ping(ctx).Err()
		cancel()
		if pgErr == nil && redisErr == nil {
			return dsn, redisURL
		}
		if time.Now().After(deadline) {
			t.Fatalf("owned storage startup: PostgreSQL=%v Redis=%v", pgErr, redisErr)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// lockedBuffer collects a subprocess's output.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}
func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

type socialProcess struct {
	plainURL, privateURL string
	server, ranking      testpki.Leaf
	pool, creator        string
	restart              func()
}

// startSocial builds and starts services/launchpad/social sociald and runs its production platform
// runtime against the identity listener.
func (e *identityE2E) startSocial() socialProcess {
	t := e.t
	t.Helper()
	binary := buildService(t, "social", "sociald")
	dsn := e.database.AdminDSN
	ctx := context.Background()
	db, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close(ctx)
	sp := socialProcess{server: e.pki.Leaf("social-server", "social.railway.internal"), ranking: e.peers["ranking"].leaf, pool: "0x" + strings.Repeat("9a", 20), creator: "0x" + strings.Repeat("c0", 20)}
	if _, err = db.Exec(ctx, `INSERT INTO indexer.pools VALUES($1,$2,$3,$4,$5,1,$6,100,$7)`, sp.pool, "0x"+strings.Repeat("70", 20), sp.creator, "0x"+strings.Repeat("0b", 20), "0x"+strings.Repeat("ab", 32), time.Now().Unix(), "0x"+strings.Repeat("cd", 32)); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) SELECT token_address,pool_address,creator,$1,optical,nft_id,created_block,$2,tx_hash,0,0,created_at,'{}','{}' FROM indexer.pools WHERE pool_address=$3`, "0x"+strings.Repeat("f0", 20), "0x"+strings.Repeat("de", 32), sp.pool); err != nil {
		t.Fatal(err)
	}
	cursor := make([]byte, 32)
	_, _ = rand.Read(cursor)
	ports := internaltest.FreePorts(t, 2)
	plainPort, privatePort := ports[0], ports[1]
	social := e.peers["social"]
	schemaFile, schemaDigest := e.database.ExpectedSchemaFile(t, "chat_runtime")
	env := []string{
		"NODE_ENV=production", "RUNTIME_PROFILE=platform", "ZONE=zone-a", "METRICS_ENVIRONMENT=production", "LOG_LEVEL=error",
		"SECRETS_PROVIDER=env", "SECRETS_REFRESH_SECONDS=30", "DB_MAX_CONNS=5",
		fmt.Sprintf("PORT=%d", plainPort), "BIND_ADDRESS=0.0.0.0",
		"DATABASE_URL=" + e.database.Runtimes["chat_runtime"].DSN,
		"EXPECTED_SCHEMA_FILE=" + schemaFile,
		"EXPECTED_SCHEMA_SHA256=" + schemaDigest,
		"REDIS_URL=" + e.redisURL,
		"SOCIAL_CURSOR_KEY=" + base64.RawURLEncoding.EncodeToString(cursor),
		"CORS_ALLOWED_ORIGINS=" + e2eOrigin,
		"IDENTITY_SERVICE_URL=" + e.identityURL + "/",
		"IDENTITY_MTLS_CA_FILE=" + e.pki.CAFile,
		"IDENTITY_MTLS_CLIENT_CERT_FILE=" + social.leaf.CertFile,
		"IDENTITY_MTLS_CLIENT_KEY_PEM=" + string(social.leaf.KeyPEM),
		"IDENTITY_MTLS_PEER_CERT_SHA256=" + e.gatewayServer.SHA256,
		"IDENTITY_ASSERTION_SIGNING_KEY=" + social.key.StdBase64,
		fmt.Sprintf("SOCIAL_PRIVATE_LISTEN_ADDRESS=127.0.0.1:%d", privatePort),
		"SOCIAL_TLS_CERT_FILE=" + sp.server.CertFile,
		"SOCIAL_TLS_KEY_PEM=" + string(sp.server.KeyPEM),
		"SOCIAL_TLS_CLIENT_CA_FILE=" + e.pki.CAFile,
		"GATEWAY_PEER_CERT_SHA256=" + e.gatewayClient.SHA256,
		"SOCIAL_AUTHORITY_PEERS=ranking.railway.internal=" + sp.ranking.SHA256,
	}
	sp.plainURL = fmt.Sprintf("http://127.0.0.1:%d", plainPort)
	sp.privateURL = fmt.Sprintf("https://127.0.0.1:%d", privatePort)
	// Readiness includes the identity authority probe: social's client
	// certificate, the pinned gateway server certificate and its request
	// assertion, all verified by the gateway's production verifiers.
	stop := runService(t, "sociald", binary, env, sp.plainURL)
	sp.restart = func() { stop(); stop = runService(t, "sociald", binary, env, sp.plainURL) }
	return sp
}

// buildService compiles the real executable in its owning Go module.
func buildService(t *testing.T, module, command string) string {
	t.Helper()
	dir := filepath.Join(proofRepositoryRoot(t), "services/launchpad", module)
	binary := filepath.Join(t.TempDir(), command)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/"+command)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build %s/cmd/%s: %v\n%s", dir, command, err, out)
	}
	return binary
}

// runService starts binary with exactly env, stops it (SIGINT) at cleanup
// and waits until plainURL/health/ready answers 200.
func runService(t *testing.T, name, binary string, env []string, plainURL string) func() {
	t.Helper()
	output := &lockedBuffer{}
	runCtx, cancel := context.WithCancel(context.Background())
	cmd := exec.CommandContext(runCtx, binary)
	cmd.Env = env
	cmd.Stdout, cmd.Stderr = output, output
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 10 * time.Second
	if err := cmd.Start(); err != nil {
		cancel()
		t.Fatal(err)
	}
	exited := make(chan struct{})
	var processErr error
	go func() { processErr = cmd.Wait(); close(exited) }()
	var once sync.Once
	stop := func() { once.Do(func() { cancel(); <-exited }) }
	t.Cleanup(func() {
		stop()
		if t.Failed() {
			t.Logf("%s output:\n%s", name, output.String())
		}
	})
	deadline := time.Now().Add(60 * time.Second)
	for {
		select {
		case <-exited:
			t.Fatalf("%s exited: %v\n%s", name, processErr, output.String())
		default:
		}
		res, err := http.Get(plainURL + "/health/ready")
		if err == nil {
			raw, _ := io.ReadAll(res.Body)
			_ = res.Body.Close()
			if res.StatusCode == http.StatusOK {
				return stop
			}
			if time.Now().After(deadline) {
				t.Fatalf("%s not ready: %d %s\n%s", name, res.StatusCode, raw, output.String())
			}
		} else if time.Now().After(deadline) {
			t.Fatalf("%s unreachable: %v\n%s", name, err, output.String())
		}
		time.Sleep(200 * time.Millisecond)
	}
}

type livestreamProcess struct {
	plainURL, privateURL, dsn string
	restart                   func()
}

// startLivestream runs media/livestream's livestreamd in the production
// platform runtime with its private mTLS listener.
func (e *identityE2E) startLivestream() livestreamProcess {
	t := e.t
	t.Helper()
	binary := buildService(t, "livestream", "livestreamd")
	dsn := e.database.AdminDSN
	server := e.pki.Leaf("livestream-server", "livestream.railway.internal")
	ports := internaltest.FreePorts(t, 2)
	plainPort, privatePort := ports[0], ports[1]
	lp := livestreamProcess{plainURL: fmt.Sprintf("http://127.0.0.1:%d", plainPort), privateURL: fmt.Sprintf("https://127.0.0.1:%d", privatePort), dsn: dsn}
	env := []string{
		"NODE_ENV=production", "RUNTIME_PROFILE=platform", "ZONE=zone-a", "METRICS_ENVIRONMENT=production", "LOG_LEVEL=error",
		"SECRETS_PROVIDER=env", "SECRETS_REFRESH_SECONDS=30", "DB_MAX_CONNS=5",
		fmt.Sprintf("PORT=%d", plainPort), "BIND_ADDRESS=0.0.0.0",
		"DATABASE_URL=" + e.database.Runtimes["livestream_runtime"].DSN,
		"REDIS_URL=" + e.redisURL,
		"LIVEPEER_PROVIDER_MODE=unavailable", "MAX_STREAMS_PER_WALLET=3", "MAX_GLOBAL_STREAMS=100",
		"LIVESTREAM_VIEWER_TOKEN_KEY=" + strings.Repeat("v7", 20),
		"CORS_ALLOWED_ORIGINS=" + e2eOrigin,
		fmt.Sprintf("LIVESTREAM_PRIVATE_LISTEN_ADDRESS=127.0.0.1:%d", privatePort),
		"LIVESTREAM_TLS_CERT_FILE=" + server.CertFile,
		"LIVESTREAM_TLS_KEY_FILE=" + server.KeyFile,
		"LIVESTREAM_TLS_CLIENT_CA_FILE=" + e.pki.CAFile,
		"GATEWAY_PEER_CERT_SHA256=" + e.gatewayClient.SHA256,
	}
	env = append(env, internaltest.LivestreamDocuments(t, e.database.Runtimes["livestream_runtime"])...)
	// Exercise the real entrypoint's diagnostic before starting the configured
	// producer. A missing certificate must fail closed and retain its cause.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	refused := exec.CommandContext(ctx, binary)
	for _, value := range env {
		if !strings.HasPrefix(value, "LIVESTREAM_TLS_CERT_FILE=") {
			refused.Env = append(refused.Env, value)
		}
	}
	out, err := refused.CombinedOutput()
	exit, ok := err.(*exec.ExitError)
	if !ok || exit.ExitCode() != 1 || !bytes.Contains(out, []byte("reason=livestream_runtime_failed")) || !bytes.Contains(out, []byte("exactly one of LIVESTREAM_TLS_CERT_FILE and LIVESTREAM_TLS_CERT_PEM is required")) {
		t.Fatalf("services/launchpad/livestream/cmd/livestreamd startup refusal: %v\n%s", err, out)
	}
	stop := runService(t, "livestreamd", binary, env, lp.plainURL)
	lp.restart = func() { stop(); stop = runService(t, "livestreamd", binary, env, lp.plainURL) }
	return lp
}

// upstream holds the real configured gateway crossing, including its client
// transport; tests never replace Connected.client or its RoundTripper.
type upstream struct {
	proxy *Connected
	edge  *httptest.Server
}

func proofUpstream(t *testing.T, e *identityE2E, name, endpoint string, routes []ConnectedRoute) upstream {
	t.Helper()
	p, err := NewConnected(ConnectedDeps{Name: name, URL: endpoint, Routes: routes, Identity: e.authorityForGateway, SigningKey: e.gatewayKey.Private, TLS: e.gatewayClientTLS()})
	if err != nil {
		t.Fatal(err)
	}
	edge := httptest.NewServer(p)
	t.Cleanup(edge.Close)
	t.Cleanup(p.client.CloseIdleConnections)
	return upstream{p, edge}
}

func TestProofLaunchpadGateway(t *testing.T) {
	e := newIdentityE2E(t)
	alice := e.signIn()
	e.prepareSocialAccount(alice.wallet)
	lp := e.startLivestream()
	sp := e.startSocial()
	live := proofUpstream(t, e, "livestream", lp.privateURL, LivestreamConnectedRoutes())
	social := proofUpstream(t, e, "social", sp.privateURL, SocialConnectedRoutes())
	headers := map[string]string{"Authorization": "Bearer " + alice.token, "Origin": e2eOrigin}

	t.Run("directory twice without cache", func(t *testing.T) {
		var requestID string
		for i := 0; i < 2; i++ {
			resp, raw := e2eSend(t, nil, http.MethodGet, live.edge.URL+"/v2/streams", nil, nil)
			page, err := v2.DecodeStreamListResponse(raw)
			if resp.StatusCode != 200 || err != nil || len(page.Data.Items) != 0 || resp.Header.Get("Cache-Control") != "no-store" || page.RequestId == requestID {
				t.Fatalf("directory observation %d: %d %s (%v)", i, resp.StatusCode, raw, err)
			}
			requestID = page.RequestId
			// Fault injection is a mutation of bytes emitted by the real producer,
			// at the decoder boundary, without substituting the HTTP transport.
			var malformed map[string]any
			if err = json.Unmarshal(raw, &malformed); err != nil {
				t.Fatal(err)
			}
			malformed["data"] = map[string]any{"items": "not-an-array"}
			invalid, err := json.Marshal(malformed)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = v2.DecodeStreamListResponse(invalid); err == nil {
				t.Fatal("consumer accepted malformed producer list")
			}
		}
	})

	t.Run("real web consumer observes producer revisions", func(t *testing.T) {
		ctx := context.Background()
		db, err := pgx.Connect(ctx, lp.dsn)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close(ctx)
		id := uuid.NewString()
		if _, err = db.Exec(ctx, `INSERT INTO livestream.stream_operations(id,wallet,pool,title,idem_key,request_hash,state,provider_id) VALUES($1,$2,$3,'First observation',$4,'proof','ENDED',$5)`, id, alice.wallet, sp.pool, "proof_"+id, "prov_"+strings.ReplaceAll(id, "-", "")); err != nil {
			t.Fatal(err)
		}
		for i, title := range []string{"First observation", "Second observation"} {
			if i == 1 {
				if _, err = db.Exec(ctx, `UPDATE livestream.stream_operations SET title=$2,revision=revision+1 WHERE id=$1`, id, title); err != nil {
					t.Fatal(err)
				}
			}
			lp.restart()
			proofWebConsumer(t, live.edge.URL, id, title, alice, strconv.Itoa(i+1))
		}
	})

	t.Run("duplicate livestream signature refused", func(t *testing.T) {
		req, err := http.NewRequest(http.MethodPost, live.edge.URL+"/v2/streams/"+uuid.NewString()+"/viewers:heartbeat", strings.NewReader("{}"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Add("X-Kindle-Signature", "one")
		req.Header.Add("X-Kindle-Signature", "two")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != 400 || !bytes.Contains(raw, []byte(`"code":"ambiguous_header"`)) {
			t.Fatalf("duplicate operation signature crossed gateway: %d %s", resp.StatusCode, raw)
		}
	})
	t.Run("start end heartbeat reach real decoders", func(t *testing.T) {
		id := uuid.NewString()
		for _, path := range []string{"/v2/streams/" + id + ":start", "/v2/streams/" + id + ":end", "/v2/streams/" + id + "/viewers:heartbeat"} {
			// Wrong wire fields must be refused by the producer's own decoder.
			resp, raw := e2eSend(t, nil, http.MethodPost, live.edge.URL+path, map[string]string{"unexpected": "field"}, headers)
			failure, err := v2.DecodeError(raw)
			if strings.Contains(path, "viewers:heartbeat") {
				if resp.StatusCode != http.StatusBadRequest || err != nil || failure.Code != "FIELD_INVALID" {
					t.Fatalf("real decoder %s: %d %s (%v)", path, resp.StatusCode, raw, err)
				}
			} else {
				// NewPublicReads intentionally guards provider-bound actions before decoding.
				if resp.StatusCode != http.StatusConflict || err != nil || failure.Code != "BROADCAST_NOT_PROVISIONED" {
					t.Fatalf("unprovisioned lifecycle %s: %d %s (%v)", path, resp.StatusCode, raw, err)
				}
			}
		}
	})
	t.Run("viewer producer through gateway into heartbeat consumer", func(t *testing.T) {
		ctx := context.Background()
		db, err := pgx.Connect(ctx, lp.dsn)
		if err != nil {
			t.Fatal(err)
		}
		defer db.Close(ctx)
		id := uuid.NewString()
		if _, err = db.Exec(ctx, `INSERT INTO livestream.stream_operations(id,wallet,pool,title,idem_key,request_hash,state,provider_id) VALUES($1,$2,$3,'Heartbeat proof',$4,'proof','LIVE',$5)`, id, alice.wallet, sp.pool, "proof_"+id, "prov_"+strings.ReplaceAll(id, "-", "")); err != nil {
			t.Fatal(err)
		}
		var requestID string
		for i := 0; i < 2; i++ {
			resp, raw := e2eSend(t, nil, http.MethodPost, live.edge.URL+"/v2/streams/"+id+"/viewer-session", struct{}{}, nil)
			var session struct {
				StreamID  string `json:"streamId"`
				SessionID string `json:"sessionId"`
				Token     string `json:"token"`
			}
			if err := json.Unmarshal(raw, &session); err != nil || resp.StatusCode != 200 || session.StreamID != id || session.Token == "" {
				t.Fatalf("real viewer producer: %d %s (%v)", resp.StatusCode, raw, err)
			}
			// Restart preserves both the lifecycle row and the token authority;
			// Redis remains the real viewer counter across service restarts.
			lp.restart()
			body := map[string]string{"playbackSessionId": session.SessionID, "viewerToken": session.Token}
			resp, raw = e2eSend(t, nil, http.MethodPost, live.edge.URL+"/v2/streams/"+id+"/viewers:heartbeat", body, nil)
			observed, err := v2.DecodeStreamHeartbeatResponse(raw)
			if resp.StatusCode != 200 || err != nil {
				t.Fatalf("heartbeat consumer: %d %s (%v)", resp.StatusCode, raw, err)
			}
			if observed.Data.ViewerCount != v2.SafeInteger(i+1) || observed.RequestId == requestID || resp.Header.Get("Cache-Control") != "no-store" {
				t.Fatal("heartbeat reused an observation")
			}
			requestID = observed.RequestId
			var malformed map[string]any
			if err := json.Unmarshal(raw, &malformed); err != nil {
				t.Fatal(err)
			}
			malformed["data"].(map[string]any)["viewerCount"] = -1
			invalid, err := json.Marshal(malformed)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := v2.DecodeStreamHeartbeatResponse(invalid); err == nil {
				t.Fatal("heartbeat consumer accepted malformed producer result")
			}
			body["viewerToken"] = session.Token + "corrupt"
			resp, raw = e2eSend(t, nil, http.MethodPost, live.edge.URL+"/v2/streams/"+id+"/viewers:heartbeat", body, nil)
			failure, err := v2.DecodeError(raw)
			if resp.StatusCode != http.StatusUnauthorized || err != nil || failure.Code != "AUTH_EXPIRED" {
				t.Fatalf("corrupt viewer token: %d %s (%v)", resp.StatusCode, raw, err)
			}
		}
	})

	t.Run("voice create moderation and tickets", func(t *testing.T) {
		resp, raw := e2eSend(t, nil, http.MethodPost, social.edge.URL+"/v2/voice-rooms", map[string]string{"scopeType": "pool", "scopeId": sp.pool, "title": "Gateway proof", "visibility": "private"}, headers)
		created, err := v2.DecodeVoiceCreateResponse(raw)
		if resp.StatusCode != http.StatusCreated || err != nil || created.Data.Room.RoomId == "" {
			t.Fatalf("voice producer: %d %s (%v)", resp.StatusCode, raw, err)
		}
		room := created.Data.Room.RoomId
		sp.restart()
		var malformed map[string]any
		if err := json.Unmarshal(raw, &malformed); err != nil {
			t.Fatal(err)
		}
		malformed["data"] = map[string]any{"room": "invalid"}
		invalid, err := json.Marshal(malformed)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := v2.DecodeVoiceCreateResponse(invalid); err == nil {
			t.Fatal("consumer accepted malformed voice producer result")
		}
		for i := 0; i < 2; i++ {
			resp, raw = e2eSend(t, nil, http.MethodGet, social.edge.URL+"/v2/voice-rooms/"+room, nil, headers)
			read, err := v2.DecodeVoiceReadResponse(raw)
			if resp.StatusCode != 200 || err != nil || read.Data.Room.RoomId != room {
				t.Fatalf("voice consumer observation %d: %d %s (%v)", i, resp.StatusCode, raw, err)
			}
		}
		for _, action := range []string{"moderation-actions", "tickets"} {
			resp, raw = e2eSend(t, nil, http.MethodPost, social.edge.URL+"/v2/voice-rooms/"+room+"/"+action, map[string]string{"unexpected": "field"}, headers)
			failure, err := v2.DecodeError(raw)
			if resp.StatusCode != http.StatusBadRequest || err != nil || failure.Code != "INVALID_REQUEST" {
				t.Fatalf("voice malformed %s: %d %s (%v)", action, resp.StatusCode, raw, err)
			}
		}
		// No publish authority exists in the real platform runtime.
		resp, raw = e2eSend(t, nil, http.MethodPost, social.edge.URL+"/v2/voice-rooms/"+room+"/tickets", map[string]string{"role": "listener", "clientSessionId": uuid.NewString()}, headers)
		denied, err := v2.DecodeError(raw)
		if resp.StatusCode != 503 || err != nil || denied.Code != "DEPENDENCY_UNAVAILABLE" {
			t.Fatalf("unqualified provider ticket: %d %s (%v)", resp.StatusCode, raw, err)
		}
	})

	t.Run("retired routes stay unserved", func(t *testing.T) {
		for _, route := range RetiredConnectedRoutes() {
			path := strings.ReplaceAll(strings.ReplaceAll(route.Template, "{assetId}", uuid.NewString()), "{roomId}", uuid.NewString())
			for _, edge := range []*httptest.Server{live.edge, social.edge} {
				resp, raw := e2eSend(t, nil, route.Method, edge.URL+path, nil, headers)
				if resp.StatusCode != 404 {
					t.Fatalf("retired %s %s: %d %s", route.Method, path, resp.StatusCode, raw)
				}
			}
		}
	})
}

// proofWebConsumer loads the actual web modules using Node's TypeScript
// transformation and resolver hooks. fetch, signing and parsing stay real.
func proofWebConsumer(t *testing.T, origin, id, title string, session e2eSession, revision string) {
	t.Helper()
	// The browser's wallet callback signs with the same real EOA used by
	// identity sign-in; no request or response in the gateway path is replaced.
	wallet := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		message, err := io.ReadAll(io.LimitReader(r.Body, 16384))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		signature, err := crypto.Sign(accounts.TextHash(message), session.key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		signature[64] += 27
		_, _ = io.WriteString(w, "0x"+hex.EncodeToString(signature))
	}))
	defer wallet.Close()
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../.."))
	script := `
import {registerHooks,stripTypeScriptTypes} from 'node:module';
import {readFileSync,existsSync} from 'node:fs';
import {pathToFileURL,fileURLToPath} from 'node:url';
import assert from 'node:assert/strict';
const [root,origin,id,title,token,revision,wallet,signerURL]=process.argv.slice(1);
const source=root+'/apps/launchpad/web/src/';
registerHooks({
 resolve(specifier,context,next){
  if(specifier.startsWith('@/'))return {url:pathToFileURL(source+specifier.slice(2)+'.ts').href,shortCircuit:true};
  if(specifier.startsWith('.')&&context.parentURL?.startsWith('file:')){
   const target=new URL(specifier,context.parentURL);
   if(!existsSync(fileURLToPath(target))&&existsSync(fileURLToPath(target)+'.ts'))return {url:target.href+'.ts',shortCircuit:true};
  }
  return next(specifier,context);
 },
 load(url,context,next){
  if(url.endsWith('.ts'))return {format:'module',source:stripTypeScriptTypes(readFileSync(fileURLToPath(url),'utf8'),{mode:'transform'}),shortCircuit:true};
  return next(url,context);
 }
});
process.env.NEXT_PUBLIC_LIVESTREAM_API_URL=origin;
const api=await import(pathToFileURL(source+'core/clients/livestream-api.ts').href);
const signing={wallet,environment:'production',assertCurrent(){assert.equal(signing.wallet,wallet);assert.ok(token)},async signMessage(message){const response=await fetch(signerURL,{method:'POST',body:message});assert.equal(response.status,200);return response.text()}};
const observed=await api.getStream(id);
assert.equal(observed.id,id);assert.equal(observed.title,title);assert.equal(observed.status,'ended');assert.equal(observed.revision,revision);
await assert.rejects(api.getStreamOperation(id,token),error=>error.status===409&&error.code==='BROADCAST_NOT_PROVISIONED');
for(const action of ['start','end'])await assert.rejects(api.controlStream(id,observed.revision,action,token,signing),error=>error.status===409&&error.code==='BROADCAST_NOT_PROVISIONED');
const definitions=await import(pathToFileURL(root+'/sdk/launchpad/packages/sdk/src/operations.gen.ts').href);
assert.equal(definitions.operationDefinitions.IF_STREAM_END.body,'StreamStartBody');
const response=await fetch(origin+'/v2/streams/'+id,{cache:'no-store'});
assert.equal(response.status,200);
const produced=await response.json();
assert.throws(()=>api.parseStream({...produced,viewerCount:-1}));
console.log(JSON.stringify(observed));
`
	cmd := exec.Command("node", "--input-type=module", "-e", script, root, origin, id, title, session.token, revision, session.wallet, wallet.URL)
	cmd.Env = []string{"NODE_ENV=test"}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("real web consumer: %v\n%s", err, out)
	}
}

func TestProofLaunchpadGatewayCatalog(t *testing.T) {
	catalog := readFrozenMediaCatalog(t)
	owners := map[string]int{}
	tableRoutes := map[string]map[string]bool{}
	for _, table := range frozenMediaConnectedTables() {
		tableRoutes[table.service] = map[string]bool{}
		for _, route := range table.routes {
			if !allowedConnectedRoute(table.service, route) {
				t.Fatalf("producer route rejected: %s %+v", table.service, route)
			}
			owners[route.Method+" "+route.Template]++
			tableRoutes[table.service][route.Method+" "+route.Template] = true
		}
	}
	// newGatewayConnectedRuntime retains the binding's frozen routes when
	// appending each service's connected table. Count those real admission
	// paths too, without counting a table route again for the same service.
	for service, surface := range FrozenConnectedSurface() {
		for path, methods := range surface {
			for _, method := range strings.Fields(methods) {
				key := method + " " + path
				if tableRoutes[service][key] {
					continue
				}
				route := ConnectedRoute{Method: method, Template: path, Authenticate: true}
				route.Upload = service == "metadata" && method == http.MethodPut && strings.Contains(path, "/assets/")
				if !allowedConnectedRoute(service, route) {
					t.Fatalf("frozen binding route rejected: %s %+v", service, route)
				}
				owners[key]++
			}
		}
	}
	for _, route := range RetiredConnectedRoutes() {
		owners[route.Method+" "+route.Template]++
		for _, service := range []string{"livestream", "social", "metadata", "user"} {
			if allowedConnectedRoute(service, route) {
				t.Fatalf("retired producer admitted by %s: %+v", service, route)
			}
		}
	}
	for id, operation := range catalog {
		if operation.Service == "media" && owners[operation.Method+" "+operation.Path] != 1 {
			t.Fatalf("SDK consumer %s has %d gateway owners", id, owners[operation.Method+" "+operation.Path])
		}
	}
}

func TestProofLaunchpadGatewayOperationProducer(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	path := filepath.Join(filepath.Dir(file), "../../../../../apps/launchpad/web/src/core/test-fixtures/operation-envelope/go-vectors.json")
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		cmd := exec.Command("go", "run", filepath.Join(filepath.Dir(path), "main.go"))
		cmd.Dir = filepath.Join(proofRepositoryRoot(t), "services/launchpad/gateway")
		produced, err := cmd.Output()
		if err != nil {
			t.Fatalf("%s producer: %v", filepath.Join(filepath.Dir(path), "main.go"), err)
		}
		if !bytes.Equal(bytes.TrimSpace(produced), bytes.TrimSpace(raw)) {
			t.Fatal("operation-envelope producer differs from checked-in consumer vectors")
		}
	}
	var fixture struct {
		Actor   string `json:"actor"`
		Vectors []struct {
			Body, Statement, Encoded, Signature, SignatureHeader string
			Envelope                                             sharedauth.OperationEnvelope
		} `json:"vectors"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Vectors) == 0 {
		t.Fatal("producer emitted no vectors")
	}
	for _, vector := range fixture.Vectors {
		statement, err := vector.Envelope.Statement()
		if err != nil || statement != vector.Statement {
			t.Fatalf("Go producer statement changed: %v", err)
		}
		if _, err = sharedauth.VerifyEOA(fixture.Actor, statement, vector.Signature, nil); err != nil {
			t.Fatal(err)
		}
		if _, err = sharedauth.VerifyEOA("0x"+strings.Repeat("ab", 20), statement, vector.Signature, nil); err == nil {
			t.Fatal("wrong signer accepted")
		}
		if err = vector.Envelope.Validate(time.Unix(vector.Envelope.IssuedAt+1, 0), vector.Envelope.Environment, fixture.Actor, vector.Envelope.Action, vector.Envelope.Method, vector.Envelope.Path, []byte(vector.Body)); err != nil {
			t.Fatal(err)
		}
		encoded, err := base64.RawURLEncoding.Strict().DecodeString(vector.Encoded)
		if err != nil {
			t.Fatal(err)
		}
		canonical, err := json.Marshal(vector.Envelope)
		if err != nil || !bytes.Equal(encoded, canonical) {
			t.Fatal("producer encoded bytes differ from canonical Go envelope")
		}
		want := sharedauth.OperationSignatureHeader
		if strings.HasPrefix(vector.Envelope.Action, "stream.") {
			want = "X-Kindle-Signature"
		}
		if vector.SignatureHeader != want {
			t.Fatalf("producer signature header = %s; want %s", vector.SignatureHeader, want)
		}
	}
}

func proofRepositoryRoot(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../.."))
}

type proofDatabase = coordinatortest.Database

func proofBootstrap(t testing.TB, adminDSN string, runtimeRoles ...string) proofDatabase {
	t.Helper()
	return coordinatortest.Bootstrap(t, adminDSN, runtimeRoles...)
}
