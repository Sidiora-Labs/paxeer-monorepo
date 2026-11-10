package fanout_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/devsocial"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

const repair1027Namespace = "repair1027:"
const repair1027Origin = "https://chat.example"

type repair1027Fixture struct {
	t            *testing.T
	database     *pgxpool.Pool
	openDatabase func() *pgxpool.Pool
	redis        *repair1027Redis
	identity     *httptest.Server
	authority    sharedauth.HTTPStreamAuthority
	mu           sync.Mutex
	tickets      map[string]sharedauth.StreamPrincipal
	principals   map[string]sharedauth.StreamPrincipal
}

func newRepair1027Fixture(t *testing.T) *repair1027Fixture {
	t.Helper()
	database := internaltest.NewPostgres(t)
	open := func() *pgxpool.Pool {
		db, err := pgxpool.NewWithConfig(context.Background(), database.Config().Copy())
		if err != nil {
			t.Fatal(err)
		}
		return db
	}
	if _, err := database.Exec(context.Background(), `INSERT INTO indexer.indexer_canonical_creations(token,pool,creator,factory,optical,nft_id,height,block_hash,transaction_hash,transaction_index,log_index,block_timestamp,provenance,payload) VALUES($1,$2,$3,$4,$5,1,100,$6,$7,0,0,$8,'{}','{}')`, addr(2), addr(1), addr(0xbb), addr(3), addr(4), "0x"+strings.Repeat("ab", 32), "0x"+strings.Repeat("cd", 32), time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	f := &repair1027Fixture{t: t, database: database, openDatabase: open, redis: newRepair1027Redis(t), tickets: map[string]sharedauth.StreamPrincipal{}, principals: map[string]sharedauth.StreamPrincipal{}}
	token := strings.Repeat("private-fixture-token", 2)
	f.identity = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", 401)
			return
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		switch r.URL.Path {
		case "/internal/v2/identity/chat-ticket:consume":
			var body struct {
				Ticket  string                         `json:"ticket"`
				Binding sharedauth.StreamTicketBinding `json:"binding"`
			}
			if json.NewDecoder(r.Body).Decode(&body) != nil {
				http.Error(w, "bad ticket", 400)
				return
			}
			principal, ok := f.tickets[body.Ticket]
			if !ok || body.Binding.Origin != repair1027Origin || sharedauth.ValidateChatStreamBinding(principal, body.Binding) != nil {
				http.Error(w, "ticket refused", 401)
				return
			}
			delete(f.tickets, body.Ticket)
			_ = json.NewEncoder(w).Encode(principal)
		case "/internal/v2/identity/session:check":
			var p sharedauth.StreamPrincipal
			if json.NewDecoder(r.Body).Decode(&p) != nil || f.principals[p.Wallet] != p {
				http.Error(w, "session refused", 401)
				return
			}
			w.WriteHeader(200)
		default:
			http.Error(w, "unknown identity operation", 404)
		}
	}))
	t.Cleanup(f.identity.Close)
	f.authority = sharedauth.HTTPStreamAuthority{Endpoint: f.identity.URL, ServiceToken: token, Client: f.identity.Client()}
	f.principal(addr(0xaa))
	f.principal(addr(0xbb))
	return f
}
func (f *repair1027Fixture) principal(wallet string) sharedauth.StreamPrincipal {
	f.mu.Lock()
	defer f.mu.Unlock()
	if p, ok := f.principals[wallet]; ok {
		return p
	}
	p := sharedauth.StreamPrincipal{AccountID: uuid.NewString(), SessionID: uuid.NewString(), Wallet: wallet}
	now := time.Now()
	hash := sha256.Sum256([]byte(p.SessionID))
	for _, statement := range []struct {
		sql  string
		args []any
	}{
		{`INSERT INTO users.identity_accounts VALUES($1,0,$2)`, []any{p.AccountID, now}},
		{`INSERT INTO users.identity_wallet_links VALUES($1,$2,125,$3,'private','verified',$4)`, []any{uuid.NewString(), p.AccountID, wallet, now}},
		{`INSERT INTO users.identity_sessions(session_id,account_id,wallet,auth_epoch,access_hash,csrf_hash,issued_at,access_expires_at,refresh_expires_at,reauthenticated_at,last_seen_at,device_label) VALUES($1,$2,$3,0,$4,$4,$5,$6,$6,$5,$5,'repair1027')`, []any{p.SessionID, p.AccountID, wallet, hash[:], now, now.Add(time.Hour)}},
		{`INSERT INTO users.account_state(account_id,wallet,profile,preferences) VALUES($1,$2,'{}','{}')`, []any{p.AccountID, wallet}},
		{`INSERT INTO chat.interaction_preferences(wallet,inbox) VALUES($1,'everyone')`, []any{wallet}},
	} {
		if _, err := f.database.Exec(context.Background(), statement.sql, statement.args...); err != nil {
			f.t.Fatal(err)
		}
	}
	f.principals[wallet] = p
	return p
}
func (f *repair1027Fixture) requestPrincipal(r *http.Request) (fanout.Principal, error) {
	f.mu.Lock()
	p, ok := f.principals[r.Header.Get("X-Fixture-Session")]
	f.mu.Unlock()
	if !ok {
		return fanout.Principal{}, sharedauth.ErrScopeDenied
	}
	return fanout.Principal{Wallet: p.Wallet, SessionID: p.SessionID, ExpiresAt: time.Now().Add(time.Minute), Validate: func(ctx context.Context) error { return f.authority.CheckStreamSession(ctx, p) }}, nil
}

