package auth

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
)

// The database peer is a committed in-memory store, not an alternate auth
// service: all nonces, messages, signatures, consumes and cleanup requests come
// from Service through database/sql. Its rows survive closing/reopening the
// service's DB handle, just as the shared database survives a gateway restart.
// It supports only the three auth statements and rejects other SQL. PostgreSQL
// planning and gateway lifecycle integration belong to the wiring requests.
type repair1485Row struct {
	nonce, address  []byte
	message         string
	issued, expires time.Time
	used            *time.Time
}

type repair1485Store struct {
	mu     sync.Mutex
	rows   map[string]repair1485Row
	passes chan int64
}

type repair1485Conn struct{ store *repair1485Store }
type repair1485Driver struct{}

func (repair1485Driver) Open(string) (driver.Conn, error) {
	return nil, errors.New("use the in-memory connector")
}
func (s *repair1485Store) Driver() driver.Driver { return repair1485Driver{} }
func (s *repair1485Store) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return &repair1485Conn{store: s}, nil
}
func (*repair1485Conn) Prepare(string) (driver.Stmt, error) {
	return nil, errors.New("auth statements must use context")
}
func (*repair1485Conn) Begin() (driver.Tx, error) {
	return nil, errors.New("auth statements are atomic")
}
func (*repair1485Conn) Close() error { return nil }

func repair1485SQL(query string) string { return strings.Join(strings.Fields(query), " ") }

func (c *repair1485Conn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	s := c.store
	s.mu.Lock()
	defer s.mu.Unlock()
	switch repair1485SQL(query) {
	case "INSERT INTO perps.auth_challenges (nonce, address, message, issued_at, expires_at) VALUES ($1, $2, $3, $4, $5)":
		if len(args) != 5 {
			return nil, errors.New("insert arity")
		}
		r := repair1485Row{
			nonce: bytes.Clone(args[0].Value.([]byte)), address: bytes.Clone(args[1].Value.([]byte)),
			message: args[2].Value.(string), issued: args[3].Value.(time.Time), expires: args[4].Value.(time.Time),
		}
		if len(r.nonce) != 32 || len(r.address) != 20 || !r.expires.After(r.issued) {
			return nil, errors.New("challenge constraint")
		}
		if _, exists := s.rows[string(r.nonce)]; exists {
			return nil, errors.New("duplicate nonce")
		}
		s.rows[string(r.nonce)] = r
		return driver.RowsAffected(1), nil
	case "WITH expired AS ( SELECT nonce FROM perps.auth_challenges WHERE expires_at <= $1 ORDER BY expires_at, nonce LIMIT $2 FOR UPDATE SKIP LOCKED ) DELETE FROM perps.auth_challenges AS challenges USING expired WHERE challenges.nonce = expired.nonce":
		if len(args) != 2 {
			return nil, errors.New("cleanup arity")
		}
		cutoff, limit := args[0].Value.(time.Time), args[1].Value.(int64)
		if limit <= 0 || limit > 512 {
			return nil, errors.New("cleanup must be bounded")
		}
		var expired []repair1485Row
		for _, row := range s.rows {
			if !row.expires.After(cutoff) {
				expired = append(expired, row)
			}
		}
		sort.Slice(expired, func(i, j int) bool {
			if expired[i].expires.Equal(expired[j].expires) {
				return bytes.Compare(expired[i].nonce, expired[j].nonce) < 0
			}
			return expired[i].expires.Before(expired[j].expires)
		})
		if int64(len(expired)) > limit {
			expired = expired[:limit]
		}
		for _, row := range expired {
			delete(s.rows, string(row.nonce))
		}
		n := int64(len(expired))
		s.passes <- n
		return driver.RowsAffected(n), nil
	default:
		return nil, fmt.Errorf("unsupported auth SQL: %s", query)
	}
}

func (c *repair1485Conn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if repair1485SQL(query) != "UPDATE perps.auth_challenges SET used_at = $3 WHERE address = $1 AND message = $2 AND used_at IS NULL AND expires_at > $3 RETURNING nonce" || len(args) != 3 {
		return nil, fmt.Errorf("unsupported auth consume: %s", query)
	}
	s := c.store
	s.mu.Lock()
	defer s.mu.Unlock()
	at := args[2].Value.(time.Time)
	for key, row := range s.rows {
		if bytes.Equal(row.address, args[0].Value.([]byte)) && row.message == args[1].Value.(string) && row.used == nil && row.expires.After(at) {
			row.used = &at
			s.rows[key] = row
			return &repair1485Rows{nonce: bytes.Clone(row.nonce)}, nil
		}
	}
	return &repair1485Rows{}, nil
}

