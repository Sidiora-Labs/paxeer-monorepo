// Proves: LifecycleStore and AdaptV2StreamStart/End/Heartbeat/List -> Handlers.RegisterRoutes over loopback HTTP -> v2.ValidateJSON and typed Go consumers.
//
// Repairs handlers.go:RegisterRoutes/listV2 so fresh observations and refusals
// cannot be cached, v2_list_wire.go:AdaptV2StreamList so an unset observation
// time is refused, and v2_lifecycle_wire.go:marshalV2Lifecycle so malformed
// producer results retain the ErrInvalid classification. Every dependency is
// owned by this proof: Postgres and Redis child processes, a chain-125 geth
// node, and the real HTTP handlers. No operator endpoint or fixture is read.
package streams

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

// These consumers use the exported contract validator before decoding the
// generated types, exactly at the boundary where an SDK consumer admits data.
func proofLivestreamDecode[T any](schema string, raw []byte) (T, error) {
	var result T
	if err := v2.ValidateJSON(schema, raw); err != nil {
		return result, err
	}
	err := json.Unmarshal(raw, &result)
	return result, err
}

func proofLivestreamConsume[T any](t *testing.T, schema string, raw []byte) T {
	t.Helper()
	value, err := proofLivestreamDecode[T](schema, raw)
	if err != nil {
		t.Fatalf("consumer rejected %s from real producer: %v; %s", schema, err, raw)
	}
	// Corrupt an actual producer result, retaining its other fields. The same
	// consumer must reject this semantic error even though it is valid JSON.
	var damaged map[string]json.RawMessage
	if err = json.Unmarshal(raw, &damaged); err != nil {
		t.Fatal(err)
	}
	damaged["requestId"] = json.RawMessage(`42`)
	malformed, err := json.Marshal(damaged)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = proofLivestreamDecode[T](schema, malformed); err == nil {
		t.Fatalf("%s consumer admitted malformed producer requestId", schema)
	}
	return value
}

func proofLivestreamProcess(t *testing.T, credential *syscall.Credential, name string, args ...string) {
	t.Helper()
	log, err := os.CreateTemp("", "livestream-proof-process-*.log")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(log.Name()) })
	cmd := exec.Command(name, args...)
	cmd.Stdout, cmd.Stderr = log, log
	if credential != nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
	}
	if err = cmd.Start(); err != nil {
		_ = log.Close()
		t.Fatalf("missing producer %s: %v", name, err)
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	t.Cleanup(func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		select {
		case err := <-done:
			if err != nil {
				raw, _ := os.ReadFile(log.Name())
				t.Errorf("producer %s exited: %v: %s", name, err, raw)
			}
		case <-time.After(10 * time.Second):
			_ = cmd.Process.Kill()
			<-done
			t.Errorf("producer %s did not stop", name)
		}
		_ = log.Close()
	})
}

