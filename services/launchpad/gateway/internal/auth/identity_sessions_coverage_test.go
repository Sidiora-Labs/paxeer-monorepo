package auth

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/google/uuid"
)

func TestIdentitySessionPagesAndRevocation(t *testing.T) {
	f := ownerHTTPFixture(t)
	first, _ := f.loginHTTP(t)
	second, _ := f.loginHTTP(t)
	current, _ := f.loginHTTP(t)
	want := map[string]bool{
		first.Session.SessionID:   true,
		second.Session.SessionID:  true,
		current.Session.SessionID: true,
	}
	seen := map[string]bool{}
	cursor := ""
	for page := 0; page < len(want); page++ {
		path := "/v2/auth/sessions?limit=1"
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor)
		}
		status, raw, headers := f.request(t, http.MethodGet, path, current.AccessToken, "", nil)
		if status != http.StatusOK || headers.Get("Cache-Control") != "no-store" {
			t.Fatalf("session page %d: status=%d headers=%v body=%s", page, status, headers, raw)
		}
		var response Envelope[struct {
			Items []Session `json:"items"`
		}]
		if err := json.Unmarshal(raw, &response); err != nil {
			t.Fatal(err)
		}
		if len(response.Data.Items) != 1 {
			t.Fatalf("session page %d contains %d sessions", page, len(response.Data.Items))
		}
		id := response.Data.Items[0].SessionID
		if !want[id] || seen[id] {
			t.Fatalf("session page %d returned unrelated or repeated session %q", page, id)
		}
		seen[id] = true
		if page < len(want)-1 {
			if response.NextCursor == nil || *response.NextCursor == "" {
				t.Fatalf("session page %d has no continuation", page)
			}
			cursor = *response.NextCursor
		} else if response.NextCursor != nil {
			t.Fatalf("final session page has continuation %q", *response.NextCursor)
		}
	}
	for _, tc := range []struct {
		path   string
		status int
	}{
		{"/v2/auth/sessions?limit=01", http.StatusUnprocessableEntity},
		{"/v2/auth/sessions?limit=101", http.StatusUnprocessableEntity},
		{"/v2/auth/sessions?limit=1&limit=2", http.StatusUnprocessableEntity},
		{"/v2/auth/sessions?unknown=1", http.StatusUnprocessableEntity},
		{"/v2/auth/sessions?limit=1&cursor=invalid", http.StatusBadRequest},
		{"/v2/auth/sessions?cursor=first&cursor=second", http.StatusBadRequest},
		{"/v2/auth/sessions?cursor=" + strings.Repeat("a", 2049), http.StatusBadRequest},
		{"/v2/auth/sessions?cursor=a.b", http.StatusBadRequest},
		{"/v2/auth/sessions?cursor=YQ.invalid", http.StatusBadRequest},
		{"/v2/auth/sessions?cursor=YQ.00", http.StatusBadRequest},
		{"/v2/auth/sessions?limit=2&cursor=" + url.QueryEscape(cursor), http.StatusBadRequest},
	} {
		status, raw, _ := f.request(t, http.MethodGet, tc.path, current.AccessToken, "", nil)
		if status != tc.status {
			t.Fatalf("invalid session page %q: status=%d, want=%d body=%s", tc.path, status, tc.status, raw)
		}
	}
	if err := f.s.LogoutSession(f.ctx, currentPrincipal(t, f, current.AccessToken), "invalid"); err == nil {
		t.Fatal("invalid session identifier accepted")
	}
	status, raw, _ := f.request(t, http.MethodDelete, "/v2/auth/sessions/"+uuid.NewString(), current.AccessToken, "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("unowned session revocation: status=%d body=%s", status, raw)
	}
	for attempt := 0; attempt < 2; attempt++ {
		status, raw, _ = f.request(t, http.MethodDelete, "/v2/auth/sessions/"+first.Session.SessionID, current.AccessToken, "", nil)
		if status != http.StatusNoContent {
			t.Fatalf("session revocation %d: status=%d body=%s", attempt, status, raw)
		}
	}
	status, raw, _ = f.request(t, http.MethodGet, "/v2/auth/session", first.AccessToken, "", nil)
	if status != http.StatusUnauthorized {
		t.Fatalf("revoked session remained authenticated: status=%d body=%s", status, raw)
	}
}

