package app

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/config"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
)

func TestProductionGatewayChecksRuntimeDocumentsBeforeOpeningListeners(t *testing.T) {
	for _, name := range []string{"REDIS_URL", "GATEWAY_JWT_SECRET", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY", "DATABASE_URL", "IDENTITY_SERVICE_TOKEN", "IDENTITY_LIMITER_KEY", "IDENTITY_SIGNING_SECRET", "RPC_URL"} {
		t.Setenv(name, "owned-fixture-value")
	}
	configuration := config.Config{NodeEnv: "production", SecretsProvider: "env", SecretsRefreshSeconds: 1, RuntimeMetricsEnvironment: "qualification", RuntimeZone: "zone-a"}
	for _, tc := range []struct {
		name, metricsEnvironment, document, want string
	}{
		{"metric identity", "outside-inventory", "", "telemetry_identity_outside_inventory"},
		{"runtime document", "qualification", "", "binding"},
		{"runtime pool split", "qualification", "{}", "explicit_runtime_pool_split_required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := configuration
			candidate.RuntimeMetricsEnvironment = tc.metricsEnvironment
			if tc.document != "" {
				path := filepath.Join(t.TempDir(), "runtime.json")
				if err := os.WriteFile(path, []byte(tc.document), 0600); err != nil {
					t.Fatal(err)
				}
				candidate.GatewayRuntimeFile = path
				candidate.GatewayRuntimeSHA256 = fmt.Sprintf("%x", sha256.Sum256([]byte(tc.document)))
			}
			gateway, err := New(context.Background(), candidate, nil)
			if gateway != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				if gateway != nil {
					gateway.Close()
				}
				t.Fatalf("production binding: app=%v err=%v, want %q", gateway, err, tc.want)
			}
		})
	}
}

func TestGatewayExposesAnswerHeadersAcrossOrigins(t *testing.T) {
	t.Parallel()
	const origin = "https://kindle.test"
	const exposed = "X-Schema-Version, Retry-After, X-Request-Id, Etag, X-Csrf-Token, X-Recovery-Receipt"
	router := sharedhttp.NewRouter(sharedhttp.ServerOptions{
		CORSOrigins:              origin,
		AdditionalExposedHeaders: []string{"X-CSRF-Token", "X-Recovery-Receipt"},
	})
	router.Get("/unavailable", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Retry-After", "7")
		w.WriteHeader(http.StatusServiceUnavailable)
	})
	server := httptest.NewServer(publicGatewayHandler(router, config.Config{NodeEnv: "production", CORSAllowedOrigins: origin}))
	t.Cleanup(server.Close)
	for _, tc := range []struct {
		name, method, path, origin, exposed, retryAfter string
		status                                          int
	}{
		{"success", http.MethodGet, "/health", origin, exposed, "", http.StatusOK},
		{"unavailable", http.MethodGet, "/unavailable", origin, exposed, "7", http.StatusServiceUnavailable},
		{"success without origin", http.MethodGet, "/health", "", "", "", http.StatusOK},
		{"unavailable without origin", http.MethodGet, "/unavailable", "", "", "7", http.StatusServiceUnavailable},
		{"disallowed origin", http.MethodGet, "/health", "https://other.test", "", "", http.StatusForbidden},
		{"preflight", http.MethodOptions, "/unavailable", origin, "X-CSRF-Token, X-Request-ID, ETag, Retry-After, X-Recovery-Receipt, X-Schema-Version", "", http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			request, err := http.NewRequestWithContext(context.Background(), tc.method, server.URL+tc.path, http.NoBody)
			if err != nil {
				t.Fatal(err)
			}
			if tc.origin != "" {
				request.Header.Set("Origin", tc.origin)
			}
			if tc.method == http.MethodOptions {
				request.Header.Set("Access-Control-Request-Method", http.MethodGet)
			}
			response, err := server.Client().Do(request)
			if err != nil {
				t.Fatal(err)
			}
			defer response.Body.Close()
			if response.StatusCode != tc.status {
				t.Fatalf("status = %d, want %d", response.StatusCode, tc.status)
			}
			values := response.Header.Values("Access-Control-Expose-Headers")
			if tc.exposed == "" {
				if len(values) != 0 {
					t.Errorf("exposed headers = %v, want none", values)
				}
			} else if len(values) != 1 || values[0] != tc.exposed {
				t.Errorf("exposed headers = %v, want [%s]", values, tc.exposed)
			}
			if got := response.Header.Get("Retry-After"); got != tc.retryAfter {
				t.Errorf("Retry-After = %q, want %q", got, tc.retryAfter)
			}
		})
	}
}

func TestGatewayPrivatePeerAndOriginBindings(t *testing.T) {
	pki := testpki.New(t)
	own := pki.Leaf("gateway", "localhost")
	key := testpki.Ed25519(t)
	for _, tc := range []struct {
		name  string
		peers []gatewayIdentityPeer
		want  string
	}{
		{"missing peers", nil, "identity service peers required"},
		{"invalid peer key", []gatewayIdentityPeer{{Issuer: "media", PeerCertSHA256: strings.Repeat("a", 64), PublicKey: base64.RawURLEncoding.EncodeToString([]byte("short"))}}, "identity service peer key invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			verifiers, err := identityServiceVerifiers(tc.peers, own.SHA256, key.Private.Public().(ed25519.PublicKey))
			if verifiers != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("private peer binding: verifiers=%v err=%v", verifiers, err)
			}
		})
	}
	if _, err := gatewayBoundBytes(filepath.Join(t.TempDir(), "absent.pem"), strings.Repeat("0", 64)); err == nil || !strings.Contains(err.Error(), "binding unavailable") {
		t.Fatalf("unbound TLS file: %v", err)
	}
	handler := publicGatewayHandler(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }), config.Config{NodeEnv: "production", CORSAllowedOrigins: "https://kindle.test"})
	for _, tc := range []struct {
		origin string
		status int
	}{
		{"https://kindle.test", http.StatusNoContent},
		{"https://other.test", http.StatusForbidden},
	} {
		request := httptest.NewRequest(http.MethodGet, "/v2/me", nil)
		request.Header.Set("Origin", tc.origin)
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != tc.status || (tc.status == http.StatusForbidden && !strings.Contains(response.Body.String(), "SCOPE_DENIED")) || (tc.status == http.StatusNoContent && response.Header().Get("Access-Control-Expose-Headers") == "") {
			t.Fatalf("gateway origin %q: status=%d headers=%v body=%s", tc.origin, response.Code, response.Header(), response.Body.String())
		}
	}
}
