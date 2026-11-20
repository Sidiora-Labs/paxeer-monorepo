package app

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/cache"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/proxy"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/serve"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/storage/testdata/objects"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

func TestMetadataConnectedRoutesRuntimeDocument(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "..", "..", "deploy", "launchpad", "documents", "values", "gateway.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	var document struct{ Runtime gatewayRuntimeBinding }
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatal(err)
	}
	binding := document.Runtime
	pki := testpki.New(t)
	client := pki.Leaf("gateway-client", "localhost")
	server := pki.Leaf("gateway-server", "localhost")
	peer := pki.Leaf("metadata-client", "localhost")
	key, peerKey := testpki.Ed25519(t), testpki.Ed25519(t)
	digest := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("%x", sha256.Sum256(raw))
	}
	binding.CAFile, binding.CASHA256 = pki.CAFile, digest(pki.CAFile)
	binding.ClientCertFile, binding.ClientCertSHA256 = client.CertFile, digest(client.CertFile)
	binding.ServerCertFile, binding.ServerCertSHA256 = server.CertFile, digest(server.CertFile)
	binding.ClientKeyReference, binding.ServerKeyReference, binding.AssertionKeyReference = "GATEWAY_CLIENT_KEY", "GATEWAY_SERVER_KEY", "GATEWAY_ASSERTION_KEY"
	t.Setenv(binding.ClientKeyReference, string(client.KeyPEM))
	t.Setenv(binding.ServerKeyReference, string(server.KeyPEM))
	t.Setenv(binding.AssertionKeyReference, base64.RawURLEncoding.EncodeToString(key.Private))
	binding.IdentityServerPeerCertSHA256 = server.SHA256
	binding.IdentityPeers = []gatewayIdentityPeer{{Issuer: "metadata", PeerCertSHA256: peer.SHA256, PublicKey: peerKey.PublicRawURL}}
	binding.PurgeMode, binding.PurgeEndpoint, binding.PurgeTokenReference = "origin_cache", "", ""
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, "postgres://identity_runtime@localhost/gateway")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	rdb := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})
	t.Cleanup(func() { _ = rdb.Close() })
	cfg := config.Config{SecretsProvider: "env", CORSAllowedOrigins: "https://app.kindlelaunch.example", ObjectCacheMaxBytes: 1 << 20}
	objectCache := cache.New(rdb, time.Minute)
	readers := map[string]serve.Reader{binding.ApprovedBucket: objects.Store{Root: t.TempDir()}}
	metadataIndex := -1
	for i, upstream := range binding.Upstreams {
		if upstream.Name == "metadata" {
			metadataIndex = i
		}
	}
	if metadataIndex < 0 || len(binding.Upstreams[metadataIndex].Routes) == 0 {
		t.Fatal("example metadata routes missing")
	}
	example := append([]proxy.ConnectedRoute(nil), binding.Upstreams[metadataIndex].Routes...)
	publicTemplate := "/v2/tokens/{tokenAddress}/metadata"
	for _, tc := range []struct {
		name   string
		routes []proxy.ConnectedRoute
	}{
		{"code routes", nil},
		{"example document", example},
		{"document optional read", []proxy.ConnectedRoute{{Method: http.MethodGet, Template: publicTemplate, Optional: true}}},
		{"document session read", []proxy.ConnectedRoute{{Method: http.MethodGet, Template: publicTemplate, Authenticate: true}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := binding
			candidate.Upstreams = append([]gatewayUpstreamBinding(nil), binding.Upstreams...)
			candidate.Upstreams[metadataIndex].Routes = append([]proxy.ConnectedRoute(nil), tc.routes...)
			prepared, err := prepareGatewayConnectedRuntime(ctx, cfg, pool, pool, candidate, rdb, objectCache, readers, nil)
			if err != nil {
				t.Fatalf("metadata runtime: %v", err)
			}
			defer prepared.runtime.close()
			upstream := prepared.runtime.upstreams[metadataIndex]
			want := map[string]proxy.ConnectedRoute{}
			for _, route := range proxy.MetadataConnectedRoutes() {
				want[route.Method+" "+route.Template] = route
			}
			for _, route := range tc.routes {
				want[route.Method+" "+route.Template] = route
			}
			seen := map[string]bool{}
			router := chi.NewRouter()
			for _, route := range upstream.routes {
				key := route.Method + " " + route.Template
				if expected, ok := want[key]; !ok || expected != route || seen[key] {
					t.Fatalf("metadata route entry: %+v, want %+v once", route, expected)
				}
				seen[key] = true
				handler := http.Handler(upstream.handler)
				if route.Upload && route.Method == http.MethodPut {
					if upstream.upload == nil {
						t.Fatal("metadata asset upload handler missing")
					}
					handler = upstream.upload
				}
				router.Method(route.Method, route.Template, handler)
			}
			if len(seen) != len(want) {
				t.Fatalf("metadata route count=%d, want %d", len(seen), len(want))
			}
			for template, methods := range proxy.FrozenConnectedSurface()["metadata"] {
				for _, method := range strings.Fields(methods) {
					key := method + " " + template
					route, ok := want[key]
					if !ok || !seen[key] {
						t.Fatalf("frozen metadata route missing: %s", key)
					}
					path := strings.NewReplacer("{tokenAddress}", "0x"+strings.Repeat("1", 40), "{wallet}", "0x"+strings.Repeat("2", 40), "{assetId}", "8a5f44d2-8ee0-42c5-b365-cffdd2e91723").Replace(template)
					request := httptest.NewRequest(method, path+"?token=denied", nil)
					request.Header.Add("Accept", "application/json")
					request.Header.Add("Accept", "application/json")
					response := httptest.NewRecorder()
					router.ServeHTTP(response, request)
					status, code := http.StatusBadRequest, "ambiguous_header"
					if route.Authenticate {
						status, code = http.StatusForbidden, "identity_unavailable"
						if route.Upload {
							code = "SCOPE_DENIED"
						}
					}
					if response.Code != status || !strings.Contains(response.Body.String(), code) {
						t.Fatalf("metadata route %s: status=%d body=%s, want %d %s", key, response.Code, response.Body.String(), status, code)
					}
				}
			}
		})
	}
	for _, route := range []proxy.ConnectedRoute{
		{Method: http.MethodGet, Template: publicTemplate, Upload: true},
		{Method: http.MethodPatch, Template: publicTemplate},
		{Method: http.MethodGet, Template: "/v2/accounts/{wallet}/assets/{assetId}"},
		{Method: http.MethodPut, Template: "/v2/tokens/{tokenAddress}/assets/{assetId}", Authenticate: true},
	} {
		t.Run("refuse/"+route.Method+" "+route.Template, func(t *testing.T) {
			candidate := binding
			candidate.Upstreams = append([]gatewayUpstreamBinding(nil), binding.Upstreams...)
			candidate.Upstreams[metadataIndex].Routes = []proxy.ConnectedRoute{route}
			prepared, err := prepareGatewayConnectedRuntime(ctx, cfg, pool, pool, candidate, rdb, objectCache, readers, nil)
			if prepared != nil || err == nil || err.Error() != "unknown media route" {
				if prepared != nil {
					prepared.runtime.close()
				}
				t.Fatalf("metadata policy: runtime=%v err=%v", prepared, err)
			}
		})
	}
}
