package auth

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"testing"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/google/uuid"
)

func TestIdentityOwnerAndStreamRoutesRejectInvalidRequests(t *testing.T) {
	f := ownerHTTPFixture(t)
	credentials, principal := f.loginHTTP(t)
	link := uuid.NewString()
	for _, tc := range []struct {
		name, method, path, token string
		body                      any
		want                      int
	}{
		{"wallet link challenge unauthenticated", http.MethodPost, "/v2/me/wallet-links/challenges", "", map[string]any{}, http.StatusUnauthorized},
		{"wallet link challenge malformed", http.MethodPost, "/v2/me/wallet-links/challenges", credentials.AccessToken, map[string]any{"unknown": true}, http.StatusUnprocessableEntity},
		{"wallet link verify unauthenticated", http.MethodPost, "/v2/me/wallet-links/" + link + ":verify", "", map[string]any{}, http.StatusUnauthorized},
		{"wallet link verify malformed", http.MethodPost, "/v2/me/wallet-links/" + link + ":verify", credentials.AccessToken, map[string]any{"unknown": true}, http.StatusUnprocessableEntity},
		{"wallet link removal unauthenticated", http.MethodDelete, "/v2/me/wallet-links/" + link, "", map[string]any{}, http.StatusUnauthorized},
		{"wallet link removal malformed", http.MethodDelete, "/v2/me/wallet-links/" + link, credentials.AccessToken, map[string]any{"unknown": true}, http.StatusUnprocessableEntity},
		{"wallet links unauthenticated", http.MethodGet, "/v2/me/wallet-links", "", nil, http.StatusUnauthorized},
		{"logout all unauthenticated", http.MethodPost, "/v2/auth/sessions:logout-all", "", map[string]any{}, http.StatusUnauthorized},
		{"logout all malformed", http.MethodPost, "/v2/auth/sessions:logout-all", credentials.AccessToken, map[string]any{"unknown": true}, http.StatusUnprocessableEntity},
		{"reauth challenge unauthenticated", http.MethodPost, "/v2/auth/reauth/challenges", "", map[string]any{}, http.StatusUnauthorized},
		{"reauth challenge malformed", http.MethodPost, "/v2/auth/reauth/challenges", credentials.AccessToken, map[string]any{"unknown": true}, http.StatusUnprocessableEntity},
		{"reauth challenge unknown action", http.MethodPost, "/v2/auth/reauth/challenges", credentials.AccessToken, map[string]any{"action": "other"}, http.StatusUnprocessableEntity},
		{"reauth verify unauthenticated", http.MethodPost, "/v2/auth/reauth:verify", "", map[string]any{}, http.StatusUnauthorized},
		{"reauth verify malformed", http.MethodPost, "/v2/auth/reauth:verify", credentials.AccessToken, map[string]any{"unknown": true}, http.StatusUnprocessableEntity},
		{"session list unauthenticated", http.MethodGet, "/v2/auth/sessions", "", nil, http.StatusUnauthorized},
		{"current session unauthenticated", http.MethodGet, "/v2/auth/session", "", nil, http.StatusUnauthorized},
		{"session revocation unauthenticated", http.MethodDelete, "/v2/auth/sessions/" + principal.SessionID(), "", nil, http.StatusUnauthorized},
		{"stream ticket unauthenticated", http.MethodPost, "/v2/stream/tickets", "", map[string]any{}, http.StatusUnauthorized},
		{"stream ticket malformed", http.MethodPost, "/v2/stream/tickets", credentials.AccessToken, map[string]any{"unknown": true}, http.StatusUnprocessableEntity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, body, _ := f.request(t, tc.method, tc.path, tc.token, "", tc.body)
			if status != tc.want {
				t.Fatalf("status=%d want=%d body=%s", status, tc.want, body)
			}
		})
	}
	topics := []string{"private.chat." + principal.Wallet()}
	binding := sharedauth.StreamTicketBinding{SchemaMajor: 2, Topics: topics, FilterDigest: sharedauth.StreamFilterDigest(topics), Origin: "https://kindle.test", Endpoint: "/v2/stream"}
	status, body, _ := f.request(t, http.MethodPost, "/v2/stream/tickets", credentials.AccessToken, "", binding)
	if status != http.StatusOK || len(body) == 0 {
		t.Fatalf("chat stream ticket route: status=%d body=%s", status, body)
	}
}

