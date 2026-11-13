package httpapi

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/publisher"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store/canonicaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This peer stores transactions and operator records in memory. The components
// under repair are the real mounted handlers, Store, canonical committer, signed
// assertion issuer/verifier and Publisher; no external database is required.
type repair996State struct {
	height, cursor, outbox, quarantine int64
	commits                            int
	age                                float64
}
type repair996DB struct {
	mu        sync.Mutex
	state     repair996State
	failures  map[string]bool
	sessions  map[string]repair996Session
	letters   []deadLetterMetadata
	decisions [][]any
	reads     int
}
type repair996Session struct {
	actor, audience, wallet string
	issued, expires, mfa    time.Time
	scopes                  []string
	revoked                 bool
}

func repair996NewDB() *repair996DB {
	return &repair996DB{state: repair996State{height: 100, cursor: 7}, failures: map[string]bool{}, sessions: map[string]repair996Session{}}
}

type repair996Row struct {
	values []any
	err    error
}

func (r repair996Row) Scan(dest ...any) error {
	if r.err != nil {
		return r.err
	}
	if len(dest) != len(r.values) {
		return fmt.Errorf("column count %d != %d", len(dest), len(r.values))
	}
	for i, d := range dest {
		target := reflect.ValueOf(d).Elem()
		value := reflect.ValueOf(r.values[i])
		if !value.IsValid() {
			target.SetZero()
		} else if value.Type().AssignableTo(target.Type()) {
			target.Set(value)
		} else {
			return fmt.Errorf("column %d: %T into %T", i, r.values[i], d)
		}
	}
	return nil
}
func (db *repair996DB) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.queryRow(ctx, sql, args...)
}
func (db *repair996DB) queryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	row := func(v ...any) pgx.Row { return repair996Row{values: v} }
	if err := ctx.Err(); err != nil {
		return repair996Row{err: err}
	}
	for fragment, fail := range db.failures {
		if fail && strings.Contains(sql, fragment) {
			return repair996Row{err: errors.New("private-database-error")}
		}
	}
	switch {
	case strings.Contains(sql, "FROM indexer.indexer_ingest_watermarks"):
		if args[0] != int32(125) || args[1] != "live" {
			return repair996Row{err: pgx.ErrNoRows}
		}
		return row(db.state.height, db.state.height)
	case strings.Contains(sql, "SELECT fence_token FROM indexer.indexer_ingest_leases"):
		if args[2] != "owner" {
			return repair996Row{err: pgx.ErrNoRows}
		}
		return row(int64(1))
	case strings.Contains(sql, "-- name: GetCursor"):
		return row(int32(125), db.state.cursor, pgtype.Timestamp{Time: time.Now(), Valid: true})
	case strings.Contains(sql, "-- name: GetPoolCount"), strings.Contains(sql, "-- name: GetSwapCount"):
		return row(int64(0))
	case strings.Contains(sql, `"operator_sessions"`):
		session, ok := db.sessions[args[0].(string)]
		if !ok || session.revoked || session.wallet != args[1] || !session.expires.After(time.Now()) {
			return repair996Row{err: pgx.ErrNoRows}
		}
		return row(session.actor, session.audience, session.issued, session.expires, session.mfa, session.scopes)
	}
	return repair996Row{err: fmt.Errorf("unsupported memory query: %s", sql)}
}
func (db *repair996DB) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	return db.exec(ctx, sql, args...)
}
func (db *repair996DB) exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	if err := ctx.Err(); err != nil {
		return pgconn.CommandTag{}, err
	}
	for fragment, fail := range db.failures {
		if fail && strings.Contains(sql, fragment) {
			return pgconn.CommandTag{}, errors.New("private-database-error")
		}
	}
	switch {
	case strings.HasPrefix(sql, "INSERT INTO indexer.indexer_raw_logs"), strings.HasPrefix(sql, "INSERT INTO indexer.indexer_block_completeness"):
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	case strings.HasPrefix(sql, "INSERT INTO indexer.indexer_ingest_outbox"):
		db.state.outbox++
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	case strings.HasPrefix(sql, "UPDATE indexer.indexer_ingest_watermarks"):
		if db.state.height != args[2] || args[4] != int64(1) || args[5] != "owner" {
			return pgconn.NewCommandTag("UPDATE 0"), nil
		}
		db.state.height = args[3].(int64)
		return pgconn.NewCommandTag("UPDATE 1"), nil
	case strings.HasPrefix(sql, "INSERT INTO indexer.indexer_ingest_commits"):
		db.state.commits++
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	case strings.Contains(sql, `"authorization_decisions"`):
		db.decisions = append(db.decisions, append([]any(nil), args...))
		return pgconn.NewCommandTag("INSERT 0 1"), nil
	}
	return pgconn.CommandTag{}, fmt.Errorf("unsupported memory mutation: %s", sql)
}

