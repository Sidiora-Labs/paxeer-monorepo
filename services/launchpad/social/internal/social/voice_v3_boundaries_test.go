package social

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/voice"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestVoiceControlRoutesRefuseUnboundProviderAndStore(t *testing.T) {
	store := &safety.Store{DB: internaltest.NewPostgres(t)}
	control := &voice.Control{}
	router := chi.NewRouter()
	RegisterSafetyRoutes(router, store, control)
	principal := safety.Principal{Wallet: "0x1111111111111111111111111111111111111111", SessionID: "voice-v3-boundary", Role: "room_moderator", Expires: time.Now().Add(time.Hour)}
	id := uuid.NewString()
	for _, tc := range []struct {
		method, path, body string
		status             int
	}{
		{http.MethodPost, "/v3/voice-rooms", `{"poolId":"pool-a"}`, 503},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/start", `{"expectedRevision":1}`, 503},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/end", `{"expectedRevision":1}`, 503},
		{http.MethodPost, "/v3/voice-rooms/" + id + "/moderation-actions", `{"wallet":"0x2222222222222222222222222222222222222222","expectedRevision":1,"action":"mute","reason":"reviewed"}`, 503},
		{http.MethodPost, "/v3/voice-rooms/" + id + "/moderation-actions", `{"wallet":"0x2222222222222222222222222222222222222222","expectedRevision":1,"action":"unknown","reason":"reviewed"}`, 400},
		{http.MethodPost, "/v3/voice-rooms/" + id + "/tickets", `{}`, 503},
		{http.MethodGet, "/v2/voice-rooms/" + id + "/recording", ``, 503},
		{http.MethodGet, "/v2/voice-rooms/" + id + "/recording/object", ``, 503},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/recording", ``, 503},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/recording/consent", `{"recordingId":"` + uuid.NewString() + `","expectedRevision":1,"agree":true}`, 503},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/recording/start", `{"expectedRevision":1}`, 503},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/recording/stop", `{"expectedRevision":1}`, 503},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/speaker-requests", `{"expectedRevision":1}`, 503},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/speakers", `{"wallet":"0x2222222222222222222222222222222222222222","expectedRevision":1}`, 503},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/leave", ``, 503},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			request := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			request = request.WithContext(safety.WithPrincipal(request.Context(), principal))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != tc.status {
				t.Fatalf("voice control route = %d %s", response.Code, response.Body.String())
			}
		})
	}
}
