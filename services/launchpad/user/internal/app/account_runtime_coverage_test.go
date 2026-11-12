package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	sharedredis "github.com/Sidiora-Technologies/KindleLaunch/shared/redis"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/go-chi/chi/v5"
	"github.com/nats-io/nkeys"
)

func TestConnectedAccountRuntimeWiresBrokerAndRoutes(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.NewPostgres(t)
	objects := internaltest.NewStore(t, "user-runtime-assets")
	redisURL := internaltest.NewRedisURL(t)
	redis, err := sharedredis.NewClient(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = redis.Close() })
	ddl, err := delivery.SchemaDDL("public")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	pki := testpki.New(t)
	brokerLeaf := pki.Leaf("broker", "localhost")
	clientLeaf := pki.Leaf("user", "localhost")
	ca, err := os.ReadFile(pki.CAFile)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	broker := exec.Command("nats-server", "-js", "-a", "127.0.0.1", "-p", strconv.Itoa(port), "-sd", filepath.Join(t.TempDir(), "jetstream"), "--tls", "--tlscert", brokerLeaf.CertFile, "--tlskey", brokerLeaf.KeyFile, "--tlscacert", pki.CAFile)
	broker.Stdout, broker.Stderr = io.Discard, io.Discard
	if err := broker.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = broker.Process.Kill()
		_ = broker.Wait()
	})
	ready := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		conn, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)), 100*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			ready = true
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !ready {
		t.Fatal("broker did not accept connections")
	}
	userKey, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	seed, err := userKey.Seed()
	if err != nil {
		t.Fatal(err)
	}
	jwt := base64.RawURLEncoding.EncodeToString([]byte(`{"typ":"JWT","alg":"ed25519-nkey"}`)) + "." + base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"user"}`)) + "." + base64.RawURLEncoding.EncodeToString([]byte("signature"))
	creds := "-----BEGIN NATS USER JWT-----\n" + jwt + "\n------END NATS USER JWT------\n\n-----BEGIN USER NKEY SEED-----\n" + string(seed) + "\n------END USER NKEY SEED------\n"
	subjects := map[string]string{}
	for _, kind := range []string{"profile.updated", "preferences.updated", "watchlist.changed", "consent.updated", "endpoint.bound", "endpoint.revoked", "account.export.requested", "account.deletion.requested", "account.deletion.job", "account.backfill.completed"} {
		subjects[kind] = "users." + kind
	}
	document, err := json.Marshal(subjects)
	if err != nil {
		t.Fatal(err)
	}
	subjectPath := filepath.Join(t.TempDir(), "subjects.json")
	if err := os.WriteFile(subjectPath, document, 0600); err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(document)
	key := base64.StdEncoding.EncodeToString(bytes.Repeat([]byte{1}, 32))
	cfg := config.Config{
		AccountCursorKey: key, AccountExportKey: key, AccountEndpointKey: key, AccountEndpointFingerprintKey: key,
		AccountAuditKey: key, AccountEndpointProofPublicKey: key, AccountGatewayPublicKey: key, AccountServiceSigningSeed: key,
		AccountGatewayCertSHA256: brokerLeaf.SHA256, NotificationCertSHA256: brokerLeaf.SHA256, IdentityCertSHA256: brokerLeaf.SHA256, MetadataCertSHA256: brokerLeaf.SHA256,
		AccountDownloadBaseURL: "https://localhost", NotificationServiceURL: "https://localhost", IdentityURL: "https://localhost", MetadataServiceURL: "https://localhost",
		S3Endpoint: "https://localhost", S3Region: "us-east-1", S3Bucket: "user-assets", AccountExportBucket: "user-private-assets", S3AccessKeyID: "test", S3SecretAccessKey: "test",
		NotificationServiceToken: strings.Repeat("a", 32), NotificationInboundToken: strings.Repeat("b", 32),
		AccountNATSStream: "USERS", AccountSubjectsFile: subjectPath, AccountSubjectsSHA256: hex.EncodeToString(digest[:]), AccountEnvironment: "test", AccountDeliverySchema: "public",
		AccountNATSURL: "tls://localhost:" + strconv.Itoa(port), RuntimeInstanceID: "user-test", AccountRPCURL: "http://127.0.0.1:9",
		AccountTLSCertPEM: string(clientLeaf.CertPEM), AccountTLSKeyPEM: string(clientLeaf.KeyPEM), AccountTLSCAPEM: string(ca),
		AccountNATSCreds: creds, AccountNATSCertPEM: string(clientLeaf.CertPEM), AccountNATSKeyPEM: string(clientLeaf.KeyPEM), AccountNATSCAPEM: string(ca),
		AccountAssertionIssuer: "gateway", AccountAssertionAudience: "user", AccountAssertionHeader: "X-Test-Assertion", AccountAssertionSignatureHeader: "X-Test-Signature",
	}
	mux := chi.NewRouter()
	health := &accountRuntimeHealth{}
	wiring, err := connectAccountRuntime(ctx, cfg, slog.New(slog.NewTextHandler(io.Discard, nil)), pool, redis, objects, mux, health)
	if err != nil {
		t.Fatalf("account runtime wiring = %v", err)
	}
	t.Cleanup(func() {
		wiring.stop()
		<-wiring.done
		wiring.natsConn.Close()
		wiring.rpcBackend.Close()
	})
	if wiring.tlsConfig == nil || wiring.natsConn == nil || !wiring.natsConn.IsConnected() || wiring.rpcBackend == nil || wiring.done == nil {
		t.Fatalf("connected runtime resources = %+v", wiring)
	}
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/v1/profiles/0x1111111111111111111111111111111111111111", nil))
	if response.Code != http.StatusNotFound {
		t.Fatalf("unknown public profile = %d %s", response.Code, response.Body.String())
	}
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/internal/v2/accounts/00000000-0000-0000-0000-000000000001/verified-account", nil))
	if response.Code != http.StatusForbidden {
		t.Fatalf("internal account without client certificate = %d %s", response.Code, response.Body.String())
	}
	boundSubjects := func(t *testing.T, next *config.Config, values map[string]string) {
		t.Helper()
		raw, err := json.Marshal(values)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(t.TempDir(), "subjects.json")
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		digest := sha256.Sum256(raw)
		next.AccountSubjectsFile = path
		next.AccountSubjectsSHA256 = hex.EncodeToString(digest[:])
	}
	for _, tc := range []struct {
		name   string
		change func(*testing.T, *config.Config)
	}{
		{"missing account transport", func(_ *testing.T, next *config.Config) { next.AccountNATSURL = "" }},
		{"invalid account certificate", func(_ *testing.T, next *config.Config) { next.AccountTLSCertPEM = "invalid" }},
		{"unsupported chain transport", func(_ *testing.T, next *config.Config) { next.AccountRPCURL = "ftp://localhost" }},
		{"invalid broker credentials", func(_ *testing.T, next *config.Config) { next.AccountNATSCreds = "invalid" }},
		{"unavailable broker", func(_ *testing.T, next *config.Config) { next.AccountNATSURL = "tls://localhost:1" }},
		{"unbound subject document", func(_ *testing.T, next *config.Config) { next.AccountSubjectsSHA256 = strings.Repeat("0", 64) }},
		{"duplicate subject", func(t *testing.T, next *config.Config) {
			values := map[string]string{}
			for k, v := range subjects {
				values[k] = v
			}
			values["profile.updated"] = values["preferences.updated"]
			boundSubjects(t, next, values)
		}},
		{"empty subject", func(t *testing.T, next *config.Config) {
			values := map[string]string{}
			for k, v := range subjects {
				values[k] = v
			}
			values["profile.updated"] = ""
			boundSubjects(t, next, values)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			broken := cfg
			tc.change(t, &broken)
			failed, err := connectAccountRuntime(ctx, broken, slog.New(slog.NewTextHandler(io.Discard, nil)), pool, redis, objects, chi.NewRouter(), &accountRuntimeHealth{})
			if err == nil || failed.natsConn != nil || failed.rpcBackend != nil || failed.done != nil {
				t.Fatalf("invalid account wiring = %+v, %v", failed, err)
			}
		})
	}
}
