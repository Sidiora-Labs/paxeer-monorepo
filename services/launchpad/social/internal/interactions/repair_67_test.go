package interactions

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Compile the repository SDK in a disposable directory with its pinned validator
// dependencies. The producer proof never substitutes a decoder or a transport.
func repair67SDK(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs("../../../../../sdk/launchpad/packages")
	if err != nil {
		t.Fatal(err)
	}
	scratch := t.TempDir()
	sdk := filepath.Join(scratch, "sdk")
	for _, relative := range []string{
		"sdk/package.json", "sdk/src/transport.ts", "sdk/src/validate.ts",
		"sdk/src/schemas.gen.json", "sdk/src/operations.gen.ts",
		"sdk/scripts/compile-validators.mjs", "sdk-v2/src/schemas.gen.json",
	} {
		raw, err := os.ReadFile(filepath.Join(root, relative))
		if err != nil {
			t.Fatal(err)
		}
		target := filepath.Join(scratch, relative)
		if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(sdk, "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Dependencies map[string]string `json:"dependencies"`
	}
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	// sdk-v2 is a type-only transport import. Only the real validator's runtime
	// dependencies are needed after Node transforms TypeScript in this fixture.
	if err := os.WriteFile(filepath.Join(sdk, "package.json"), []byte(`{"type":"module","private":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, dependency := range []string{"ajv", "ajv-formats"} {
		if manifest.Dependencies[dependency] == "" {
			t.Fatal("SDK dependency missing", dependency)
		}
	}
	repair67Command(t, "npm", "install", "--prefix", sdk, "--ignore-scripts", "--no-audit", "--no-fund", "--package-lock=false", "--save=false", "ajv@"+manifest.Dependencies["ajv"], "ajv-formats@"+manifest.Dependencies["ajv-formats"])
	repair67Command(t, "node", filepath.Join(sdk, "scripts/compile-validators.mjs"))
	repair67Command(t, "node", "repair_67_sdk.mjs", sdk, "prepare")
	return sdk
}

func repair67Command(t *testing.T, binary string, args ...string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	output, err := exec.CommandContext(ctx, binary, args...).CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", binary, err, output)
	}
}

func repair67AssertSDK(t *testing.T, sdk string, handler http.Handler, method, path string, status int, code, message string) {
	t.Helper()
	upstream := httptest.NewServer(handler)
	defer upstream.Close()
	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	gateway := httptest.NewServer(httputil.NewSingleHostReverseProxy(upstreamURL))
	defer gateway.Close()
	repair67Command(t, "node", "repair_67_sdk.mjs", sdk, "check", gateway.URL, path, method, uuid.NewString(), strconv.Itoa(status), code, message, strconv.FormatBool(status == 503))
}

func TestRepair_67_FrozenErrorsDriveSDK(t *testing.T) {
	sdk := repair67SDK(t)
	t.Run("writer-mappings", func(t *testing.T) { testContractErrorMapping(t, sdk) })
	t.Run("request-identity", TestContractErrorRequestIdentity)
	// A real pool closed before use exercises dependency failures without any
	// PostgreSQL process, schema, credentials or connections.
	cfg, err := pgxpool.ParseConfig("host=" + t.TempDir() + " user=repair67 dbname=repair67 sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	cfg.MinConns, cfg.MinIdleConns = 0, 0
	db, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()
	policy := &PGPolicy{DB: db,
		Sessions:         pgx.Identifier{"repair67", "identity_sessions"},
		IdentityAccounts: pgx.Identifier{"repair67", "identity_accounts"},
		WalletLinks:      pgx.Identifier{"repair67", "identity_wallet_links"},
		SessionGuard:     pgx.Identifier{"repair67", "social_session_live"},
	}
	h := &HTTP{Store: Store{DB: db, Authority: policy}, Policy: policy}
	actor, target := "0x"+strings.Repeat("ab", 20), "0x"+strings.Repeat("cd", 20)
	for _, tc := range []struct {
		name, method, path, code, message string
		status                            int
		unavailable, expired              bool
	}{
		{"invalid-wallet-followers", "GET", "/v2/profiles/invalid/followers", "FIELD_INVALID", "invalid follow request", 422, false, false},
		{"invalid-wallet-stats", "GET", "/v2/profiles/invalid/follow-stats", "FIELD_INVALID", "invalid follow request", 422, false, false},
		{"invalid-limit", "GET", "/v2/profiles/" + target + "/followers?limit=invalid", "FIELD_INVALID", "invalid follow request", 422, false, false},
		{"invalid-cursor", "GET", "/v2/profiles/" + target + "/followers?cursor=" + strings.Repeat("a", 257), "FIELD_INVALID", "invalid follow request", 422, false, false},
		{"denied-identity", "PUT", "/v2/me/following/" + target, "SCOPE_DENIED", "not permitted", 403, false, false},
		{"denied-identity-unfollow", "DELETE", "/v2/me/following/" + target, "SCOPE_DENIED", "not permitted", 403, false, false},
		{"denied-policy", "PUT", "/v2/me/following/" + target, "SCOPE_DENIED", "not permitted", 403, false, true},
		{"denied-policy-unfollow", "DELETE", "/v2/me/following/" + target, "SCOPE_DENIED", "not permitted", 403, false, true},
		{"denied-viewer-stats", "GET", "/v2/profiles/" + target + "/follow-stats", "SCOPE_DENIED", "not permitted", 403, false, true},
		{"dependency-follow", "PUT", "/v2/me/following/" + target, "DEPENDENCY_UNAVAILABLE", "store unavailable", 503, true, false},
		{"dependency-unfollow", "DELETE", "/v2/me/following/" + target, "DEPENDENCY_UNAVAILABLE", "store unavailable", 503, true, false},
		{"dependency-followers", "GET", "/v2/profiles/" + target + "/followers", "DEPENDENCY_UNAVAILABLE", "store unavailable", 503, true, false},
		{"dependency-stats", "GET", "/v2/profiles/" + target + "/follow-stats", "DEPENDENCY_UNAVAILABLE", "store unavailable", 503, true, false},
		{"closed-store-followers", "GET", "/v2/profiles/" + target + "/followers", "DEPENDENCY_UNAVAILABLE", "store unavailable", 503, false, false},
		{"closed-store-stats", "GET", "/v2/profiles/" + target + "/follow-stats", "DEPENDENCY_UNAVAILABLE", "store unavailable", 503, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := chi.NewRouter()
			if tc.expired {
				router.Use(func(next http.Handler) http.Handler {
					return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
						principal := fanout.Principal{Wallet: actor, SessionID: uuid.NewString(), ExpiresAt: time.Now().Add(-time.Hour)}
						next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), principal)))
					})
				})
			}
			handler := h
			if tc.unavailable {
				handler = nil
			}
			handler.RegisterContractRoutes(router)
			repair67AssertSDK(t, sdk, router, tc.method, tc.path, tc.status, tc.code, tc.message)
		})
	}
	for _, tc := range []struct{ name, method, path string }{
		{"session-dependency-follow", "PUT", "/v2/me/following/" + target},
		{"session-dependency-unfollow", "DELETE", "/v2/me/following/" + target},
		{"session-dependency-stats", "GET", "/v2/profiles/" + target + "/follow-stats"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			router := chi.NewRouter()
			router.Use(func(next http.Handler) http.Handler {
				return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					session := uuid.NewString()
					principal := fanout.Principal{Wallet: actor, SessionID: session, ExpiresAt: time.Now().Add(time.Hour)}
					principal.Validate = func(ctx context.Context) error { return policy.ValidateSession(ctx, actor, session) }
					next.ServeHTTP(w, r.WithContext(WithIdentity(r.Context(), principal)))
				})
			})
			h.RegisterContractRoutes(router)
			repair67AssertSDK(t, sdk, router, tc.method, tc.path, 503, "DEPENDENCY_UNAVAILABLE", "store unavailable")
		})
	}
	// Exercise real follow validation before the transaction boundary. The
	// frozen writer, HTTP proxy and SDK consume the store's actual error.
	for _, tc := range []struct{ name, target, method string }{
		{"invalid-wallet-follow", "invalid", "PUT"},
		{"invalid-wallet-unfollow", "invalid", "DELETE"},
		{"self-follow", actor, "PUT"},
		{"self-unfollow", actor, "DELETE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				err := h.Store.Follow(r.Context(), Request{Actor: actor, ID: tc.target, Key: contractKey(r), Remove: r.Method == http.MethodDelete})
				if err != ErrInvalid {
					t.Errorf("real store validation = %v; want ErrInvalid", err)
				}
				contractHeaders(w)
				contractError(w, r, err)
			})
			repair67AssertSDK(t, sdk, handler, tc.method, "/v2/me/following/"+tc.target, 422, "FIELD_INVALID", "invalid follow request")
		})
	}
	if db.Stat().TotalConns() != 0 || db.Stat().AcquireCount() != 0 {
		t.Fatal("refused operations acquired a connection and could persist state", db.Stat())
	}
}
