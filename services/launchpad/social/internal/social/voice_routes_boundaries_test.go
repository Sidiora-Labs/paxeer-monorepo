package social

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/voice"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestVoiceRoutesPreserveMissingRoomAndInputBoundaries(t *testing.T) {
	db := internaltest.NewPostgres(t)
	store := &safety.Store{DB: db}
	control := &voice.Control{DB: db, Policy: store}
	router := chi.NewRouter()
	RegisterSafetyRoutes(router, store, control)
	id := uuid.NewString()
	actor := safety.Principal{Wallet: "0x1111111111111111111111111111111111111111", SessionID: "voice-route-session", Expires: time.Now().Add(time.Hour)}
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodPost, "/v3/voice-rooms", `{"poolId":"unknown"}`, 503},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/start", `{"expectedRevision":1}`, 404},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/end", `{"expectedRevision":1}`, 404},
		{http.MethodPost, "/v3/voice-rooms/" + id + "/moderation-actions", `{"wallet":"0x2222222222222222222222222222222222222222","expectedRevision":1,"action":"mute","reason":"room policy"}`, 404},
		{http.MethodPost, "/v3/voice-rooms/" + id + "/moderation-actions", `{"wallet":"0x2222222222222222222222222222222222222222","expectedRevision":1,"action":"unknown","reason":"room policy"}`, 400},
		{http.MethodPost, "/v3/voice-rooms/" + id + "/tickets", `{}`, 503},
		{http.MethodGet, "/v2/voice-rooms/" + id + "/recording", ``, 404},
		{http.MethodGet, "/v2/voice-rooms/" + id + "/recording/object", ``, 503},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/recording", ``, 503},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/recording/consent", `{"recordingId":"` + uuid.NewString() + `","expectedRevision":1,"agree":true}`, 503},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/recording/start", `{"expectedRevision":1}`, 503},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/recording/stop", `{"expectedRevision":1}`, 404},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/speaker-requests", `{"expectedRevision":1}`, 404},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/speakers", `{"wallet":"0x2222222222222222222222222222222222222222","expectedRevision":1}`, 404},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/leave", ``, 404},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body))
			request = request.WithContext(safety.WithPrincipal(request.Context(), actor))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("voice route = %d %s", response.Code, response.Body.String())
			}
		})
	}
}
