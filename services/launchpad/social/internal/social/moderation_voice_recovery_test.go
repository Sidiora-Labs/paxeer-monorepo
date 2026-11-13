package social

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/voice"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/pkg/types"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

const moderationRecoveryHost = "0x1111111111111111111111111111111111111111"
const moderationRecoveryGuest = "0x2222222222222222222222222222222222222222"
const moderationRecoveryPool = "0x3333333333333333333333333333333333333333"
const moderationRecoveryKey = "private-m1029-admin-key"

type moderationRecoveryCase struct {
	Case                                      safety.Case
	Reporter, Affected, Category, Description string
}
type moderationRecoveryBan struct {
	Ban     safety.Ban
	Expires *time.Time
	Revoked bool
}
type moderationRecoveryParticipant struct {
	Role, Grant    string
	Muted, Revoked bool
	Revision       int64
}
type moderationRecoveryData struct {
	Cases        map[string]moderationRecoveryCase
	Bans         map[string]moderationRecoveryBan
	Actions      map[string][]string
	Participants map[string]map[string]moderationRecoveryParticipant
}

func newModerationRecoveryDatabase(t *testing.T) (*pgxpool.Pool, func() *pgxpool.Pool) {
	t.Helper()
	d := durableRoutesEmptyData()
	d.Moderation1029 = &moderationRecoveryData{Cases: map[string]moderationRecoveryCase{}, Bans: map[string]moderationRecoveryBan{}, Actions: map[string][]string{}, Participants: map[string]map[string]moderationRecoveryParticipant{}}
	return newDurableRoutesDatabaseWithData(t, d)
}

