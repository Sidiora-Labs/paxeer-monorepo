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
	"regexp"
	"runtime"
	"sort"
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
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
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
	pc, err := pgx.ParseConfig(e.adminDSN)
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", pc.User, pc.Password, pc.Host, pc.Port, name)
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
	schemaFile, schemaDigest := e.database.ExpectedSchemaFile(t, "livestream_runtime")
	env := []string{
		"NODE_ENV=production", "RUNTIME_PROFILE=platform", "ZONE=zone-a", "METRICS_ENVIRONMENT=production", "LOG_LEVEL=error",
		"SECRETS_PROVIDER=env", "SECRETS_REFRESH_SECONDS=30", "DB_MAX_CONNS=5",
		fmt.Sprintf("PORT=%d", plainPort), "BIND_ADDRESS=0.0.0.0",
		"DATABASE_URL=" + e.database.Runtimes["livestream_runtime"].DSN,
		"EXPECTED_SCHEMA_FILE=" + schemaFile,
		"EXPECTED_SCHEMA_SHA256=" + schemaDigest,
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

// The following bootstrap uses the shared production coordinator primitives,
// retaining its reviewed checks while resolving relocated service migrations.
// proofRegion is the recovery region the private bootstrap seeds.
const proofRegion = "fixture-loopback"

// proofEpoch is the recovery epoch the private bootstrap seeds.
const proofEpoch = 1

const proofMigrationsDir = "migrations"

// proofRuntime is one bootstrapped least-privilege runtime role.
type proofRuntime struct {
	// Role is the per-test role name (the reviewed name plus a suffix).
	Role string
	// Service is the reviewed manifest service (ExpectedSchema.service).
	Service string
	// DSN logs in as Role.
	DSN string
	// ExpectedSchema is the canonical ExpectedSchema JSON the runtime loads
	// through EXPECTED_SCHEMA_FILE / EXPECTED_SCHEMA_SHA256, and
	// ExpectedSchemaSHA256 its digest.
	ExpectedSchema       []byte
	ExpectedSchemaSHA256 string
}

// proofDatabase is a private coordinator-bootstrapped database.
type proofDatabase struct {
	// AdminDSN reaches the database as the cluster administrator (test
	// seeding stands in for the producers that own the other schemas).
	AdminDSN string
	Name     string
	// Applied maps every applied registered migration to its checksum.
	Applied map[int64]string
	// Runtimes is keyed by the reviewed runtime role name.
	Runtimes map[string]proofRuntime
	// Rename maps a reviewed role name to its per-test name.
	Rename func(string) string
}

// ExpectedSchemaFile writes the runtime role's ExpectedSchema document to a
// test-owned file and returns its path and sha256.
func (d proofDatabase) ExpectedSchemaFile(t testing.TB, role string) (string, string) {
	t.Helper()
	rt, ok := d.Runtimes[role]
	if !ok {
		t.Fatalf("runtime role %s was not bootstrapped", role)
	}
	path := filepath.Join(t.TempDir(), "expected-schema.json")
	if err := os.WriteFile(path, rt.ExpectedSchema, 0o600); err != nil {
		t.Fatal(err)
	}
	return path, rt.ExpectedSchemaSHA256
}

type proofLegacyHistories struct {
	Histories []struct {
		Service string `json:"service"`
		Schema  string `json:"schema"`
		History string `json:"history"`
		Sources []struct {
			OriginalVersion int64  `json:"original_version"`
			Path            string `json:"path"`
			SourceSHA256    string `json:"source_sha256"`
			UpSHA256        string `json:"up_sha256"`
		} `json:"sources"`
	} `json:"histories"`
}

type proofRegistryDocument struct {
	production.MigrationRegistry
	Extensions []struct {
		Extension string `json:"extension"`
		Schema    string `json:"schema"`
	} `json:"extension_prerequisites"`
}

// proofRepositoryRoot is the monorepo root (the reviewed migration inputs live
// under spec/production-system).
func proofRepositoryRoot(t testing.TB) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../.."))
}

