package social

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/voice"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestModerationRoutesRejectInvalidBodiesAndUnverifiedActors(t *testing.T) {
	store := &safety.Store{DB: internaltest.NewPostgres(t)}
	router := chi.NewRouter()
	RegisterSafetyRoutes(router, store, nil)
	principal := safety.Principal{Wallet: "0x1111111111111111111111111111111111111111", SessionID: "validation-session", Role: "room_moderator", Expires: time.Now().Add(time.Hour)}
	for _, route := range []struct {
		method, path string
		status       int
	}{
		{http.MethodPost, "/v2/safety/reports", http.StatusBadRequest},
		{http.MethodPatch, "/v2/safety/cases/invalid", http.StatusBadRequest},
		{http.MethodPost, "/v2/safety/cases/invalid/bans", http.StatusBadRequest},
		{http.MethodPost, "/v2/safety/cases/invalid/restore", http.StatusBadRequest},
		{http.MethodPost, "/v2/safety/appeals", http.StatusBadRequest},
		{http.MethodPost, "/v2/safety/appeals/invalid/reverse-ban", http.StatusBadRequest},
		{http.MethodPost, "/v2/voice-rooms", http.StatusBadRequest},
		{http.MethodPost, "/v2/voice-rooms/invalid:end", http.StatusBadRequest},
		{http.MethodPost, "/v2/voice-rooms/invalid:leave", http.StatusBadRequest},
		{http.MethodPost, "/v2/voice-rooms/invalid/tickets", http.StatusBadRequest},
		{http.MethodPost, "/v2/voice-rooms/invalid/moderation-actions", http.StatusBadRequest},
	} {
		t.Run(route.path, func(t *testing.T) {
			request := httptest.NewRequest(route.method, route.path, bytes.NewBufferString(`{"unknown":true}`))
			request = request.WithContext(safety.WithPrincipal(request.Context(), principal))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != route.status {
				t.Fatalf("invalid request = %d %s", response.Code, response.Body.String())
			}
		})
	}
	for _, route := range []struct {
		method, path string
	}{
		{http.MethodGet, "/internal/v2/safety/decision?wallet=" + principal.Wallet + "&scope=pool&resource=pool"},
		{http.MethodGet, "/internal/v2/safety/visibility?targetType=message&targetId=unknown"},
	} {
		request := httptest.NewRequest(route.method, route.path, nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != safety.HTTPStatus(safety.ErrForbidden) {
			t.Fatalf("unverified %s = %d %s", route.path, response.Code, response.Body.String())
		}
	}
	request := httptest.NewRequest(http.MethodGet, "/internal/v2/safety/decision?wallet=other&scope=pool&resource=pool", nil)
	request = request.WithContext(safety.WithPrincipal(context.Background(), principal))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != safety.HTTPStatus(safety.ErrForbidden) {
		t.Fatalf("other wallet decision = %d %s", response.Code, response.Body.String())
	}
}

func TestSafetyRoutesPreserveCommittedInputAndVoiceAuthority(t *testing.T) {
	db := internaltest.NewPostgres(t)
	store := &safety.Store{DB: db}
	control := &voice.Control{DB: db, Policy: store}
	router := chi.NewRouter()
	RegisterSafetyRoutes(router, store, control)
	wallet := "0x1111111111111111111111111111111111111111"
	principal := safety.Principal{Wallet: wallet, SessionID: uuid.NewString(), Role: "room_moderator", Expires: time.Now().Add(time.Hour), StepUp: time.Now()}
	roomID := uuid.NewString()
	if _, err := db.Exec(context.Background(), `INSERT INTO voice_rooms(id,pool_id,host_wallet,start_key) VALUES($1,$2,$3,$4)`, roomID, wallet, wallet, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	request := func(method, path, body string) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, bytes.NewBufferString(body))
		req.Header.Set("X-Request-ID", uuid.NewString())
		req = req.WithContext(safety.WithPrincipal(req.Context(), principal))
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodGet, "/v2/safety/cases?status=unknown", "", http.StatusOK},
		{http.MethodPatch, "/v2/safety/cases/" + uuid.NewString(), `{"status":"dismissed","expected_revision":1,"reason":"reviewed","policy_version":"v2"}`, http.StatusNotFound},
		{http.MethodPost, "/v2/safety/cases/" + uuid.NewString() + "/bans", `{"wallet":"` + wallet + `","scope":"pool","resource":"` + wallet + `","expectedRevision":1,"reason":"reviewed","policyVersion":"v2"}`, http.StatusNotFound},
		{http.MethodPost, "/v2/safety/cases/" + uuid.NewString() + "/restore", `{"caseId":"` + uuid.NewString() + `","expectedRevision":1,"reason":"reviewed","policyVersion":"v2"}`, http.StatusBadRequest},
		{http.MethodPost, "/v2/safety/appeals", `{"actionId":"` + uuid.NewString() + `","reason":"reviewed"}`, http.StatusNotFound},
		{http.MethodGet, "/v2/safety/appeals/" + uuid.NewString(), "", http.StatusNotFound},
		{http.MethodPost, "/v2/safety/appeals/" + uuid.NewString() + "/reverse-ban", `{"appealId":"` + uuid.NewString() + `","banId":"` + uuid.NewString() + `","expectedRevision":1,"expectedBanRevision":1,"reason":"reviewed","policyVersion":"v2"}`, http.StatusBadRequest},
		{http.MethodGet, "/internal/v2/safety/decision?wallet=" + wallet + "&scope=invalid&resource=" + wallet, "", http.StatusServiceUnavailable},
		{http.MethodGet, "/internal/v2/safety/visibility?targetType=invalid&targetId=" + roomID, "", http.StatusNotFound},
		{http.MethodPost, "/v2/voice-rooms", `{"scopeType":"squad","scopeId":"` + wallet + `","title":"Review room","visibility":"public"}`, http.StatusServiceUnavailable},
		{http.MethodPost, "/v2/voice-rooms", `{"scopeType":"pool","scopeId":"` + wallet + `","title":"Review room","visibility":"public"}`, http.StatusServiceUnavailable},
		{http.MethodPost, "/v2/voice-rooms/" + roomID + `:end`, `{"expectedRevision":"1"}`, http.StatusServiceUnavailable},
		{http.MethodPost, "/v2/voice-rooms/" + roomID + `/moderation-actions`, `{"participantAccountId":"` + uuid.NewString() + `","action":"mute","reason":"reviewed","expectedRevision":"1"}`, http.StatusServiceUnavailable},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			response := request(tc.method, tc.path, tc.body)
			if response.Code != tc.status {
				t.Fatalf("route status = %d, want %d: %s", response.Code, tc.status, response.Body.String())
			}
		})
	}
}

func TestSafetyMiddlewareRequiresBoundIdentityAndModerationStore(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/internal/v2/safety/decision", nil)
	request.Header.Set("Authorization", "Bearer unbound-session")
	for _, tc := range []struct {
		name   string
		store  *safety.Store
		status int
	}{
		{"missing moderation store", nil, http.StatusServiceUnavailable},
		{"unverified identity", &safety.Store{DB: internaltest.NewPostgres(t)}, http.StatusForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := httptest.NewRecorder()
			SafetyMiddleware(sharedauth.HTTPIdentityAuthority{}, tc.store)(http.NotFoundHandler()).ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("moderation boundary = %d %s", response.Code, response.Body.String())
			}
		})
	}
}