type repair996Rows struct {
	pgx.Rows
	values [][]any
	index  int
}

func (r *repair996Rows) Next() bool { r.index++; return r.index <= len(r.values) }
func (r *repair996Rows) Scan(dest ...any) error {
	return (repair996Row{values: r.values[r.index-1]}).Scan(dest...)
}
func (r *repair996Rows) Close()     {}
func (r *repair996Rows) Err() error { return nil }
func (db *repair996DB) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if !strings.Contains(sql, `"delivery_quarantine"`) {
		return nil, fmt.Errorf("unsupported memory rows: %s", sql)
	}
	if db.failures["quarantine"] {
		return nil, errors.New("private-quarantine-error")
	}
	db.reads++
	rows := &repair996Rows{}
	for _, entry := range db.letters {
		key := entry.EnvelopeHash + "\x00" + entry.Consumer + "\x00" + entry.Generation
		after := args[0].(string) + "\x00" + args[1].(string) + "\x00" + args[2].(string)
		if key > after {
			rows.values = append(rows.values, []any{entry.Consumer, entry.Generation, entry.EnvelopeHash, entry.FailureClass, entry.Created})
		}
		if len(rows.values) == args[3].(int) {
			break
		}
	}
	return rows, nil
}

type repair996Tx struct {
	pgx.Tx
	db     *repair996DB
	before repair996State
	closed bool
}

