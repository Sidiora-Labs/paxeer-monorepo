package social

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/db/sqlcdb"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/voice"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

// Missing production authority must refuse before consulting either legacy
// tables or room metadata. This boundary needs no external service.
func TestRepair_68_MissingAuthoritiesFailClosed(t *testing.T) {
	const wallet = "0x1111111111111111111111111111111111111111"
	router := chi.NewRouter()
	New(Deps{}).RegisterContractRoutes(router)
	server := httptest.NewServer(router)
	defer server.Close()
	cases := []repair68ErrorCase{
		{Method: "GET", Path: "/v2/profiles/bad/follow-stats", Status: 400, Code: "INVALID_REQUEST", RequestID: uuid.NewString()},
		{Method: "PUT", Path: "/v2/me/following/" + wallet, Status: 401, Code: "AUTH_REQUIRED", RequestID: uuid.NewString()},
		{Method: "DELETE", Path: "/v2/me/following/" + wallet, Status: 401, Code: "AUTH_REQUIRED", RequestID: uuid.NewString()},
		{Method: "PUT", Path: "/v2/me/following/" + wallet, Actor: "malformed", Status: 401, Code: "AUTH_REQUIRED", RequestID: uuid.NewString()},
		{Method: "DELETE", Path: "/v2/me/following/" + wallet, Actor: "malformed", Status: 401, Code: "AUTH_REQUIRED", RequestID: uuid.NewString()},
		{Method: "PUT", Path: "/v2/me/following/bad", Actor: wallet, Status: 400, Code: "INVALID_REQUEST", RequestID: uuid.NewString()},
		{Method: "DELETE", Path: "/v2/me/following/bad", Actor: wallet, Status: 400, Code: "INVALID_REQUEST", RequestID: uuid.NewString()},
		{Method: "PUT", Path: "/v2/me/following/" + wallet, Actor: wallet, Status: 400, Code: "INVALID_REQUEST", RequestID: uuid.NewString()},
		{Method: "GET", Path: "/v2/profiles/" + wallet + "/follow-stats", Status: 503, Code: "DEPENDENCY_UNAVAILABLE", RequestID: uuid.NewString()},
		{Method: "PUT", Path: "/v2/me/following/" + wallet, Actor: "0x2222222222222222222222222222222222222222", Status: 503, Code: "DEPENDENCY_UNAVAILABLE", RequestID: uuid.NewString()},
		{Method: "DELETE", Path: "/v2/me/following/" + wallet, Actor: "0x2222222222222222222222222222222222222222", Status: 503, Code: "DEPENDENCY_UNAVAILABLE", RequestID: uuid.NewString()},
	}
	repair68SDK(t, server.URL, cases)
	for _, deps := range []Deps{
		{},
		{Durable: &fanout.DurableStore{}, AuthenticateChat: fanout.VerifiedRequestAuthenticator(sharedauth.HTTPIdentityAuthority{})},
	} {
		router := chi.NewRouter()
		New(deps).RegisterRoutes(router)
		for _, route := range []struct{ method, path string }{
			{"GET", "/pool/" + wallet + "/messages"},
			{"PATCH", "/pool/" + wallet + "/messages/" + uuid.NewString()},
			{"DELETE", "/pool/" + wallet + "/messages/" + uuid.NewString()},
			{"GET", "/dm/conversations"},
			{"GET", "/dm/conversations/" + uuid.NewString() + "/messages"},
		} {
			req := httptest.NewRequest(route.method, route.path, nil)
			req.Header.Set("X-Actor-Wallet", wallet)
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, req)
			if rec.Code != 503 || rec.Header().Get("Cache-Control") != "private, no-store" || !bytes.Contains(rec.Body.Bytes(), []byte("chat_unavailable")) {
				t.Fatalf("unwired %s %s: %d %s", route.method, route.path, rec.Code, rec.Body)
			}
		}
	}
	voiceRouter := chi.NewRouter()
	RegisterSafetyRoutes(voiceRouter, nil, nil)
	for _, suffix := range []string{"", "/prompt"} {
		req := httptest.NewRequest("GET", "/v2/voice-rooms/"+uuid.NewString()+suffix, nil)
		req = req.WithContext(safety.WithPrincipal(req.Context(), safety.Principal{Wallet: wallet, SessionID: uuid.NewString(), Expires: time.Now().Add(time.Hour)}))
		rec := httptest.NewRecorder()
		voiceRouter.ServeHTTP(rec, req)
		if rec.Code != 503 || v2.ValidateJSON("Error", rec.Body.Bytes()) != nil || bytes.Contains(rec.Body.Bytes(), []byte(wallet)) {
			t.Fatalf("unwired room read %s: %d %s", suffix, rec.Code, rec.Body)
		}
	}
}

