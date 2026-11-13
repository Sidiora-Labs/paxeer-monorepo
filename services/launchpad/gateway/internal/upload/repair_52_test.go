package upload

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	gatewayauth "github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/proxy"
	mediastore "github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/storage"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	redis "github.com/redis/go-redis/v9"
)

// This verifier performs real EIP-191 signature recovery for the test's EOA.
// Identity storage, session issuance, authentication and HTTP are production code.
type repair52EOAVerifier struct{}

func (repair52EOAVerifier) VerifyIdentity(_ context.Context, message string, p gatewayauth.WalletProof) error {
	if p.Kind != "eoa_eip191" || !sharedauth.VerifyWalletSignature(p.Wallet, message, p.Signature) {
		return errors.New("invalid EOA proof")
	}
	return nil
}

type repair52Fixture struct {
	authority sharedauth.HTTPIdentityAuthority
	server    *httptest.Server
	token     string
	wallet    string
}

func newRepair52Fixture(t *testing.T) *repair52Fixture {
	t.Helper()
	ctx := context.Background()
	dsn := os.Getenv("L33_POSTGRES_DSN")
	if dsn == "" {
		dsn = "postgres://kindle_fixture:kindle_fixture_password@127.0.0.1:25432/kindle_fixture?sslmode=disable"
	}
	admin, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	schema := "repair52_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	if _, err = admin.Exec(ctx, `CREATE SCHEMA `+schema); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := admin.Exec(ctx, `DROP SCHEMA `+schema+` CASCADE`); err != nil {
			t.Error(err)
		}
	})
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["search_path"] = schema
	cfg.MaxConns = 4
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	raw, err := os.ReadFile("../devidentity/migrations/00001_identity.sql")
	if err != nil {
		t.Fatal(err)
	}
	ddl, _, ok := strings.Cut(string(raw), "-- +goose Down")
	if !ok {
		t.Fatal("identity migration down boundary missing")
	}
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	t.Setenv("REPAIR52_IDENTITY_KEY", strings.Repeat("s", 48))
	now := time.Now()
	tokens, err := gatewayauth.NewIdentityTokens("https://kindle.test", "repair52", "repair52", []gatewayauth.SigningKey{{ID: "repair52", SecretReference: "REPAIR52_IDENTITY_KEY", ActiveAt: now.Add(-time.Minute), RetireAt: now.Add(time.Hour)}}, gatewayauth.EnvironmentIdentitySecrets{}, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	identity, err := gatewayauth.NewIdentity(gatewayauth.IdentityConfig{Pool: pool, Domain: "kindle.test", URI: "https://kindle.test/auth", AllowedOrigins: []string{"https://kindle.test"}, Verifier: repair52EOAVerifier{}, LimiterKey: []byte(strings.Repeat("k", 32)), Tokens: tokens})
	if err != nil {
		t.Fatal(err)
	}
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	wallet := strings.ToLower(crypto.PubkeyToAddress(key.PublicKey).Hex())
	challenge, err := identity.IssueLogin(ctx, gatewayauth.ChallengeRequest{Wallet: wallet, Origin: "https://kindle.test"}, "127.0.0.1")
	if err != nil {
		t.Fatal(err)
	}
	signature, err := crypto.Sign(accounts.TextHash([]byte(challenge.Message)), key)
	if err != nil {
		t.Fatal(err)
	}
	signature[64] += 27
	credentials, err := identity.Login(ctx, gatewayauth.LoginRequest{ChallengeID: challenge.ChallengeID, Proof: gatewayauth.WalletProof{Kind: "eoa_eip191", Wallet: wallet, Signature: "0x" + hex.EncodeToString(signature)}, DeviceLabel: "upload repair"})
	if err != nil {
		t.Fatal(err)
	}
	router := chi.NewRouter()
	serviceToken := strings.Repeat("t", 32)
	if err = identity.RegisterV2Routes(router, gatewayauth.IdentityHTTPConfig{ServiceToken: serviceToken}); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(router)
	t.Cleanup(server.Close)
	return &repair52Fixture{sharedauth.HTTPIdentityAuthority{Endpoint: server.URL, ServiceToken: serviceToken, Client: server.Client()}, server, credentials.AccessToken, wallet}
}