func (db *repair996DB) Begin(context.Context) (pgx.Tx, error) {
	db.mu.Lock()
	return &repair996Tx{db: db, before: db.state}, nil
}
func (tx *repair996Tx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	return tx.db.exec(ctx, sql, args...)
}
func (tx *repair996Tx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	return tx.db.queryRow(ctx, sql, args...)
}
func (tx *repair996Tx) Commit(context.Context) error {
	if tx.closed {
		return pgx.ErrTxClosed
	}
	tx.closed = true
	tx.db.mu.Unlock()
	return nil
}
func (tx *repair996Tx) Rollback(context.Context) error {
	if tx.closed {
		return pgx.ErrTxClosed
	}
	tx.db.state = tx.before
	tx.closed = true
	tx.db.mu.Unlock()
	return nil
}
func (db *repair996DB) Stats(ctx context.Context) (delivery.Stats, error) {
	db.mu.Lock()
	defer db.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return delivery.Stats{}, err
	}
	return delivery.Stats{PendingRows: db.state.outbox, QuarantinedRows: db.state.quarantine, OldestUnpublishedAgeSeconds: db.state.age}, nil
}
func repair996GetStatus(t *testing.T, deps StatusDeps) (int, map[string]any) {
	t.Helper()
	r := chi.NewRouter()
	RegisterStatus(r, deps)
	server := httptest.NewServer(r)
	defer server.Close()
	response, err := server.Client().Get(server.URL + "/status")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	return response.StatusCode, body
}
func TestRepairM_996_CanonicalCommitsAndDurableHealth(t *testing.T) {
	db := repair996NewDB()
	st := store.NewFromDBTX(db)
	hash := "0x" + strings.Repeat("1", 64)
	txHash := "0x" + strings.Repeat("2", 64)
	batch := store.CanonicalBatch{ChainID: 125, StreamID: "live", Owner: "owner", BatchID: "one", Fence: 1, PreviousHeight: 100,
		Blocks:  []store.BlockCompleteness{{Height: 101, Hash: hash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true}},
		Records: []store.RawRecord{{TransactionHash: txHash, BlockHash: hash, Emitter: "0x" + strings.Repeat("3", 40), Height: 101}},
		Outbox:  []store.OutboxIntent{{TransactionHash: txHash, Kind: "Swap", Payload: json.RawMessage(`{"amount":"1"}`)}}}
	if err := st.CommitCanonical(context.Background(), batch, nil); err != nil {
		t.Fatal(err)
	}
	if db.state.commits != 1 {
		t.Fatal("canonical commit absent", db.state)
	}
	deps := StatusDeps{Store: st, ChainID: 125, Mode: StatusCanonical, CanonicalStreamID: "live", Delivery: db, MaxDeliveryLag: time.Minute, StartTime: time.Now(), Publisher: publisher.New(publisher.Options{})}
	code, body := repair996GetStatus(t, deps)
	if code != 200 || body["status"] != "ok" || body["mode"] != "canonical" || body["lastProcessedBlock"] != float64(101) || body["canonicalStreamId"] != "live" {
		t.Fatal(code, body)
	}
	if _, ok := body["webhooks"]; ok {
		t.Fatal("canonical status exposed compatibility metrics", body)
	}
	health := body["delivery"].(map[string]any)
	if health["pendingRows"] != float64(1) {
		t.Fatal("committed delivery intent absent", body)
	}
	db.state.quarantine = 1
	code, body = repair996GetStatus(t, deps)
	if code != 503 || body["status"] != "degraded" {
		t.Fatal(code, body)
	}
	db.state.quarantine = 0
	db.state.age = 61
	code, body = repair996GetStatus(t, deps)
	if code != 503 || body["status"] != "degraded" {
		t.Fatal("delivery lag budget ignored", code, body)
	}
	db.state.age = 60
	code, body = repair996GetStatus(t, deps)
	if code != 200 || body["status"] != "ok" {
		t.Fatal("delivery lag boundary rejected", code, body)
	}
	deps.MaxDeliveryLag = 0
	code, body = repair996GetStatus(t, deps)
	if code != 503 || body["status"] != "unavailable" {
		t.Fatal("missing delivery budget advertised health", code, body)
	}
	deps.MaxDeliveryLag = time.Minute
	deps.CanonicalStreamID = "other"
	code, body = repair996GetStatus(t, deps)
	if code != 503 || body["status"] != "unavailable" || body["lastProcessedBlock"] != nil {
		t.Fatal("wrong stream fell back to cursor", code, body)
	}
}
func TestRepairM_996_LegacyAndFailedCounts(t *testing.T) {
	db := repair996NewDB()
	deps := StatusDeps{Store: store.NewFromDBTX(db), ChainID: 125, Mode: StatusLegacy, StartTime: time.Now(), Publisher: publisher.New(publisher.Options{})}
	code, body := repair996GetStatus(t, deps)
	if code != 200 || body["mode"] != "legacy" || body["lastProcessedBlock"] != float64(7) || body["totalPoolsIndexed"] != float64(0) || body["webhooks"] == nil {
		t.Fatal(code, body)
	}
	db.failures["GetPoolCount"] = true
	code, body = repair996GetStatus(t, deps)
	if code != 503 || body["status"] != "unavailable" || body["totalPoolsIndexed"] != nil || body["totalSwapsIndexed"] != float64(0) {
		t.Fatal(code, body)
	}
	wire, _ := json.Marshal(body)
	if strings.Contains(string(wire), "private-database-error") {
		t.Fatal("private error disclosed", body)
	}
	deps.Mode = ""
	code, body = repair996GetStatus(t, deps)
	if code != 503 || body["lastProcessedBlock"] != nil {
		t.Fatal("unconfigured adapter advertised health", code, body)
	}
}
func TestRepairM_996_RealDatabaseOutage(t *testing.T) {
	pool, err := pgxpool.New(context.Background(), "postgres://fixture:fixture@127.0.0.1:1/fixture?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	pool.Close()
	durable, err := delivery.NewStore(pool, "indexer", delivery.Policy{Subjects: map[string]bool{"events.swap": true}, EventTypes: map[string]bool{"Swap": true}})
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []StatusMode{StatusCanonical, StatusLegacy} {
		deps := StatusDeps{Store: store.New(pool), ChainID: 125, Mode: mode, CanonicalStreamID: "live", Delivery: durable, MaxDeliveryLag: time.Minute, StartTime: time.Now()}
		code, body := repair996GetStatus(t, deps)
		if code != 503 || body["status"] != "unavailable" || body["lastProcessedBlock"] != nil || body["totalPoolsIndexed"] != nil || body["totalSwapsIndexed"] != nil {
			t.Fatal(mode, code, body)
		}
		if mode == StatusCanonical && body["delivery"] != nil {
			t.Fatal("failed durable read reported metrics", body)
		}
	}
	db := repair996NewDB()
	code, body := repair996GetStatus(t, StatusDeps{Store: store.NewFromDBTX(db), ChainID: 125, Mode: StatusCanonical, CanonicalStreamID: "live", Delivery: durable, MaxDeliveryLag: time.Minute, StartTime: time.Now()})
	if code != 503 || body["status"] != "unavailable" || body["lastProcessedBlock"] != float64(100) || body["delivery"] != nil {
		t.Fatal("durable outage hidden by healthy ingestion", code, body)
	}
}

func TestRepairM_996_DeadLetterAuthorizationAndRetention(t *testing.T) {
	db := repair996NewDB()
	operatorSession, ordinarySession, accountID, requestID := uuid.NewString(), uuid.NewString(), uuid.NewString(), uuid.NewString()
	policy := delivery.Policy{Subjects: map[string]bool{"events.swap": true}, EventTypes: map[string]bool{"Swap": true}}
	payload := json.RawMessage(`{"token":"private-payload-canary","target":"https://private-target.invalid"}`)
	event := delivery.Envelope{EventID: "quarantined-event", EventType: "Swap", SchemaVersion: 1, ChainID: 125, SourceOrder: delivery.SourceOrder{Height: "101", TransactionIndex: "0", LogIndex: "0"}, EmittedAt: time.Now().UTC(), TraceID: "quarantine-trace", PayloadHash: delivery.Hash(payload), Payload: payload}
	wire, err := event.Encode(policy, "events.swap")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = delivery.Decode(wire, policy, "events.swap"); err != nil {
		t.Fatal(err)
	}
	hash := delivery.Hash(wire)
	db.letters = []deadLetterMetadata{{Consumer: "stats", Generation: "g1", EnvelopeHash: hash, FailureClass: "private-error-canary", Created: time.Now().UTC()}}
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	certPublic, certKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(996), Subject: pkix.Name{CommonName: "repair996-workload"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, certPublic, certKey)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(certificate)
	pair := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: certKey}
	fingerprint := sha256.Sum256(der)
	transport := auth.AssertionTransport{AssertionHeader: "X-Service-Assertion", SignatureHeader: "X-Service-Signature"}
	verifier := auth.ServiceAssertionVerifier{Transport: transport, Issuer: "gateway", Audience: "indexer", PeerCertSHA256: hex.EncodeToString(fingerprint[:]), Key: public}
	operations := AuditedDeadLetters{DB: db, Verifier: verifier, DeliverySchema: "indexer", SecuritySchema: "security", Audience: "indexer", PolicyVersion: "repair996"}
	router := chi.NewRouter()
	RegisterStatus(router, StatusDeps{DeadLetters: operations})
	server := httptest.NewUnstartedServer(router)
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{pair}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: roots}
	server.StartTLS()
	defer server.Close()
	clientTransport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, Certificates: []tls.Certificate{pair}}}
	defer clientTransport.CloseIdleConnections()
	client := &http.Client{Transport: clientTransport, Timeout: 5 * time.Second}
	wallet, err := auth.NormalizeWallet("0x52908400098527886e0f7030069857d2e4169ee7")
	if err != nil {
		t.Fatal(err)
	}
	issuer := auth.ServiceAssertionIssuer{Issuer: "gateway", Audience: "indexer", Key: key, Transport: transport}
	db.sessions[operatorSession] = repair996Session{actor: "operator-a", audience: "indexer", wallet: wallet, issued: now.Add(-time.Second), expires: now.Add(time.Minute), mfa: now.Add(time.Minute), scopes: []string{"inspect"}}
	send := func(method, path, session string, want int, alter func(*http.Request)) string {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Actor-Role", "operator")
		req.Header.Set("X-Request-ID", requestID)
		if session != "" {
			principal := auth.IdentityPrincipal{AccountID: accountID, SessionID: session, Wallet: wallet, ProofAt: time.Now(), AccessExpiresAt: time.Now().Add(time.Minute)}
			if err := issuer.SignRequest(req, principal, []string{"operator", "inspect", "retention-delete"}, nil); err != nil {
				t.Fatal(err)
			}
		}
		if alter != nil {
			alter(req)
		}
		response, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		body, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != want {
			t.Fatalf("%s %s: got %d want %d: %s", method, path, response.StatusCode, want, body)
		}
		for _, secret := range []string{"private-payload-canary", "private-target.invalid", "private-error-canary", "private-database-error", "private-quarantine-error"} {
			if strings.Contains(string(body), secret) {
				t.Fatal("private delivery data disclosed", string(body))
			}
		}
		return string(body)
	}
	send("GET", "/status/dead-letter", "", 401, nil)
	send("POST", "/status/dead-letter/drain", "", 401, nil)
	send("GET", "/status/dead-letter", ordinarySession, 403, nil)
	if db.reads != 0 {
		t.Fatal("unauthorized inspection reached durable data", db.reads)
	}
	body := send("GET", "/status/dead-letter?limit=1", operatorSession, 200, nil)
	var page struct {
		Entries []deadLetterMetadata `json:"entries"`
		Next    string               `json:"nextCursor"`
	}
	if err := json.Unmarshal([]byte(body), &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 1 || page.Entries[0].EnvelopeHash != hash || page.Entries[0].FailureClass != "unclassified" || page.Next == "" {
		t.Fatal("durable metadata/redaction/pagination", body)
	}
	next := send("GET", "/status/dead-letter?limit=1&after="+page.Next, operatorSession, 200, nil)
	if !strings.Contains(next, `"entries":[]`) {
		t.Fatal("pagination repeated durable entry", next)
	}
	for _, query := range []string{"?limit=0", "?limit=101", "?limit=no", "?after=invalid"} {
		send("GET", "/status/dead-letter"+query, operatorSession, 400, nil)
	}
	send("GET", "/status/dead-letter", operatorSession, 401, func(r *http.Request) { r.Header.Set("X-Request-ID", uuid.NewString()) })
	send("POST", "/status/dead-letter/drain", operatorSession, 403, nil)
	session := db.sessions[operatorSession]
	session.scopes = []string{"inspect", "retention-delete"}
	db.sessions[operatorSession] = session
	send("POST", "/status/dead-letter/drain", operatorSession, 403, nil)
	decision := db.decisions[len(db.decisions)-1]
	if decision[1] != "retention-delete" || decision[4] != true {
		t.Fatal("retention authority was not audited before requiring separate approval", decision)
	}
	if len(db.letters) != 1 || db.letters[0].EnvelopeHash != hash {
		t.Fatal("drain deleted quarantine", db.letters)
	}
	session.mfa = now.Add(-time.Second)
	db.sessions[operatorSession] = session
	send("GET", "/status/dead-letter", operatorSession, 403, nil)
	session.mfa = now.Add(time.Minute)
	session.audience = "other"
	db.sessions[operatorSession] = session
	send("GET", "/status/dead-letter", operatorSession, 403, nil)
	session.audience = "indexer"
	session.revoked = true
	db.sessions[operatorSession] = session
	send("GET", "/status/dead-letter", operatorSession, 403, nil)
	session.revoked = false
	db.sessions[operatorSession] = session
	db.failures[`"authorization_decisions"`] = true
	reads := db.reads
	send("GET", "/status/dead-letter", operatorSession, 503, nil)
	if db.reads != reads {
		t.Fatal("inspection proceeded without durable audit")
	}
	db.failures[`"authorization_decisions"`] = false
	db.failures["quarantine"] = true
	send("GET", "/status/dead-letter", operatorSession, 503, nil)
	if len(db.decisions) < 10 {
		t.Fatal("missing durable allow/deny decisions", db.decisions)
	}
	for _, decision := range db.decisions {
		if decision[3] != "repair996" {
			t.Fatal("missing policy revision", decision)
		}
	}
}