func currentPrincipal(t *testing.T, f *identityHTTPFixture, token string) Principal {
	t.Helper()
	p, err := f.s.AuthenticateJWT(f.ctx, f.s.tokens, token)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestDevelopmentSessionsResolveIdentityCredentials(t *testing.T) {
	f := ownerHTTPFixture(t)
	credentials, _ := f.loginHTTP(t)
	sessions := DevSessions{Identity: f.s}
	type observation struct {
		status int
		actor  string
		query  string
	}
	for _, tc := range []struct {
		name     string
		required bool
		prepare  func(*http.Request)
		want     observation
	}{
		{"bearer", true, func(r *http.Request) { r.Header.Set("Authorization", "Bearer "+credentials.AccessToken) }, observation{200, f.wallet, ""}},
		{"query token", true, func(r *http.Request) { r.URL.RawQuery = "token=" + url.QueryEscape(credentials.AccessToken) }, observation{200, f.wallet, "token=" + url.QueryEscape(credentials.AccessToken)}},
		{"access cookie", true, func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: accessCookieName, Value: credentials.AccessToken})
		}, observation{200, f.wallet, ""}},
		{"invalid bearer", true, func(r *http.Request) { r.Header.Set("Authorization", "Bearer invalid") }, observation{401, "", ""}},
		{"missing required session", true, func(*http.Request) {}, observation{401, "", ""}},
		{"anonymous optional session", false, func(*http.Request) {}, observation{200, "", ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/social/ws", nil)
			tc.prepare(req)
			got := observation{}
			next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got.actor = Actor(r.Context())
				got.query = r.URL.RawQuery
				w.WriteHeader(http.StatusOK)
			})
			var handler http.Handler
			if tc.required {
				handler = sessions.Require(next)
			} else {
				handler = sessions.Optional(next)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)
			got.status = w.Code
			if got != tc.want {
				t.Fatalf("session result = %+v, want %+v; body=%s", got, tc.want, strings.TrimSpace(w.Body.String()))
			}
		})
	}
}

func TestIdentityInternalSessionAndStreamRoutes(t *testing.T) {
	f := ownerHTTPFixture(t)
	credentials, principal := f.loginHTTP(t)
	serviceToken := strings.Repeat("t", 32)
	topics := []string{"market.price." + f.wallet}
	binding := sharedauth.StreamTicketBinding{SchemaMajor: 2, Topics: topics, FilterDigest: sharedauth.StreamFilterDigest(topics), Origin: "https://kindle.test", Endpoint: "/v2/stream"}
	status, raw, _ := f.request(t, http.MethodPost, "/v2/stream/tickets", credentials.AccessToken, "", binding)
	if status != http.StatusOK {
		t.Fatalf("stream ticket issue: status=%d body=%s", status, raw)
	}
	var issued Envelope[struct {
		Ticket string `json:"ticket"`
	}]
	if err := json.Unmarshal(raw, &issued); err != nil || len(issued.Data.Ticket) != 64 {
		t.Fatalf("stream ticket: %+v err=%v", issued.Data, err)
	}
	consume := map[string]any{"ticket": issued.Data.Ticket, "binding": binding}
	status, raw, _ = f.request(t, http.MethodPost, "/internal/v2/identity/ticket:consume", "", "", consume)
	if status != http.StatusUnauthorized {
		t.Fatalf("private ticket without service authority: status=%d body=%s", status, raw)
	}
	status, raw, _ = f.request(t, http.MethodPost, "/internal/v2/identity/ticket:consume", serviceToken, "", consume)
	if status != http.StatusOK {
		t.Fatalf("private ticket consume: status=%d body=%s", status, raw)
	}
	var stream sharedauth.StreamPrincipal
	if err := json.Unmarshal(raw, &stream); err != nil || stream.Wallet != f.wallet || stream.SessionID != principal.SessionID() {
		t.Fatalf("consumed stream principal: %+v err=%v", stream, err)
	}
	status, raw, _ = f.request(t, http.MethodPost, "/internal/v2/identity/ticket:consume", serviceToken, "", consume)
	if status != http.StatusUnauthorized {
		t.Fatalf("replayed private ticket: status=%d body=%s", status, raw)
	}
	status, raw, _ = f.request(t, http.MethodPost, "/internal/v2/identity/session:check", serviceToken, "", stream)
	if status != http.StatusOK {
		t.Fatalf("live stream session: status=%d body=%s", status, raw)
	}
	authentication := sharedauth.IdentityAuthenticationRequest{Authorization: "Bearer " + credentials.AccessToken, Origin: "https://kindle.test", Method: http.MethodGet}
	status, raw, _ = f.request(t, http.MethodPost, "/internal/v2/identity/request:authenticate", serviceToken, "", authentication)
	if status != http.StatusOK {
		t.Fatalf("private request authentication: status=%d body=%s", status, raw)
	}
	var verified sharedauth.IdentityPrincipal
	if err := json.Unmarshal(raw, &verified); err != nil || verified.Wallet != f.wallet || verified.SessionID != principal.SessionID() {
		t.Fatalf("verified identity principal: %+v err=%v", verified, err)
	}
	status, raw, headers := f.request(t, http.MethodDelete, "/v2/auth/sessions/"+principal.SessionID(), credentials.AccessToken, "", nil)
	if status != http.StatusNoContent || headers.Get("Cache-Control") != "no-store" || len(headers.Values("Set-Cookie")) != 3 {
		t.Fatalf("self revocation: status=%d cookies=%v body=%s", status, headers.Values("Set-Cookie"), raw)
	}
	status, raw, _ = f.request(t, http.MethodPost, "/internal/v2/identity/session:check", serviceToken, "", stream)
	if status != http.StatusUnauthorized {
		t.Fatalf("revoked stream session: status=%d body=%s", status, raw)
	}
	status, raw, _ = f.request(t, http.MethodPost, "/internal/v2/identity/request:authenticate", serviceToken, "", authentication)
	if status != http.StatusUnauthorized {
		t.Fatalf("revoked identity request: status=%d body=%s", status, raw)
	}
}
