// Proves: voice.Control and AdaptV2VoiceCreate/Read/End/Leave/JoinTicket -> RegisterSafetyRoutes over net/http -> v2.ValidateJSON and typed Go consumer.
// The consumer is this test, the first caller of the exported Go surface. It
// starts disposable PostgreSQL and LiveKit processes, applies the embedded
// safety migration, and uses safety.VerifySession for the signed fixture session
// at the trusted principal boundary. HTTP, the route handlers, and the voice
// control/provider are never replaced. Reopening the HTTP server and database
// pool proves recovery, and fresh request IDs prove repeated reads cross HTTP.
// Repairs: moderation.go RegisterSafetyRoutes admitted uninvited private-room
// ticket requests despite refusing the same reader; voice_v2_wire.go
// AdaptV2VoiceJoinTicket accepted expired or overlong issuance lifetimes.
// RegisterSafetyRoutes deliberately refuses v2 moderation-actions until account
// to participant authority is mapped; this proof asserts that refusal and no
// participant mutation instead of inventing a successful moderation result.
package social

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/interactions"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/voice"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/migrations"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// This is the consumer's decoding boundary: a Go DTO alone does not enforce
// required fields, canonical revisions, or the frozen response schema.
func proofLaunchpadSocialDecode(schema string, raw []byte, into any) error {
	if err := v2.ValidateJSON(schema, raw); err != nil {
		return err
	}
	return json.Unmarshal(raw, into)
}

// PostgreSQL owns real durable state; only its temporary cluster and OS process
// are test infrastructure. No operator database or environment URL is used.
func proofLaunchpadSocialPostgres(t *testing.T) string {
	t.Helper()
	bindir, err := exec.Command("pg_config", "--bindir").Output()
	if err != nil {
		t.Fatalf("proof_launchpad_social_test.go requires PostgreSQL pg_config: %v", err)
	}
	dir, err := os.MkdirTemp("", "proof-social-postgres-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Error(err)
		}
	})
	var credential *syscall.Credential
	if os.Geteuid() == 0 {
		owner, err := user.Lookup("postgres")
		if err != nil {
			t.Fatalf("PostgreSQL cannot run as root and requires the postgres OS user: %v", err)
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
		if err := os.Chown(dir, int(uid), int(gid)); err != nil {
			t.Fatal(err)
		}
	}
	command := func(name string, args ...string) *exec.Cmd {
		cmd := exec.Command(filepath.Join(strings.TrimSpace(string(bindir)), name), args...)
		cmd.Dir = dir
		cmd.SysProcAttr = &syscall.SysProcAttr{Credential: credential}
		return cmd
	}
	data := filepath.Join(dir, "data")
	initCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	init := exec.CommandContext(initCtx, filepath.Join(strings.TrimSpace(string(bindir)), "initdb"), "-D", data, "-U", "proof_social", "-A", "trust", "--no-locale", "--encoding=UTF8")
	init.Dir, init.SysProcAttr = dir, &syscall.SysProcAttr{Credential: credential}
	if output, err := init.CombinedOutput(); err != nil {
		t.Fatalf("initialize real PostgreSQL cluster: %v; %s", err, output)
	}
	logPath := filepath.Join(dir, "postgres.log")
	log, err := os.Create(logPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log.Close() })
	server := command("postgres", "-D", data, "-h", "", "-k", dir, "-F")
	server.Stdout, server.Stderr = log, log
	if err := server.Start(); err != nil {
		t.Fatalf("start real PostgreSQL: %v", err)
	}
	t.Cleanup(func() {
		_ = server.Process.Signal(syscall.SIGTERM)
		if err := server.Wait(); err != nil {
			t.Errorf("stop real PostgreSQL: %v", err)
		}
	})
	dsn := "postgres://proof_social@/postgres?host=" + url.QueryEscape(dir) + "&sslmode=disable"
	readyCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	db, err := pgxpool.New(readyCtx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	for readyCtx.Err() == nil {
		if err := db.Ping(readyCtx); err == nil {
			return dsn
		}
		time.Sleep(25 * time.Millisecond)
	}
	output, _ := os.ReadFile(logPath)
	t.Fatalf("real PostgreSQL did not become ready: %s", output)
	return ""
}