func repair68Database(t *testing.T) (*pgxpool.Pool, func() *pgxpool.Pool) {
	t.Helper()
	return newRepair68Database(t)
}

func repair68Exec(t *testing.T, db *pgxpool.Pool, query string, args ...any) {
	t.Helper()
	if _, err := db.Exec(context.Background(), query, args...); err != nil {
		t.Fatalf("fixture SQL: %v", err)
	}
}

type repair68ErrorCase struct {
	Method    string `json:"method"`
	Path      string `json:"path"`
	Actor     string `json:"actor"`
	Status    int    `json:"status"`
	Code      string `json:"code"`
	RequestID string `json:"requestId"`
}

// The SDK's generated Go codec consumes the frozen Error schema in-process.
// It validates required/null fields before decoding, just like the SDK's
// transport boundary; a DTO-only decode would accept the legacy problem body.
func repair68SDK(t *testing.T, origin string, cases []repair68ErrorCase) {
	t.Helper()
	if err := v2.ValidateJSON("Error", []byte(`{"type":"about:blank","title":"Unauthorized","status":401,"detail":"missing actor"}`)); err == nil {
		t.Fatal("SDK codec accepted the legacy problem body")
	}
	for _, entry := range cases {
		req, err := http.NewRequest(entry.Method, origin+entry.Path, nil)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Request-ID", entry.RequestID)
		if entry.Actor != "" {
			req.Header.Set("X-Actor-Wallet", entry.Actor)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		var got v2.Error
		if err := proofLaunchpadSocialDecode("Error", raw, &got); err != nil {
			t.Fatalf("SDK Error decoding: %v: %s", err, raw)
		}
		if resp.StatusCode != entry.Status || got.Code != entry.Code || got.RequestId != entry.RequestID || resp.Header.Get("X-Request-ID") != entry.RequestID || got.Retryable != (entry.Status == 503) || got.RetryAfterMs != nil || got.FieldErrors == nil || len(got.FieldErrors) != 0 || got.Details != nil || resp.Header.Get("Cache-Control") != "private, no-store" {
			t.Fatalf("%s %s: status %d; SDK refusal %+v; headers %v", entry.Method, entry.Path, resp.StatusCode, got, resp.Header)
		}
	}
}

func TestRepair_68_FollowRefusalsDecodeInSDK(t *testing.T) {
	db, _ := repair68Database(t)
	router := chi.NewRouter()
	h := New(Deps{Queries: sqlcdb.New(db)})
	h.RegisterContractRoutes(router)
	server := httptest.NewServer(router)
	defer server.Close()
	const alice = "0x1111111111111111111111111111111111111111"
	const bob = "0x2222222222222222222222222222222222222222"
	cases := []repair68ErrorCase{
		{Method: "GET", Path: "/v2/profiles/bad/follow-stats", Status: 400, Code: "INVALID_REQUEST"},
		{Method: "PUT", Path: "/v2/me/following/" + bob, Status: 401, Code: "AUTH_REQUIRED"},
		{Method: "DELETE", Path: "/v2/me/following/" + bob, Status: 401, Code: "AUTH_REQUIRED"},
		{Method: "PUT", Path: "/v2/me/following/" + bob, Actor: "malformed", Status: 401, Code: "AUTH_REQUIRED"},
		{Method: "DELETE", Path: "/v2/me/following/" + bob, Actor: "malformed", Status: 401, Code: "AUTH_REQUIRED"},
		{Method: "PUT", Path: "/v2/me/following/bad", Actor: alice, Status: 400, Code: "INVALID_REQUEST"},
		{Method: "DELETE", Path: "/v2/me/following/bad", Actor: alice, Status: 400, Code: "INVALID_REQUEST"},
		{Method: "PUT", Path: "/v2/me/following/" + alice, Actor: alice, Status: 400, Code: "INVALID_REQUEST"},
	}
	for i := range cases {
		cases[i].RequestID = uuid.NewString()
	}
	repair68SDK(t, server.URL, cases)
	for _, supplied := range []string{"", "bad", uuid.Nil.String(), strings.ReplaceAll(uuid.NewString(), "-", "")} {
		req := httptest.NewRequest("PUT", "/v2/me/following/"+bob, nil)
		req.Header.Set("X-Request-ID", supplied)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		if rec.Code != 401 || v2.ValidateJSON("Error", rec.Body.Bytes()) != nil {
			t.Fatalf("uncorrelated refusal: %d %s", rec.Code, rec.Body)
		}
		var got v2.Error
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		parsed, err := uuid.Parse(got.RequestId)
		if err != nil || parsed == uuid.Nil || got.RequestId != rec.Header().Get("X-Request-ID") || len(got.RequestId) != 36 {
			t.Fatalf("invalid correlation: %+v", got)
		}
	}
	// The in-memory database peer enforces the restricted fixture role while
	// the real sqlc producer and handler execute reads and both mutations.
	repair68Exec(t, db, `CREATE ROLE repair_follow_reader LOGIN; GRANT USAGE ON SCHEMA chat TO repair_follow_reader`)
	cfg := db.Config().Copy()
	cfg.ConnConfig.User = "repair_follow_reader"
	restricted, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer restricted.Close()
	refused := chi.NewRouter()
	New(Deps{Queries: sqlcdb.New(restricted)}).RegisterContractRoutes(refused)
	dependencyServer := httptest.NewServer(refused)
	defer dependencyServer.Close()
	dependency := []repair68ErrorCase{
		{Method: "GET", Path: "/v2/profiles/" + bob + "/follow-stats", Status: 503, Code: "DEPENDENCY_UNAVAILABLE", RequestID: uuid.NewString()},
		{Method: "PUT", Path: "/v2/me/following/" + bob, Actor: alice, Status: 503, Code: "DEPENDENCY_UNAVAILABLE", RequestID: uuid.NewString()},
		{Method: "DELETE", Path: "/v2/me/following/" + bob, Actor: alice, Status: 503, Code: "DEPENDENCY_UNAVAILABLE", RequestID: uuid.NewString()},
	}
	repair68SDK(t, dependencyServer.URL, dependency)
	// A column grant lets the first count succeed while the second remains
	// refused by the authority peer, covering both read failure branches.
	repair68Exec(t, db, `GRANT SELECT(followee) ON chat.follows TO repair_follow_reader`)
	repair68SDK(t, dependencyServer.URL, dependency[:1])
	var count int
	if err := db.QueryRow(context.Background(), `SELECT count(*) FROM follows`).Scan(&count); err != nil || count != 0 {
		t.Fatalf("refusal changed graph: %d; %v", count, err)
	}
}

func TestRepair_68_PrivateVoiceReadAndPromptShareAdmission(t *testing.T) {
	db, _ := repair68Database(t)
	const hostWallet = "0x1111111111111111111111111111111111111111"
	const guestWallet = "0x2222222222222222222222222222222222222222"
	const poolID = "0x3333333333333333333333333333333333333333"
	hostID := uuid.NewString()
	repair68Exec(t, db, `INSERT INTO account_state_v2 VALUES($1,$2,false)`, hostID, hostWallet)
	repair68Exec(t, db, `INSERT INTO identity_wallet_links_v2 VALUES($1,$2,125,'verified')`, hostID, hostWallet)
	repair68Exec(t, db, `INSERT INTO indexer_canonical_creations_v1 VALUES($1,$2,$3)`, poolID, guestWallet, hostWallet)
	store := &safety.Store{DB: db}
	control := &voice.Control{DB: db, Policy: store, Pools: v2VoicePostgresPools{db: db}, Provider: &voice.LiveKit{}}
	router := chi.NewRouter()
	RegisterSafetyRoutes(router, store, control)
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p, err := safety.VerifySession(r.Header.Get("X-Fixture-Session"), r.Header.Get("X-Fixture-Signature"), public, time.Now())
		if err != nil {
			writeV2VoiceError(w, r, err)
			return
		}
		router.ServeHTTP(w, r.WithContext(safety.WithPrincipal(r.Context(), p)))
	}))
	defer server.Close()
	host := safety.Principal{Wallet: hostWallet, SessionID: uuid.NewString(), Expires: time.Now().Add(time.Hour)}
	guest := safety.Principal{Wallet: guestWallet, SessionID: uuid.NewString(), Expires: host.Expires}
	request := func(p safety.Principal, method, path, body string, status int) []byte {
		t.Helper()
		raw, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("X-Fixture-Session", base64.RawURLEncoding.EncodeToString(raw))
		req.Header.Set("X-Fixture-Signature", base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, raw)))
		req.Header.Set("X-Request-ID", uuid.NewString())
		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, err = io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != status {
			t.Fatalf("%s %s: %d want %d: %s", method, path, resp.StatusCode, status, raw)
		}
		if resp.Header.Get("Cache-Control") != "private, no-store" {
			t.Fatalf("private response cached: %v", resp.Header)
		}
		if status == 403 || status == 503 {
			if err := v2.ValidateJSON("Error", raw); err != nil {
				t.Fatalf("voice refusal: %v: %s", err, raw)
			}
			if bytes.Contains(raw, []byte("participants")) || bytes.Contains(raw, []byte("recording")) || bytes.Contains(raw, []byte(hostWallet)) {
				t.Fatalf("refusal leaked room metadata: %s", raw)
			}
		}
		return raw
	}
	created := request(host, "POST", "/v2/voice-rooms", `{"scopeType":"pool","scopeId":"`+poolID+`","title":"Private proof room","visibility":"private"}`, 201)
	if err := v2.ValidateJSON("VoiceCreateResponse", created); err != nil {
		t.Fatal(err)
	}
	var room v2.VoiceCreateResponse
	if err := json.Unmarshal(created, &room); err != nil {
		t.Fatal(err)
	}
	id := room.Data.Room.RoomId
	base := "/v2/voice-rooms/" + id
	recordingID := uuid.NewString()
	repair68Exec(t, db, `INSERT INTO voice_recordings(id,room_id,state,purpose,policy_version,delete_at) VALUES($1,$2,'consent_pending','Private recording purpose','proof-policy',now()+interval '1 day')`, recordingID, id)
	checkViews := func(p safety.Principal, status int) {
		t.Helper()
		read := request(p, "GET", base, "", status)
		prompt := request(p, "GET", base+"/prompt", "", status)
		if status == 200 {
			if err := v2.ValidateJSON("VoiceReadResponse", read); err != nil {
				t.Fatal(err)
			}
			var got struct {
				RoomID       string          `json:"roomId"`
				Recording    voice.Recording `json:"recording"`
				Participants []struct {
					Wallet string `json:"wallet"`
				} `json:"participants"`
			}
			if err := json.Unmarshal(prompt, &got); err != nil {
				t.Fatal(err)
			}
			if got.RoomID != id || got.Recording.ID != recordingID || got.Recording.Purpose != "Private recording purpose" || len(got.Participants) > 50 {
				t.Fatalf("admitted reader lost metadata or roster bound: %s", prompt)
			}
		}
	}
	checkViews(guest, 403)
	request(guest, "POST", base+"/tickets", `{"role":"listener","clientSessionId":"`+guest.SessionID+`"}`, 403)
	checkViews(host, 200)
	repair68Exec(t, db, `INSERT INTO voice_participants(room_id,wallet,role) VALUES($1,$2,'listener')`, id, guestWallet)
	checkViews(guest, 200)
	admitted := request(guest, "GET", base+"/prompt", "", 200)
	if !bytes.Contains(admitted, []byte(guestWallet)) {
		t.Fatalf("admitted participant absent from prompt: %s", admitted)
	}
	// Admission lookup is independent of the bounded roster page.
	for i := 0; i < 51; i++ {
		repair68Exec(t, db, `INSERT INTO voice_participants(room_id,wallet,role) VALUES($1,$2,'listener')`, id, fmt.Sprintf("0x%040x", i+1))
	}
	checkViews(guest, 200)
	for _, scope := range []struct{ kind, resource string }{{"voice", id}, {"pool", poolID}, {"global", ""}} {
		banID := uuid.NewString()
		repair68Exec(t, db, `INSERT INTO safety_bans(id,wallet,scope,resource,actor) VALUES($1,$2,$3,$4,$5)`, banID, guestWallet, scope.kind, scope.resource, hostWallet)
		checkViews(guest, 403)
		request(guest, "POST", base+"/tickets", `{"role":"listener","clientSessionId":"`+guest.SessionID+`"}`, 403)
		repair68Exec(t, db, `UPDATE safety_bans SET revoked_at=now() WHERE id=$1`, banID)
		checkViews(guest, 200)
	}
	repair68Exec(t, db, `UPDATE voice_participants SET revoked=true WHERE room_id=$1 AND wallet=$2`, id, guestWallet)
	checkViews(guest, 403)
	checkViews(host, 200)
	repair68Exec(t, db, `ALTER TABLE safety_bans RENAME TO unavailable_bans`)
	checkViews(host, 503)
}

