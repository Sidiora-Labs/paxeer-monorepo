package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/google/uuid"
)

func TestIdentityMeasuredJourneyChecksEachTransportTransition(t *testing.T) {
	for _, tc := range []struct {
		name, failure string
		requests      int64
		acknowledged  int
	}{
		{"challenge signing bytes", "challenge_signing_bytes_invalid", 1, 0},
		{"login wire", "login_wire_invalid", 2, 0},
		{"acknowledgement capacity", "acknowledgement_capture_saturated", 2, 0},
		{"refresh cookie policy", "refresh_cookie_policy_invalid", 2, 1},
		{"missing refresh transport", "refresh_transport_missing", 2, 1},
		{"refresh family", "refresh_family_invalid", 3, 1},
		{"refresh secret rotation", "refresh_secret_rotation_invalid", 3, 1},
		{"logout response", "logout_unavailable", 4, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			issued := time.Now().UTC().Truncate(time.Second)
			var challenge Challenge
			session := Session{SessionID: uuid.NewString(), RefreshExpiresAt: issued.Add(time.Hour)}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Request-Id", r.Header.Get("X-Request-Id"))
				switch r.URL.Path {
				case "/v2/auth/challenges":
					var request ChallengeRequest
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.Origin != "https://kindle.test" {
						t.Errorf("challenge request: %+v err=%v", request, err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					challenge = Challenge{ChallengeID: uuid.NewString(), Wallet: request.Wallet, Nonce: strings.Repeat("a", 64), Domain: "kindle.test", URI: "https://kindle.test", ChainID: "125", Purpose: loginOperation, IssuedAt: issued, ExpiresAt: issued.Add(300 * time.Second)}
					message, err := BuildSIWE(challenge, "")
					if err != nil {
						t.Errorf("challenge signing bytes: %v", err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					challenge.Message = message
					if tc.name == "challenge signing bytes" {
						challenge.Message = "different signing bytes"
					}
					w.WriteHeader(http.StatusCreated)
					_ = json.NewEncoder(w).Encode(Envelope[struct {
						Challenge Challenge `json:"challenge"`
					}]{Data: struct {
						Challenge Challenge `json:"challenge"`
					}{challenge}})
				case "/v2/auth/sessions":
					var request LoginRequest
					if err := json.NewDecoder(r.Body).Decode(&request); err != nil || request.ChallengeID != challenge.ChallengeID || !sharedauth.VerifyWalletSignature(challenge.Wallet, challenge.Message, request.Proof.Signature) {
						t.Errorf("signed login request: %+v err=%v", request, err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					if tc.name != "login wire" {
						session.Wallet = challenge.Wallet
					}
					if tc.name != "missing refresh transport" {
						cookie := &http.Cookie{Name: refreshCookieName, Value: strings.Repeat("r", 129), Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode}
						if tc.name == "refresh cookie policy" {
							cookie.Secure = false
						}
						http.SetCookie(w, cookie)
						w.Header().Set("X-CSRF-Token", strings.Repeat("a", 64))
					}
					_ = json.NewEncoder(w).Encode(Envelope[LoginData]{Data: LoginData{Session: session, AccessToken: "access-1"}})
				case "/v2/auth/sessions:refresh":
					if cookie, err := r.Cookie(refreshCookieName); err != nil || len(cookie.Value) != 129 || r.Header.Get("X-CSRF-Token") != strings.Repeat("a", 64) {
						t.Errorf("refresh request: cookie=%v err=%v", cookie, err)
						w.WriteHeader(http.StatusBadRequest)
						return
					}
					if tc.name == "refresh family" {
						session.SessionID = uuid.NewString()
					}
					cookieValue := strings.Repeat("s", 129)
					csrf := strings.Repeat("b", 64)
					if tc.name == "refresh secret rotation" {
						cookieValue = strings.Repeat("r", 129)
						csrf = strings.Repeat("a", 64)
					}
					http.SetCookie(w, &http.Cookie{Name: refreshCookieName, Value: cookieValue, Path: "/", Secure: true, HttpOnly: true, SameSite: http.SameSiteLaxMode})
					w.Header().Set("X-CSRF-Token", csrf)
					_ = json.NewEncoder(w).Encode(Envelope[LoginData]{Data: LoginData{Session: session, AccessToken: "access-2"}})
				case "/v2/auth/sessions:logout-all":
					if tc.name == "logout response" {
						w.WriteHeader(http.StatusServiceUnavailable)
					} else {
						w.WriteHeader(http.StatusNoContent)
					}
				default:
					w.WriteHeader(http.StatusNotFound)
				}
			}))
			t.Cleanup(server.Close)
			recorder, path := requestMeasurementRecorder(t)
			recorder.refreshesPerLogin = 1
			recorder.maxAcknowledgements = 8
			if tc.name == "acknowledgement capacity" {
				recorder.maxAcknowledgements = 0
			}
			recorder.journey(context.Background(), IdentityMeasurementClient{BaseURL: server.URL, Origin: "https://kindle.test", Client: server.Client()}, IdentityMeasurementPhase{Name: "observed"}, time.Now(), time.Second)
			raw, err := os.ReadFile(path)
			if err != nil || recorder.summary.Requests != tc.requests || len(recorder.summary.Failures) != 1 || recorder.summary.Failures[0] != tc.failure || len(recorder.acknowledged) != tc.acknowledged || strings.Count(string(raw), "\n") != int(tc.requests) {
				t.Fatalf("measured journey: requests=%d failures=%v acknowledgements=%d capture=%q read=%v, want %s", recorder.summary.Requests, recorder.summary.Failures, len(recorder.acknowledged), raw, err, tc.failure)
			}
		})
	}
}