func TestRepairM_996_NoCompatibilityDeadLetterEscape(t *testing.T) {
	ctx := context.Background()
	_, pool := canonicaltest.New(t)
	for _, schema := range []func(string) (string, error){delivery.SchemaDDL, delivery.WebhookDDL} {
		ddl, err := schema("indexer")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	policy := delivery.Policy{Subjects: map[string]bool{"events.swap": true}, EventTypes: map[string]bool{"Swap": true}}
	durable, err := delivery.NewStore(pool, "indexer", policy)
	if err != nil {
		t.Fatal(err)
	}
	received := make(chan struct{}, 1)
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case received <- struct{}{}:
		default:
		}
		http.Error(w, "private-target-failure", http.StatusBadRequest)
	}))
	defer target.Close()
	t.Setenv("REPAIR162_WEBHOOK_SECRET", strings.Repeat("private-webhook-key", 2))
	authorizer := func(c context.Context, action string) error {
		return store.New(pool).CheckOperationAuthorization(c, store.OperationScope{AuthorizationID: "unissued", Operator: "repair996", Action: action, Target: "webhook", Generation: "repair996", Revision: "repair996", From: 101, To: 101})
	}
	bridge, err := delivery.NewWebhookBridge(durable, []delivery.WebhookTarget{{ID: "private-target", URL: target.URL, SecretReference: "REPAIR162_WEBHOOK_SECRET", Subjects: policy.Subjects}}, target.Client(), secrets.EnvProvider{}, authorizer, "repair996", 10*time.Second, time.Second, time.Second, 1)
	if err != nil {
		t.Fatal(err)
	}
	stager, err := publisher.NewDurableStager(durable, "repair996", map[string]string{"Swap": "events.swap"})
	if err != nil {
		t.Fatal(err)
	}
	event := publisher.RawCanonicalEvent{EventType: "Swap", TxHash: "0x" + strings.Repeat("9", 64), TraceID: "repair996", SourceOrder: delivery.SourceOrder{Height: "101", TransactionIndex: "0", LogIndex: "0"}, EmittedAt: time.Now().UTC(), Payload: json.RawMessage(`{"canary":"private-payload-canary"}`)}
	subject, envelope, err := stager.Envelope(event)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	if err = stager.Stage(ctx, tx, event); err != nil {
		t.Fatal(err)
	}
	if err = bridge.StageTx(ctx, tx, "repair996", "market:private", 1, envelope.EventID, "", subject, envelope); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err = bridge.RunOnce(ctx); err == nil {
		t.Fatal("failed target was acknowledged")
	}
	select {
	case <-received:
	default:
		t.Fatal("durable delivery never reached the real HTTP target")
	}
	retained := func() {
		t.Helper()
		var quarantined, pending, watermark int
		if err := pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM indexer.delivery_webhooks WHERE state='quarantined' AND response_status=400), (SELECT count(*) FROM indexer.delivery_outbox WHERE state='pending'), (SELECT committed_sequence FROM indexer.delivery_webhook_cursors)`).Scan(&quarantined, &pending, &watermark); err != nil || quarantined != 1 || pending != 1 || watermark != 0 {
			t.Fatalf("durable failed delivery state %d/%d/%d: %v", quarantined, pending, watermark, err)
		}
	}
	retained()
	router := chi.NewRouter()
	RegisterStatus(router, StatusDeps{Delivery: durable})
	server := httptest.NewServer(router)
	defer server.Close()
	for _, route := range []struct{ method, path string }{{"GET", "/status/dead-letter"}, {"POST", "/status/dead-letter/drain"}} {
		req, err := http.NewRequest(route.method, server.URL+route.path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Actor-Role", "operator")
		response, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(response.Body)
		response.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if response.StatusCode != 503 || strings.Contains(string(body), "private-") || strings.Contains(string(body), target.URL) {
			t.Fatal(route, response.StatusCode, string(body))
		}
	}
	retained()
}
