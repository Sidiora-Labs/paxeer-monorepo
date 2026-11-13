package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/cache"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/proxy"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/serve"
	mediastore "github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/storage"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/storage/testdata/objects"
	control "github.com/Sidiora-Technologies/KindleLaunch/security/control"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
	"github.com/nats-io/nkeys"
)

func TestConnectedGatewayBindingUsesBoundIdentityAndMediaRoutes(t *testing.T) {
	dsn := os.Getenv("L41_POSTGRES_URL")
	if dsn == "" {
		t.Fatal("L41_POSTGRES_URL is required for the gateway authority fixture")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	pki := testpki.New(t)
	serverLeaf := pki.Leaf("gateway-server", "localhost")
	clientLeaf := pki.Leaf("gateway-client", "localhost")
	peerLeaf := pki.Leaf("media-peer", "localhost")
	assertion := testpki.Ed25519(t)
	peerAssertion := testpki.Ed25519(t)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNotFound) }))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{serverLeaf.Pair}}
	server.StartTLS()
	t.Cleanup(server.Close)
	address := server.Listener.Addr().(*net.TCPAddr)
	privateListener, err := net.Listen("tcp", net.JoinHostPort(address.IP.String(), "0"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = privateListener.Close() })
	privateAddress := privateListener.Addr().String()
	endpoint, err := url.Parse(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	endpoint.Host = net.JoinHostPort("localhost", endpoint.Port())
	fileSHA := func(path string) string {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return fmt.Sprintf("%x", sha256.Sum256(raw))
	}
	t.Setenv("GATEWAY_TEST_CLIENT_KEY", string(clientLeaf.KeyPEM))
	t.Setenv("GATEWAY_TEST_SERVER_KEY", string(serverLeaf.KeyPEM))
	t.Setenv("GATEWAY_TEST_ASSERTION_KEY", base64.RawURLEncoding.EncodeToString(assertion.Private))
	cfg := config.Config{SecretsProvider: "env", CORSAllowedOrigins: "https://kindle.test"}
	binding := gatewayRuntimeBinding{
		IdentityEndpoint:             endpoint.String(),
		IdentityServerPeerCertSHA256: serverLeaf.SHA256,
		IdentityListenAddress:        privateAddress,
		CAFile:                       pki.CAFile,
		CASHA256:                     fileSHA(pki.CAFile),
		ClientCertFile:               clientLeaf.CertFile,
		ClientCertSHA256:             fileSHA(clientLeaf.CertFile),
		ClientKeyReference:           "GATEWAY_TEST_CLIENT_KEY",
		ServerCertFile:               serverLeaf.CertFile,
		ServerCertSHA256:             fileSHA(serverLeaf.CertFile),
		ServerKeyReference:           "GATEWAY_TEST_SERVER_KEY",
		AssertionKeyReference:        "GATEWAY_TEST_ASSERTION_KEY",
		IdentityPeers:                []gatewayIdentityPeer{{Issuer: "metadata", PeerCertSHA256: peerLeaf.SHA256, PublicKey: peerAssertion.PublicRawURL}},
		ConsumerGeneration:           "gateway-fixture",
		ApprovedBucket:               "approved",
		MediaSchema:                  "gateway",
		MetadataSchema:               "metadata",
		SafetySchema:                 "chat",
		PurgeMode:                    "invalid",
	}
	for _, name := range []string{"user", "metadata", "social", "livestream"} {
		binding.Upstreams = append(binding.Upstreams, gatewayUpstreamBinding{Name: name, URL: endpoint.String()})
	}
	readers := map[string]serve.Reader{"approved": objects.Store{Root: t.TempDir()}}
	assertBinding := func(candidate gatewayRuntimeBinding, configuration config.Config, want string) {
		t.Helper()
		runtime, err := newGatewayConnectedRuntime(ctx, configuration, pool, pool, candidate, nil, nil, readers, nil)
		if runtime != nil || err == nil || !strings.Contains(err.Error(), want) {
			t.Fatalf("gateway binding: runtime=%v err=%v, want %q", runtime, err, want)
		}
	}
	assertBinding(binding, cfg, "gateway purge mode invalid")
	for _, tc := range []struct {
		name   string
		change func(*gatewayRuntimeBinding)
		want   string
	}{
		{"proxy count", func(b *gatewayRuntimeBinding) { b.TrustedProxies = make([]string, 17) }, "proxy trust bound exceeded"},
		{"proxy CIDR", func(b *gatewayRuntimeBinding) { b.TrustedProxies = []string{"invalid"} }, "explicit ingress proxy CIDR required"},
		{"identity authority", func(b *gatewayRuntimeBinding) { b.IdentityEndpoint = server.URL + "/nested" }, "private identity HTTPS authority required"},
		{"private listener", func(b *gatewayRuntimeBinding) { b.IdentityListenAddress = "invalid" }, "distinct explicit private identity listener required"},
		{"upstream set", func(b *gatewayRuntimeBinding) { b.Upstreams = b.Upstreams[:3] }, "gateway upstream and durable bindings incomplete"},
		{"CA binding", func(b *gatewayRuntimeBinding) { b.CASHA256 = strings.Repeat("0", 64) }, "gateway TLS file binding mismatch"},
		{"assertion key", func(b *gatewayRuntimeBinding) { b.AssertionKeyReference = "GATEWAY_TEST_CLIENT_KEY" }, "gateway assertion signing key invalid"},
		{"duplicate upstream", func(b *gatewayRuntimeBinding) {
			b.Upstreams[3] = gatewayUpstreamBinding{Name: "social", URL: endpoint.String(), Routes: proxy.SocialConnectedRoutes()}
		}, "gateway duplicate upstream binding"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := binding
			candidate.Upstreams = append([]gatewayUpstreamBinding(nil), binding.Upstreams...)
			tc.change(&candidate)
			assertBinding(candidate, cfg, tc.want)
		})
	}
	badProvider := cfg
	badProvider.SecretsProvider = "unsupported"
	assertBinding(binding, badProvider, "provider")

	mediaSchema := "gw_cov_" + strings.ReplaceAll(uuid.NewString(), "-", "")[:12]
	if _, err := pool.Exec(ctx, "CREATE SCHEMA "+pgx.Identifier{mediaSchema}.Sanitize()); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DROP SCHEMA "+pgx.Identifier{mediaSchema}.Sanitize()+" CASCADE"); err != nil {
			t.Error(err)
		}
	})
	media, err := mediastore.NewAuthority(pool, mediaSchema, mediaSchema+"_metadata", mediaSchema+"_safety")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, media.SchemaDDL()); err != nil {
		t.Fatal(err)
	}
	rdb, _ := internaltest.NewRedis(t)
	t.Cleanup(func() { _ = rdb.Close() })
	controlPool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(controlPool.Close)
	t.Setenv("GATEWAY_TEST_CONTROL_DSN", dsn)
	connectedBinding := binding
	connectedBinding.MediaSchema = media.Schema
	connectedBinding.MetadataSchema = media.MetadataSchema
	connectedBinding.SafetySchema = media.SafetySchema
	connectedBinding.PurgeMode = "origin_cache"
	connectedBinding.Control.Schema = "pnl"
	connectedBinding.Control.AuthoritySchema = "pnl"
	connectedBinding.Pools.ControlCredentialReference = "GATEWAY_TEST_CONTROL_DSN"
	for _, tc := range []struct {
		name   string
		change func(*gatewayRuntimeBinding)
		want   string
	}{
		{"client certificate binding", func(b *gatewayRuntimeBinding) { b.ClientCertSHA256 = strings.Repeat("0", 64) }, "gateway TLS file binding mismatch"},
		{"client key reference", func(b *gatewayRuntimeBinding) { b.ClientKeyReference = "GATEWAY_TEST_ABSENT_CLIENT_KEY" }, "gateway runtime secret unavailable"},
		{"client key mismatch", func(b *gatewayRuntimeBinding) { b.ClientKeyReference = "GATEWAY_TEST_SERVER_KEY" }, "gateway private client identity invalid"},
		{"server certificate binding", func(b *gatewayRuntimeBinding) { b.ServerCertSHA256 = strings.Repeat("0", 64) }, "gateway TLS file binding mismatch"},
		{"server key reference", func(b *gatewayRuntimeBinding) { b.ServerKeyReference = "GATEWAY_TEST_ABSENT_SERVER_KEY" }, "gateway runtime secret unavailable"},
		{"server key mismatch", func(b *gatewayRuntimeBinding) { b.ServerKeyReference = "GATEWAY_TEST_CLIENT_KEY" }, "gateway private server identity invalid"},
		{"assertion key reference", func(b *gatewayRuntimeBinding) { b.AssertionKeyReference = "GATEWAY_TEST_ABSENT_ASSERTION_KEY" }, "gateway runtime secret unavailable"},
		{"identity peer pin", func(b *gatewayRuntimeBinding) { b.IdentityPeers[0].PeerCertSHA256 = "invalid" }, "identity service peer binding invalid"},
		{"upstream transport", func(b *gatewayRuntimeBinding) { b.Upstreams[0].URL = "http://localhost:4443" }, "private media channel unavailable"},
		{"media authority schema", func(b *gatewayRuntimeBinding) { b.MediaSchema = "" }, mediastore.ErrUnavailable.Error()},
		{"control authority schema", func(b *gatewayRuntimeBinding) { b.Control.Schema = "other" }, "exact control authority schemas required"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := connectedBinding
			candidate.Upstreams = append([]gatewayUpstreamBinding(nil), connectedBinding.Upstreams...)
			candidate.IdentityPeers = append([]gatewayIdentityPeer(nil), connectedBinding.IdentityPeers...)
			tc.change(&candidate)
			runtime, err := newGatewayConnectedRuntime(ctx, cfg, pool, controlPool, candidate, rdb, cache.New(rdb, time.Minute), readers, nil)
			if runtime != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("gateway binding: runtime=%v err=%v, want %q", runtime, err, tc.want)
			}
		})
	}
	connected, err := newGatewayConnectedRuntime(ctx, cfg, pool, controlPool, connectedBinding, rdb, cache.New(rdb, time.Minute), readers, nil)
	if connected != nil || !errors.Is(err, control.Denied) {
		t.Fatalf("connected control readiness: runtime=%v err=%v", connected, err)
	}

	admin, identityPool, scopedControlPool := connectedControlRolePools(t)
	if _, err := admin.Exec(ctx, "GRANT USAGE ON SCHEMA "+pgx.Identifier{media.Schema}.Sanitize()+" TO identity_runtime"); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "GRANT SELECT ON ALL TABLES IN SCHEMA "+pgx.Identifier{media.Schema}.Sanitize()+" TO identity_runtime"); err != nil {
		t.Fatal(err)
	}
	connectedBinding.Pools.ControlCredentialReference = "GATEWAY_TEST_CONTROL_ROLE_DSN"
	connectedBinding.RecoverySchema = "users"
	connectedBinding.RecoveryEpoch = 1
	connectedBinding.RecoveryRegion = "fixture"
	assertScoped := func(candidate gatewayRuntimeBinding, configuration config.Config, want string) {
		t.Helper()
		runtime, err := newGatewayConnectedRuntime(ctx, configuration, identityPool, scopedControlPool, candidate, rdb, cache.New(rdb, time.Minute), readers, nil)
		if runtime != nil || err == nil || !strings.Contains(err.Error(), want) {
			if runtime != nil {
				runtime.close()
			}
			t.Fatalf("scoped gateway binding: runtime=%v err=%v, want %q", runtime, err, want)
		}
	}
	assertScoped(connectedBinding, cfg, "explicit scoped object workers required")
	for index, kind := range []string{"variant", "original", "temporary"} {
		var references [4]string
		for role := range references {
			reference := fmt.Sprintf("GATEWAY_TEST_ROLE_%d_%d", index, role)
			t.Setenv(reference, fmt.Sprintf("gateway-object-role-%d-%d", index, role))
			references[role] = reference
		}
		connectedBinding.ObjectWorkers = append(connectedBinding.ObjectWorkers, gatewayObjectWorkerBinding{
			Kind: kind, Bucket: "approved", Prefix: map[string]string{"variant": "approved/", "original": "quarantine/", "temporary": "temporary/"}[kind], InventoryLimit: 8,
			DeleteAccessKeyReference: references[0], DeleteSecretKeyReference: references[1],
			InventoryAccessKeyReference: references[2], InventorySecretKeyReference: references[3],
		})
	}
	cfg.S3Endpoint = endpoint.String()
	cfg.S3Region = "us-east-1"
	cfg.S3AccessKeyID = "gateway-main-object-role"
	assertScoped(connectedBinding, cfg, "gateway private TLS broker required")
	connectedBinding.NATSURL = "tls://127.0.0.1:1"
	assertScoped(connectedBinding, cfg, "gateway secret reference required")
	brokerURL := startGatewayBrokerFixture(t, pki, serverLeaf)
	connectedBinding.NATSURL = brokerURL
	connectedBinding.NATSCredentialsReference = "GATEWAY_TEST_BROKER_CREDENTIALS"
	brokerCredentials := gatewayBrokerCredentials(t)
	t.Setenv(connectedBinding.NATSCredentialsReference, brokerCredentials)
	connectedBinding.Stream = "GW_IDENTITY"
	connectedBinding.Durable = "GW_IDENTITY_CONSUMER"
	connectedBinding.Audit = gatewayAuditBinding{
		KeyReference: "GATEWAY_TEST_AUDIT_KEY", DeliverySchema: "gateway_audit_source",
		SinkDeliverySchema: "gateway_audit_delivery", SinkSchema: "gateway_audit_sink",
		Subject: "gateway.audit.v1", EventType: "gateway.audit.revision",
		Stream: "GW_AUDIT", Durable: "GW_AUDIT_CONSUMER",
	}
	t.Setenv("GATEWAY_TEST_AUDIT_KEY", strings.Repeat("a", 48))
	nc, err := nats.Connect(brokerURL, nats.RootCAs(pki.CAFile), nats.Timeout(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(nc.Close)
	js, err := jetstream.New(nc)
	if err != nil {
		t.Fatal(err)
	}
	for _, item := range []struct{ stream, durable, subject string }{
		{connectedBinding.Stream, connectedBinding.Durable, auth.IdentityRevisionSubject},
		{connectedBinding.Audit.Stream, connectedBinding.Audit.Durable, connectedBinding.Audit.Subject},
	} {
		if _, err := js.CreateStream(ctx, jetstream.StreamConfig{Name: item.stream, Subjects: []string{item.subject}, Storage: jetstream.FileStorage}); err != nil {
			t.Fatal(err)
		}
		config, err := (natsdriver.ConsumerConfig{Stream: item.stream, Durable: item.durable, Subject: item.subject, Batch: 32, MaxAckPending: 128, MaxWaiting: 4, MaxDeliver: 10, FetchWait: 500 * time.Millisecond, AckWait: 30 * time.Second}).BrokerConsumerConfig()
		if err != nil {
			t.Fatal(err)
		}
		if _, err := js.CreateConsumer(ctx, item.stream, config); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("GATEWAY_TEST_SHORT_AUDIT_KEY", "short")
	for _, tc := range []struct {
		name string
		edit func(*gatewayRuntimeBinding)
		want string
	}{
		{"identity stream", func(b *gatewayRuntimeBinding) { b.Stream = "" }, "valid connection and stream required"},
		{"identity durable", func(b *gatewayRuntimeBinding) { b.Durable = "" }, "invalid bounded durable configuration"},
		{"audit separation", func(b *gatewayRuntimeBinding) { b.Audit.Durable = b.Durable }, "distinct security audit authority required"},
		{"audit key reference", func(b *gatewayRuntimeBinding) { b.Audit.KeyReference = "" }, "gateway secret reference required"},
		{"audit key length", func(b *gatewayRuntimeBinding) { b.Audit.KeyReference = "GATEWAY_TEST_SHORT_AUDIT_KEY" }, "audit privacy key invalid"},
		{"audit source schema", func(b *gatewayRuntimeBinding) { b.Audit.DeliverySchema = "invalid schema" }, "invalid store configuration"},
		{"audit sink delivery schema", func(b *gatewayRuntimeBinding) { b.Audit.SinkDeliverySchema = "invalid schema" }, "invalid store configuration"},
		{"audit sink schema", func(b *gatewayRuntimeBinding) { b.Audit.SinkSchema = "" }, "audit sink authority required"},
		{"audit stream", func(b *gatewayRuntimeBinding) { b.Audit.Stream = "" }, "valid connection and stream required"},
		{"audit durable", func(b *gatewayRuntimeBinding) { b.Audit.Durable = "" }, "invalid bounded durable configuration"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := connectedBinding
			tc.edit(&candidate)
			runtime, err := newGatewayConnectedRuntime(ctx, cfg, identityPool, scopedControlPool, candidate, rdb, cache.New(rdb, time.Minute), readers, nil)
			if runtime != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				if runtime != nil {
					runtime.close()
				}
				t.Fatalf("connected runtime %q: runtime=%v err=%v, want %q", tc.name, runtime, err, tc.want)
			}
		})
	}
	runtime, err := newGatewayConnectedRuntime(ctx, cfg, identityPool, scopedControlPool, connectedBinding, rdb, cache.New(rdb, time.Minute), readers, nil)
	if err != nil || runtime == nil || runtime.nc == nil || !runtime.nc.IsConnected() || len(runtime.upstreams) != 4 {
		if runtime != nil {
			runtime.close()
		}
		t.Fatalf("bound connected runtime: runtime=%v err=%v", runtime, err)
	}
	if err := runtime.ready(ctx); err != nil {
		t.Fatalf("connected runtime readiness: %v", err)
	}
	t.Setenv(connectedBinding.NATSCredentialsReference, "changed broker credential")
	if err := runtime.ready(ctx); err == nil || !strings.Contains(err.Error(), "secret changed or unavailable") {
		t.Fatalf("changed broker binding remained ready: %v", err)
	}
	t.Setenv(connectedBinding.NATSCredentialsReference, brokerCredentials)
	runtime.start(nil)
	deadline := time.Now().Add(3 * time.Second)
	for runtime.failure.Load() == nil && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	if runtime.failure.Load() == nil || runtime.ready(ctx) == nil {
		t.Fatal("connected worker dependency loss did not change readiness")
	}
	application := &App{runtimeStop: runtime.stop, runtimeWG: &runtime.wg, broker: runtime.nc, consumer: runtime.consumer, auditConsumer: runtime.auditConsumer, redis: rdb}
	application.Close()
	if !runtime.nc.IsClosed() {
		t.Fatal("connected runtime retained its broker connection")
	}
}

