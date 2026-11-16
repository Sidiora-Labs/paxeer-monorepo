package app

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/social"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/voice"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

func TestReadinessWithoutVoice(t *testing.T) {
	broker := brokerRecoveryNewBroker(t)
	runtime := brokerRecoveryRuntime(t, broker)
	runtime.voice = &voice.Control{}
	if err := runtime.ready(context.Background()); err != nil {
		t.Fatalf("voice-off runtime readiness: %v", err)
	}
	failure := errors.New("durable work failed")
	runtime.durableFailure.Store(&failure)
	if err := runtime.ready(context.Background()); !errors.Is(err, failure) {
		t.Fatalf("durable readiness = %v, want %v", err, failure)
	}
}

func TestReadinessWithoutVoiceRoutes(t *testing.T) {
	router := chi.NewRouter()
	social.RegisterSafetyRoutes(router, &safety.Store{}, &voice.Control{})
	id := uuid.NewString()
	principal := safety.Principal{Wallet: "0x1111111111111111111111111111111111111111", SessionID: "voice-off", Expires: time.Now().Add(time.Hour)}
	create := `{"scopeType":"pool","scopeId":"0x2222222222222222222222222222222222222222","title":"Room","visibility":"public"}`
	moderation := `{"wallet":"0x2222222222222222222222222222222222222222","expectedRevision":1,"action":"mute","reason":"reviewed"}`
	v2Moderation := `{"participantAccountId":"` + uuid.NewString() + `","action":"mute","reason":"reviewed","expectedRevision":"1"}`
	clientSession := uuid.NewString()
	requests := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/v2/voice-rooms", create},
		{http.MethodGet, "/v2/voice-rooms/" + id, ""},
		{http.MethodPost, "/v2/voice-rooms/" + id + ":end", `{"expectedRevision":"1"}`},
		{http.MethodPost, "/v2/voice-rooms/" + id + ":leave", `{"clientSessionId":"` + clientSession + `"}`},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/tickets", `{"role":"listener","clientSessionId":"` + clientSession + `"}`},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/moderation-actions", v2Moderation},
		{http.MethodGet, "/v2/voice-rooms/" + id + "/prompt", ""},
		{http.MethodPost, "/v3/voice-rooms", `{"poolId":"pool"}`},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/start", `{"expectedRevision":1}`},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/end", `{"expectedRevision":1}`},
		{http.MethodPost, "/v3/voice-rooms/" + id + "/moderation-actions", moderation},
		{http.MethodPost, "/v3/voice-rooms/" + id + "/tickets", `{}`},
		{http.MethodGet, "/v2/voice-rooms/" + id + "/recording", ""},
		{http.MethodGet, "/v2/voice-rooms/" + id + "/recording/object", ""},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/recording", ""},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/recording/consent", `{"recordingId":"` + uuid.NewString() + `","expectedRevision":1,"agree":true}`},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/recording/start", `{"expectedRevision":1}`},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/recording/stop", `{"expectedRevision":1}`},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/speaker-requests", `{"expectedRevision":1}`},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/speakers", `{"wallet":"0x2222222222222222222222222222222222222222","expectedRevision":1}`},
		{http.MethodPost, "/v2/voice-rooms/" + id + "/leave", ""},
	}
	for _, tc := range requests {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, bytes.NewBufferString(tc.body))
			req = req.WithContext(safety.WithPrincipal(req.Context(), principal))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, req)
			if response.Code != http.StatusServiceUnavailable {
				t.Fatalf("voice-off route = %d %s", response.Code, response.Body.String())
			}
			if strings.Contains(response.Body.String(), "provider") {
				t.Fatalf("voice-off route exposed provider detail: %s", response.Body.String())
			}
		})
	}
}