func proofReadBound(t testing.TB, root, relative, want string) []byte {
	t.Helper()
	// Repository relocation preserves the reviewed SQL bytes and digests.
	if strings.HasPrefix(relative, "spec/production-system/") {
		relative = "launchpad/" + relative
	}
	if strings.HasPrefix(relative, "core/") || strings.HasPrefix(relative, "media/") {
		relative = "services/launchpad/" + strings.SplitN(relative, "/", 2)[1]
	}
	raw, err := os.ReadFile(filepath.Join(root, relative))
	if err != nil {
		t.Fatal(err)
	}
	if want != "" && production.Checksum(raw) != want {
		t.Fatalf("%s differs from its reviewed sha256", relative)
	}
	return raw
}

var proofIdentifier = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// proofBootstrap creates a private database on the server adminDSN names and
// bootstraps it for the given reviewed runtime roles (grants.json
// runtime_role names).
func proofBootstrap(t testing.TB, adminDSN string, runtimeRoles ...string) proofDatabase {
	t.Helper()
	if adminDSN == "" {
		t.Fatal("admin DSN is empty: point it at a running PostgreSQL whose role may CREATE DATABASE and CREATE ROLE; tests never start containers")
	}
	if len(runtimeRoles) == 0 {
		t.Fatal("no runtime role requested")
	}
	ctx := context.Background()
	root := proofRepositoryRoot(t)

	registryBytes := proofReadBound(t, root, proofMigrationsDir+"/registry.json", "")
	var registry proofRegistryDocument
	if err := json.Unmarshal(registryBytes, &registry); err != nil {
		t.Fatal(err)
	}
	registered := map[int64]production.MigrationManifest{}
	for _, m := range registry.Registrations {
		if _, dup := registered[m.MigrationID]; dup {
			t.Fatalf("migration %d registered twice", m.MigrationID)
		}
		registered[m.MigrationID] = m
	}
	// The registry's application order must be a topological order of the
	// predecessor DAG covering every registration exactly once.
	ordered := map[int64]bool{}
	for _, id := range registry.ApplicationOrder {
		m, ok := registered[id]
		if !ok || ordered[id] {
			t.Fatalf("application order names migration %d that is unregistered or repeated", id)
		}
		for _, p := range m.PredecessorIDs {
			if !ordered[p] {
				t.Fatalf("migration %d is ordered before its predecessor %d", id, p)
			}
		}
		ordered[id] = true
	}
	if len(ordered) != len(registered) {
		t.Fatal("application order does not cover every registration")
	}
	rolesBytes := proofReadBound(t, root, proofMigrationsDir+"/grants.json", "")
	var roleDoc struct {
		Roles []production.RoleManifest `json:"roles"`
	}
	if err := json.Unmarshal(rolesBytes, &roleDoc); err != nil {
		t.Fatal(err)
	}
	var legacy proofLegacyHistories
	if err := json.Unmarshal(proofReadBound(t, root, proofMigrationsDir+"/legacy-history-aliases.json", ""), &legacy); err != nil {
		t.Fatal(err)
	}

	owners := map[string]string{} // schema -> reviewed owner role
	reviewedRoles := map[string]bool{}
	byRuntime := map[string]production.RoleManifest{}
	for _, m := range roleDoc.Roles {
		if err := m.Validate(); err != nil {
			t.Fatalf("reviewed role manifest %s: %v", m.RuntimeRole, err)
		}
		if _, ok := owners[m.Schema]; !ok {
			owners[m.Schema] = m.OwnerRole
		}
		for _, c := range m.SchemaCapabilities {
			if _, ok := owners[c.Schema]; !ok {
				owners[c.Schema] = c.OwnerRole
			}
			reviewedRoles[c.OwnerRole] = true
		}
		reviewedRoles[m.OwnerRole], reviewedRoles[m.RuntimeRole] = true, true
		if _, ok := byRuntime[m.RuntimeRole]; !ok {
			byRuntime[m.RuntimeRole] = m
		}
	}
	requested := []production.RoleManifest{}
	readSchemas := map[string]map[string]bool{} // runtime role -> schemas its readiness fingerprints
	allRead := map[string]bool{}
	for _, role := range runtimeRoles {
		m, ok := byRuntime[role]
		if !ok {
			t.Fatalf("grants.json has no manifest for runtime role %s", role)
		}
		requested = append(requested, m)
		schemas := map[string]bool{m.Schema: true}
		for _, g := range m.Grants {
			if g.Schema != "migration_control" {
				schemas[g.Schema] = true
			}
		}
		readSchemas[role] = schemas
		for s := range schemas {
			allRead[s] = true
		}
	}

	// Every registered migration of the read schemas and, transitively, its
	// predecessors, in the reviewed application order.
	needed := map[int64]bool{}
	var include func(int64)
	include = func(id int64) {
		if needed[id] {
			return
		}
		m, ok := registered[id]
		if !ok {
			t.Fatalf("migration %d is not registered", id)
		}
		needed[id] = true
		for _, p := range m.PredecessorIDs {
			include(p)
		}
	}
	for _, m := range registry.Registrations {
		if allRead[m.Schema] {
			include(m.MigrationID)
		}
	}
	plan := []production.MigrationManifest{}
	schemas := map[string]bool{}
	services := map[string]bool{}
	for _, id := range registry.ApplicationOrder {
		if needed[id] {
			plan = append(plan, registered[id])
			schemas[registered[id].Schema] = true
			services[registered[id].Service] = true
		}
	}
	for s := range allRead {
		if !schemas[s] {
			t.Fatalf("schema %s is read by a requested runtime role but no registered migration creates it", s)
		}
	}

	var suffix [5]byte
	if _, err := rand.Read(suffix[:]); err != nil {
		t.Fatal(err)
	}
	tag := "_t" + hex.EncodeToString(suffix[:])
	rename := func(role string) string { return role + tag }
	// Registered SQL may name reviewed roles; bind those names to the
	// per-test roles (longest names first so no name rewrites a prefix).
	names := make([]string, 0, len(reviewedRoles))
	for name := range reviewedRoles {
		if !proofIdentifier.MatchString(name) {
			t.Fatalf("reviewed role %q is not an proofIdentifier", name)
		}
		names = append(names, name)
	}
	sort.Slice(names, func(i, j int) bool {
		return len(names[i]) > len(names[j]) || (len(names[i]) == len(names[j]) && names[i] < names[j])
	})
	roleReference := regexp.MustCompile(`\b(` + strings.Join(names, "|") + `)\b`)
	bind := func(sql []byte) string {
		return roleReference.ReplaceAllStringFunc(string(sql), rename)
	}

	adminConfig, err := pgx.ParseConfig(adminDSN)
	if err != nil {
		t.Fatalf("parse admin DSN: %v", err)
	}
	control, err := pgx.ConnectConfig(ctx, adminConfig)
	if err != nil {
		t.Fatalf("connect admin DSN: %v", err)
	}
	defer control.Close(ctx)
	name := "coordinator" + tag
	created := []string{}
	// Registered first so it runs last: the database goes first, then every
	// cluster-global role this bootstrap created, then the shared ledger
	// owner if no other private database still uses it.
	t.Cleanup(func() {
		c, err := pgx.ConnectConfig(context.Background(), adminConfig)
		if err != nil {
			t.Logf("cleanup: %v", err)
			return
		}
		defer c.Close(context.Background())
		if _, err = c.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Logf("drop test database: %v", err)
		}
		for i := len(created) - 1; i >= 0; i-- {
			if _, err = c.Exec(context.Background(), "DROP ROLE IF EXISTS "+pgx.Identifier{created[i]}.Sanitize()); err != nil {
				t.Logf("drop test role %s: %v", created[i], err)
			}
		}
		var used bool
		if err = c.QueryRow(context.Background(), "SELECT EXISTS(SELECT 1 FROM pg_shdepend d JOIN pg_roles r ON r.oid=d.refobjid WHERE r.rolname='migration_control_owner')").Scan(&used); err == nil && !used {
			_, _ = c.Exec(context.Background(), "DROP ROLE IF EXISTS migration_control_owner")
		}
	})
	if _, err = control.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); err != nil {
		t.Fatalf("create test database: %v", err)
	}
	adminURL, err := url.Parse(adminDSN)
	if err != nil || (adminURL.Scheme != "postgres" && adminURL.Scheme != "postgresql") {
		t.Fatal("admin DSN must be a postgres:// URL")
	}
	adminURL.Path = "/" + name
	db := proofDatabase{AdminDSN: adminURL.String(), Name: name, Runtimes: map[string]proofRuntime{}, Rename: rename}
	conn, err := pgx.Connect(ctx, db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close(ctx)
	exec := func(step, sql string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", step, err)
		}
	}

	// kindle-migrate: no CREATE on public for PUBLIC (CheckReadiness rejects
	// any CREATE) and the registry's extension prerequisites installed by the
	// administrator, never at runtime.
	exec("public create", "REVOKE CREATE ON SCHEMA public FROM PUBLIC")
	for _, x := range registry.Extensions {
		if !proofIdentifier.MatchString(x.Extension) || !proofIdentifier.MatchString(x.Schema) {
			t.Fatal("invalid extension prerequisite")
		}
		exec("extension", "CREATE EXTENSION IF NOT EXISTS "+pgx.Identifier{x.Extension}.Sanitize()+" WITH SCHEMA "+pgx.Identifier{x.Schema}.Sanitize())
	}
	// migration_control: the reviewed ledger DDL. Its owner role is
	// cluster-global and unsuffixed; it is created once per cluster.
	ddl := production.MigrationControlDDL()
	first := strings.Index(ddl, ";")
	if !strings.HasPrefix(ddl, "CREATE ROLE migration_control_owner ") || first < 0 {
		t.Fatal("migration_control DDL no longer starts with its owner role")
	}
	exec("migration control owner", "DO $$ BEGIN "+ddl[:first+1]+" EXCEPTION WHEN duplicate_object THEN NULL; END $$")
	exec("migration control", ddl[first+1:])

	// Owner roles of every planned schema (NOLOGIN, no attributes).
	planSchemas := make([]string, 0, len(schemas))
	for schema := range schemas {
		if _, ok := owners[schema]; !ok {
			t.Fatalf("schema %s has no reviewed owner", schema)
		}
		planSchemas = append(planSchemas, schema)
	}
	sort.Strings(planSchemas)
	roleCreated := map[string]bool{}
	for _, schema := range planSchemas {
		owner := rename(owners[schema])
		if !roleCreated[owner] {
			exec("owner role", "CREATE ROLE "+pgx.Identifier{owner}.Sanitize()+" NOLOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS")
			created = append(created, owner)
			roleCreated[owner] = true
		}
	}
	database := pgx.Identifier{name}.Sanitize()

	// Legacy goose baselines of the planned services, as their owners.
	histories := map[string]bool{}
	for _, h := range legacy.Histories {
		if !services[h.Service] {
			continue
		}
		owner := rename(owners[h.Schema])
		history, err := production.HistoryTable(h.Service, h.Schema)
		if err != nil || history != h.History {
			t.Fatalf("legacy history %s: %v", h.Service, err)
		}
		historyDDL, err := production.HistoryDDL(h.Service, h.Schema)
		if err != nil {
			t.Fatal(err)
		}
		exec("legacy history", historyDDL)
		histories[h.Service] = true
		exec("legacy create", "GRANT CREATE ON DATABASE "+database+" TO "+pgx.Identifier{owner}.Sanitize())
		for _, source := range h.Sources {
			full := proofReadBound(t, root, source.Path, source.SourceSHA256)
			up := strings.SplitN(string(full), "-- +goose Down", 2)[0]
			if production.Checksum([]byte(up)) != source.UpSHA256 {
				t.Fatalf("legacy %s up SQL differs from its reviewed hash", source.Path)
			}
			tx, err := conn.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, sql := range []string{"SET LOCAL lock_timeout='2s'; SET LOCAL ROLE " + pgx.Identifier{owner}.Sanitize(), bind([]byte(up)), "RESET ROLE"} {
				if _, err = tx.Exec(ctx, sql); err != nil {
					t.Fatalf("legacy %s: %v", source.Path, err)
				}
			}
			fingerprint, err := production.CatalogFingerprint(ctx, tx, h.Schema)
			if err != nil {
				t.Fatal(err)
			}
			if _, err = tx.Exec(ctx, "INSERT INTO "+history+"(version_id,is_applied) VALUES($1,true)", source.OriginalVersion); err != nil {
				t.Fatal(err)
			}
			if _, err = tx.Exec(ctx, "INSERT INTO migration_control.legacy_baselines(service,original_version,source_checksum,up_checksum,structural_fingerprint) VALUES($1,$2,$3,$4,$5)", h.Service, source.OriginalVersion, source.SourceSHA256, source.UpSHA256, fingerprint); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
		}
		exec("legacy revoke", "REVOKE CREATE ON DATABASE "+database+" FROM "+pgx.Identifier{owner}.Sanitize())
	}

	// proofRuntime roles and schemas: the reviewed ProvisionSQL of every manifest
	// of a planned schema (registered SQL may grant to any of their runtime
	// roles). Schema capabilities outside the plan are not provisioned.
	for _, m := range roleDoc.Roles {
		if !schemas[m.Schema] {
			continue
		}
		scoped := production.RoleManifest{Service: m.Service, Schema: m.Schema, OwnerRole: rename(m.OwnerRole), RuntimeRole: rename(m.RuntimeRole)}
		statements, err := scoped.ProvisionSQL()
		if err != nil {
			t.Fatal(err)
		}
		for _, statement := range statements {
			if strings.HasPrefix(statement, "CREATE ROLE ") {
				role := strings.Trim(strings.SplitN(strings.TrimPrefix(statement, "CREATE ROLE "), " ", 2)[0], `"`)
				if roleCreated[role] {
					continue
				}
				roleCreated[role] = true
				created = append(created, role)
			}
			exec("provision", statement)
		}
	}

	// Owner interfaces (owner_read_grants) of every planned owner, rendered
	// by the reviewed GrantSQL; each is granted once its source exists.
	type ownerRead struct {
		manifest production.RoleManifest
		grant    production.Grant
	}
	ownerReads := []ownerRead{}
	for _, m := range roleDoc.Roles {
		if !roleCreated[rename(m.OwnerRole)] {
			continue
		}
		for _, g := range m.OwnerReadGrants {
			if schemas[g.Schema] {
				scoped := production.RoleManifest{Service: m.Service, Schema: m.Schema, OwnerRole: rename(m.OwnerRole), RuntimeRole: rename(m.RuntimeRole), OwnerReadGrants: []production.Grant{g}}
				ownerReads = append(ownerReads, ownerRead{scoped, g})
			}
		}
	}
	grantOwnerReads := func(tx pgx.Tx, owner string) {
		t.Helper()
		for _, read := range ownerReads {
			if owner != "" && read.manifest.OwnerRole != rename(owner) {
				continue
			}
			var present bool
			if err := tx.QueryRow(ctx, "SELECT to_regclass($1) IS NOT NULL", pgx.Identifier{read.grant.Schema, read.grant.Object}.Sanitize()).Scan(&present); err != nil {
				t.Fatal(err)
			}
			if !present {
				continue
			}
			statements, err := read.manifest.GrantSQL()
			if err != nil {
				t.Fatalf("owner interface %s.%s: %v", read.grant.Schema, read.grant.Object, err)
			}
			for _, statement := range statements {
				if _, err = tx.Exec(ctx, statement); err != nil {
					t.Fatalf("owner interface %s.%s: %v", read.grant.Schema, read.grant.Object, err)
				}
			}
		}
	}

	// The registered migrations, in application order, each as its owner in
	// one transaction with its history row and applied_migrations row.
	registryDigest := production.Checksum(registryBytes)
	db.Applied = map[int64]string{}
	for _, m := range plan {
		raw := proofReadBound(t, root, m.Path, m.SHA256)
		owner := owners[m.Schema]
		history, err := production.HistoryTable(m.Service, m.Schema)
		if err != nil {
			t.Fatal(err)
		}
		if !histories[m.Service] {
			historyDDL, err := production.HistoryDDL(m.Service, m.Schema)
			if err != nil {
				t.Fatal(err)
			}
			exec("history", historyDDL)
			histories[m.Service] = true
		}
		tx, err := conn.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		grantOwnerReads(tx, owner)
		as := pgx.Identifier{rename(owner)}.Sanitize()
		if _, err = tx.Exec(ctx, "GRANT CREATE ON DATABASE "+database+" TO "+as+";\nSET LOCAL lock_timeout='2s'; SELECT pg_advisory_xact_lock(718623014); SET LOCAL ROLE "+as+";\n"+bind(raw)+"\n;\nRESET ROLE;\nREVOKE CREATE ON DATABASE "+database+" FROM "+as); err != nil {
			t.Fatalf("migration %d: %v", m.MigrationID, err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO "+history+"(version_id,is_applied) VALUES($1,true)", m.MigrationID); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO migration_control.applied_migrations(migration_id,service,schema_name,checksum,registry_digest) VALUES($1,$2,$3,$4,$5)", m.MigrationID, m.Service, m.Schema, m.SHA256, registryDigest); err != nil {
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatalf("migration %d: %v", m.MigrationID, err)
		}
		db.Applied[m.MigrationID] = m.SHA256
	}
	// Functions are created EXECUTE-able by PUBLIC; close them (the reviewed
	// grants below re-add exactly the reviewed EXECUTEs).
	for _, schema := range planSchemas {
		owner := pgx.Identifier{rename(owners[schema])}.Sanitize()
		exec("revoke execute", "REVOKE EXECUTE ON ALL FUNCTIONS IN SCHEMA "+pgx.Identifier{schema}.Sanitize()+" FROM PUBLIC")
		exec("default execute", "ALTER DEFAULT PRIVILEGES FOR ROLE "+owner+" REVOKE EXECUTE ON FUNCTIONS FROM PUBLIC")
	}
	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	grantOwnerReads(tx, "")
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	// The reviewed runtime privileges of every requested role.
	for _, m := range requested {
		scoped := m
		scoped.OwnerRole, scoped.RuntimeRole, scoped.OwnerReadGrants, scoped.SchemaCapabilities = rename(m.OwnerRole), rename(m.RuntimeRole), nil, nil
		grants, err := scoped.GrantSQL()
		if err != nil {
			t.Fatal(err)
		}
		for _, statement := range grants {
			exec("runtime grant "+m.RuntimeRole, statement)
		}
	}
	if schemas["recovery"] {
		exec("recovery seed", "INSERT INTO recovery.recovery_control(singleton,epoch,region,revision,checkpoint_sha256) VALUES(true,$1,$2,1,$3)", proofEpoch, proofRegion, registryDigest)
	}

	fingerprints := map[string]string{}
	read, err := conn.BeginTx(ctx, pgx.TxOptions{AccessMode: pgx.ReadOnly})
	if err != nil {
		t.Fatal(err)
	}
	for schema := range allRead {
		if fingerprints[schema], err = production.CatalogFingerprint(ctx, read, schema); err != nil {
			t.Fatalf("fingerprint %s: %v", schema, err)
		}
	}
	if err = read.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	for _, m := range requested {
		role := rename(m.RuntimeRole)
		var password [24]byte
		if _, err = rand.Read(password[:]); err != nil {
			t.Fatal(err)
		}
		secret := hex.EncodeToString(password[:])
		var literal string
		if err = conn.QueryRow(ctx, "SELECT quote_literal($1)", secret).Scan(&literal); err != nil {
			t.Fatal(err)
		}
		exec("runtime login", "ALTER ROLE "+pgx.Identifier{role}.Sanitize()+" LOGIN PASSWORD "+literal)
		runtimeURL := *adminURL
		runtimeURL.User = url.UserPassword(role, secret)
		expected := production.ExpectedSchema{Service: m.Service, RuntimeRole: role, RegistryDigest: registryDigest, InterfaceDigest: production.Checksum(rolesBytes), ReaderVersion: 1, Migrations: db.Applied, Schemas: map[string]string{}, Epoch: proofEpoch, Region: proofRegion, Privileges: m.Grants}
		for schema := range readSchemas[m.RuntimeRole] {
			expected.Schemas[schema] = fingerprints[schema]
		}
		if err = production.RecordRuntimeReadiness(ctx, conn, expected); err != nil {
			t.Fatalf("record readiness %s: %v", m.RuntimeRole, err)
		}
		document, err := json.Marshal(expected)
		if err != nil {
			t.Fatal(err)
		}
		digest := production.Checksum(document)
		if _, err = production.LoadExpectedSchema(document, digest); err != nil {
			t.Fatalf("expected schema document %s: %v", m.RuntimeRole, err)
		}
		db.Runtimes[m.RuntimeRole] = proofRuntime{Role: role, Service: m.Service, DSN: runtimeURL.String(), ExpectedSchema: document, ExpectedSchemaSHA256: digest}
	}
	return db
}