func TestIdentityTransportRejectsUnboundSessionProofs(t *testing.T) {
	f := ownerHTTPFixture(t)
	credentials, _ := f.loginHTTP(t)
	challenge := f.challenge(t, f.wallet)
	proof := proofIdentity(t, f.key, challenge)
	proof.Signature = proof.Signature[:len(proof.Signature)-1] + "0"
	status, raw, _ := f.request(t, http.MethodPost, "/v2/auth/sessions", "", "", LoginRequest{ChallengeID: challenge.ChallengeID, Proof: proof, DeviceLabel: "browser"})
	if status != http.StatusUnauthorized {
		t.Fatalf("changed wallet proof: status=%d body=%s", status, raw)
	}

	for _, tc := range []struct {
		name, path, origin, body string
		cookie                   *http.Cookie
		csrf                     string
		want                     int
	}{
		{"challenge size", "/v2/auth/challenges", "https://kindle.test", strings.Repeat("x", 16385), nil, "", http.StatusUnprocessableEntity},
		{"login origin", "/v2/auth/sessions", "https://other.test", `{}`, nil, "", http.StatusForbidden},
		{"refresh CSRF", "/v2/auth/sessions:refresh", "https://kindle.test", `{}`, &http.Cookie{Name: refreshCookieName, Value: credentials.RefreshSecret}, "invalid", http.StatusForbidden},
		{"unknown refresh", "/v2/auth/sessions:refresh", "https://kindle.test", `{}`, &http.Cookie{Name: refreshCookieName, Value: strings.Repeat("0", 64) + "." + strings.Repeat("0", 64)}, f.s.refreshCSRFSecret(strings.Repeat("0", 64) + "." + strings.Repeat("0", 64)), http.StatusUnauthorized},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, f.server.URL+tc.path, bytes.NewBufferString(tc.body))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("Origin", tc.origin)
			request.Header.Set("Content-Type", "application/json")
			if tc.cookie != nil {
				request.AddCookie(tc.cookie)
			}
			if tc.csrf != "" {
				request.Header.Set("X-CSRF-Token", tc.csrf)
			}
			response, err := f.server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			body, err := io.ReadAll(response.Body)
			if closeErr := response.Body.Close(); closeErr != nil {
				t.Fatal(closeErr)
			}
			if err != nil || response.StatusCode != tc.want || response.Header.Get("Cache-Control") != "no-store" {
				t.Fatalf("%s: status=%d headers=%v body=%s error=%v", tc.name, response.StatusCode, response.Header, body, err)
			}
		})
	}
}

func TestIdentityTransportRoutesEnforceBoundCredentials(t *testing.T) {
	f := ownerHTTPFixture(t)
	credentials, principal := f.loginHTTP(t)
	serviceToken := strings.Repeat("t", 32)
	for _, tc := range []struct {
		name, method, path, token string
		body                      any
		want                      int
	}{
		{"challenge origin", http.MethodPost, "/v2/auth/challenges", "", ChallengeRequest{Wallet: f.wallet, Origin: "https://other.test"}, http.StatusForbidden},
		{"challenge key", http.MethodPost, "/v2/auth/challenges", "", ChallengeRequest{Wallet: f.wallet, Origin: "https://kindle.test"}, http.StatusUnprocessableEntity},
		{"login origin", http.MethodPost, "/v2/auth/sessions", "", map[string]any{"unknown": true}, http.StatusUnprocessableEntity},
		{"login proof shape", http.MethodPost, "/v2/auth/sessions", "", LoginRequest{ChallengeID: uuid.NewString()}, http.StatusUnprocessableEntity},
		{"refresh body", http.MethodPost, "/v2/auth/sessions:refresh", "", map[string]any{"unknown": true}, http.StatusUnprocessableEntity},
		{"refresh cookie", http.MethodPost, "/v2/auth/sessions:refresh", "", RefreshRequest{}, http.StatusUnauthorized},
		{"ticket body", http.MethodPost, "/internal/v2/identity/ticket:consume", serviceToken, map[string]any{"unknown": true}, http.StatusUnprocessableEntity},
		{"ticket authority", http.MethodPost, "/internal/v2/identity/ticket:consume", serviceToken, map[string]any{"ticket": strings.Repeat("0", 64), "binding": sharedauth.StreamTicketBinding{}}, http.StatusUnauthorized},
		{"request body", http.MethodPost, "/internal/v2/identity/request:authenticate", serviceToken, map[string]any{"unknown": true}, http.StatusUnprocessableEntity},
		{"request credential", http.MethodPost, "/internal/v2/identity/request:authenticate", serviceToken, sharedauth.IdentityAuthenticationRequest{Authorization: "Bearer invalid", Origin: "https://kindle.test", Method: http.MethodGet}, http.StatusUnauthorized},
		{"session body", http.MethodPost, "/internal/v2/identity/session:check", serviceToken, map[string]any{"unknown": true}, http.StatusUnprocessableEntity},
		{"session authority", http.MethodPost, "/internal/v2/identity/session:check", serviceToken, sharedauth.StreamPrincipal{AccountID: principal.AccountID(), SessionID: principal.SessionID(), Wallet: principal.Wallet(), AuthEpoch: principal.AuthEpoch()}, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			status, raw, headers := f.request(t, tc.method, tc.path, tc.token, "", tc.body)
			if status != tc.want || (tc.want == http.StatusOK && (len(raw) != 0 || headers.Get("X-Request-Id") == "")) || (tc.want != http.StatusOK && headers.Get("Cache-Control") != "no-store") {
				t.Fatalf("%s: status=%d headers=%v body=%s, want=%d", tc.name, status, headers, raw, tc.want)
			}
		})
	}
	topics := []string{"private.chat." + principal.Wallet()}
	binding := sharedauth.StreamTicketBinding{SchemaMajor: 2, Topics: topics, FilterDigest: sharedauth.StreamFilterDigest(topics), Origin: "https://other.test", Endpoint: "/v2/stream"}
	status, raw, _ := f.request(t, http.MethodPost, "/v2/stream/tickets", credentials.AccessToken, "", binding)
	if status != http.StatusForbidden || len(raw) == 0 {
		t.Fatalf("ticket origin binding: status=%d body=%s", status, raw)
	}
}