type repair1485Rows struct{ nonce []byte }

func (*repair1485Rows) Columns() []string { return []string{"nonce"} }
func (*repair1485Rows) Close() error      { return nil }
func (r *repair1485Rows) Next(dest []driver.Value) error {
	if r.nonce == nil {
		return io.EOF
	}
	dest[0], r.nonce = r.nonce, nil
	return nil
}

func repair1485Memory() *repair1485Store {
	return &repair1485Store{rows: make(map[string]repair1485Row), passes: make(chan int64, 64)}
}
func (s *repair1485Store) snapshot() map[string]repair1485Row {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make(map[string]repair1485Row, len(s.rows))
	for k, v := range s.rows {
		out[k] = v
	}
	return out
}

type repair1485Clock struct{ nanos atomic.Int64 }

func (c *repair1485Clock) now() time.Time   { return time.Unix(0, c.nanos.Load()).UTC() }
func (c *repair1485Clock) set(at time.Time) { c.nanos.Store(at.UnixNano()) }
func repair1485NewClock() *repair1485Clock {
	c := &repair1485Clock{}
	c.set(time.Unix(1_900_000_000, 0).UTC())
	return c
}
func repair1485Open(t *testing.T, store *repair1485Store, clock *repair1485Clock) (*sql.DB, *Service) {
	t.Helper()
	db := sql.OpenDB(store)
	t.Cleanup(func() { _ = db.Close() })
	svc, err := NewService(db, testSecret, clock.now)
	if err != nil {
		t.Fatal(err)
	}
	return db, svc
}
func repair1485HTTP(t *testing.T, svc *Service) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("POST /v2/auth/challenge", svc.Challenge)
	mux.HandleFunc("POST /v2/auth/verify", svc.Verify)
	mux.Handle("GET /account", Middleware(svc, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		address, ok := Address(r.Context())
		if !ok {
			writeError(w, 500, "missing address")
			return
		}
		writeJSON(w, 200, map[string]string{"address": address})
	})))
	server := httptest.NewServer(mux)
	server.Client().Timeout = 5 * time.Second
	t.Cleanup(server.Close)
	return server
}
func repair1485Request(server *httptest.Server, path string, body any) (result, error) {
	raw, err := json.Marshal(body)
	if err != nil {
		return result{}, err
	}
	response, err := server.Client().Post(server.URL+path, "application/json", bytes.NewReader(raw))
	if err != nil {
		return result{}, err
	}
	defer response.Body.Close()
	var out map[string]any
	err = json.NewDecoder(response.Body).Decode(&out)
	return result{code: response.StatusCode, body: out}, err
}
func repair1485Post(t *testing.T, server *httptest.Server, path string, body any) result {
	t.Helper()
	r, err := repair1485Request(server, path, body)
	if err != nil {
		t.Fatal(err)
	}
	return r
}
func repair1485Challenge(t *testing.T, server *httptest.Server, address string) (string, string) {
	t.Helper()
	r := repair1485Post(t, server, "/v2/auth/challenge", map[string]string{"address": address})
	if r.code != 200 {
		t.Fatalf("challenge: %+v", r)
	}
	nonce, err := hexutil.Decode(str(t, r, "nonce"))
	if err != nil {
		t.Fatal(err)
	}
	return string(nonce), str(t, r, "message")
}
func repair1485WaitPass(t *testing.T, store *repair1485Store, want int64) {
	t.Helper()
	select {
	case got := <-store.passes:
		if got != want {
			t.Fatalf("cleanup deleted %d, want %d", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup did not run")
	}
}
func repair1485Run(t *testing.T, svc *Service) func() {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- svc.RunChallengeCleanup(ctx) }()
	var once sync.Once
	stop := func() {
		once.Do(func() {
			cancel()
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Errorf("cleanup exit: %v", err)
				}
			case <-time.After(5 * time.Second):
				t.Error("cleanup did not stop after cancellation")
			}
		})
	}
	t.Cleanup(stop)
	return stop
}

