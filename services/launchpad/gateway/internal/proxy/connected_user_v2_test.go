package proxy

import (
	"bytes"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/google/uuid"
)

func TestUserConnectedRoutesForwardFrozenV2Owner(t *testing.T) {
	want := []ConnectedRoute{
		{Method: http.MethodGet, Template: "/v2/me/watchlist", Authenticate: true},
		{Method: http.MethodPut, Template: "/v2/me/watchlist/{poolAddress}", Authenticate: true},
		{Method: http.MethodDelete, Template: "/v2/me/watchlist/{poolAddress}", Authenticate: true},
		{Method: http.MethodPatch, Template: "/v2/profiles/{wallet}", Authenticate: true},
		{Method: http.MethodPut, Template: "/v2/me/devices/{deviceId}", Authenticate: true},
		{Method: http.MethodDelete, Template: "/v2/me/devices/{deviceId}", Authenticate: true},
		{Method: http.MethodPost, Template: "/v2/me/exports", Authenticate: true},
		{Method: http.MethodGet, Template: "/v2/me/exports/{jobId}", Authenticate: true},
		{Method: http.MethodPost, Template: "/v2/me/deletion-requests", Authenticate: true},
		{Method: http.MethodGet, Template: "/v2/me/deletion-requests/{jobId}", Authenticate: true},
	}
	routes := UserConnectedRoutes()
	for _, expected := range want {
		t.Run(expected.Method+" "+expected.Template, func(t *testing.T) {
			found := 0
			for _, route := range routes {
				if route.Method != expected.Method || route.Template != expected.Template {
					continue
				}
				found++
				if route != expected {
					t.Errorf("owner route = %#v, want %#v", route, expected)
				}
				if !allowedConnectedRoute("user", route) {
					t.Errorf("user upstream rejected owner route %#v", route)
				}
			}
			if found != 1 {
				t.Errorf("found %d owner routes, want exactly one", found)
			}
		})
	}
}

func TestUserConnectedRoutesKeepV2ProfileReadPublic(t *testing.T) {
	routes := UserConnectedRoutes()
	for _, expected := range []ConnectedRoute{
		{Method: http.MethodGet, Template: "/v2/profiles/{wallet}"},
		{Method: http.MethodPatch, Template: "/v2/profiles/{wallet}", Authenticate: true},
	} {
		t.Run(expected.Method, func(t *testing.T) {
			found := 0
			for _, route := range routes {
				if route.Method != expected.Method || route.Template != expected.Template {
					continue
				}
				found++
				if route != expected {
					t.Errorf("profile route = %#v, want %#v", route, expected)
				}
			}
			if found != 1 {
				t.Errorf("found %d profile routes, want exactly one", found)
			}
		})
	}
}

func TestUserConnectedOwnerRoutesSignAccountAudience(t *testing.T) {
	pki := testpki.New(t)
	gateway := pki.Leaf("gateway", "gateway.internal")
	account := pki.Leaf("account", "account.internal")
	key := testpki.Ed25519(t)
	verifier := sharedauth.ServiceAssertionVerifier{
		Issuer: "gateway", Audience: "account", PeerCertSHA256: gateway.SHA256,
		Key: key.Private.Public().(ed25519.PublicKey), Transport: actorAssertionTransport("user"),
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		principal, err := verifier.Authenticate(r, body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(principal); err != nil {
			t.Errorf("encode authenticated principal: %v", err)
		}
	}))
	server.TLS = &tls.Config{
		MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{account.Pair},
		ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.Roots(),
	}
	server.StartTLS()
	defer server.Close()
	transport := &http.Transport{TLSClientConfig: &tls.Config{
		MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), ServerName: "account.internal",
		Certificates: []tls.Certificate{gateway.Pair},
	}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	identity := sharedauth.IdentityPrincipal{
		AccountID: uuid.NewString(), SessionID: uuid.NewString(),
		Wallet:    "0x1111111111111111111111111111111111111111",
		AuthEpoch: 1, ProofAt: time.Now(), AccessExpiresAt: time.Now().Add(time.Hour),
	}
	paths := strings.NewReplacer(
		"{wallet}", identity.Wallet, "{poolAddress}", identity.Wallet,
		"{deviceId}", uuid.NewString(), "{jobId}", uuid.NewString(),
	)
	covered := map[string]bool{}
	for _, route := range UserConnectedRoutes() {
		if !route.Authenticate {
			continue
		}
		name := route.Method + " " + route.Template
		covered[name] = true
		t.Run(name, func(t *testing.T) {
			audience := connectedAssertionAudience("user", route.Template)
			if audience != "account" {
				t.Fatalf("assertion audience = %q, want account", audience)
			}
			body := []byte(`{}`)
			req, err := http.NewRequest(route.Method, server.URL+paths.Replace(route.Template), bytes.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			requestID := uuid.NewString()
			req.Header.Set("X-Request-ID", requestID)
			signer := sharedauth.ServiceAssertionIssuer{Issuer: "gateway", Audience: audience, Key: key.Private, Transport: actorAssertionTransport("user")}
			if err := signer.SignRequest(req, identity, []string{"owner"}, body); err != nil {
				t.Fatalf("sign owner request: %v", err)
			}
			resp, err := client.Do(req)
			if err != nil {
				t.Fatalf("send owner request over mTLS: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("account verifier returned %s, want 200 OK", resp.Status)
			}
			var principal sharedauth.Principal
			if err := json.NewDecoder(resp.Body).Decode(&principal); err != nil {
				t.Fatal(err)
			}
			if principal.Wallet != identity.Wallet || principal.SessionID != identity.SessionID || principal.TraceID != requestID || len(principal.Roles) != 1 || principal.Roles[0] != "owner" {
				t.Fatalf("authenticated principal = %#v, want signed owner identity", principal)
			}
		})
	}
	for _, name := range []string{
		"GET /v2/me/watchlist", "PUT /v2/me/devices/{deviceId}",
		"POST /v2/me/exports", "POST /v2/me/deletion-requests", "PATCH /v2/profiles/{wallet}",
	} {
		if !covered[name] {
			t.Errorf("missing authenticated owner route %s", name)
		}
	}
	for _, upstream := range []string{"metadata", "social", "livestream"} {
		t.Run(upstream, func(t *testing.T) {
			for _, route := range UserConnectedRoutes() {
				if audience := connectedAssertionAudience(upstream, route.Template); audience != upstream {
					t.Errorf("%s audience for %s = %q, want %q", upstream, route.Template, audience, upstream)
				}
			}
		})
	}
}