func startGatewayBrokerFixture(t *testing.T, pki *testpki.PKI, leaf testpki.Leaf) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	directory := t.TempDir()
	configuration := fmt.Sprintf("host: 127.0.0.1\nport: %d\njetstream { store_dir: %q }\ntls { cert_file: %q; key_file: %q; ca_file: %q }\n", port, filepath.Join(directory, "store"), leaf.CertFile, leaf.KeyFile, pki.CAFile)
	path := filepath.Join(directory, "broker.conf")
	if err := os.WriteFile(path, []byte(configuration), 0600); err != nil {
		t.Fatal(err)
	}
	binary := os.Getenv("NATS_SERVER_BINARY")
	if binary == "" {
		binary, err = exec.LookPath("nats-server")
		if err != nil {
			t.Fatal(err)
		}
	}
	var output bytes.Buffer
	command := exec.Command(binary, "-c", path)
	command.Stdout, command.Stderr = &output, &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Kill()
		if err := command.Wait(); err != nil && !strings.Contains(err.Error(), "killed") {
			t.Error(err)
		}
	})
	address := "tls://localhost:" + strconv.Itoa(port)
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		nc, err := nats.Connect(address, nats.RootCAs(pki.CAFile), nats.Timeout(100*time.Millisecond), nats.NoReconnect())
		if err == nil {
			nc.Close()
			return address
		}
		time.Sleep(20 * time.Millisecond)
	}
	_ = command.Process.Kill()
	_ = command.Wait()
	t.Fatalf("TLS broker did not become ready: %s", output.String())
	return ""
}

func gatewayBrokerCredentials(t *testing.T) string {
	t.Helper()
	account, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	user, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	accountPublic, err := account.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	userPublic, err := user.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	encode := base64.RawURLEncoding.EncodeToString
	signing := encode([]byte(`{"typ":"JWT","alg":"ed25519-nkey"}`)) + "." + encode([]byte(`{"sub":"`+userPublic+`","iss":"`+accountPublic+`","nats":{"type":"user","version":2}}`))
	signature, err := account.Sign([]byte(signing))
	if err != nil {
		t.Fatal(err)
	}
	seed, err := user.Seed()
	if err != nil {
		t.Fatal(err)
	}
	jwt := signing + "." + encode(signature)
	return "-----BEGIN NATS USER JWT-----\n" + jwt + "\n------END NATS USER JWT------\n\n-----BEGIN USER NKEY SEED-----\n" + string(seed) + "\n------END USER NKEY SEED------\n"
}