func TestRepairM_1485_RestartRetention(t *testing.T) {
	store, clock := repair1485Memory(), repair1485NewClock()
	db, svc := repair1485Open(t, store, clock)
	server := repair1485HTTP(t, svc)
	key := newKey(t)
	address := addrOf(key).Hex()
	start := clock.now()
	oldNonce, oldMessage := repair1485Challenge(t, server, address)
	usedNonce, usedMessage := repair1485Challenge(t, server, address)
	usedReq := verifyRequest{Address: address, Signature: personalSign(t, key, usedMessage), Message: usedMessage}
	wantToken(t, repair1485Post(t, server, "/v2/auth/verify", usedReq))
	stop := repair1485Run(t, svc)
	repair1485WaitPass(t, store, 0)
	if row := store.snapshot()[usedNonce]; row.used == nil || !row.used.Equal(start) {
		t.Fatal("consumed challenge must remain marked until expiry")
	}
	stop()
	server.Close()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	clock.set(start.Add(ChallengeTTL - time.Second))
	db, svc = repair1485Open(t, store, clock)
	server = repair1485HTTP(t, svc)
	validNonce, validMessage := repair1485Challenge(t, server, address)
	recentNonce, recentMessage := repair1485Challenge(t, server, address)
	recentReq := verifyRequest{Address: address, Signature: personalSign(t, key, recentMessage), Message: recentMessage}
	wantToken(t, repair1485Post(t, server, "/v2/auth/verify", recentReq))
	// Reopen the gateway's auth component after expiry, including a closed DB
	// handle and stopped HTTP transport/worker; the database peer retains rows.
	server.Close()
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	clock.set(start.Add(ChallengeTTL))
	_, svc = repair1485Open(t, store, clock)
	server = repair1485HTTP(t, svc)
	stop = repair1485Run(t, svc)
	repair1485WaitPass(t, store, 2)
	rows := store.snapshot()
	if len(rows) != 2 || rows[validNonce].message != validMessage || rows[recentNonce].used == nil {
		t.Fatalf("restart retained %d rows, want both unexpired challenges with consumption preserved", len(rows))
	}
	if _, exists := rows[oldNonce]; exists {
		t.Fatal("expired challenge retained")
	}
	if _, exists := rows[usedNonce]; exists {
		t.Fatal("expired consumed challenge retained")
	}
	wantError(t, repair1485Post(t, server, "/v2/auth/verify", verifyRequest{Address: address, Signature: personalSign(t, key, oldMessage), Message: oldMessage}), 401, "unknown, used or expired challenge")
	wantError(t, repair1485Post(t, server, "/v2/auth/verify", usedReq), 401, "unknown, used or expired challenge")
	wantError(t, repair1485Post(t, server, "/v2/auth/verify", recentReq), 401, "unknown, used or expired challenge")
	validReq := verifyRequest{Address: address, Signature: personalSign(t, key, validMessage), Message: validMessage}
	token := wantToken(t, repair1485Post(t, server, "/v2/auth/verify", validReq))
	wantError(t, repair1485Post(t, server, "/v2/auth/verify", validReq), 401, "unknown, used or expired challenge")
	request, err := http.NewRequest(http.MethodGet, server.URL+"/account", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var account map[string]string
	if err := json.NewDecoder(response.Body).Decode(&account); err != nil {
		t.Fatal(err)
	}
	if response.StatusCode != 200 || account["address"] != strings.ToLower(address) {
		t.Fatalf("middleware consumer: %d %v", response.StatusCode, account)
	}
	// The running worker, without a restart or request hook, also removes
	// challenges as their persisted expiry becomes eligible.
	clock.set(start.Add(2*ChallengeTTL - time.Second))
	// Zero-row ticks may have happened while HTTP assertions were in flight.
	// The first pass after advancing the clock must remove both retained rows.
	deadline := time.After(5 * time.Second)
waiting:
	for {
		select {
		case n := <-store.passes:
			if n == 2 {
				break waiting
			}
			if n != 0 {
				t.Fatalf("periodic cleanup deleted %d, want exactly 2", n)
			}
		case <-deadline:
			t.Fatal("periodic cleanup did not delete both consumed challenges")
		}
	}
	if len(store.snapshot()) != 0 {
		t.Fatal("periodic cleanup left consumed row")
	}
	wantError(t, repair1485Post(t, server, "/v2/auth/verify", validReq), 401, "unknown, used or expired challenge")
	wantError(t, repair1485Post(t, server, "/v2/auth/verify", recentReq), 401, "unknown, used or expired challenge")
	stop()
}

func TestRepairM_1485_BoundedCleanupAndAtomicUse(t *testing.T) {
	store, clock := repair1485Memory(), repair1485NewClock()
	_, svc := repair1485Open(t, store, clock)
	server := repair1485HTTP(t, svc)
	key := newKey(t)
	address := addrOf(key).Hex()
	start := clock.now()
	for i := 0; i < 515; i++ {
		repair1485Challenge(t, server, address)
	}
	clock.set(start.Add(time.Second))
	boundaryNonce, _ := repair1485Challenge(t, server, address)
	clock.set(start.Add(ChallengeTTL))
	liveNonce, liveMessage := repair1485Challenge(t, server, address)
	if n, err := svc.CleanupChallenges(context.Background()); err != nil || n != 512 {
		t.Fatalf("first batch: %d %v", n, err)
	}
	rows := store.snapshot()
	if len(rows) != 5 || rows[liveNonce].message != liveMessage || rows[boundaryNonce].nonce == nil {
		t.Fatalf("bounded cleanup retained wrong rows: %d", len(rows))
	}
	if n, err := svc.CleanupChallenges(context.Background()); err != nil || n != 3 {
		t.Fatalf("second batch: %d %v", n, err)
	}
	if n, err := svc.CleanupChallenges(context.Background()); err != nil || n != 0 {
		t.Fatalf("idempotent batch: %d %v", n, err)
	}
	request := verifyRequest{Address: address, Signature: personalSign(t, key, liveMessage), Message: liveMessage}
	type outcome struct {
		r   result
		err error
	}
	out := make(chan outcome, 16)
	for i := 0; i < 16; i++ {
		go func() { r, err := repair1485Request(server, "/v2/auth/verify", request); out <- outcome{r, err} }()
	}
	// Cleanup and concurrent verifies share the same production DB path.
	if n, err := svc.CleanupChallenges(context.Background()); err != nil || n != 0 {
		t.Fatalf("concurrent cleanup: %d %v", n, err)
	}
	successes := 0
	for i := 0; i < 16; i++ {
		got := <-out
		if got.err != nil {
			t.Fatal(got.err)
		}
		if got.r.code == 200 {
			wantToken(t, got.r)
			successes++
		} else {
			wantError(t, got.r, 401, "unknown, used or expired challenge")
		}
	}
	if successes != 1 {
		t.Fatalf("nonce consumed %d times, want exactly one", successes)
	}
	clock.set(start.Add(ChallengeTTL + time.Second))
	if n, err := svc.CleanupChallenges(context.Background()); err != nil || n != 1 {
		t.Fatalf("expiry boundary: %d %v", n, err)
	}
	if row := store.snapshot()[liveNonce]; row.used == nil {
		t.Fatal("cleanup removed still-valid consumed challenge")
	}
}

func TestRepairM_1485_CancellationAndFailure(t *testing.T) {
	store, clock := repair1485Memory(), repair1485NewClock()
	db, svc := repair1485Open(t, store, clock)
	f := fixture{svc: svc}
	_, message := f.challenge(t, addrOf(newKey(t)).Hex())
	clock.set(clock.now().Add(ChallengeTTL))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if n, err := svc.CleanupChallenges(ctx); n != 0 || !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled pass: %d %v", n, err)
	}
	if err := svc.RunChallengeCleanup(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled worker: %v", err)
	}
	if len(store.snapshot()) != 1 {
		t.Fatalf("canceled cleanup changed %q", message)
	}
	// Exhaust the actual database/sql pool to prove in-flight work, including
	// waiting to acquire a connection, exits when its owner cancels.
	db.SetMaxOpenConns(1)
	conn, err := db.Conn(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	ctx, cancel = context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- svc.RunChallengeCleanup(ctx) }()
	deadline := time.Now().Add(5 * time.Second)
	for db.Stats().WaitCount == 0 {
		if time.Now().After(deadline) {
			cancel()
			_ = conn.Close()
			t.Fatal("cleanup did not wait for a database connection")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("in-flight cancellation: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cleanup ignored in-flight cancellation")
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	if n, err := svc.CleanupChallenges(context.Background()); n != 0 || err == nil || !strings.Contains(err.Error(), "auth: cleanup challenges:") {
		t.Fatalf("closed database failure hidden: %d %v", n, err)
	}
	if err := svc.RunChallengeCleanup(context.Background()); err == nil || !strings.Contains(err.Error(), "auth: cleanup challenges:") {
		t.Fatalf("worker failure hidden: %v", err)
	}
	if len(store.snapshot()) != 1 {
		t.Fatal("failed cleanup changed rows")
	}
}
