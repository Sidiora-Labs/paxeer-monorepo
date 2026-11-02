package social

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strings"
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

func TestPostgresV2VoiceEndLeaveAndTicket(t *testing.T) {
	ctx := context.Background()
	dsn := os.Getenv("L38_POSTGRES_DSN")
	if dsn == "" {
		t.Fatal("L38_POSTGRES_DSN is required for the real PostgreSQL voice fixture")
	}
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := "voice_v2_actions_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	ident := pgx.Identifier{schema}.Sanitize()
	if _, err = admin.Exec(ctx, "CREATE SCHEMA "+ident); err != nil {
		t.Fatalf("create PostgreSQL voice fixture (L38_POSTGRES_DSN): %v", err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(context.Background(), "DROP SCHEMA "+ident+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
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
	ddl, err := migrations.FS.ReadFile("00003_safety.sql")
	if err != nil {
		t.Fatal(err)
	}
	up := strings.SplitN(string(ddl), "-- +goose Down", 2)[0]
	if _, err = db.Exec(ctx, strings.ReplaceAll(up, "chat.", ident+".")); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `
CREATE TABLE identity_wallet_links_v2(account_id uuid NOT NULL,wallet text NOT NULL,chain_id bigint NOT NULL,state text NOT NULL);
CREATE TABLE account_state_v2(account_id uuid PRIMARY KEY,wallet text NOT NULL,deleted boolean NOT NULL);
CREATE TABLE indexer_canonical_creations_v1(pool text PRIMARY KEY,token text NOT NULL,creator text NOT NULL);`); err != nil {
		t.Fatal(err)
	}
	guardStart := strings.Index(interactions.RuntimeAuthoritySQL, "CREATE FUNCTION chat.social_account_for_wallet(")
	if guardStart < 0 {
		t.Fatal("production account guard is absent")
	}
	guard := strings.SplitN(interactions.RuntimeAuthoritySQL[guardStart:], ";", 2)[0] + ";"
	guard = strings.NewReplacer("chat.", ident+".", "users.", ident+".").Replace(guard)
	if _, err = db.Exec(ctx, guard); err != nil {
		t.Fatal(err)
	}
	const hostWallet = "0x1111111111111111111111111111111111111111"
	const guestWallet = "0x2222222222222222222222222222222222222222"
	const poolID = "0x3333333333333333333333333333333333333333"
	hostID := uuid.NewString()
	if _, err = db.Exec(ctx, `INSERT INTO account_state_v2 VALUES($1,$2,false)`, hostID, hostWallet); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO identity_wallet_links_v2 VALUES($1,$2,125,'verified')`, hostID, hostWallet); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, `INSERT INTO indexer_canonical_creations_v1 VALUES($1,$2,$3)`, poolID, guestWallet, hostWallet); err != nil {
		t.Fatal(err)
	}
	store := &safety.Store{DB: db}
	control := &voice.Control{DB: db, Policy: store, Pools: v2VoicePostgresPools{db: db}}
	router := chi.NewRouter()
	RegisterSafetyRoutes(router, store, control)
	host := safety.Principal{Wallet: hostWallet, SessionID: uuid.NewString(), Expires: time.Now().Add(time.Hour)}
	guest := safety.Principal{Wallet: guestWallet, SessionID: uuid.NewString(), Expires: time.Now().Add(time.Hour)}
	requestID := uuid.NewString()
	request := func(routes http.Handler, principal *safety.Principal, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("X-Request-ID", requestID)
		req.Header.Set("Content-Type", "application/json")
		if principal != nil {
			req = req.WithContext(safety.WithPrincipal(req.Context(), *principal))
		}
		rec := httptest.NewRecorder()
		routes.ServeHTTP(rec, req)
		return rec
	}
	assertError := func(rec *httptest.ResponseRecorder, status int, code string) {
		t.Helper()
		if rec.Code != status {
			t.Fatalf("status = %d; want %d; body %s", rec.Code, status, rec.Body.String())
		}
		if err := v2.ValidateJSON("Error", rec.Body.Bytes()); err != nil {
			t.Fatalf("frozen Error: %v; body %s", err, rec.Body.String())
		}
		var got v2.Error
		if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
			t.Fatal(err)
		}
		if got.Code != code || got.Retryable != (status == 503) || got.RequestId != requestID || rec.Header().Get("X-Request-ID") != requestID || rec.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("error = %+v; headers %v", got, rec.Header())
		}
	}
	created := request(router, &host, "/v2/voice-rooms", `{"scopeType":"pool","scopeId":"`+poolID+`","title":"Actions room","visibility":"private"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create status = %d; body %s", created.Code, created.Body.String())
	}
	var create v2.VoiceCreateResponse
	if err := json.Unmarshal(created.Body.Bytes(), &create); err != nil {
		t.Fatal(err)
	}
	id := create.Data.Room.RoomId
	base := "/v2/voice-rooms/" + id
	endBody := `{"expectedRevision":"1"}`
	leaveBody := `{"clientSessionId":"` + guest.SessionID + `"}`
	ticketBody := `{"role":"speaker","clientSessionId":"` + guest.SessionID + `"}`
	moderateBody := `{"participantAccountId":"` + hostID + `","action":"mute","reason":"Room policy","expectedRevision":"1"}`
	for _, action := range []struct{ suffix, body string }{
		{":end", endBody}, {":leave", leaveBody}, {"/tickets", ticketBody}, {"/moderation-actions", moderateBody},
	} {
		t.Run("validation"+action.suffix, func(t *testing.T) {
			for _, body := range []string{"", "{}", "null", action.body + " {}", strings.TrimSuffix(action.body, "}") + `,"unknown":true}`} {
				assertError(request(router, &host, base+action.suffix, body), 400, "INVALID_REQUEST")
			}
			assertError(request(router, &host, "/v2/voice-rooms/not-a-uuid"+action.suffix, action.body), 400, "INVALID_REQUEST")
			unavailable := chi.NewRouter()
			RegisterSafetyRoutes(unavailable, store, nil)
			assertError(request(unavailable, &host, base+action.suffix, action.body), 503, "DEPENDENCY_UNAVAILABLE")
		})
	}
	for _, revision := range []string{`1`, `"-1"`, `"01"`, `"9223372036854775808"`} {
		assertError(request(router, &host, base+":end", `{"expectedRevision":`+revision+`}`), 400, "INVALID_REQUEST")
	}
	assertError(request(router, nil, base+":end", endBody), 403, "SCOPE_DENIED")
	assertError(request(router, &guest, base+":end", endBody), 403, "SCOPE_DENIED")
	assertError(request(router, &host, base+":end", endBody), 409, "CONFLICT")
	assertError(request(router, &host, base+"/tickets", ticketBody), 503, "DEPENDENCY_UNAVAILABLE")
	if _, err := db.Exec(ctx, `UPDATE voice_rooms SET state='live' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	assertError(request(router, &host, base+":end", `{"expectedRevision":"0"}`), 409, "CONFLICT")
	assertError(request(router, &host, base+":end", endBody), 503, "DEPENDENCY_UNAVAILABLE")
	room, err := control.Read(ctx, id)
	if err != nil || room.State != "ending" || room.Revision != 2 {
		t.Fatalf("end did not persist cleanup intent: %+v; %v", room, err)
	}
	assertError(request(router, &host, base+":end", endBody), 409, "CONFLICT")
	if _, err := db.Exec(ctx, `UPDATE voice_rooms SET state='ended',revision=3 WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	ended := request(router, &host, base+":end", `{"expectedRevision":"3"}`)
	if ended.Code != http.StatusOK {
		t.Fatalf("idempotent end status = %d; body %s", ended.Code, ended.Body.String())
	}
	if err := v2.ValidateJSON("VoiceEndResponse", ended.Body.Bytes()); err != nil {
		t.Fatal(err)
	}
	var end v2.VoiceEndResponse
	if err := json.Unmarshal(ended.Body.Bytes(), &end); err != nil {
		t.Fatal(err)
	}
	wantRoom := create.Data.Room
	wantRoom.State, wantRoom.Revision = "ended", "3"
	if end.Data.Room != wantRoom || end.RequestId != requestID || end.Revision == nil || *end.Revision != "3" || end.Quality != "complete" || ended.Header().Get("Content-Type") != "application/json" || ended.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("end = %+v; headers %v", end, ended.Header())
	}
	for _, secret := range []string{hostWallet, room.StartKey, "providerSession"} {
		if strings.Contains(ended.Body.String(), secret) {
			t.Fatalf("end exposed internal field %q", secret)
		}
	}
	if _, err := db.Exec(ctx, `UPDATE voice_rooms SET state='live' WHERE id=$1`, id); err != nil {
		t.Fatal(err)
	}
	control.Provider = &voice.LiveKit{}
	control.Gates = voice.Gates{ProviderQualified: true, EjectionQualified: true, ACLQualified: true, RetentionQualified: true}
	assertError(request(router, nil, base+"/tickets", ticketBody), 403, "SCOPE_DENIED")
	assertError(request(router, &guest, base+"/tickets", ticketBody), 503, "DEPENDENCY_UNAVAILABLE")
	var role, grant string
	var muted, revoked bool
	var revision int64
	readParticipant := func() {
		t.Helper()
		if err := db.QueryRow(ctx, `SELECT role,muted,revoked,grant_state,revision FROM voice_participants WHERE room_id=$1 AND wallet=$2`, id, guestWallet).Scan(&role, &muted, &revoked, &grant, &revision); err != nil {
			t.Fatal(err)
		}
	}
	readParticipant()
	if role != "listener" || muted || revoked || grant != "active" || revision != 1 {
		t.Fatalf("ticket admission escalated requested speaker: %s %t %t %s %d", role, muted, revoked, grant, revision)
	}
	for _, action := range []string{"mute", "unmute", "demote", "remove", "ban"} {
		assertError(request(router, &host, base+"/moderation-actions", strings.Replace(moderateBody, `"mute"`, `"`+action+`"`, 1)), 503, "DEPENDENCY_UNAVAILABLE")
	}
	readParticipant()
	if role != "listener" || muted || revoked || grant != "active" || revision != 1 {
		t.Fatal("unmapped moderation changed participant")
	}
	assertError(request(router, nil, base+":leave", leaveBody), 403, "SCOPE_DENIED")
	assertError(request(router, &guest, base+":leave", leaveBody), 503, "DEPENDENCY_UNAVAILABLE")
	readParticipant()
	if !muted || !revoked || grant != "revoking" || revision != 2 {
		t.Fatalf("leave did not persist cleanup intent: %s %t %t %s %d", role, muted, revoked, grant, revision)
	}
	recordingID := uuid.NewString()
	if _, err := db.Exec(ctx, `INSERT INTO voice_recordings(id,room_id,state,purpose,policy_version,delete_at) VALUES($1,$2,'consent_pending','Explicit purpose','v2',now()+interval '1 day')`, recordingID, id); err != nil {
		t.Fatal(err)
	}
	assertError(request(router, &guest, base+"/tickets", ticketBody), 403, "SCOPE_DENIED")
	readParticipant()
	if !muted || !revoked || grant != "revoking" || revision != 2 {
		t.Fatal("ticket without consent changed revoked participant")
	}
	var consents, bans, ejections int
	if err := db.QueryRow(ctx, `SELECT (SELECT count(*) FROM voice_consents),(SELECT count(*) FROM safety_bans),(SELECT count(*) FROM voice_ejections)`).Scan(&consents, &bans, &ejections); err != nil || consents != 0 || bans != 0 || ejections != 0 {
		t.Fatalf("actions invented consent or moderation authority: %d %d %d; %v", consents, bans, ejections, err)
	}

	provider := v2VoiceActionsLiveKit(t)
	control.Provider = provider
	created = request(router, &host, "/v2/voice-rooms", `{"scopeType":"pool","scopeId":"`+poolID+`","title":"Live actions room","visibility":"public"}`)
	if created.Code != http.StatusCreated {
		t.Fatalf("create live room: %d; %s", created.Code, created.Body.String())
	}
	if err := json.Unmarshal(created.Body.Bytes(), &create); err != nil {
		t.Fatal(err)
	}
	id = create.Data.Room.RoomId
	base = "/v2/voice-rooms/" + id
	started := request(router, &host, base+"/start", `{"expectedRevision":1}`)
	if started.Code != http.StatusOK {
		t.Fatalf("start real provider room: %d; %s", started.Code, started.Body.String())
	}
	t.Cleanup(func() {
		room, err := control.Read(ctx, id)
		if err == nil && room.State != "ended" {
			if err := provider.End(ctx, room.ProviderSession); err != nil {
				t.Error(err)
			}
		}
	})
	ticketed := request(router, &guest, base+"/tickets", ticketBody)
	if ticketed.Code != http.StatusOK {
		t.Fatalf("ticket status = %d; body %s", ticketed.Code, ticketed.Body.String())
	}
	if err := v2.ValidateJSON("VoiceJoinTicketResponse", ticketed.Body.Bytes()); err != nil {
		t.Fatal(err)
	}
	var ticket v2.VoiceJoinTicketResponse
	if err := json.Unmarshal(ticketed.Body.Bytes(), &ticket); err != nil {
		t.Fatal(err)
	}
	expires, err := time.Parse(time.RFC3339Nano, ticket.Data.ExpiresAt)
	if err != nil || !expires.After(time.Now()) || expires.After(time.Now().Add(time.Minute)) || ticket.Data.ProviderEndpoint != provider.Endpoint || ticket.RequestId != requestID || ticket.Quality != "complete" || ticket.Revision != nil || ticketed.Header().Get("Cache-Control") != "private, no-store" {
		t.Fatalf("ticket envelope = %+v; headers %v; error %v", ticket, ticketed.Header(), err)
	}
	parts := strings.Split(ticket.Data.Ticket, ".")
	if len(parts) != 3 {
		t.Fatal("ticket is not a JWT")
	}
	mac := hmac.New(sha256.New, []byte(provider.APISecret))
	_, _ = mac.Write([]byte(parts[0] + "." + parts[1]))
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(signature, mac.Sum(nil)) {
		t.Fatal("ticket signature does not match the real provider key")
	}
	claimsJSON, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		t.Fatal(err)
	}
	var claims struct {
		Subject string `json:"sub"`
		Video   struct {
			Room      string `json:"room"`
			Join      bool   `json:"roomJoin"`
			Publish   bool   `json:"canPublish"`
			Subscribe bool   `json:"canSubscribe"`
		} `json:"video"`
	}
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		t.Fatal(err)
	}
	room, err = control.Read(ctx, id)
	if err != nil || room.State != "live" || room.Revision != 2 || claims.Subject != guestWallet || claims.Video.Room != room.ProviderSession || !claims.Video.Join || !claims.Video.Subscribe || claims.Video.Publish {
		t.Fatalf("ticket did not preserve listener authority: %+v; %+v; %v", claims, room, err)
	}
	readParticipant()
	if role != "listener" || muted || revoked || grant != "active" || revision != 1 {
		t.Fatalf("real ticket admission: %s %t %t %s %d", role, muted, revoked, grant, revision)
	}
	assertError(request(router, &host, base+"/moderation-actions", moderateBody), 503, "DEPENDENCY_UNAVAILABLE")
	for i := 0; i < 2; i++ {
		left := request(router, &guest, base+":leave", leaveBody)
		if left.Code != http.StatusOK {
			t.Fatalf("leave status = %d; body %s", left.Code, left.Body.String())
		}
		if err := v2.ValidateJSON("VoiceLeaveResponse", left.Body.Bytes()); err != nil {
			t.Fatal(err)
		}
		var leave v2.VoiceLeaveResponse
		if err := json.Unmarshal(left.Body.Bytes(), &leave); err != nil {
			t.Fatal(err)
		}
		if leave.Data.GrantState != "revoked" || leave.Data.Revision != "2" || leave.Revision == nil || *leave.Revision != "2" || leave.RequestId != requestID || leave.Quality != "complete" || left.Header().Get("Cache-Control") != "private, no-store" {
			t.Fatalf("leave = %+v; headers %v", leave, left.Header())
		}
	}
	readParticipant()
	if !muted || !revoked || grant != "revoked" || revision != 2 {
		t.Fatalf("real leave did not revoke participant: %s %t %t %s %d", role, muted, revoked, grant, revision)
	}
	ended = request(router, &host, base+":end", `{"expectedRevision":"2"}`)
	if ended.Code != http.StatusOK {
		t.Fatalf("end real provider room: %d; %s", ended.Code, ended.Body.String())
	}
	if err := v2.ValidateJSON("VoiceEndResponse", ended.Body.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(ended.Body.Bytes(), &end); err != nil {
		t.Fatal(err)
	}
	wantRoom = create.Data.Room
	wantRoom.State, wantRoom.Revision = "ended", "4"
	if end.Data.Room != wantRoom || end.RequestId != requestID || end.Revision == nil || *end.Revision != "4" || end.Quality != "complete" {
		t.Fatalf("real end = %+v", end)
	}
	if _, exists, err := provider.Find(ctx, room.StartKey); err != nil || exists {
		t.Fatalf("provider room survived end: exists %t; %v", exists, err)
	}
}

func v2VoiceActionsLiveKit(t *testing.T) *voice.LiveKit {
	t.Helper()
	binary, err := exec.LookPath("livekit-server")
	if err != nil {
		t.Fatalf("real LiveKit fixture requires livekit-server on PATH: %v", err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	const key = "voice-actions"
	const secret = "voice-actions-integration-secret-32-bytes"
	log, err := os.CreateTemp(t.TempDir(), "livekit-*.log")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log.Close() })
	cmd := exec.Command(binary, "--dev", "--bind", "127.0.0.1", "--node-ip", "127.0.0.1", "--config-body", fmt.Sprintf("port: %d\n", port), "--keys", key+": "+secret)
	cmd.Stdout, cmd.Stderr = log, log
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	})
	upstream, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(httputil.NewSingleHostReverseProxy(upstream))
	t.Cleanup(server.Close)
	provider := &voice.LiveKit{Endpoint: server.URL, APIKey: key, APISecret: secret, Client: server.Client()}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, err := provider.Find(context.Background(), uuid.NewString()); err == nil {
			return provider
		}
		time.Sleep(50 * time.Millisecond)
	}
	output, _ := os.ReadFile(log.Name())
	t.Fatalf("real LiveKit fixture did not become ready: %s", output)
	return nil
}
