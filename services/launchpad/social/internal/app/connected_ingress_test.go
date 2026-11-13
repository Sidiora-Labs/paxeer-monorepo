package app

import (
	"crypto/tls"
	"crypto/x509"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
)

func TestConnectedIngressRequiresGatewayIdentityAndResolvesActor(t *testing.T) {
	h := newPlatformHarness(t)
	actor := addr(0xa11ce)
	h.wire.session("Bearer connected-actor", actor)
	peer, err := x509.ParseCertificate(h.pki.gatewayClient.Pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	runtime := &socialConnectedRuntime{
		binding:      config.SocialRuntimeBinding{GatewayDNS: "gateway.railway.internal", GatewayCertSHA256: h.pki.gatewayClient.SHA256},
		identity:     h.app.platform.authority,
		chatIdentity: h.app.platform.authority,
		safety:       &safety.Store{DB: h.app.pool},
	}
	runtime.request = runtime.authenticateRequest
	next := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path == "/v2/private" {
			principal, ok := fanout.PrincipalFromContext(req.Context())
			if !ok || principal.Wallet != actor || req.Header.Get("X-Actor-Wallet") != actor {
				t.Errorf("resolved actor = %+v, present %t", principal, ok)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})
	call := func(path, token string, trusted bool) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("X-Actor-Wallet", addr(0xb0b))
		if token != "" {
			req.Header.Set("Authorization", token)
		}
		if trusted {
			req.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{peer}, VerifiedChains: [][]*x509.Certificate{{peer}}}
		}
		rw := httptest.NewRecorder()
		runtime.ingress(next).ServeHTTP(rw, req)
		return rw
	}
	if got := call("/health/live", "", false); got.Code != http.StatusNoContent {
		t.Fatalf("health ingress = %d", got.Code)
	}
	if got := call("/v2/private", "Bearer connected-actor", false); got.Code != http.StatusForbidden {
		t.Fatalf("untrusted ingress = %d", got.Code)
	}
	if got := call("/v2/private", "", true); got.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous private ingress = %d", got.Code)
	}
	if got := call("/v2/private", "Bearer connected-actor", true); got.Code != http.StatusNoContent {
		t.Fatalf("authenticated ingress = %d %s", got.Code, got.Body.String())
	}
	if got := call("/v2/bounties", "", true); got.Code != http.StatusNoContent {
		t.Fatalf("public contract ingress = %d", got.Code)
	}
	if got := call("/v2/stream", "", true); got.Code != http.StatusNoContent {
		t.Fatalf("stream ingress = %d", got.Code)
	}
	check := runtime.safetyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		principal, err := safety.Actor(req.Context(), time.Now())
		if err != nil || principal.Wallet != actor {
			t.Errorf("moderation principal = %+v: %v", principal, err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	request := httptest.NewRequest(http.MethodGet, "/v2/safety/cases", nil)
	request.Header.Set("Authorization", "Bearer connected-actor")
	rw := httptest.NewRecorder()
	check.ServeHTTP(rw, request)
	if rw.Code != http.StatusNoContent {
		t.Fatalf("moderation ingress = %d %s", rw.Code, rw.Body.String())
	}
	rw = httptest.NewRecorder()
	check.ServeHTTP(rw, httptest.NewRequest(http.MethodGet, "/v2/safety/cases", nil))
	if rw.Code != http.StatusForbidden {
		t.Fatalf("anonymous moderation = %d", rw.Code)
	}
}
