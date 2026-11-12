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