// The committed pgx wire peer stores only peer state. safety.Store,
// voice.Control, their transactions, the HTTP handlers and codecs are real.
// Unknown queries still fail, and a transaction publishes only at COMMIT.
func (d *durableRoutesData) executeModeration(q string, a []string) (durableRoutesResult, bool) {
	m := d.Moderation1029
	rows := func(v ...[]any) (durableRoutesResult, bool) { return durableRoutesRows(v...), true }
	write := func(tag string) (durableRoutesResult, bool) { return durableRoutesWrite(tag), true }
	caseRow := func(c safety.Case) []any { return []any{c.ID, c.TargetType, c.TargetID, c.PoolID, c.State, c.Revision} }
	switch {
	case strings.HasPrefix(q, "SELECT CASE WHEN c.kind='pool'"):
		message, ok := d.Messages[a[0]]
		if !ok {
			return rows()
		}
		pool := ""
		if c := d.Conversations[message.ConversationID]; c != nil && c.Kind == "pool" {
			pool = strings.TrimPrefix(message.ConversationID, "pool:")
		}
		return rows([]any{pool, message.Sender})
	case strings.HasPrefix(q, "INSERT INTO safety_cases"):
		for _, c := range m.Cases {
			if c.Reporter == a[5] && c.Case.TargetType == a[1] && c.Case.TargetID == a[2] {
				return rows(caseRow(c.Case))
			}
		}
		c := moderationRecoveryCase{Case: safety.Case{ID: a[0], TargetType: a[1], TargetID: a[2], PoolID: a[3], State: "open", Revision: 1}, Reporter: a[5], Affected: a[4], Category: a[6], Description: a[7]}
		m.Cases[a[0]] = c
		return rows(caseRow(c.Case))
	case strings.HasPrefix(q, "SELECT pool_id,state,revision FROM safety_cases"):
		c, ok := m.Cases[a[0]]
		if !ok {
			return rows()
		}
		return rows([]any{c.Case.PoolID, c.Case.State, c.Case.Revision})
	case strings.HasPrefix(q, "UPDATE safety_cases SET state="):
		c := m.Cases[a[0]]
		c.Case.State = a[1]
		c.Case.Revision = durableRoutesNumber(a[2])
		m.Cases[a[0]] = c
		return write("UPDATE 1")
	case strings.HasPrefix(q, "SELECT id,target_type,target_id,pool_id,state,revision FROM safety_cases"):
		ids := []string{}
		for id, c := range m.Cases {
			if (a[0] == "" || c.Case.State == a[0]) && id > a[1] && ((a[2] == "true" || a[2] == "t") || strings.Contains(a[3], c.Case.PoolID)) {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		result := [][]any{}
		for _, id := range ids {
			if int64(len(result)) >= durableRoutesNumber(a[4]) {
				break
			}
			result = append(result, caseRow(m.Cases[id].Case))
		}
		return rows(result...)
	case strings.HasPrefix(q, "INSERT INTO safety_actions"):
		if _, exists := m.Actions[a[0]]; exists {
			return durableRoutesFailure("duplicate action"), true
		}
		m.Actions[a[0]] = append([]string(nil), a...)
		return write("INSERT 0 1")
	case strings.HasPrefix(q, "SELECT actor,session_id,kind,revision FROM safety_actions"):
		result := [][]any{}
		for _, v := range m.Actions {
			if v[6] == a[0] {
				result = append(result, []any{v[2], v[3], v[6], durableRoutesNumber(v[9])})
			}
		}
		return rows(result...)
	case strings.HasPrefix(q, "INSERT INTO safety_bans"):
		b := moderationRecoveryBan{Ban: safety.Ban{ID: a[0], Wallet: a[1], Scope: a[2], Resource: a[3], Actor: a[4], Revision: 1}}
		if len(a) > 5 && a[5] != "" {
			expires := durableRoutesTime(a[5])
			b.Expires = &expires
		}
		m.Bans[a[0]] = b
		return write("INSERT 0 1")
	case strings.HasPrefix(q, "SELECT scope,resource,revision FROM safety_bans"):
		b, ok := m.Bans[a[0]]
		if !ok {
			return rows()
		}
		return rows([]any{b.Ban.Scope, b.Ban.Resource, b.Ban.Revision})
	case strings.HasPrefix(q, "UPDATE safety_bans SET revoked_at"):
		b, ok := m.Bans[a[0]]
		if !ok || b.Revoked {
			return write("UPDATE 0")
		}
		b.Revoked = true
		b.Ban.Revision++
		m.Bans[a[0]] = b
		return write("UPDATE 1")
	case strings.HasPrefix(q, "SELECT id,wallet,scope,resource,actor,revision FROM safety_bans"):
		ids := []string{}
		for id, b := range m.Bans {
			if id > a[0] && ((a[1] == "true" || a[1] == "t") || b.Ban.Scope == "pool" && strings.Contains(a[2], b.Ban.Resource)) {
				ids = append(ids, id)
			}
		}
		sort.Strings(ids)
		result := [][]any{}
		for _, id := range ids {
			if int64(len(result)) >= durableRoutesNumber(a[3]) {
				break
			}
			b := m.Bans[id].Ban
			result = append(result, []any{b.ID, b.Wallet, b.Scope, b.Resource, b.Actor, b.Revision})
		}
		return rows(result...)
	case strings.HasPrefix(q, "SELECT EXISTS(SELECT 1 FROM safety_bans") && strings.Contains(q, "scope=$2"):
		banned := false
		for _, b := range m.Bans {
			banned = banned || !b.Revoked && (b.Expires == nil || b.Expires.After(durableRoutesTime(a[3]))) && b.Ban.Wallet == a[0] && (b.Ban.Scope == "global" || b.Ban.Scope == a[1] && b.Ban.Resource == a[2])
		}
		return rows([]any{banned})
	case strings.HasPrefix(q, "INSERT INTO safety_outbox") && !strings.Contains(q, "'voice_room_created_v2'"):
		d.Outbox[a[0]] = durableRoutesBytes(a[4])
		return write("INSERT 0 1")
	case strings.HasPrefix(q, "SELECT count(*) FROM safety_cases"):
		return rows([]any{int64(len(m.Cases))})
	case strings.HasPrefix(q, "SELECT count(*) FROM safety_bans"):
		return rows([]any{int64(len(m.Bans))})
	case strings.HasPrefix(q, "DELETE FROM safety_outbox WHERE kind='voice_room_created_v2'"):
		if _, ok := d.Metadata[a[0]]; !ok {
			return write("DELETE 0")
		}
		delete(d.Metadata, a[0])
		return write("DELETE 1")
	case strings.HasPrefix(q, "SELECT count(*) FROM voice_rooms"):
		return rows([]any{int64(len(d.Rooms))})
	case strings.HasPrefix(q, "UPDATE voice_rooms SET state='live'"):
		r := d.Rooms[a[0]]
		r.State = "live"
		if len(a) > 1 {
			r.ProviderSession = a[1]
		}
		d.Rooms[a[0]] = r
		return write("UPDATE 1")
	case strings.HasPrefix(q, "SELECT state FROM voice_rooms"):
		r, ok := d.Rooms[a[0]]
		if !ok {
			return rows()
		}
		return rows([]any{r.State})
	case strings.HasPrefix(q, "SELECT EXISTS(SELECT 1 FROM voice_recordings"):
		_, missing := d.Recordings[a[0]]
		return rows([]any{missing})
	case strings.HasPrefix(q, "SELECT EXISTS(SELECT 1 FROM voice_ejections"):
		return rows([]any{false})
	case strings.HasPrefix(q, "SELECT count(*) FROM voice_participants"):
		var count int64
		for w, p := range m.Participants[a[0]] {
			if (len(a) < 2 || w == a[1]) && (!strings.Contains(q, "NOT revoked") || !p.Revoked) {
				count++
			}
		}
		return rows([]any{count})
	case strings.HasPrefix(q, "SELECT EXISTS(SELECT 1 FROM voice_participants"):
		p, ok := m.Participants[a[0]][a[1]]
		return rows([]any{ok && !p.Revoked})
	case strings.HasPrefix(q, "INSERT INTO voice_participants"):
		if m.Participants[a[0]] == nil {
			m.Participants[a[0]] = map[string]moderationRecoveryParticipant{}
		}
		p, exists := m.Participants[a[0]][a[1]]
		if exists && (!p.Revoked || p.Grant != "revoked") {
			return write("INSERT 0 0")
		}
		m.Participants[a[0]][a[1]] = moderationRecoveryParticipant{Role: "listener", Grant: "active", Revision: p.Revision + 1}
		return write("INSERT 0 1")
	case strings.HasPrefix(q, "SELECT role,muted,revoked,grant_state"):
		p, ok := m.Participants[a[0]][a[1]]
		if !ok {
			return rows()
		}
		row := []any{p.Role, p.Muted, p.Revoked, p.Grant}
		if strings.Contains(q, "grant_state,revision") {
			row = append(row, p.Revision)
		}
		return rows(row)
	case strings.HasPrefix(q, "SELECT grant_state,revision FROM voice_participants"):
		p, ok := m.Participants[a[0]][a[1]]
		if !ok {
			return rows()
		}
		return rows([]any{p.Grant, p.Revision})
	case strings.HasPrefix(q, "UPDATE voice_participants SET revoked=true"):
		p, ok := m.Participants[a[0]][a[1]]
		if !ok || p.Revoked {
			return write("UPDATE 0")
		}
		p.Revoked = true
		p.Muted = true
		p.Grant = "revoking"
		p.Revision++
		m.Participants[a[0]][a[1]] = p
		return write("UPDATE 1")
	case strings.HasPrefix(q, "UPDATE voice_participants SET grant_state='revoked'"):
		p, ok := m.Participants[a[0]][a[1]]
		if !ok || !p.Revoked || p.Grant != "revoking" {
			return write("UPDATE 0")
		}
		p.Grant = "revoked"
		m.Participants[a[0]][a[1]] = p
		return write("UPDATE 1")
	}
	return durableRoutesResult{}, false
}

// This resolver follows the production application's message lookup against
// the durable producer's committed message and conversation identities.
type moderationRecoveryTargets struct{ db *pgxpool.Pool }

func (r moderationRecoveryTargets) Resolve(ctx context.Context, kind, id string) (string, string, error) {
	if kind != "message" {
		return "", "", safety.ErrInvalid
	}
	var pool, wallet string
	err := r.db.QueryRow(ctx, `SELECT CASE WHEN c.kind='pool' THEN substring(c.id from 6) ELSE '' END,m.sender FROM chat_messages m JOIN chat_conversations c ON c.id=m.conversation_id WHERE m.id=$1`, id).Scan(&pool, &wallet)
	return pool, wallet, err
}

type moderationRecoveryIngress struct {
	t       *testing.T
	public  ed25519.PublicKey
	private ed25519.PrivateKey
}

func newModerationRecoveryIngress(t *testing.T) *moderationRecoveryIngress {
	t.Helper()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return &moderationRecoveryIngress{t: t, public: pub, private: key}
}
func (i *moderationRecoveryIngress) serve(routes http.Handler, store *safety.Store) *httptest.Server {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		if store != nil {
			ctx = WithModeration(ctx, store)
		}
		if payload := r.Header.Get("X-Fixture-Session"); payload != "" {
			p, err := safety.VerifySession(payload, r.Header.Get("X-Fixture-Signature"), i.public, time.Now())
			if err != nil {
				moderationError(w, err)
				return
			}
			ctx = safety.WithPrincipal(ctx, p)
		}
		routes.ServeHTTP(w, r.WithContext(ctx))
	}))
	i.t.Cleanup(server.Close)
	return server
}
func (i *moderationRecoveryIngress) request(server *httptest.Server, p *safety.Principal, method, path, body, key string, status int) []byte {
	i.t.Helper()
	req, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
	if err != nil {
		i.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Request-ID", uuid.NewString())
	req.Header.Set("X-Actor-Wallet", moderationRecoveryGuest)
	if key != "" {
		req.Header.Set("X-API-Key", key)
	}
	if p != nil {
		raw, err := json.Marshal(p)
		if err != nil {
			i.t.Fatal(err)
		}
		req.Header.Set("X-Fixture-Session", base64.RawURLEncoding.EncodeToString(raw))
		req.Header.Set("X-Fixture-Signature", base64.RawURLEncoding.EncodeToString(ed25519.Sign(i.private, raw)))
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		i.t.Fatal(err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		i.t.Fatal(err)
	}
	if resp.StatusCode != status {
		i.t.Fatalf("%s %s: %d want %d: %s", method, path, resp.StatusCode, status, raw)
	}
	if strings.Contains(path, "/voice-rooms") {
		if resp.Header.Get("Cache-Control") != "private, no-store" {
			i.t.Fatal("voice response cache policy", resp.Header)
		}
		if status >= 400 {
			if err := v2.ValidateJSON("Error", raw); err != nil {
				i.t.Fatalf("voice error schema: %v: %s", err, raw)
			}
			var envelope v2.Error
			if err := json.Unmarshal(raw, &envelope); err != nil {
				i.t.Fatal(err)
			}
			if envelope.RequestId != req.Header.Get("X-Request-ID") || envelope.Retryable != (status == 503) {
				i.t.Fatalf("uncorrelated voice error: %+v", envelope)
			}
		}
	}
	return raw
}
func moderationRecoveryPrincipal(wallet string) safety.Principal {
	return safety.Principal{Wallet: wallet, SessionID: uuid.NewString(), Expires: time.Now().Add(time.Hour)}
}
func moderationRecoveryDecode(t *testing.T, raw []byte, into any) {
	t.Helper()
	decoder := json.NewDecoder(strings.NewReader(string(raw)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(into); err != nil {
		t.Fatalf("response schema: %v: %s", err, raw)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		t.Fatal("trailing response JSON", err)
	}
}

func TestModerationVerifiedLifecycle(t *testing.T) { RunVerifiedModerationLifecycle(t) }

// Shared by the repaired legacy TestModeration and this task's declared gate.
func RunVerifiedModerationLifecycle(t *testing.T) {
	t.Helper()
	db, reopen := newModerationRecoveryDatabase(t)
	ingress := newModerationRecoveryIngress(t)
	reporter := moderationRecoveryPrincipal(moderationRecoveryHost)
	admin := moderationRecoveryPrincipal(moderationRecoveryHost)
	admin.Role = "global_administrator"
	admin.StepUp = time.Now()
	// Use the real durable producer, not a hand-invented message ID.
	now := time.Now().UTC()
	account := uuid.NewString()
	durableRoutesExec(t, db, `INSERT INTO identity_accounts VALUES($1,0,$2)`, account, now)
	durableRoutesExec(t, db, `INSERT INTO account_state(account_id,wallet,profile,preferences) VALUES($1,$2,'{}','{}')`, account, reporter.Wallet)
	durableRoutesExec(t, db, `INSERT INTO identity_wallet_links VALUES($1,$2,125,$3,'private','verified',$4)`, uuid.NewString(), account, reporter.Wallet, now)
	durableRoutesExec(t, db, `INSERT INTO identity_sessions(session_id,account_id,wallet,auth_epoch,access_hash,csrf_hash,issued_at,access_expires_at,refresh_expires_at,reauthenticated_at,last_seen_at,device_label) VALUES($1,$2,$3,0,$4,$4,$5,$6,$6,$5,$5,'repair-proof')`, reporter.SessionID, account, reporter.Wallet, []byte("fixture-hash"), now, now.Add(time.Hour))
	durableRoutesExec(t, db, `INSERT INTO indexer_canonical_creations_v1 VALUES($1,$2,$3)`, moderationRecoveryPool, moderationRecoveryGuest, reporter.Wallet)
	policy := durableRoutesPolicy(db)
	producer := &fanout.DurableStore{Pool: db, Policy: policy.SendPolicy, ReadPolicy: policy.ReadChatPolicy}
	sendCtx := fanout.WithPrincipal(context.Background(), fanout.Principal{Wallet: reporter.Wallet, SessionID: reporter.SessionID, ExpiresAt: reporter.Expires, Validate: func(ctx context.Context) error {
		return policy.ValidateSession(ctx, reporter.Wallet, reporter.SessionID)
	}})
	if err := producer.JoinPool(sendCtx, reporter.Wallet, moderationRecoveryPool); err != nil {
		t.Fatal(err)
	}
	message, err := producer.Send(sendCtx, fanout.SendInput{Actor: reporter.Wallet, Kind: "pool", Target: moderationRecoveryPool, Content: "Reported content", ClientID: uuid.NewString()})
	if err != nil {
		t.Fatal(err)
	}
	body := `{"message_id":"` + message.ID + `","reason":"spam"}`
	routes := chi.NewRouter()
	New(Deps{AdminAPIKey: moderationRecoveryKey}).RegisterRoutes(routes)
	store := &safety.Store{DB: db, Targets: moderationRecoveryTargets{db: db}}
	server := ingress.serve(routes, store)
	t.Run("missing store refuses even verified actor", func(t *testing.T) {
		unwired := ingress.serve(routes, nil)
		ingress.request(unwired, &reporter, "POST", "/report", body, "", 503)
	})
	t.Run("report requires verified actor", func(t *testing.T) {
		ingress.request(server, nil, "POST", "/report", body, "", 403)
		expired := reporter
		expired.Expires = time.Now().Add(-time.Minute)
		ingress.request(server, &expired, "POST", "/report", body, "", 403)
		var count int
		if err := db.QueryRow(context.Background(), `SELECT count(*) FROM safety_cases`).Scan(&count); err != nil || count != 0 {
			t.Fatalf("refused report committed: %d; %v", count, err)
		}
	})
	var report types.CreatedResponse
	t.Run("report ok", func(t *testing.T) {
		raw := ingress.request(server, &reporter, "POST", "/report", body, "", 200)
		moderationRecoveryDecode(t, raw, &report)
		if !report.Success || report.ID == "" {
			t.Fatalf("created report: %+v", report)
		}
		var repeated types.CreatedResponse
		moderationRecoveryDecode(t, ingress.request(server, &reporter, "POST", "/report", body, "", 200), &repeated)
		if repeated != report {
			t.Fatalf("report retry changed identity: %+v != %+v", repeated, report)
		}
		ingress.request(server, &reporter, "POST", "/report", `{"message_id":"missing-target","reason":"spam"}`, "", 503)
	})
	t.Run("admin missing key 401", func(t *testing.T) { ingress.request(server, &admin, "GET", "/admin/bans", "", "", 401) })
	t.Run("admin wrong key 403", func(t *testing.T) { ingress.request(server, &admin, "GET", "/admin/bans", "", "wrong", 403) })
	t.Run("admin ban lifecycle + reports", func(t *testing.T) {
		banBody := `{"wallet":"` + moderationRecoveryGuest + `","banned_by":"` + moderationRecoveryGuest + `","reason":"abuse","duration_seconds":3600}`
		ingress.request(server, nil, "POST", "/admin/bans", banBody, moderationRecoveryKey, 403)
		ingress.request(server, &reporter, "POST", "/admin/bans", banBody, moderationRecoveryKey, 403)
		stale := admin
		stale.StepUp = time.Now().Add(-6 * time.Minute)
		ingress.request(server, &stale, "POST", "/admin/bans", banBody, moderationRecoveryKey, 403)
		var count int
		if err := db.QueryRow(context.Background(), `SELECT count(*) FROM safety_bans`).Scan(&count); err != nil || count != 0 {
			t.Fatalf("missing authority created ban: %d; %v", count, err)
		}
		var created types.BanResponse
		moderationRecoveryDecode(t, ingress.request(server, &admin, "POST", "/admin/bans", banBody, moderationRecoveryKey, 200), &created)
		if !created.OK || created.Ban.ID == "" || created.Ban.Wallet != moderationRecoveryGuest || created.Ban.BannedBy != admin.Wallet || created.Ban.ExpiresAt == nil {
			t.Fatalf("ban response: %+v", created)
		}
		if decision := store.Check(context.Background(), moderationRecoveryGuest, "pool", moderationRecoveryPool); decision != safety.Deny {
			t.Fatal("committed ban not enforced", decision)
		}
		var bans struct {
			Bans []safety.Ban `json:"bans"`
		}
		moderationRecoveryDecode(t, ingress.request(server, &admin, "GET", "/admin/bans", "", moderationRecoveryKey, 200), &bans)
		if len(bans.Bans) != 1 || bans.Bans[0].ID != created.Ban.ID || bans.Bans[0].Actor != admin.Wallet || bans.Bans[0].Revision != 1 {
			t.Fatalf("actual ban schema: %+v", bans)
		}
		var reports struct {
			Reports []safety.Case `json:"reports"`
		}
		moderationRecoveryDecode(t, ingress.request(server, &admin, "GET", "/admin/reports?status=pending", "", moderationRecoveryKey, 200), &reports)
		if len(reports.Reports) != 1 || reports.Reports[0] != (safety.Case{ID: report.ID, TargetType: "message", TargetID: message.ID, PoolID: moderationRecoveryPool, State: "open", Revision: 1}) {
			t.Fatalf("actual report schema: %+v", reports)
		}
		path := "/admin/reports/" + report.ID
		ingress.request(server, &admin, "PATCH", path, `{"status":"bogus","expected_revision":1,"reason":"review","policy_version":"v1"}`, moderationRecoveryKey, 409)
		ingress.request(server, &admin, "PATCH", path, `{"status":"resolved"}`, moderationRecoveryKey, 400)
		ingress.request(server, &admin, "PATCH", path, `{"status":"resolved","expected_revision":2,"reason":"review","policy_version":"v1"}`, moderationRecoveryKey, 409)
		var resolved struct {
			OK       bool  `json:"ok"`
			Revision int64 `json:"revision"`
		}
		moderationRecoveryDecode(t, ingress.request(server, &admin, "PATCH", path, `{"status":"resolved","expected_revision":1,"reason":"review","policy_version":"v1"}`, moderationRecoveryKey, 200), &resolved)
		if !resolved.OK || resolved.Revision != 2 {
			t.Fatalf("resolve schema: %+v", resolved)
		}
		ingress.request(server, &admin, "PATCH", path, `{"status":"resolved","expected_revision":1,"reason":"review","policy_version":"v1"}`, moderationRecoveryKey, 409)
		moderationRecoveryDecode(t, ingress.request(server, &admin, "GET", "/admin/reports?status=pending", "", moderationRecoveryKey, 200), &reports)
		if len(reports.Reports) != 0 {
			t.Fatal("pending report survived resolution", reports)
		}
		var actor, session, kind string
		var revision int64
		if err := db.QueryRow(context.Background(), `SELECT actor,session_id,kind,revision FROM safety_actions WHERE kind=$1`, "actioned").Scan(&actor, &session, &kind, &revision); err != nil || actor != admin.Wallet || session != admin.SessionID || revision != 2 {
			t.Fatalf("audit lost verified authority: %s %s %s %d; %v", actor, session, kind, revision, err)
		}
		revoke := "/admin/bans/" + created.Ban.ID
		ingress.request(server, &admin, "DELETE", revoke, `{"expected_revision":2,"reason":"review complete"}`, moderationRecoveryKey, 409)
		var revoked types.OKResponse
		moderationRecoveryDecode(t, ingress.request(server, &admin, "DELETE", revoke, `{"expected_revision":1,"reason":"review complete"}`, moderationRecoveryKey, 200), &revoked)
		if !revoked.OK || store.Check(context.Background(), moderationRecoveryGuest, "pool", moderationRecoveryPool) != safety.Allow {
			t.Fatal("revocation did not restore policy")
		}
		server.Close()
		db.Close()
		db = reopen()
		recoveredStore := &safety.Store{DB: db, Targets: moderationRecoveryTargets{db: db}}
		recovered := ingress.serve(routes, recoveredStore)
		moderationRecoveryDecode(t, ingress.request(recovered, &admin, "GET", "/admin/reports?status=actioned", "", moderationRecoveryKey, 200), &reports)
		if len(reports.Reports) != 1 || reports.Reports[0].Revision != 2 || recoveredStore.Check(context.Background(), moderationRecoveryGuest, "pool", moderationRecoveryPool) != safety.Allow {
			t.Fatalf("restart lost moderation state: %+v", reports)
		}
	})
}

func moderationRecoveryVoice(t *testing.T, db *pgxpool.Pool) (*safety.Store, *voice.Control) {
	t.Helper()
	store := &safety.Store{DB: db}
	control := &voice.Control{DB: db, Policy: store, Pools: v2VoicePostgresPools{db: db}, Provider: &voice.LiveKit{}, Gates: voice.Gates{ProviderQualified: true, EjectionQualified: true, ACLQualified: true, RetentionQualified: true}}
	return store, control
}
func moderationRecoverySeedVoice(t *testing.T, db *pgxpool.Pool) {
	t.Helper()
	account := uuid.NewString()
	durableRoutesExec(t, db, `INSERT INTO account_state_v2 VALUES($1,$2,false)`, account, moderationRecoveryHost)
	durableRoutesExec(t, db, `INSERT INTO identity_wallet_links_v2 VALUES($1,$2,125,'verified')`, account, moderationRecoveryHost)
	durableRoutesExec(t, db, `INSERT INTO indexer_canonical_creations_v1 VALUES($1,$2,$3)`, moderationRecoveryPool, moderationRecoveryGuest, moderationRecoveryHost)
}
func moderationRecoveryVoiceServer(t *testing.T, ingress *moderationRecoveryIngress, store *safety.Store, control *voice.Control) *httptest.Server {
	t.Helper()
	router := chi.NewRouter()
	RegisterSafetyRoutes(router, store, control)
	return ingress.serve(router, store)
}
func moderationRecoveryCreateRoom(t *testing.T, ingress *moderationRecoveryIngress, server *httptest.Server, host safety.Principal) v2.VoiceCreateResponse {
	t.Helper()
	raw := ingress.request(server, &host, "POST", "/v2/voice-rooms", `{"scopeType":"pool","scopeId":"`+moderationRecoveryPool+`","title":"Private recovery room","visibility":"private"}`, "", 201)
	var created v2.VoiceCreateResponse
	if err := proofLaunchpadSocialDecode("VoiceCreateResponse", raw, &created); err != nil {
		t.Fatal(err)
	}
	return created
}
func moderationRecoveryReadParticipant(t *testing.T, db *pgxpool.Pool, id, wallet string) moderationRecoveryParticipant {
	t.Helper()
	var p moderationRecoveryParticipant
	if err := db.QueryRow(context.Background(), `SELECT role,muted,revoked,grant_state,revision FROM voice_participants WHERE room_id=$1 AND wallet=$2`, id, wallet).Scan(&p.Role, &p.Muted, &p.Revoked, &p.Grant, &p.Revision); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestModerationRecoveryPrivateGuestAndOwnerCleanupRecovery(t *testing.T) {
	db, reopen := newModerationRecoveryDatabase(t)
	moderationRecoverySeedVoice(t, db)
	ingress := newModerationRecoveryIngress(t)
	host, guest := moderationRecoveryPrincipal(moderationRecoveryHost), moderationRecoveryPrincipal(moderationRecoveryGuest)
	store, control := moderationRecoveryVoice(t, db)
	server := moderationRecoveryVoiceServer(t, ingress, store, control)
	created := moderationRecoveryCreateRoom(t, ingress, server, host)
	id := created.Data.Room.RoomId
	base := "/v2/voice-rooms/" + id
	room, err := control.Read(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	providerSession := "kindle-" + room.StartKey
	durableRoutesExec(t, db, `UPDATE voice_rooms SET state='live',provider_session=$2 WHERE id=$1`, id, providerSession)
	guestTicket := `{"role":"speaker","clientSessionId":"` + guest.SessionID + `"}`
	ingress.request(server, &guest, "POST", base+"/tickets", guestTicket, "", 403)
	var count int
	if err := db.QueryRow(context.Background(), `SELECT count(*) FROM voice_participants WHERE room_id=$1 AND wallet=$2`, id, guest.Wallet).Scan(&count); err != nil || count != 0 {
		t.Fatalf("private denial changed guest state: %d; %v", count, err)
	}
	ownerTicket := `{"role":"speaker","clientSessionId":"` + host.SessionID + `"}`
	// Owner admission is legitimate even in a private room. The real provider's
	// missing configuration fails only after JoinWithConsent commits admission.
	ingress.request(server, &host, "POST", base+"/tickets", ownerTicket, "", 503)
	admitted := moderationRecoveryReadParticipant(t, db, id, host.Wallet)
	if admitted != (moderationRecoveryParticipant{Role: "listener", Grant: "active", Revision: 1}) {
		t.Fatalf("provider failure changed listener authority: %+v", admitted)
	}
	ingress.request(server, &guest, "POST", base+"/tickets", guestTicket, "", 403)
	if got := moderationRecoveryReadParticipant(t, db, id, host.Wallet); got != admitted {
		t.Fatal("guest denial changed admitted owner", got)
	}
	leave := `{"clientSessionId":"` + host.SessionID + `"}`
	ingress.request(server, &host, "POST", base+":leave", leave, "", 503)
	pending := moderationRecoveryReadParticipant(t, db, id, host.Wallet)
	if pending != (moderationRecoveryParticipant{Role: "listener", Grant: "revoking", Muted: true, Revoked: true, Revision: 2}) {
		t.Fatalf("provider failure lost cleanup intent: %+v", pending)
	}
	server.Close()
	db.Close()
	db = reopen()
	store, control = moderationRecoveryVoice(t, db)
	server = moderationRecoveryVoiceServer(t, ingress, store, control)
	if recovered := moderationRecoveryReadParticipant(t, db, id, host.Wallet); recovered != pending {
		t.Fatalf("restart lost cleanup intent: %+v != %+v", recovered, pending)
	}
	ingress.request(server, &host, "POST", base+"/tickets", ownerTicket, "", 503)
	if got := moderationRecoveryReadParticipant(t, db, id, host.Wallet); got != pending {
		t.Fatalf("ticket during recovery changed revocation: %+v", got)
	}
	// A loopback transport peer acknowledges the real LiveKit revocation call.
	// Its fixed transcript checks the provider request and JWT signature; the
	// component under repair and the provider client are never replaced.
	providerCalls := make(chan string, 4)
	endpoint := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" || r.URL.Path != "/twirp/livekit.RoomService/RemoveParticipant" {
			t.Errorf("unexpected provider request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "unexpected operation", 400)
			return
		}
		var request struct {
			Room          string `json:"room"`
			Identity      string `json:"identity"`
			RevokeTokenTS string `json:"revoke_token_ts"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Identity != host.Wallet || request.Room != providerSession || request.RevokeTokenTS == "" || !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
			t.Errorf("invalid cleanup request: %+v; %v", request, err)
			http.Error(w, "invalid cleanup", 400)
			return
		}
		parts := strings.Split(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "), ".")
		if len(parts) != 3 {
			t.Error("provider JWT malformed")
			http.Error(w, "bad token", 401)
			return
		}
		mac := hmac.New(sha256.New, []byte(strings.Repeat("m1029-secret", 4)))
		_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
		signature, err := base64.RawURLEncoding.DecodeString(parts[2])
		if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
			t.Error("provider JWT signature invalid")
			http.Error(w, "bad token", 401)
			return
		}
		providerCalls <- request.Identity
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{}`))
	}))
	defer endpoint.Close()
	control.Provider = &voice.LiveKit{Endpoint: endpoint.URL, Client: endpoint.Client(), APIKey: "m1029-key", APISecret: strings.Repeat("m1029-secret", 4)}
	for n := 0; n < 2; n++ {
		raw := ingress.request(server, &host, "POST", base+":leave", leave, "", 200)
		var response v2.VoiceLeaveResponse
		if err := proofLaunchpadSocialDecode("VoiceLeaveResponse", raw, &response); err != nil {
			t.Fatal(err)
		}
		if response.Data.GrantState != "revoked" || response.Data.Revision != "2" || response.Revision == nil || *response.Revision != "2" {
			t.Fatalf("recovered leave: %+v", response)
		}
		if got := moderationRecoveryReadParticipant(t, db, id, host.Wallet); got != (moderationRecoveryParticipant{Role: "listener", Grant: "revoked", Muted: true, Revoked: true, Revision: 2}) {
			t.Fatalf("cleanup not idempotent: %+v", got)
		}
	}
	if len(providerCalls) != 2 {
		t.Fatalf("cleanup did not cross provider transport: %d calls", len(providerCalls))
	}
	ingress.request(server, &guest, "POST", base+"/tickets", guestTicket, "", 403)
	if err := db.QueryRow(context.Background(), `SELECT count(*) FROM voice_participants WHERE room_id=$1 AND wallet=$2`, id, guest.Wallet).Scan(&count); err != nil || count != 0 {
		t.Fatalf("cleanup admitted private guest: %d; %v", count, err)
	}
}

// Finding 11's atomic create/metadata transaction belongs to the voice owner.
// This proof qualifies the in-scope restart boundary and fails-closed behavior;
// it does not claim that the separately recorded wiring request is implemented.
func TestModerationRecoveryMetadataRestartBoundary(t *testing.T) {
	db, reopen := newModerationRecoveryDatabase(t)
	moderationRecoverySeedVoice(t, db)
	ingress := newModerationRecoveryIngress(t)
	host, guest := moderationRecoveryPrincipal(moderationRecoveryHost), moderationRecoveryPrincipal(moderationRecoveryGuest)
	store, control := moderationRecoveryVoice(t, db)
	server := moderationRecoveryVoiceServer(t, ingress, store, control)
	created := moderationRecoveryCreateRoom(t, ingress, server, host)
	id := created.Data.Room.RoomId
	base := "/v2/voice-rooms/" + id
	before, err := control.Read(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	var metadata []byte
	if err := db.QueryRow(context.Background(), `SELECT payload FROM safety_outbox WHERE kind='voice_room_created_v2' AND entity_id=$1 ORDER BY created_at,id LIMIT 1`, id).Scan(&metadata); err != nil {
		t.Fatal(err)
	}
	// Stop both consumers and reopen the database transport and HTTP server.
	server.Close()
	db.Close()
	db = reopen()
	store, control = moderationRecoveryVoice(t, db)
	server = moderationRecoveryVoiceServer(t, ingress, store, control)
	recovered, err := control.Read(context.Background(), id)
	if err != nil || recovered != before {
		t.Fatalf("room identity changed across restart: %+v; %v", recovered, err)
	}
	raw := ingress.request(server, &host, "GET", base, "", "", 200)
	var read v2.VoiceReadResponse
	if err := proofLaunchpadSocialDecode("VoiceReadResponse", raw, &read); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(read.Data.Room, created.Data.Room) {
		t.Fatalf("recovered room metadata: %+v != %+v", read.Data.Room, created.Data.Room)
	}
	var body v2.VoiceCreateBody
	if err := proofLaunchpadSocialDecode("VoiceCreateBody", metadata, &body); err != nil || body.Visibility != "private" || body.Title != "Private recovery room" || body.ScopeId != moderationRecoveryPool {
		t.Fatalf("create metadata: %+v; %v", body, err)
	}
	ingress.request(server, &guest, "GET", base, "", "", 403)
	// The current authority cannot recover a room if the separate metadata write
	// was interrupted. Reproduce that durable boundary without widening access.
	durableRoutesExec(t, db, `DELETE FROM safety_outbox WHERE kind='voice_room_created_v2' AND entity_id=$1`, id)
	server.Close()
	db.Close()
	db = reopen()
	store, control = moderationRecoveryVoice(t, db)
	server = moderationRecoveryVoiceServer(t, ingress, store, control)
	ingress.request(server, &host, "GET", base, "", "", 503)
	ingress.request(server, &guest, "GET", base, "", "", 503)
	after, err := control.Read(context.Background(), id)
	if err != nil || after != before {
		t.Fatalf("missing metadata altered durable room: %+v; %v", after, err)
	}
	var count int
	if err := db.QueryRow(context.Background(), `SELECT count(*) FROM voice_rooms`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("read allocated a duplicate room: %d; %v", count, err)
	}
}

func TestModerationRecoveryInterruptedCreateRequiresVoiceWiring(t *testing.T) {
	db, reopen := newModerationRecoveryDatabase(t)
	moderationRecoverySeedVoice(t, db)
	host := moderationRecoveryPrincipal(moderationRecoveryHost)
	_, control := moderationRecoveryVoice(t, db)
	// This is the first producer operation in the frozen create handler. Stopping
	// here models interruption before its separate metadata persistence call.
	room, err := control.Create(safety.WithPrincipal(context.Background(), host), moderationRecoveryPool)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	db = reopen()
	store, recovered := moderationRecoveryVoice(t, db)
	got, err := recovered.Read(context.Background(), room.ID)
	if err != nil || got != room {
		t.Fatalf("interrupted room identity lost: %+v; %v", got, err)
	}
	ingress := newModerationRecoveryIngress(t)
	server := moderationRecoveryVoiceServer(t, ingress, store, recovered)
	ingress.request(server, &host, "GET", "/v2/voice-rooms/"+room.ID, "", "", 503)
	var count int
	if err := db.QueryRow(context.Background(), `SELECT count(*) FROM voice_rooms`).Scan(&count); err != nil || count != 1 {
		t.Fatalf("interrupted create changed room inventory: %d; %v", count, err)
	}
}