func TestRepair_68_SocketRESTAndRestartUseDurableMessages(t *testing.T) {
	db, reopen := repair68Database(t)
	policyFor := repair68Policy
	policy := policyFor(db)
	const alice = "0x1111111111111111111111111111111111111111"
	const bob = "0x2222222222222222222222222222222222222222"
	const carol = "0x3333333333333333333333333333333333333333"
	const poolID = "0x4444444444444444444444444444444444444444"
	const otherPool = "0x5555555555555555555555555555555555555555"
	sessions := map[string]string{}
	for _, wallet := range []string{alice, bob, carol} {
		account, session := uuid.NewString(), uuid.NewString()
		hash := sha256.Sum256([]byte(session))
		now := time.Now().UTC()
		repair68Exec(t, db, `INSERT INTO identity_accounts VALUES($1,0,$2)`, account, now)
		repair68Exec(t, db, `INSERT INTO identity_wallet_links VALUES($1,$2,125,$3,'private','verified',$4)`, uuid.NewString(), account, wallet, now)
		repair68Exec(t, db, `INSERT INTO identity_sessions(session_id,account_id,wallet,auth_epoch,access_hash,csrf_hash,issued_at,access_expires_at,refresh_expires_at,reauthenticated_at,last_seen_at,device_label) VALUES($1,$2,$3,0,$4,$4,$5,$6,$6,$5,$5,'repair-proof')`, session, account, wallet, hash[:], now, now.Add(time.Hour))
		repair68Exec(t, db, `INSERT INTO account_state(account_id,wallet,profile,preferences) VALUES($1,$2,'{}','{}')`, account, wallet)
		repair68Exec(t, db, `INSERT INTO interaction_preferences(wallet,inbox) VALUES($1,'everyone')`, wallet)
		sessions[wallet] = session
	}
	repair68Exec(t, db, `INSERT INTO pools VALUES($1,$3),($2,$3)`, poolID, otherPool, bob)
	redisOptions := &goredis.Options{Addr: newRepair68Broker(t), Protocol: 2, DisableIdentity: true}
	pub := goredis.NewClient(redisOptions)
	defer pub.Close()
	namespace := "repair68:" + uuid.NewString() + ":"
	var store *fanout.DurableStore
	var authenticate fanout.AuthenticateUpgrade
	var server *httptest.Server
	var stop func()
	open := func() {
		policy = policyFor(db)
		store = &fanout.DurableStore{Pool: db, Policy: policy.SendPolicy, ReadPolicy: policy.ReadChatPolicy, CursorKey: []byte("repair-68-private-cursor-signing-key"), ChannelNamespace: namespace}
		authenticate = func(r *http.Request) (fanout.Principal, error) {
			var principal fanout.Principal
			err := db.QueryRow(r.Context(), `SELECT wallet,session_id::text,access_expires_at FROM identity_sessions WHERE session_id=$1::uuid`, r.Header.Get("X-Fixture-Session")).Scan(&principal.Wallet, &principal.SessionID, &principal.ExpiresAt)
			if err != nil {
				return principal, err
			}
			principal.Validate = func(ctx context.Context) error {
				return policy.ValidateSession(ctx, principal.Wallet, principal.SessionID)
			}
			return principal, nil
		}
		sub := goredis.NewClient(redisOptions)
		hub := fanout.New(fanout.Deps{Durable: store, Authenticate: authenticate, Pub: pub, Sub: sub, ChannelNamespace: namespace, MaxPoolMsgs: 100, MaxDmMsgs: 100})
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() { done <- hub.Run(ctx) }()
		router := chi.NewRouter()
		New(Deps{Queries: sqlcdb.New(db), Durable: store, AuthenticateChat: authenticate}).RegisterRoutes(router)
		router.Get("/ws", hub.ServeWS)
		server = httptest.NewServer(router)
		stop = func() {
			server.Close()
			cancel()
			if err := <-done; err != nil {
				t.Error(err)
			}
			sub.Close()
		}
	}
	open()
	defer func() { stop() }()
	unwired := chi.NewRouter()
	New(Deps{Queries: sqlcdb.New(db)}).RegisterRoutes(unwired)
	unwiredRequest := httptest.NewRequest("GET", "/pool/"+poolID+"/messages", nil)
	unwiredRequest.Header.Set("X-Actor-Wallet", alice)
	unwiredResponse := httptest.NewRecorder()
	unwired.ServeHTTP(unwiredResponse, unwiredRequest)
	if unwiredResponse.Code != 503 {
		t.Fatalf("unwired chat read a parallel legacy store: %d %s", unwiredResponse.Code, unwiredResponse.Body)
	}
	request := func(wallet, method, path, body, revision string, status int) []byte {
		t.Helper()
		req, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if wallet != "" {
			req.Header.Set("X-Fixture-Session", sessions[wallet])
		}
		if revision != "" {
			req.Header.Set("If-Match", revision)
		}
		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != status {
			t.Fatalf("%s %s: %d want %d: %s", method, path, resp.StatusCode, status, raw)
		}
		if resp.Header.Get("Cache-Control") != "private, no-store" {
			t.Fatalf("chat response cached: %v", resp.Header)
		}
		return raw
	}
	conn, _, err := websocket.DefaultDialer.Dial(strings.Replace(server.URL, "http:", "ws:", 1)+"/ws", http.Header{"X-Fixture-Session": {sessions[alice]}})
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	frame := func(want string) map[string]string {
		t.Helper()
		if err := conn.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
			t.Fatal(err)
		}
		for {
			_, raw, err := conn.ReadMessage()
			if err != nil {
				t.Fatal(err)
			}
			var value map[string]json.RawMessage
			if err := json.Unmarshal(raw, &value); err != nil {
				t.Fatal(err)
			}
			var kind string
			if err := json.Unmarshal(value["type"], &kind); err != nil {
				t.Fatal(err)
			}
			if kind == "error" {
				t.Fatalf("socket refused send: %s", raw)
			}
			if kind == want {
				out := map[string]string{}
				for key, rawValue := range value {
					var text string
					if json.Unmarshal(rawValue, &text) == nil {
						out[key] = text
					}
				}
				return out
			}
		}
	}
	send := func(value any) {
		t.Helper()
		if err := conn.WriteJSON(value); err != nil {
			t.Fatal(err)
		}
	}
	send(map[string]any{"type": "join_pool", "pool_address": poolID})
	frame("joined_pool")
	send(map[string]any{"type": "pool_message", "pool_address": poolID, "content": "<b>committed pool message</b>", "client_message_id": "repair68-pool"})
	persisted := frame("message_persisted")
	if persisted["sequence"] != "1" || persisted["revision"] != "1" || persisted["conversation_id"] != "pool:"+poolID || persisted["id"] == "" {
		t.Fatalf("not a durable acknowledgement: %v", persisted)
	}
	path := "/pool/" + poolID + "/messages"
	decodePage := func(raw []byte) fanout.HistoryPage {
		t.Helper()
		var page fanout.HistoryPage
		if err := json.Unmarshal(raw, &page); err != nil {
			t.Fatal(err)
		}
		return page
	}
	page := decodePage(request(alice, "GET", path, "", "", 200))
	if len(page.Messages) != 1 || page.Messages[0].ID != persisted["id"] || page.Messages[0].Content != "committed pool message" || page.Messages[0].Revision != 1 || page.Next == "" {
		t.Fatalf("socket did not round-trip through REST: %+v", page)
	}
	cursor := page.Next
	if next := decodePage(request(alice, "GET", path+"?cursor="+cursor, "", "", 200)); len(next.Messages) != 0 {
		t.Fatalf("replay duplicated message: %+v", next)
	}
	request("", "GET", path, "", "", 401)
	request(carol, "GET", path, "", "", 404)
	request(carol, "GET", path+"?cursor="+cursor, "", "", 400)
	request(alice, "GET", path+"?before="+persisted["id"], "", "", 400)
	// Another admitted pool must not make a message from this pool mutable.
	// Join through the socket, so membership comes from the real policy path.
	send(map[string]any{"type": "join_pool", "pool_address": otherPool})
	frame("joined_pool")
	request(alice, "PATCH", "/pool/"+otherPool+"/messages/"+persisted["id"], `{"content":"wrong pool"}`, "1", 404)
	request(alice, "PATCH", path+"/"+persisted["id"], `{"content":"edited durable content"}`, "", 400)
	request(alice, "PATCH", path+"/"+persisted["id"], `{"content":"edited durable content"}`, "1", 200)
	request(alice, "PATCH", path+"/"+persisted["id"], `{"content":"stale"}`, "1", 409)
	reset := request(alice, "GET", path+"?cursor="+cursor, "", "", 410)
	if got := decodePage(reset); !got.Reset || got.Next == "" {
		t.Fatalf("revision did not invalidate cursor: %s", reset)
	}
	page = decodePage(request(alice, "GET", path, "", "", 200))
	if len(page.Messages) != 1 || page.Messages[0].ID != persisted["id"] || page.Messages[0].Sequence != 1 || page.Messages[0].Revision != 2 || page.Messages[0].Content != "edited durable content" {
		t.Fatalf("edit changed durable identity: %+v", page)
	}
	send(map[string]any{"type": "dm", "to": bob, "content": "committed private DM", "client_message_id": "repair68-dm"})
	dm := frame("message_persisted")
	raw := request(bob, "GET", "/dm/conversations", "", "", 200)
	var conversations struct {
		Conversations []struct {
			ID, Peer string
			Last     *int64 `json:"last_message_at"`
		} `json:"conversations"`
	}
	if err := json.Unmarshal(raw, &conversations); err != nil {
		t.Fatal(err)
	}
	if len(conversations.Conversations) != 1 || conversations.Conversations[0].ID != dm["conversation_id"] || conversations.Conversations[0].Peer != alice || conversations.Conversations[0].Last == nil {
		t.Fatalf("DM summary missed socket producer: %s", raw)
	}
	dmPath := "/dm/conversations/" + dm["conversation_id"] + "/messages"
	dmPage := decodePage(request(bob, "GET", dmPath, "", "", 200))
	if len(dmPage.Messages) != 1 || dmPage.Messages[0].ID != dm["id"] || dmPage.Messages[0].Content != "committed private DM" {
		t.Fatalf("DM history missed socket send: %+v", dmPage)
	}
	request(carol, "GET", dmPath, "", "", 404)
	conn.Close()
	stop()
	db.Close()
	db = reopen()
	open()
	page = decodePage(request(alice, "GET", path, "", "", 200))
	if len(page.Messages) != 1 || page.Messages[0].ID != persisted["id"] || page.Messages[0].Revision != 2 || page.Messages[0].Content != "edited durable content" {
		t.Fatalf("restart lost edit: %+v", page)
	}
	request(alice, "DELETE", path+"/"+persisted["id"], "", "2", 200)
	page = decodePage(request(alice, "GET", path, "", "", 200))
	if len(page.Messages) != 1 || page.Messages[0].ID != persisted["id"] || !page.Messages[0].Deleted || page.Messages[0].Revision != 3 || page.Messages[0].Content != "" {
		t.Fatalf("delete lost durable tombstone: %+v", page)
	}
	stop()
	db.Close()
	db = reopen()
	open()
	page = decodePage(request(alice, "GET", path, "", "", 200))
	if len(page.Messages) != 1 || page.Messages[0].ID != persisted["id"] || !page.Messages[0].Deleted || page.Messages[0].Revision != 3 || page.Messages[0].Content != "" {
		t.Fatalf("restart resurrected message: %+v", page)
	}
	dmPage = decodePage(request(bob, "GET", dmPath, "", "", 200))
	if len(dmPage.Messages) != 1 || dmPage.Messages[0].ID != dm["id"] || dmPage.Messages[0].Content != "committed private DM" {
		t.Fatalf("restart lost DM: %+v", dmPage)
	}
	var messages, outbox, legacy int
	if err := db.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM chat_messages),(SELECT count(*) FROM chat_outbox),(SELECT count(*) FROM pool_messages)+(SELECT count(*) FROM dm_messages)`).Scan(&messages, &outbox, &legacy); err != nil || messages != 2 || outbox != 4 || legacy != 0 {
		t.Fatalf("parallel persistence or duplicate messages: %d %d %d; %v", messages, outbox, legacy, err)
	}
	repair68Exec(t, db, `UPDATE identity_sessions SET revoked_at=now() WHERE session_id=$1`, sessions[bob])
	request(bob, "GET", dmPath, "", "", 401)
}