func (f *repair52Fixture) handler(t *testing.T) *Connected {
	t.Helper()
	roots := x509.NewCertPool()
	roots.AddCert(f.server.Certificate())
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	upstream, err := proxy.NewConnected(proxy.ConnectedDeps{Name: "metadata", URL: f.server.URL, Identity: f.authority, SigningKey: key, TLS: &tls.Config{RootCAs: roots, Certificates: []tls.Certificate{f.server.TLS.Certificates[0]}}, Routes: []proxy.ConnectedRoute{
		{Method: http.MethodPut, Template: "/v2/tokens/{tokenAddress}/assets/{assetId}", Authenticate: true, Upload: true},
		{Method: http.MethodPut, Template: "/v2/accounts/{wallet}/assets/{assetId}", Authenticate: true, Upload: true},
	}})
	if err != nil {
		t.Fatal(err)
	}
	db := redis.NewClient(&redis.Options{Addr: "127.0.0.1:26379", Password: "kindle_fixture_password"})
	t.Cleanup(func() { _ = db.Close() })
	namespace := "repair52_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	admission, err := NewAdmission(db, namespace, 2)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		keys, err := db.Keys(context.Background(), namespace+":*").Result()
		if err == nil && len(keys) > 0 {
			if err = db.Del(context.Background(), keys...).Err(); err != nil {
				t.Error(err)
			}
		}
	})
	handler, err := NewConnected(ConnectedDeps{Upstream: upstream, Identity: f.authority, Admission: admission, IngressBytesPerSecond: 1 << 30})
	if err != nil {
		t.Fatal(err)
	}
	return handler
}

func repair52Request(t *testing.T, server *httptest.Server, path, token, origin, requestID string, body io.Reader) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodPut, server.URL+path, body)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	request.Header.Set("X-Request-ID", requestID)
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = response.Body.Close() })
	return response
}

func repair52AssertError(t *testing.T, response *http.Response, status int, code string, retryable bool, retryHeader string, retryMS int64) v2.Error {
	t.Helper()
	raw, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := v2.DecodeError(raw)
	if err != nil {
		t.Fatalf("SDK error decoder: %v; HTTP %d %s", err, response.StatusCode, raw)
	}
	if response.StatusCode != status || envelope.Code != code || envelope.Retryable != retryable || envelope.Message == "" {
		t.Fatalf("HTTP %d envelope %+v, want status=%d code=%s retryable=%v", response.StatusCode, envelope, status, code, retryable)
	}
	if response.Header.Get("Content-Type") != "application/json" || response.Header.Get("Cache-Control") != "no-store" || envelope.RequestId != response.Header.Get("X-Request-ID") {
		t.Fatalf("error headers %v, requestId=%s", response.Header, envelope.RequestId)
	}
	if response.Header.Get("Retry-After") != retryHeader {
		t.Fatalf("Retry-After=%q want %q", response.Header.Get("Retry-After"), retryHeader)
	}
	if retryMS == 0 {
		if envelope.RetryAfterMs != nil {
			t.Fatalf("unexpected retryAfterMs %d", *envelope.RetryAfterMs)
		}
	} else if envelope.RetryAfterMs == nil || int64(*envelope.RetryAfterMs) != retryMS {
		t.Fatalf("retryAfterMs=%v want %d", envelope.RetryAfterMs, retryMS)
	}
	return envelope
}