type repair1027Runtime struct {
	fixture  *repair1027Fixture
	store    *fanout.DurableStore
	pool     *pgxpool.Pool
	pub, sub *goredis.Client
	hub      *fanout.Hub
	server   *httptest.Server
	cancel   context.CancelFunc
	done     chan error
	once     sync.Once
}

func (f *repair1027Fixture) start(maxLength, poolQuota, dmQuota int) *repair1027Runtime {
	f.t.Helper()
	r := &repair1027Runtime{fixture: f, pool: f.openDatabase(), pub: f.redis.client(f.t), sub: f.redis.client(f.t), done: make(chan error, 1)}
	// Use the production platform's real DurableStore assembly and policy, then
	// the same fanout.New protocol-v2 socket path selected by internal/app.
	dev, err := devsocial.New(context.Background(), devsocial.Deps{Pool: r.pool, Redis: r.pub, CursorKey: []byte("repair1027-private-replay-cursor-key"), ChannelNamespace: repair1027Namespace, Pools: devsocial.CanonicalPools, Authenticate: f.requestPrincipal})
	if err != nil {
		f.t.Fatal(err)
	}
	r.store = dev.Chat
	r.hub = fanout.New(fanout.Deps{ProtocolVersion: 2, Durable: r.store, Pub: r.pub, Sub: r.sub, ChannelNamespace: repair1027Namespace, Authenticate: fanout.TicketAuthenticator(f.authority, nil), MaxMessageLength: maxLength, MaxPoolMsgs: poolQuota, MaxDmMsgs: dmQuota, RateWindow: time.Second})
	router := chi.NewRouter()
	router.Get("/v2/stream", r.hub.ServeWS)
	if err := fanout.RegisterChatRoutes(router, r.store, f.requestPrincipal); err != nil {
		f.t.Fatal(err)
	}
	r.server = httptest.NewServer(router)
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	go func() { r.done <- r.hub.Run(ctx) }()
	f.t.Cleanup(r.stop)
	return r
}
func (r *repair1027Runtime) stop() {
	r.once.Do(func() {
		r.cancel()
		select {
		case <-r.done:
		case <-time.After(3 * time.Second):
			r.fixture.t.Error("hub did not stop")
		}
		r.server.Close()
		r.pool.Close()
		_ = r.pub.Close()
		_ = r.sub.Close()
	})
}
func (r *repair1027Runtime) dial(wallet string) *websocket.Conn {
	t := r.fixture.t
	t.Helper()
	p := r.fixture.principal(wallet)
	ticket := strings.ReplaceAll(uuid.NewString(), "-", "") + strings.ReplaceAll(uuid.NewString(), "-", "")
	r.fixture.mu.Lock()
	r.fixture.tickets[ticket] = p
	r.fixture.mu.Unlock()
	topics := []string{"chat.pool." + addr(1), "private.chat." + wallet}
	u := strings.Replace(r.server.URL, "http", "ws", 1) + "/v2/stream?topics=" + url.QueryEscape(strings.Join(topics, ","))
	dialer := websocket.Dialer{Subprotocols: []string{"sidiora.v2", "ticket." + ticket}, HandshakeTimeout: 2 * time.Second}
	c, response, err := dialer.Dial(u, http.Header{"Origin": []string{repair1027Origin}})
	if err != nil {
		t.Fatalf("platform socket upgrade: %v (%v)", err, response)
	}
	t.Cleanup(func() { _ = c.Close() })
	if c.Subprotocol() != "sidiora.v2" {
		t.Fatal("wrong socket protocol", c.Subprotocol())
	}
	if got := repair1027Frame(t, c); got["type"] != "session_accepted" || got["protocol_version"] != float64(2) {
		t.Fatal(got)
	}
	return c
}
func repair1027Frame(t *testing.T, c *websocket.Conn) map[string]any {
	t.Helper()
	_ = c.SetReadDeadline(time.Now().Add(3 * time.Second))
	var frame map[string]any
	if err := c.ReadJSON(&frame); err != nil {
		t.Fatal("socket frame:", err)
	}
	return frame
}
func repair1027WriteFrame(t *testing.T, c *websocket.Conn, frame any) map[string]any {
	t.Helper()
	if err := c.WriteJSON(frame); err != nil {
		t.Fatal(err)
	}
	return repair1027Frame(t, c)
}
func repair1027Send(t *testing.T, c *websocket.Conn, kind, id, content string) map[string]any {
	t.Helper()
	frame := map[string]any{"type": kind, "client_message_id": id, "content": content}
	if kind == "pool_message" {
		frame["pool_address"] = addr(1)
	} else {
		frame["to"] = addr(0xbb)
	}
	return repair1027WriteFrame(t, c, frame)
}
func repair1027Persisted(t *testing.T, frame map[string]any, id string) {
	t.Helper()
	messageID, _ := frame["id"].(string)
	sequence, _ := frame["sequence"].(string)
	revision, _ := frame["revision"].(string)
	if frame["type"] != "message_persisted" || frame["client_message_id"] != id || messageID == "" || sequence == "" || revision == "" {
		t.Fatalf("expected durable identity for %s: %v", id, frame)
	}
}
func repair1027Refused(t *testing.T, frame map[string]any, id, reason string) {
	t.Helper()
	if frame["type"] != "error" || frame["client_message_id"] != id || frame["message"] != reason {
		t.Fatalf("expected %s refusal for %s: %v", reason, id, frame)
	}
}
func (r *repair1027Runtime) subscribe(c *websocket.Conn, kind string) {
	t := r.fixture.t
	t.Helper()
	topic := repair1027Namespace + "dm:wallet:" + addr(0xaa)
	var frame map[string]any
	if kind == "pool_message" {
		topic = repair1027Namespace + "pool:" + addr(1)
		frame = repair1027WriteFrame(t, c, map[string]any{"type": "join_pool", "pool_address": addr(1)})
		if frame["type"] != "joined_pool" {
			t.Fatal(frame)
		}
	} else {
		frame = repair1027WriteFrame(t, c, map[string]any{"type": "subscribe_dms"})
		if frame["type"] != "subscribed_dms" {
			t.Fatal(frame)
		}
	}
	deadline := time.NewTimer(2 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for {
		r.fixture.redis.mu.Lock()
		ready := false
		for peer := range r.fixture.redis.peers {
			ready = ready || peer.topics[topic]
		}
		r.fixture.redis.mu.Unlock()
		if ready {
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("Redis subscription not established")
		case <-tick.C:
		}
	}
}
func (r *repair1027Runtime) publish() int {
	t := r.fixture.t
	t.Helper()
	n, err := r.store.DrainOutbox(context.Background(), 100, func(ctx context.Context, _ string, channels []string, payload []byte) error {
		for _, channel := range channels {
			if err := r.pub.Publish(ctx, channel, payload).Err(); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal("durable outbox:", err)
	}
	return n
}
func (r *repair1027Runtime) revise(id, revision, content string, remove bool) (int, fanout.DurableMessage) {
	t := r.fixture.t
	t.Helper()
	raw, err := json.Marshal(map[string]any{"message_id": id, "expected_revision": revision, "content": content, "remove": remove})
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, r.server.URL+"/v2/chat/revise", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Fixture-Session", addr(0xaa))
	response, err := r.server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	var message fanout.DurableMessage
	if response.StatusCode == 200 {
		if err := json.Unmarshal(body, &message); err != nil {
			t.Fatal(err)
		}
	}
	return response.StatusCode, message
}
func (f *repair1027Fixture) rows() (int, int) {
	var messages, outbox int
	if err := f.database.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM chat.chat_messages),(SELECT count(*) FROM chat.chat_outbox)`).Scan(&messages, &outbox); err != nil {
		f.t.Fatal(err)
	}
	return messages, outbox
}
func repair1027RateKey(kind, wallet string) string {
	return repair1027Namespace + "ratelimit:" + kind + ":" + wallet + ":ws"
}

func TestRepairM_1027_MessageLengthBoundaries(t *testing.T) {
	for _, kind := range []string{"pool_message", "dm"} {
		for _, text := range []string{"abc", "é中😃"} {
			t.Run(kind+"/"+text, func(t *testing.T) {
				f := newRepair1027Fixture(t)
				r := f.start(3, 100, 100)
				c := r.dial(addr(0xaa))
				r.subscribe(c, kind)
				accepted := repair1027Send(t, c, kind, "boundary", " <b>"+text+"</b> ")
				repair1027Persisted(t, accepted, "boundary")
				if n := r.publish(); n != 1 {
					t.Fatal("publication count", n)
				}
				event := repair1027Frame(t, c)
				if event["type"] != kind || event["content"] != text || event["id"] != accepted["id"] || event["revision"] != "1" {
					t.Fatal("producer/Redis/socket mismatch", event)
				}
				repair1027Refused(t, repair1027Send(t, c, kind, "too-long", text+"x"), "too-long", fanout.ErrInvalid.Error())
				repair1027Refused(t, repair1027Send(t, c, kind, "empty", "<b> </b>"), "empty", fanout.ErrInvalid.Error())
				status, edited := r.revise(accepted["id"].(string), "1", "中😃é", false)
				if status != 200 || edited.Revision != 2 || edited.Content != "中😃é" || edited.ID != accepted["id"] {
					t.Fatal("edit boundary", status, edited)
				}
				status, _ = r.revise(edited.ID, "2", "中😃éx", false)
				if status != 400 {
					t.Fatal("oversized edit accepted", status)
				}
				status, _ = r.revise(edited.ID, "2", " ", false)
				if status != 400 {
					t.Fatal("empty edit accepted", status)
				}
				if n := r.publish(); n != 1 {
					t.Fatal("only valid edit may publish", n)
				}
				event = repair1027Frame(t, c)
				if event["content"] != "中😃é" || event["revision"] != "2" {
					t.Fatal(event)
				}
				page, err := r.store.History(fanout.WithPrincipal(context.Background(), mustRepair1027Principal(t, f, addr(0xaa))), addr(0xaa), edited.ConversationID, "", 50)
				if err != nil || len(page.Messages) != 1 || page.Messages[0].Content != edited.Content || page.Messages[0].Revision != 2 {
					t.Fatal("edit/history mismatch", page, err)
				}
				if messages, outbox := f.rows(); messages != 1 || outbox != 2 {
					t.Fatal("rejected writes changed state", messages, outbox)
				}
				status, deleted := r.revise(edited.ID, "2", "", true)
				if status != 200 || !deleted.Deleted || deleted.Content != "" || deleted.Revision != 3 {
					t.Fatal("deletion no longer works", status, deleted)
				}
			})
		}
	}
}
func TestRepairM_1027_DefaultAndConfiguredMessageLimits(t *testing.T) {
	for _, limit := range []int{0, 7} {
		t.Run(fmt.Sprint(limit), func(t *testing.T) {
			f := newRepair1027Fixture(t)
			r := f.start(limit, 100, 100)
			c := r.dial(addr(0xaa))
			bound := limit
			if bound == 0 {
				bound = fanout.DefaultMaxMessageLength
			}
			if bound != r.store.MaxMessageLength || (limit == 0 && bound != 500) {
				t.Fatal("product default/configuration drift", bound, r.store.MaxMessageLength)
			}
			text := strings.Repeat("😃", bound)
			first := repair1027Send(t, c, "dm", "boundary", text)
			repair1027Persisted(t, first, "boundary")
			repair1027Refused(t, repair1027Send(t, c, "dm", "over", text+"é"), "over", fanout.ErrInvalid.Error())
			status, m := r.revise(first["id"].(string), "1", text, false)
			if status != 200 || m.Content != text {
				t.Fatal(status, m)
			}
			status, _ = r.revise(m.ID, "2", text+"é", false)
			if status != 400 {
				t.Fatal("edit ignored configured limit", status)
			}
			for _, invalid := range []string{string([]byte{0xff}), strings.Repeat("a", 16385)} {
				_, err := r.store.Send(context.Background(), fanout.SendInput{Actor: addr(0xaa), Target: addr(0xbb), Kind: "dm", ClientID: "invalid", Content: invalid})
				if err != fanout.ErrInvalid {
					t.Fatal("invalid encoding/byte bound", err)
				}
				_, err = r.store.Revise(context.Background(), addr(0xaa), m.ID, 2, invalid, false)
				if err != fanout.ErrInvalid {
					t.Fatal("edit invalid encoding/byte bound", err)
				}
			}
		})
	}
}

func TestRepairM_1027_PlatformSocketAdmission(t *testing.T) {
	for _, kind := range []string{"pool_message", "dm"} {
		t.Run(kind, func(t *testing.T) {
			f := newRepair1027Fixture(t)
			r := f.start(500, 2, 2)
			c := r.dial(addr(0xaa))
			if kind == "pool_message" {
				r.subscribe(c, kind)
			}
			first := repair1027Send(t, c, kind, "first", "hello")
			repair1027Persisted(t, first, "first")
			second := repair1027Send(t, c, kind, "second", "second")
			repair1027Persisted(t, second, "second")
			replay := repair1027Send(t, c, kind, "first", "hello")
			repair1027Persisted(t, replay, "first")
			if replay["id"] != first["id"] || replay["sequence"] != first["sequence"] {
				t.Fatal("retry changed committed identity", first, replay)
			}
			repair1027Refused(t, repair1027Send(t, c, kind, "first", "changed"), "first", fanout.ErrConflict.Error())
			repair1027Refused(t, repair1027Send(t, c, kind, "third", "third"), "third", fanout.ErrRateLimited.Error())
			f.redis.mu.Lock()
			transactions, abuse := f.redis.transactions, f.redis.abuseCalls
			f.redis.unavailable = true
			f.redis.mu.Unlock()
			if transactions != 3 || abuse != 2 {
				t.Fatal("retries charged quota or shared abuse path missing", transactions, abuse)
			}
			replay = repair1027Send(t, c, kind, "first", "hello")
			repair1027Persisted(t, replay, "first")
			if replay["id"] != first["id"] {
				t.Fatal(replay)
			}
			repair1027Refused(t, repair1027Send(t, c, kind, "dependency", "new"), "dependency", fanout.ErrUnavailable.Error())
			if messages, outbox := f.rows(); messages != 2 || outbox != 2 {
				t.Fatal("refusal persisted work", messages, outbox)
			}
			r.stop()
			recovered := f.start(500, 2, 2)
			socket := recovered.dial(addr(0xaa))
			replay = repair1027Send(t, socket, kind, "first", "hello")
			repair1027Persisted(t, replay, "first")
			if replay["id"] != first["id"] || replay["sequence"] != first["sequence"] {
				t.Fatal("restart lost identity", replay)
			}
		})
	}
}
func TestRepairM_1027_SharedAbuseLimiter(t *testing.T) {
	for _, kind := range []string{"pool_message", "dm"} {
		t.Run(kind, func(t *testing.T) {
			f := newRepair1027Fixture(t)
			r := f.start(500, 100, 100)
			c := r.dial(addr(0xaa))
			if kind == "pool_message" {
				r.subscribe(c, kind)
			}
			bound := 10
			if kind == "dm" {
				bound = 20
			}
			var first map[string]any
			for i := 0; i < bound; i++ {
				id := fmt.Sprintf("abuse-%d", i)
				frame := repair1027Send(t, c, kind, id, "message")
				repair1027Persisted(t, frame, id)
				if i == 0 {
					first = frame
				}
			}
			repair1027Refused(t, repair1027Send(t, c, kind, "over-abuse", "message"), "over-abuse", "social rate limit exceeded")
			replay := repair1027Send(t, c, kind, "abuse-0", "message")
			repair1027Persisted(t, replay, "abuse-0")
			if replay["id"] != first["id"] {
				t.Fatal(replay)
			}
			if messages, outbox := f.rows(); messages != bound || outbox != bound {
				t.Fatal(messages, outbox)
			}
			f.redis.mu.Lock()
			calls := f.redis.abuseCalls
			f.redis.mu.Unlock()
			if calls != bound+1 {
				t.Fatal("shared safety path not used exactly once per new attempt", calls)
			}
		})
	}
	t.Run("abuse-dependency", func(t *testing.T) {
		f := newRepair1027Fixture(t)
		r := f.start(500, 100, 100)
		c := r.dial(addr(0xaa))
		f.redis.mu.Lock()
		f.redis.abuseUnavailable = true
		f.redis.mu.Unlock()
		repair1027Refused(t, repair1027Send(t, c, "dm", "unavailable", "message"), "unavailable", fanout.ErrUnavailable.Error())
		if messages, outbox := f.rows(); messages != 0 || outbox != 0 {
			t.Fatal(messages, outbox)
		}
		f.redis.mu.Lock()
		calls := f.redis.abuseCalls
		f.redis.mu.Unlock()
		if calls != 1 {
			t.Fatal("did not reach actual AbuseLimiter Redis path", calls)
		}
	})
}

func TestRepairM_1027_RestartWindowRecovery(t *testing.T) {
	for _, fault := range []string{"missing-ttl", "before", "after", "expiry-refused"} {
		t.Run(fault, func(t *testing.T) {
			f := newRepair1027Fixture(t)
			key := repair1027RateKey("dm", addr(0xaa))
			unrelated := repair1027RateKey("pool", addr(0xcc))
			f.redis.mu.Lock()
			f.redis.counters[unrelated] = 37
			otherDeadline := f.redis.now.Add(time.Hour)
			f.redis.expires[unrelated] = otherDeadline
			switch fault {
			case "missing-ttl":
				f.redis.counters[key] = 5
			case "expiry-refused":
				f.redis.refuseExpiry = true
			default:
				f.redis.interrupt = fault
				f.redis.interruptKey = key
			}
			f.redis.mu.Unlock()
			r := f.start(500, 2, 2)
			c := r.dial(addr(0xaa))
			reason := fanout.ErrUnavailable.Error()
			if fault == "missing-ttl" {
				reason = fanout.ErrRateLimited.Error()
			}
			repair1027Refused(t, repair1027Send(t, c, "dm", "interrupted", "message"), "interrupted", reason)
			if messages, outbox := f.rows(); messages != 0 || outbox != 0 {
				t.Fatal("dependency/limit refusal committed", messages, outbox)
			}
			r.stop() // stop the hub, its database pool, publisher and subscriber clients
			f.redis.mu.Lock()
			count, deadline, interruptedAt := f.redis.counters[key], f.redis.expires[key], f.redis.now
			otherCount, otherUntil := f.redis.counters[unrelated], f.redis.expires[unrelated]
			f.redis.refuseExpiry = false
			if fault == "missing-ttl" {
				f.redis.now = f.redis.now.Add(2 * time.Second)
			}
			f.redis.mu.Unlock()
			if otherCount != 37 || !otherUntil.Equal(otherDeadline) {
				t.Fatal("unrelated counter changed")
			}
			switch fault {
			case "before":
				if count != 0 || !deadline.IsZero() {
					t.Fatal("unexecuted transaction changed state", count, deadline)
				}
			case "after":
				if count != 1 || !deadline.After(interruptedAt) {
					t.Fatal("lost reply left a permanent counter", count, deadline)
				}
			case "missing-ttl":
				if count != 6 || !deadline.Equal(interruptedAt.Add(time.Second)) {
					t.Fatal("missing TTL not repaired without clearing counter", count, deadline)
				}
			case "expiry-refused":
				if count != 1 || !deadline.IsZero() {
					t.Fatal("expiry refusal transcript", count, deadline)
				}
			}
			recovered := f.start(500, 2, 2)
			socket := recovered.dial(addr(0xaa))
			committed := repair1027Send(t, socket, "dm", "interrupted", "message")
			repair1027Persisted(t, committed, "interrupted")
			f.redis.mu.Lock()
			established, reopenedAt := f.redis.expires[key], f.redis.now
			otherCount, otherUntil = f.redis.counters[unrelated], f.redis.expires[unrelated]
			f.redis.now = f.redis.now.Add(100 * time.Millisecond)
			f.redis.mu.Unlock()
			if !established.After(reopenedAt) || otherCount != 37 || !otherUntil.Equal(otherDeadline) {
				t.Fatal("restart did not preserve bounded windows")
			}
			// A quota denial in the same window must never slide its deadline.
			if fault == "before" || fault == "missing-ttl" {
				repair1027Persisted(t, repair1027Send(t, socket, "dm", "second", "message"), "second")
			}
			repair1027Refused(t, repair1027Send(t, socket, "dm", "limited", "message"), "limited", fanout.ErrRateLimited.Error())
			f.redis.mu.Lock()
			afterIncrement := f.redis.expires[key]
			f.redis.now = f.redis.now.Add(2 * time.Second)
			f.redis.mu.Unlock()
			if !afterIncrement.Equal(established) {
				t.Fatal("window deadline slid on increment")
			}
			repair1027Persisted(t, repair1027Send(t, socket, "dm", "after-window", "message"), "after-window")
			page, err := recovered.store.History(fanout.WithPrincipal(context.Background(), mustRepair1027Principal(t, f, addr(0xaa))), addr(0xaa), committed["conversation_id"].(string), "", 50)
			if err != nil || len(page.Messages) < 2 || page.Messages[0].ID != committed["id"] {
				t.Fatal("recovered committed history", page, err)
			}
		})
	}
}

func mustRepair1027Principal(t *testing.T, f *repair1027Fixture, wallet string) fanout.Principal {
	t.Helper()
	request := httptest.NewRequest("GET", "/", nil)
	request.Header.Set("X-Fixture-Session", wallet)
	principal, err := f.requestPrincipal(request)
	if err != nil {
		t.Fatal(err)
	}
	return principal
}