func TestProofLaunchpadSocial(t *testing.T) {
	ctx := context.Background()
	dsn := proofLaunchpadSocialPostgres(t)
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := "proof_social_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	openDB := func() *pgxpool.Pool {
		t.Helper()
		cfg, err := pgxpool.ParseConfig(dsn)
		if err != nil {
			t.Fatal(err)
		}
		cfg.ConnConfig.RuntimeParams["search_path"] = schema
		cfg.MaxConns = 4
		db, err := pgxpool.NewWithConfig(ctx, cfg)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(db.Close)
		return db
	}
	db := openDB()
	execSQL := func(query string, args ...any) {
		t.Helper()
		if _, err := db.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	ddl, err := migrations.FS.ReadFile("00003_safety.sql")
	if err != nil {
		t.Fatal(err)
	}
	execSQL(strings.ReplaceAll(strings.SplitN(string(ddl), "-- +goose Down", 2)[0], "chat.", ident+"."))
	execSQL(`CREATE TABLE identity_wallet_links_v2(account_id uuid NOT NULL,wallet text NOT NULL,chain_id bigint NOT NULL,state text NOT NULL);
CREATE TABLE account_state_v2(account_id uuid PRIMARY KEY,wallet text NOT NULL,deleted boolean NOT NULL);
CREATE TABLE indexer_canonical_creations_v1(pool text PRIMARY KEY,token text NOT NULL,creator text NOT NULL);`)
	guardStart := strings.Index(interactions.RuntimeAuthoritySQL, "CREATE FUNCTION chat.social_account_for_wallet(")
	if guardStart < 0 {
		t.Fatal("production account guard is absent")
	}
	guard := strings.SplitN(interactions.RuntimeAuthoritySQL[guardStart:], ";", 2)[0] + ";"
	execSQL(strings.NewReplacer("chat.", ident+".", "users.", ident+".").Replace(guard))
	const hostWallet = "0x1111111111111111111111111111111111111111"
	const guestWallet = "0x2222222222222222222222222222222222222222"
	const poolID = "0x3333333333333333333333333333333333333333"
	hostID := uuid.NewString()
	execSQL(`INSERT INTO account_state_v2 VALUES($1,$2,false)`, hostID, hostWallet)
	execSQL(`INSERT INTO identity_wallet_links_v2 VALUES($1,$2,125,'verified')`, hostID, hostWallet)
	execSQL(`INSERT INTO indexer_canonical_creations_v1 VALUES($1,$2,$3)`, poolID, guestWallet, hostWallet)

	provider := v2VoiceActionsLiveKit(t)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	host := safety.Principal{Wallet: hostWallet, SessionID: uuid.NewString(), Expires: time.Now().Add(time.Hour)}
	guest := safety.Principal{Wallet: guestWallet, SessionID: uuid.NewString(), Expires: time.Now().Add(time.Hour)}
	var control *voice.Control
	openServer := func() *httptest.Server {
		store := &safety.Store{DB: db}
		control = &voice.Control{DB: db, Policy: store, Pools: v2VoicePostgresPools{db: db}, Provider: provider,
			Gates: voice.Gates{ProviderQualified: true, EjectionQualified: true, ACLQualified: true, RetentionQualified: true}}
		router := chi.NewRouter()
		RegisterSafetyRoutes(router, store, control)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			p, err := safety.VerifySession(r.Header.Get("X-Fixture-Session"), r.Header.Get("X-Fixture-Signature"), publicKey, time.Now())
			if err != nil {
				writeV2VoiceError(w, r, err)
				return
			}
			router.ServeHTTP(w, r.WithContext(safety.WithPrincipal(r.Context(), p)))
		}))
		t.Cleanup(server.Close)
		return server
	}
	server := openServer()
	client := &http.Client{Timeout: 10 * time.Second}
	t.Cleanup(client.CloseIdleConnections)
	request := func(p safety.Principal, method, path, body string, status int, schema string, into any) []byte {
		t.Helper()
		payload, err := json.Marshal(p)
		if err != nil {
			t.Fatal(err)
		}
		req, err := http.NewRequest(method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		id := uuid.NewString()
		req.Header.Set("X-Request-ID", id)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Fixture-Session", base64.RawURLEncoding.EncodeToString(payload))
		req.Header.Set("X-Fixture-Signature", base64.RawURLEncoding.EncodeToString(ed25519.Sign(privateKey, payload)))
		resp, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		raw, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != status {
			t.Fatalf("%s %s: status %d, want %d: %s", method, path, resp.StatusCode, status, raw)
		}
		if resp.Header.Get("X-Request-ID") != id || resp.Header.Get("Content-Type") != "application/json" || resp.Header.Get("Cache-Control") != "private, no-store" {
			t.Fatalf("%s headers: %v", path, resp.Header)
		}
		if err := proofLaunchpadSocialDecode(schema, raw, into); err != nil {
			t.Fatalf("%s decode %s: %v; %s", path, schema, err, raw)
		}
		var envelope struct {
			RequestID string `json:"requestId"`
		}
		if err := json.Unmarshal(raw, &envelope); err != nil || envelope.RequestID != id {
			t.Fatalf("response did not come from this request: %s; %v", raw, err)
		}
		return raw
	}
	refused := func(p safety.Principal, method, path, body string, status int, code string) {
		t.Helper()
		var got v2.Error
		request(p, method, path, body, status, "Error", &got)
		if got.Code != code || got.Retryable != (status == http.StatusServiceUnavailable) {
			t.Fatalf("refusal: %+v", got)
		}
	}
	var created v2.VoiceCreateResponse
	createBody := `{"scopeType":"pool","scopeId":"` + poolID + `","title":"Proof room","visibility":"private"}`
	createRaw := request(host, http.MethodPost, "/v2/voice-rooms", createBody, 201, "VoiceCreateResponse", &created)
	want := created.Data.Room
	if want.HostAccountId != hostID || want.ScopeId != poolID || want.State != "scheduled" || want.Revision != "1" || want.Visibility != "private" || !created.Data.Capabilities.ProviderQualified {
		t.Fatalf("created room: %+v", created)
	}
	if _, err := uuid.Parse(want.RoomId); err != nil {
		t.Fatal(err)
	}
	base := "/v2/voice-rooms/" + want.RoomId
	// A malformed producer result reaches the same decoding function as the
	// unmodified HTTP result; DTO unmarshalling alone would accept missing data.
	var malformed map[string]json.RawMessage
	if err := json.Unmarshal(createRaw, &malformed); err != nil {
		t.Fatal(err)
	}
	delete(malformed, "data")
	badRaw, err := json.Marshal(malformed)
	if err != nil {
		t.Fatal(err)
	}
	if err := proofLaunchpadSocialDecode("VoiceCreateResponse", badRaw, &v2.VoiceCreateResponse{}); err == nil {
		t.Fatal("consumer accepted a producer envelope missing data")
	}
	if err := proofLaunchpadSocialDecode("VoiceCreateResponse", createRaw[:len(createRaw)-1], &v2.VoiceCreateResponse{}); err == nil {
		t.Fatal("consumer accepted a truncated producer result")
	}

	// No handler or connection survives the reopen; PostgreSQL and LiveKit do.
	server.Close()
	db.Close()
	db = openDB()
	server = openServer()
	readRoom := func(p safety.Principal, actions []string) {
		t.Helper()
		for i := 0; i < 2; i++ {
			var got v2.VoiceReadResponse
			request(p, http.MethodGet, base, "", 200, "VoiceReadResponse", &got)
			if got.Data.Room != want || !reflect.DeepEqual(got.Data.AllowedActions, actions) || got.Revision == nil || string(*got.Revision) != string(want.Revision) {
				t.Fatalf("read after reopen: %+v; want %+v, actions %v", got, want, actions)
			}
		}
	}
	readRoom(host, []string{})
	refused(guest, http.MethodGet, base, "", 403, "SCOPE_DENIED")
	refused(host, http.MethodGet, "/v2/voice-rooms/"+uuid.NewString(), "", 404, "NOT_FOUND")
	refused(host, http.MethodPost, "/v2/voice-rooms", createBody+" {}", 400, "INVALID_REQUEST")
	refused(guest, http.MethodPost, base+":end", `{"expectedRevision":"1"}`, 403, "SCOPE_DENIED")

	// Start the real provider via the production control method. No database
	// UPDATE invents a live room, a provider identity, or a participant grant.
	room, err := control.Start(safety.WithPrincipal(ctx, host), want.RoomId, 1)
	if err != nil || room.State != "live" || room.Revision != 2 {
		t.Fatalf("start: %+v; %v", room, err)
	}
	t.Cleanup(func() {
		if err := provider.End(context.Background(), room.ProviderSession); err != nil {
			t.Error(err)
		}
	})
	want.State, want.Revision = "live", "2"
	readRoom(host, []string{"end"})
	ticketBody := `{"role":"speaker","clientSessionId":"` + guest.SessionID + `"}`
	refused(guest, http.MethodPost, base+"/tickets", ticketBody, 403, "SCOPE_DENIED")
	var count int
	if err := db.QueryRow(ctx, `SELECT count(*) FROM voice_participants WHERE room_id=$1`, room.ID).Scan(&count); err != nil || count != 0 {
		t.Fatalf("private refusal created participant: %d; %v", count, err)
	}
	// Existing private-room participants are admitted, just as GET permits them.
	if err := control.Join(safety.WithPrincipal(ctx, guest), room.ID); err != nil {
		t.Fatal(err)
	}
	readRoom(guest, []string{"leave"})
	var ticket v2.VoiceJoinTicketResponse
	for i := 0; i < 2; i++ {
		requestedAt := time.Now()
		request(guest, http.MethodPost, base+"/tickets", ticketBody, 200, "VoiceJoinTicketResponse", &ticket)
		expiresAt, err := time.Parse(time.RFC3339Nano, ticket.Data.ExpiresAt)
		if err != nil || !expiresAt.After(requestedAt) || expiresAt.After(time.Now().Add(time.Minute)) {
			t.Fatalf("consumer observed invalid ticket expiry: %s; %v", ticket.Data.ExpiresAt, err)
		}
		parts := strings.Split(ticket.Data.Ticket, ".")
		if len(parts) != 3 {
			t.Fatal("provider did not issue a JWT")
		}
		mac := hmac.New(sha256.New, []byte(provider.APISecret))
		_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
		sig, err := base64.RawURLEncoding.DecodeString(parts[2])
		if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
			t.Fatal("provider ticket signature mismatch")
		}
		raw, err := base64.RawURLEncoding.DecodeString(parts[1])
		if err != nil {
			t.Fatal(err)
		}
		var claims struct {
			Subject string `json:"sub"`
			Issued  int64  `json:"iat"`
			Expires int64  `json:"exp"`
			Video   struct {
				Room      string `json:"room"`
				Join      bool   `json:"roomJoin"`
				Publish   bool   `json:"canPublish"`
				Subscribe bool   `json:"canSubscribe"`
			} `json:"video"`
		}
		if err := json.Unmarshal(raw, &claims); err != nil {
			t.Fatal(err)
		}
		if claims.Subject != guestWallet || claims.Video.Room != room.ProviderSession || !claims.Video.Join || !claims.Video.Subscribe || claims.Video.Publish || claims.Expires-claims.Issued != 60 || claims.Expires <= time.Now().Unix() || ticket.Data.ProviderEndpoint != provider.Endpoint {
			t.Fatalf("consumer observed wrong ticket authority: %+v; %+v", claims, ticket)
		}
	}
	// All exported adapters also compile against real producer values.
	at := time.Now().UTC()
	adapters := []struct {
		name  string
		adapt func() ([]byte, error)
	}{
		{"VoiceCreateResponse", func() ([]byte, error) {
			return AdaptV2VoiceCreate(uuid.NewString(), room, hostID, "private", created.Data.Capabilities, at)
		}},
		{"VoiceReadResponse", func() ([]byte, error) {
			return AdaptV2VoiceRead(uuid.NewString(), room, hostID, "private", []string{"leave"}, at)
		}},
		{"VoiceJoinTicketResponse", func() ([]byte, error) {
			return AdaptV2VoiceJoinTicket(uuid.NewString(), ticket.Data.Ticket, provider.Endpoint, at.Add(time.Minute), at)
		}},
	}
	for _, adapter := range adapters {
		for i := 0; i < 2; i++ {
			raw, err := adapter.adapt()
			if err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := proofLaunchpadSocialDecode(adapter.name, raw, &got); err != nil || got["quality"] != "complete" {
				t.Fatalf("%s consumer: %v; %v", adapter.name, got, err)
			}
		}
	}
	for _, expiry := range []time.Time{at.Add(-time.Second), at, at.Add(time.Minute + time.Nanosecond)} {
		if raw, err := AdaptV2VoiceJoinTicket(uuid.NewString(), ticket.Data.Ticket, provider.Endpoint, expiry, at); err == nil || raw != nil {
			t.Fatal("adapter accepted a ticket outside its issuance lifetime")
		}
	}
	moderateBody := fmt.Sprintf(`{"participantAccountId":%q,"action":"mute","reason":"Proof policy","expectedRevision":"1"}`, hostID)
	refused(host, http.MethodPost, base+"/moderation-actions", moderateBody, 503, "DEPENDENCY_UNAVAILABLE")
	refused(host, http.MethodPost, base+"/moderation-actions", "{}", 400, "INVALID_REQUEST")
	var role, grant string
	var muted, revoked bool
	var revision int64
	participant := func() {
		t.Helper()
		if err := db.QueryRow(ctx, `SELECT role,muted,revoked,grant_state,revision FROM voice_participants WHERE room_id=$1 AND wallet=$2`, room.ID, guestWallet).Scan(&role, &muted, &revoked, &grant, &revision); err != nil {
			t.Fatal(err)
		}
	}
	participant()
	if role != "listener" || muted || revoked || grant != "active" || revision != 1 {
		t.Fatalf("ticket or unavailable moderation escalated participant: %s %t %t %s %d", role, muted, revoked, grant, revision)
	}
	leaveBody := fmt.Sprintf(`{"clientSessionId":%q}`, guest.SessionID)
	for i := 0; i < 2; i++ {
		var left v2.VoiceLeaveResponse
		request(guest, http.MethodPost, base+":leave", leaveBody, 200, "VoiceLeaveResponse", &left)
		if left.Data.GrantState != "revoked" || left.Data.Revision != "2" || left.Revision == nil || *left.Revision != "2" {
			t.Fatalf("leave consumer: %+v", left)
		}
	}
	participant()
	if !muted || !revoked || grant != "revoked" || revision != 2 {
		t.Fatal("leave did not persist grant revocation")
	}
	readRoom(host, []string{"end"})
	refused(guest, http.MethodGet, base, "", 403, "SCOPE_DENIED")
	refused(guest, http.MethodPost, base+"/tickets", ticketBody, 403, "SCOPE_DENIED")
	for i, expected := range []string{"2", "4"} {
		var ended v2.VoiceEndResponse
		request(host, http.MethodPost, base+":end", fmt.Sprintf(`{"expectedRevision":%q}`, expected), 200, "VoiceEndResponse", &ended)
		want.State, want.Revision = "ended", "4"
		if ended.Data.Room != want || ended.Revision == nil || *ended.Revision != "4" {
			t.Fatalf("end consumer %d: %+v", i, ended)
		}
	}
	if _, exists, err := provider.Find(ctx, room.StartKey); err != nil || exists {
		t.Fatalf("provider session survived end: %t; %v", exists, err)
	}
	server.Close()
	db.Close()
	db = openDB()
	server = openServer()
	readRoom(host, []string{})
	endedRoom, err := control.Read(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := AdaptV2VoiceEnd(uuid.NewString(), endedRoom, hostID, "private", at)
	if err != nil {
		t.Fatal(err)
	}
	var end v2.VoiceEndResponse
	if err := proofLaunchpadSocialDecode("VoiceEndResponse", raw, &end); err != nil || end.Data.Room != want {
		t.Fatalf("exported end consumer: %+v; %v", end, err)
	}
	raw, err = AdaptV2VoiceLeave(uuid.NewString(), grant, revision, at)
	if err != nil {
		t.Fatal(err)
	}
	var leave v2.VoiceLeaveResponse
	if err := proofLaunchpadSocialDecode("VoiceLeaveResponse", raw, &leave); err != nil || leave.Data.GrantState != grant || leave.Data.Revision != "2" {
		t.Fatalf("exported leave consumer: %+v; %v", leave, err)
	}
}