func TestRepair_52_UploadErrorSchema(t *testing.T) {
	f := newRepair52Fixture(t)
	for _, resource := range []string{"tokens", "accounts"} {
		t.Run(resource, func(t *testing.T) {
			path := "/v2/" + resource + "/" + f.wallet + "/assets/" + uuid.NewString()
			for _, name := range []string{"scope", "authority_unauthorized", "authority_scope", "oversize", "saturation", "admission_busy", "admission_unavailable", "streaming_oversize", "invalid_route"} {
				t.Run(name, func(t *testing.T) {
					h := f.handler(t)
					h.observer = mediastore.NewMediaObserver()
					server := httptest.NewServer(h)
					t.Cleanup(server.Close)
					token, origin := f.token, "https://kindle.test"
					status, code, retryable, retryHeader, retryMS := 403, "SCOPE_DENIED", false, "", int64(0)
					requestPath := path
					var body io.Reader
					switch name {
					case "scope":
						token = ""
					case "authority_unauthorized":
						token = "invalid"
						status, code = 401, "AUTH_EXPIRED"
					case "authority_scope":
						origin = "https://denied.test"
					case "oversize":
						body = bytes.NewReader(make([]byte, (6<<20)+1))
						status, code = 413, "FIELD_INVALID"
					case "saturation":
						repair52Saturate(t, h, server, f.token, path)
						status, code, retryable, retryHeader, retryMS = 503, "DEPENDENCY_UNAVAILABLE", true, "1", 1000
					case "admission_busy":
						for i := 0; i < 2; i++ {
							release, err := h.admission.Acquire(context.Background(), f.wallet)
							if err != nil {
								t.Fatal(err)
							}
							t.Cleanup(release)
						}
						status, code, retryable, retryHeader, retryMS = 429, "RATE_LIMITED", true, "60", 60000
					case "admission_unavailable":
						if err := h.admission.db.Close(); err != nil {
							t.Fatal(err)
						}
						status, code, retryable, retryHeader, retryMS = 503, "DEPENDENCY_UNAVAILABLE", true, "60", 60000
					case "streaming_oversize":
						// Hiding Len forces chunked ingress through the real proxy body reader.
						body = struct{ io.Reader }{bytes.NewReader(make([]byte, (6<<20)+1))}
						status, code = 413, "FIELD_INVALID"
					case "invalid_route":
						requestPath += "/extra"
						status, code = 404, "NOT_FOUND"
					}
					requestID := uuid.NewString()
					response := repair52Request(t, server, requestPath, token, origin, requestID, body)
					envelope := repair52AssertError(t, response, status, code, retryable, retryHeader, retryMS)
					if envelope.RequestId != requestID {
						t.Fatalf("requestId=%q want %q", envelope.RequestId, requestID)
					}
					if status == 413 && (len(envelope.FieldErrors) != 1 || envelope.FieldErrors[0].Rule != "max_bytes") {
						t.Fatalf("oversize field errors %+v", envelope.FieldErrors)
					}
					outcome := "unavailable"
					switch status {
					case 401, 403:
						outcome = "unauthenticated"
					case 413:
						outcome = "rejected"
					case 429:
						outcome = "rate_limited"
					}
					counts := h.observer.Snapshot()["uploadOutcomes"].(map[string]uint64)
					if counts[outcome] < 1 {
						t.Fatalf("upload outcome %q absent: %v", outcome, counts)
					}
				})
			}
		})
	}
}

func TestRepair_52_IdentityUnavailable(t *testing.T) {
	f := newRepair52Fixture(t)
	h := f.handler(t)
	// Stop the real authority; a transport failure must remain retryable.
	f.server.Close()
	server := httptest.NewServer(h)
	defer server.Close()
	for _, resource := range []string{"tokens", "accounts"} {
		response := repair52Request(t, server, "/v2/"+resource+"/"+f.wallet+"/assets/"+uuid.NewString(), f.token, "https://kindle.test", "malformed", nil)
		envelope := repair52AssertError(t, response, 503, "DEPENDENCY_UNAVAILABLE", true, "", 0)
		if _, err := uuid.Parse(envelope.RequestId); err != nil {
			t.Fatal(err)
		}
	}
}

func repair52Saturate(t *testing.T, h *Connected, server *httptest.Server, token, path string) {
	t.Helper()
	for i := 0; i < cap(h.slots); i++ {
		reader, writer := io.Pipe()
		ctx, cancel := context.WithCancel(context.Background())
		request, err := http.NewRequestWithContext(ctx, http.MethodPut, server.URL+path, reader)
		if err != nil {
			t.Fatal(err)
		}
		request.Header.Set("Authorization", "Bearer "+token)
		request.Header.Set("Origin", "https://kindle.test")
		done := make(chan error, 1)
		go func() {
			response, err := server.Client().Do(request)
			if response != nil {
				_ = response.Body.Close()
			}
			done <- err
		}()
		t.Cleanup(func() {
			cancel()
			_ = writer.Close()
			_ = reader.Close()
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				t.Error("held upload did not stop")
			}
		})
	}
	deadline := time.Now().Add(3 * time.Second)
	for len(h.slots) < cap(h.slots) {
		if time.Now().After(deadline) {
			t.Fatal("real uploads did not occupy the ingress slots")
		}
		time.Sleep(time.Millisecond)
	}
}