func proofLivestreamStores(t *testing.T) (string, *goredis.Client) {
	t.Helper()
	// Discover the installed executable, never an operator-provided database.
	pgConfig, err := exec.LookPath("pg_config")
	if err != nil {
		t.Fatalf("missing Postgres producer executable pg_config: %v", err)
	}
	output, err := exec.Command(pgConfig, "--bindir").Output()
	if err != nil {
		t.Fatalf("Postgres producer pg_config --bindir: %v", err)
	}
	bin := strings.TrimSpace(string(output))
	dir, err := os.MkdirTemp("", "livestream-proof-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	var credential *syscall.Credential
	if os.Geteuid() == 0 {
		owner, err := user.Lookup("postgres")
		if err != nil {
			t.Fatalf("Postgres producer requires its unprivileged postgres account: %v", err)
		}
		uid, err := strconv.ParseUint(owner.Uid, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		gid, err := strconv.ParseUint(owner.Gid, 10, 32)
		if err != nil {
			t.Fatal(err)
		}
		credential = &syscall.Credential{Uid: uint32(uid), Gid: uint32(gid)}
		if err = os.Chown(dir, int(uid), int(gid)); err != nil {
			t.Fatal(err)
		}
	}
	data := filepath.Join(dir, "postgres")
	init := exec.Command(filepath.Join(bin, "initdb"), "-D", data, "-U", "proof", "--auth=trust", "--no-locale", "--encoding=UTF8", "--no-sync")
	if credential != nil {
		init.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
	}
	if raw, err := init.CombinedOutput(); err != nil {
		t.Fatalf("Postgres producer initdb: %v: %s", err, raw)
	}
	proofLivestreamProcess(t, credential, filepath.Join(bin, "postgres"), "-D", data, "-h", "", "-k", dir, "-c", "fsync=off")
	dsn := fmt.Sprintf("host=%s user=proof dbname=postgres sslmode=disable connect_timeout=2", dir)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	for {
		if err = pool.Ping(ctx); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("Postgres producer %s did not become ready: %v", filepath.Join(bin, "postgres"), err)
		case <-time.After(20 * time.Millisecond):
		}
	}
	socket := filepath.Join(dir, "redis.sock")
	proofLivestreamProcess(t, nil, "redis-server", "--port", "0", "--unixsocket", socket, "--save", "", "--appendonly", "no", "--dir", dir)
	client := goredis.NewClient(&goredis.Options{Network: "unix", Addr: socket, MaxRetries: -1})
	t.Cleanup(func() { _ = client.Close() })
	for {
		if err = client.Ping(ctx).Err(); err == nil {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatalf("Redis producer redis-server did not become ready: %v", err)
		case <-time.After(20 * time.Millisecond):
		}
	}
	return dsn, client
}

func proofLivestreamChain(t *testing.T) *sharedauth.WalletOperationVerifier {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err = listener.Close(); err != nil {
		t.Fatal(err)
	}
	chainConfig := *params.AllDevChainProtocolChanges
	chainConfig.ChainID = big.NewInt(125)
	backend := simulated.NewBackend(types.GenesisAlloc{}, func(n *node.Config, e *ethconfig.Config) {
		n.HTTPHost, n.HTTPPort, n.HTTPModules = "127.0.0.1", port, []string{"eth"}
		e.Genesis.Config, e.NetworkId = &chainConfig, 125
	})
	t.Cleanup(func() {
		if err := backend.Close(); err != nil {
			t.Error(err)
		}
	})
	backend.Commit()
	client, err := rpc.Dial(fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("chain-125 producer: %v", err)
	}
	reported := sharedauth.NewReportedHashBackend(client)
	t.Cleanup(reported.Close)
	return &sharedauth.WalletOperationVerifier{Backend: reported}
}

func TestProofLaunchpadLivestream(t *testing.T) {
	ctx := context.Background()
	dsn, redis := proofLivestreamStores(t)
	verifier := proofLivestreamChain(t)
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	wallet := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	poolAddress := "0x2222222222222222222222222222222222222222"
	secret := strings.Repeat("w", 32)
	tokens := &ViewerTokenAuthority{Key: []byte(strings.Repeat("v", 32))}
	var store *LifecycleStore
	var database *pgxpool.Pool
	var server *httptest.Server
	var webhookOutcome string
	open := func() {
		t.Helper()
		var err error
		database, err = pgxpool.New(ctx, dsn)
		if err != nil {
			t.Fatal(err)
		}
		store, err = NewLifecycleStore(database, "livestream", 3, 20)
		if err != nil {
			t.Fatal(err)
		}
		viewers, err := NewViewerCounter(redis, "proof_livestream", 100)
		if err != nil {
			t.Fatal(err)
		}
		handlers := New(Deps{
			Lifecycle: store, Restrictions: &RestrictionReader{Pool: database},
			Viewers: viewers, ViewerTokens: tokens, WebhookSecret: secret,
			OperationVerifier: verifier,
			OperationLedger:   &sharedauth.Ledger{Pool: database, Environment: "test", AuditKey: []byte(strings.Repeat("a", 32))},
			// This harness authorizes the real owner only after verifying the
			// actual signed operation against the real chain and EIP-191 code.
			Authorize: func(ctx context.Context, r *http.Request, action string, o Operation) (Authorization, error) {
				raw, err := base64.RawURLEncoding.DecodeString(r.Header.Get("X-Kindle-Operation"))
				if err != nil {
					return Authorization{}, ErrDenied
				}
				var e sharedauth.OperationEnvelope
				if err = json.Unmarshal(raw, &e); err != nil {
					return Authorization{}, ErrDenied
				}
				statement, err := e.Statement()
				if err != nil || e.Action != action || e.Actor != o.Wallet {
					return Authorization{}, ErrDenied
				}
				if err = verifier.Verify(ctx, e.Actor, statement, r.Header.Get("X-Kindle-Signature")); err != nil {
					return Authorization{}, ErrDenied
				}
				return Authorization{Wallet: e.Actor, Pool: o.Pool, Allowed: true, ExpiresAt: time.Unix(e.ExpiresAt, 0)}, nil
			},
			StudioWebhook:       true,
			WebhookMilliseconds: true,
			ObserveOutcome:      func(outcome string) { webhookOutcome = outcome },
		})
		router := chi.NewRouter()
		handlers.RegisterRoutes(router)
		server = httptest.NewServer(router)
	}
	open()
	t.Cleanup(func() { server.Close(); database.Close() })
	if _, err = database.Exec(ctx, "CREATE SCHEMA livestream; CREATE SCHEMA chat; CREATE TABLE chat.safety_bans(wallet text,revoked_at timestamptz,expires_at timestamptz,scope text,resource text); CREATE VIEW chat.safety_bans_v1 AS SELECT * FROM chat.safety_bans;"+store.SchemaSQL()+store.PlaybackPrivacySchemaSQL()+";"+sharedauth.LedgerDDL); err != nil {
		t.Fatal(err)
	}
	request := CreateIntent{Wallet: wallet, Pool: poolAddress, Title: "Launchpad proof stream", IdempotencyKey: "proof"}
	decision := CreatorDecision{Wallet: wallet, Pool: poolAddress, Allowed: true, ExpiresAt: time.Now().Add(5 * time.Minute)}
	operation, err := store.Create(ctx, request, decision, true)
	if err != nil {
		t.Fatal(err)
	}
	operation, err = store.BindProvider(ctx, operation.ID, "proof_provider")
	if err != nil {
		t.Fatal(err)
	}
	// Drop the serving component and every DB connection before consumption.
	server.Close()
	database.Close()
	open()
	again, err := store.Create(ctx, request, decision, true)
	if err != nil || again != operation {
		t.Fatalf("reopened producer lost idempotent state: %+v, %v", again, err)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	defer client.CloseIdleConnections()
	exchange := func(method, path string, raw []byte, headers map[string]string, status int) []byte {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, bytes.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		for name, value := range headers {
			req.Header.Set(name, value)
		}
		response, err := client.Do(req)
		if err != nil {
			t.Fatalf("real HTTP crossing %s: %v", path, err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != status || response.Header.Get("Content-Type") != "application/json" || response.Header.Get("Cache-Control") != "no-store" {
			t.Fatalf("%s: status=%d headers=%v body=%s; want %d JSON/no-store", path, response.StatusCode, response.Header, body, status)
		}
		return body
	}
	getPage := func(wantStatus string, wantRevision int64) {
		t.Helper()
		previous := ""
		for i := 0; i < 2; i++ {
			raw := exchange(http.MethodGet, "/v2/streams?poolAddress="+poolAddress, nil, nil, http.StatusOK)
			page := proofLivestreamConsume[v2.StreamListResponse](t, "StreamListResponse", raw)
			projection := proofLivestreamConsume[v2StreamListEnvelope](t, "StreamListResponse", raw)
			if page.RequestId == previous {
				t.Fatal("list reused cached producer result")
			}
			previous = page.RequestId
			if len(page.Data.Items) != 1 {
				t.Fatalf("list lost reopened state: %s", raw)
			}
			item := page.Data.Items[0]
			if item.StreamId != operation.ID || item.Title != request.Title || item.Status != wantStatus || item.CreatorWallet != wallet || item.PoolAddress != poolAddress || projection.Data.Items[0].Revision != strconv.FormatInt(wantRevision, 10) || page.Revision == nil || string(*page.Revision) != strconv.FormatInt(wantRevision, 10) {
				t.Fatalf("consumer observed wrong stream: %s", raw)
			}
		}
	}
	getPage("starting", operation.Revision)
	refused := proofLivestreamConsume[v2.Error](t, "Error", exchange(http.MethodGet, "/v2/streams?limit=0", nil, nil, http.StatusBadRequest))
	if refused.Code != "INVALID_REQUEST" || refused.Retryable {
		t.Fatal("invalid list request accepted", refused)
	}
	sign := func(path, action string, body []byte, signer *ecdsa.PrivateKey) map[string]string {
		t.Helper()
		hash, err := sharedauth.BodyDigest(body)
		if err != nil {
			t.Fatal(err)
		}
		var nonce [32]byte
		if _, err = rand.Read(nonce[:]); err != nil {
			t.Fatal(err)
		}
		now := time.Now().Unix()
		e := sharedauth.OperationEnvelope{Version: 1, ChainID: "125", Environment: "test", Actor: wallet, Action: "stream." + action, Method: "POST", Path: path, BodyHash: hash, Nonce: hex.EncodeToString(nonce[:]), IssuedAt: now, ExpiresAt: now + 300}
		statement, err := e.Statement()
		if err != nil {
			t.Fatal(err)
		}
		sig, err := crypto.Sign(accounts.TextHash([]byte(statement)), signer)
		if err != nil {
			t.Fatal(err)
		}
		encoded, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		return map[string]string{"X-Kindle-Operation": base64.RawURLEncoding.EncodeToString(encoded), "X-Kindle-Signature": hexutil.Encode(sig)}
	}
	action := func(name, schema, state string) {
		t.Helper()
		path := "/v2/streams/" + operation.ID + ":" + name
		for i := 0; i < 2; i++ {
			body := []byte(fmt.Sprintf(`{"expectedRevision":%q}`, strconv.FormatInt(operation.Revision, 10)))
			if i == 0 {
				wrong, err := crypto.GenerateKey()
				if err != nil {
					t.Fatal(err)
				}
				denied := proofLivestreamConsume[v2.Error](t, "Error", exchange("POST", path, body, sign(path, name, body, wrong), http.StatusForbidden))
				if denied.Code != "SCOPE_DENIED" {
					t.Fatal(denied)
				}
			}
			headers := sign(path, name, body, key)
			wantRevision := operation.Revision + 1
			raw := exchange("POST", path, body, headers, http.StatusAccepted)
			var observed v2.StreamOperation
			var revision *v2.PositiveUInt256
			if name == "end" {
				result := proofLivestreamConsume[v2.StreamEndResponse](t, schema, raw)
				observed, revision = result.Data.Operation, result.Revision
			} else {
				result := proofLivestreamConsume[v2.StreamStartResponse](t, schema, raw)
				observed, revision = result.Data.Operation, result.Revision
			}
			if observed.StreamId != operation.ID || observed.OperationId != operation.ID || observed.Status != state || string(observed.Revision) != strconv.FormatInt(wantRevision, 10) || revision == nil || *revision != observed.Revision {
				t.Fatalf("wrong lifecycle result: %s", raw)
			}
			// A signed replay crosses HTTP and the durable ledger again.
			replay := exchange("POST", path, body, headers, http.StatusAccepted)
			if !bytes.Equal(replay, raw) {
				t.Fatalf("signed replay changed response: %s / %s", raw, replay)
			}
			operation, err = store.Get(ctx, operation.ID)
			if err != nil || operation.Revision != wantRevision {
				t.Fatalf("transition lost: %+v %v", operation, err)
			}
		}
	}
	action("start", "StreamStartResponse", "ready")
	webhook := func(event ProviderEvent, valid bool, wantStatus int) []byte {
		t.Helper()
		deliveredAt := time.Now().UTC().Truncate(time.Millisecond)
		kind := "stream.started"
		if event.State == "ENDED" {
			kind = "stream.idle"
		}
		raw := repairM1001StudioBody(t, event.EventID, event.ProviderID, kind, event.ObservedAt, deliveredAt)
		signingSecret := secret
		if !valid {
			signingSecret = strings.Repeat("x", 32)
		}
		mac := hmac.New(sha256.New, []byte(signingSecret))
		_, _ = mac.Write(raw)
		return exchange("POST", "/webhooks/livepeer", raw, map[string]string{"Livepeer-Signature": fmt.Sprintf("t=%d,v1=%s", deliveredAt.UnixMilli(), hex.EncodeToString(mac.Sum(nil)))}, wantStatus)
	}
	event := ProviderEvent{EventID: "proof_live", ProviderID: operation.ProviderID, State: "LIVE", ObservedAt: time.Now().UTC().Truncate(time.Millisecond)}
	denied := proofLivestreamConsume[v2.Error](t, "Error", webhook(event, false, http.StatusUnauthorized))
	if denied.Code != "AUTH_EXPIRED" {
		t.Fatal(denied)
	}
	beforeWebhookRevision := operation.Revision
	for i, want := range []string{"applied", "duplicate"} {
		if i > 0 {
			server.Close()
			database.Close()
			open()
		}
		raw := webhook(event, true, http.StatusOK)
		observed := proofLivestreamConsume[v2.LiveWebhookResponse](t, "LiveWebhookResponse", raw)
		if !observed.Data.Accepted || webhookOutcome != want {
			t.Fatalf("webhook %d: outcome=%s body=%s", i, webhookOutcome, raw)
		}
	}
	operation, err = store.Get(ctx, operation.ID)
	if err != nil || operation.State != "LIVE" || operation.Revision != beforeWebhookRevision+1 || operation.ProviderTime == nil || !operation.ProviderTime.Equal(event.ObservedAt) {
		t.Fatalf("webhook did not durably produce LIVE exactly once: %+v %v", operation, err)
	}
	counter, err := NewViewerCounter(redis, "proof_livestream", 100)
	if err != nil {
		t.Fatal(err)
	}
	if err = counter.Initialize(ctx, operation.ID); err != nil {
		t.Fatal(err)
	}
	for i := int64(1); i <= 2; i++ {
		session, err := randomID()
		if err != nil {
			t.Fatal(err)
		}
		token, _, err := tokens.IssuePolicyBound(operation.ID, session, 0, time.Now())
		if err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(v2.StreamHeartbeatBody{ViewerToken: token, PlaybackSessionId: session})
		if err != nil {
			t.Fatal(err)
		}
		path := "/v2/streams/" + operation.ID + "/viewers:heartbeat"
		raw := exchange("POST", path, body, nil, http.StatusOK)
		result := proofLivestreamConsume[v2.StreamHeartbeatResponse](t, "StreamHeartbeatResponse", raw)
		if int64(result.Data.ViewerCount) != i || result.Data.ViewerQuality != "stale" {
			t.Fatalf("consumer did not observe fresh Redis count/recovery quality: %s", raw)
		}
		limited := proofLivestreamConsume[v2.Error](t, "Error", exchange("POST", path, body, nil, http.StatusTooManyRequests))
		if limited.Code != "RATE_LIMITED" {
			t.Fatal(limited)
		}
	}
	action("end", "StreamEndResponse", "ending")
	getPage("ending", operation.Revision)
	var inbox, starts, ends int
	if err = database.QueryRow(ctx, "SELECT count(*) FROM livestream.stream_inbox").Scan(&inbox); err != nil {
		t.Fatal(err)
	}
	if err = database.QueryRow(ctx, "SELECT count(*) FILTER(WHERE kind='start'),count(*) FILTER(WHERE kind='end') FROM livestream.stream_outbox").Scan(&starts, &ends); err != nil {
		t.Fatal(err)
	}
	if inbox != 1 || starts != 2 || ends != 2 {
		t.Fatalf("refusal/replay duplicated effects: inbox=%d starts=%d ends=%d", inbox, starts, ends)
	}
}

func TestProofLaunchpadLivestreamMalformedProducers(t *testing.T) {
	const id = "11111111-1111-4111-8111-111111111111"
	at := time.Now()
	operation := Operation{ID: id, State: "READY", Revision: 1}
	for _, adapt := range []func(string, Operation) ([]byte, error){AdaptV2StreamStart, AdaptV2StreamEnd} {
		if raw, err := adapt("bad-request-id", operation); raw != nil || !errors.Is(err, ErrInvalid) {
			t.Fatalf("malformed lifecycle result must be ErrInvalid: %s %v", raw, err)
		}
	}
	if raw, err := AdaptV2StreamHeartbeat("bad-request-id", id, 1, at); raw != nil || !errors.Is(err, ErrInvalid) {
		t.Fatalf("malformed heartbeat must be ErrInvalid: %s %v", raw, err)
	}
	if raw, err := AdaptV2StreamList(id, nil, nil, time.Time{}); raw != nil || !errors.Is(err, ErrInvalid) {
		t.Fatalf("unset list time must be refused: %s %v", raw, err)
	}
	item, err := AdaptV2Stream(Operation{ID: id, Pool: "0x" + strings.Repeat("1", 40), Wallet: "0x" + strings.Repeat("2", 40), Title: "proof", State: "READY", Revision: 1}, "", nil, "unavailable")
	if err != nil {
		t.Fatal(err)
	}
	item.Revision = "01"
	if raw, err := AdaptV2StreamList(id, []V2Stream{item}, nil, at); raw != nil || !errors.Is(err, ErrInvalid) {
		t.Fatalf("malformed list revision must be refused: %s %v", raw, err)
	}
}
