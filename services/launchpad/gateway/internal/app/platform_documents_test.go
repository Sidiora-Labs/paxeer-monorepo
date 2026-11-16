package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/cache"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/gateway/internal/proxy"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	goredis "github.com/redis/go-redis/v9"
)

func TestPlatformDocumentsBinding(t *testing.T) {
	ctx := context.Background()
	root := filepath.Join("..", "..", "..", "..", "..")
	documents := filepath.Join(root, "deploy", "launchpad", "documents")
	values, out := t.TempDir(), t.TempDir()
	read := func(path string) []byte {
		t.Helper()
		raw, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	write := func(path string, raw []byte) {
		t.Helper()
		if err := os.WriteFile(path, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	marshal := func(value any) []byte {
		t.Helper()
		raw, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	digest := func(raw []byte) string { return fmt.Sprintf("%x", sha256.Sum256(raw)) }
	write(filepath.Join(values, "common.json"), read(filepath.Join(documents, "values", "common.example.json")))
	var source map[string]any
	if err := json.Unmarshal(read(filepath.Join(documents, "values", "gateway.example.json")), &source); err != nil {
		t.Fatal(err)
	}
	pki := testpki.New(t)
	client := pki.Leaf("gateway-client", "gateway.kindlelaunch.example")
	server := pki.Leaf("gateway-server", "identity.kindlelaunch.example")
	peer := pki.Leaf("metadata-client", "metadata.kindlelaunch.example")
	assertion, peerAssertion := testpki.Ed25519(t), testpki.Ed25519(t)
	runtimeValues := source["runtime"].(map[string]any)
	for _, file := range []struct{ key, path string }{
		{"ca", pki.CAFile}, {"clientCert", client.CertFile}, {"serverCert", server.CertFile},
	} {
		runtimeValues[file.key+"File"] = file.path
		runtimeValues[file.key+"Sha256"] = digest(read(file.path))
	}
	runtimeValues["identityServerPeerCertSha256"] = server.SHA256
	peerValues := runtimeValues["identityPeers"].([]any)[0].(map[string]any)
	peerValues["peerCertSha256"], peerValues["publicKey"] = peer.SHA256, peerAssertion.PublicRawURL
	for _, credential := range source["credentials"].(map[string]any) {
		var secret [32]byte
		if _, err := rand.Read(secret[:]); err != nil {
			t.Fatal(err)
		}
		t.Setenv(credential.(map[string]any)["reference"].(string), hex.EncodeToString(secret[:]))
	}
	t.Setenv("GATEWAY_CLIENT_KEY", string(client.KeyPEM))
	t.Setenv("GATEWAY_SERVER_KEY", string(server.KeyPEM))
	t.Setenv("GATEWAY_ASSERTION_KEY", base64.RawURLEncoding.EncodeToString(assertion.Private))
	write(filepath.Join(values, "gateway.json"), marshal(source))
	if err := os.Mkdir(filepath.Join(values, "expected-schema"), 0o700); err != nil {
		t.Fatal(err)
	}
	zero := strings.Repeat("0", 64)
	write(filepath.Join(values, "expected-schema", "gateway.json"), marshal(map[string]any{
		"service": "media/gateway", "runtime_role": "mediagatewayd", "registry_digest": zero,
		"interface_digest": zero, "reader_version": 1, "migrations": map[string]string{"1": zero},
		"schemas": map[string]string{"gateway": zero}, "epoch": 1, "region": "mainnet-beta-eu",
		"privileges": []map[string]any{{"schema": "gateway", "object": "items", "privileges": []string{"SELECT"}}},
	}))
	command := exec.Command("python3", filepath.Join(documents, "compose.py"), "--values", values, "--out", out, "--service", "gateway")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compose gateway: %v: %s", err, output)
	}
	var composed map[string]string
	if err := json.Unmarshal(read(filepath.Join(out, "gateway.json")), &composed); err != nil {
		t.Fatal(err)
	}
	bound := func(name string) string {
		t.Helper()
		raw, err := base64.StdEncoding.DecodeString(composed[name+"_DOCUMENT"])
		if err != nil {
			t.Fatal(err)
		}
		if digest(raw) != composed[name+"_SHA256"] {
			t.Fatalf("%s digest does not bind composed bytes", name)
		}
		path := filepath.Join(out, name+".json")
		write(path, raw)
		return path
	}
	cfg := config.Config{
		NodeEnv: "production", RuntimeProfile: "container", BindAddress: "0.0.0.0", Port: 8080,
		RuntimePoolID: "mediagatewayd_pool", RuntimeInstanceID: "gateway-documents",
		RuntimeBudgetDocument: composed["DB_BUDGET_DOCUMENT"], RuntimeBudgetSHA256: composed["DB_BUDGET_SHA256"],
		GatewayRuntimeFile: bound("GATEWAY_RUNTIME"), GatewayRuntimeSHA256: composed["GATEWAY_RUNTIME_SHA256"],
		IdentityConfigFile: bound("IDENTITY_CONFIG"), IdentityConfigSHA256: composed["IDENTITY_CONFIG_SHA256"],
		SecretsProvider: "env", CORSAllowedOrigins: "https://app.kindlelaunch.example",
		IdentityLimiterKey: strings.Repeat("l", 32), S3Endpoint: "https://objects.kindlelaunch.example",
		S3Region: "us-east-1", S3AccessKeyID: "gateway-serving-access", S3SecretAccessKey: "gateway-serving-secret",
		TokenBucket: "media-approved", ObjectCacheMaxBytes: 1 << 20,
	}
	var binding gatewayRuntimeBinding
	if err := config.ReadBoundDocument(cfg.GatewayRuntimeFile, cfg.GatewayRuntimeSHA256, &binding); err != nil {
		t.Fatal(err)
	}
	clone := func() gatewayRuntimeBinding {
		t.Helper()
		var result gatewayRuntimeBinding
		if err := json.Unmarshal(marshal(binding), &result); err != nil {
			t.Fatal(err)
		}
		return result
	}
	pool := func(user string, max int32) *pgxpool.Pool {
		t.Helper()
		options, err := pgxpool.ParseConfig("postgres://" + user + "@localhost/gateway")
		if err != nil {
			t.Fatal(err)
		}
		options.MaxConns, options.MinConns, options.MinIdleConns = max, 0, 0
		result, err := pgxpool.NewWithConfig(ctx, options)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(result.Close)
		return result
	}
	identityPool, controlPool := pool("identity_runtime", binding.Pools.IdentityMaxConnections), pool("control_runtime", binding.Pools.ControlMaxConnections)
	rdb := goredis.NewClient(&goredis.Options{Addr: "localhost:6379"})
	t.Cleanup(func() { _ = rdb.Close() })
	objectCache := cache.New(rdb, time.Minute)
	readers, err := buildBuckets(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	prepare := func(candidate gatewayRuntimeBinding) (*gatewayRuntimePreparation, error) {
		return prepareGatewayConnectedRuntime(ctx, cfg, identityPool, controlPool, candidate, rdb, objectCache, readers, nil)
	}
	prepared, err := prepare(binding)
	if err != nil {
		t.Fatalf("composed runtime bindings: %v", err)
	}
	defer prepared.runtime.close()
	if len(prepared.runtime.upstreams) != 4 || len(prepared.runtime.identityVerifiers) != len(binding.IdentityPeers)+1 || prepared.runtime.serverTLS == nil {
		t.Fatal("composed transport bindings incomplete")
	}
	if err := prepared.runtime.approved.RegisterConnectedRoutes(chi.NewRouter(), binding.ApprovedBucket); err != nil {
		t.Fatalf("approved logical bucket: %v", err)
	}
	for _, upstream := range binding.Upstreams {
		t.Run("routes/"+upstream.Name, func(t *testing.T) {
			deps := proxy.ConnectedDeps{Name: upstream.Name, URL: upstream.URL, Routes: upstream.Routes, Identity: prepared.authority, SigningKey: assertion.Private, TLS: prepared.clientTLS}
			if _, err := proxy.NewConnected(deps); err != nil {
				t.Fatalf("bound routes: %v", err)
			}
			deps.Routes = append([]proxy.ConnectedRoute(nil), upstream.Routes...)
			if upstream.Name == "metadata" {
				deps.Routes[0].Upload = true
			} else {
				deps.Routes[0].Authenticate = !deps.Routes[0].Authenticate
			}
			if _, err := proxy.NewConnected(deps); err == nil {
				t.Fatal("route with changed flags admitted")
			}
		})
	}
	for _, tc := range []struct {
		name string
		edit func(*gatewayRuntimeBinding)
		want string
	}{
		{"approved bucket", func(b *gatewayRuntimeBinding) { b.ApprovedBucket = cfg.TokenBucket }, "gateway upstream and durable bindings incomplete"},
		{"trusted proxy", func(b *gatewayRuntimeBinding) { b.TrustedProxies = []string{"10.1.0.0/8"} }, "explicit ingress proxy CIDR required"},
		{"identity endpoint", func(b *gatewayRuntimeBinding) { b.IdentityEndpoint += "/" }, "private identity HTTPS authority required"},
		{"identity listener", func(b *gatewayRuntimeBinding) { b.IdentityListenAddress = "0.0.0.0:8080" }, "distinct explicit private identity listener required"},
		{"consumer generation", func(b *gatewayRuntimeBinding) { b.ConsumerGeneration = strings.Repeat("g", 129) }, "gateway upstream and durable bindings incomplete"},
		{"CA digest", func(b *gatewayRuntimeBinding) { b.CASHA256 = zero }, "gateway TLS file binding mismatch"},
		{"client key", func(b *gatewayRuntimeBinding) { b.ClientKeyReference = b.ServerKeyReference }, "gateway private client identity invalid"},
		{"server key", func(b *gatewayRuntimeBinding) { b.ServerKeyReference = b.ClientKeyReference }, "gateway private server identity invalid"},
		{"assertion key", func(b *gatewayRuntimeBinding) { b.AssertionKeyReference = b.ClientKeyReference }, "gateway assertion signing key invalid"},
		{"identity peer key", func(b *gatewayRuntimeBinding) { b.IdentityPeers[0].PublicKey = "invalid" }, "identity service peer key invalid"},
		{"media schema", func(b *gatewayRuntimeBinding) { b.MediaSchema = "bad schema" }, "media authority unavailable"},
		{"purge endpoint", func(b *gatewayRuntimeBinding) { b.PurgeEndpoint += "/" }, "media authority unavailable"},
		{"upload namespace", func(b *gatewayRuntimeBinding) { b.UploadNamespace = "bad namespace" }, "upload admission denied"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := clone()
			tc.edit(&candidate)
			result, err := prepare(candidate)
			if result != nil {
				result.runtime.close()
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("changed binding: %v, want %q", err, tc.want)
			}
		})
	}

	t.Run("pools", func(t *testing.T) {
		options, err := cfg.RuntimePoolOptions(binding.Pools)
		if err != nil || options.MaxConns != 12 || options.DatabaseUser != "identity_runtime" {
			t.Fatalf("composed split allocation: %+v %v", options, err)
		}
		for _, edit := range []func(*config.RuntimePoolSplit){
			func(s *config.RuntimePoolSplit) { s.ControlMaxConnections++ },
			func(s *config.RuntimePoolSplit) { s.IdentityPoolID = "other_pool" },
			func(s *config.RuntimePoolSplit) { s.ControlPoolID = s.IdentityPoolID },
		} {
			candidate := binding.Pools
			edit(&candidate)
			if _, err := cfg.RuntimePoolOptions(candidate); err == nil {
				t.Fatal("changed pool split admitted")
			}
		}
	})
	t.Run("object roles", func(t *testing.T) {
		runtime := &gatewayConnectedRuntime{binding: clone()}
		if err := runtime.configureObjects(ctx, cfg, prepared.secret); err != nil || len(runtime.objects) != 3 {
			t.Fatalf("composed object roles: %v", err)
		}
		for _, worker := range runtime.objects {
			if worker.binding.Bucket != cfg.Buckets()[binding.ApprovedBucket] {
				t.Fatalf("physical object bucket %q differs from approved origin", worker.binding.Bucket)
			}
		}
		for _, edit := range []func(*gatewayRuntimeBinding){
			func(b *gatewayRuntimeBinding) { b.ObjectWorkers[0].Prefix = "quarantine/" },
			func(b *gatewayRuntimeBinding) { b.ObjectWorkers[1].Bucket = "" },
			func(b *gatewayRuntimeBinding) {
				b.ObjectWorkers[0].InventoryAccessKeyReference = b.ObjectWorkers[0].DeleteAccessKeyReference
			},
		} {
			runtime := &gatewayConnectedRuntime{binding: clone()}
			edit(&runtime.binding)
			if err := runtime.configureObjects(ctx, cfg, prepared.secret); err == nil {
				t.Fatal("changed object role admitted")
			}
		}
		t.Run("provisioned access key", func(t *testing.T) {
			t.Setenv(binding.ObjectWorkers[0].DeleteAccessKeyReference, cfg.S3AccessKeyID)
			runtime := &gatewayConnectedRuntime{binding: clone()}
			if err := runtime.configureObjects(ctx, cfg, prepared.secret); err == nil {
				t.Fatal("serving access key admitted for deletion")
			}
		})
	})
	t.Run("control", func(t *testing.T) {
		if _, err := prepared.runtime.newControlRuntime(ctx, cfg, identityPool, controlPool, prepared.authority, prepared.clientTLS, prepared.secret); err != nil {
			t.Fatalf("composed control authority: %v", err)
		}
		runtime := &gatewayConnectedRuntime{binding: clone()}
		runtime.binding.Control.Schema = "other"
		if _, err := runtime.newControlRuntime(ctx, cfg, identityPool, controlPool, prepared.authority, prepared.clientTLS, prepared.secret); err == nil {
			t.Fatal("changed control schema admitted")
		}
	})
	t.Run("epoch and delivery", func(t *testing.T) {
		for _, service := range []string{"ranking", "social"} {
			var values map[string]any
			if err := json.Unmarshal(read(filepath.Join(documents, "values", service+".example.json")), &values); err != nil {
				t.Fatal(err)
			}
			if binding.RecoverySchema != values["recoverySchema"] {
				t.Fatalf("gateway recovery schema differs from %s", service)
			}
		}
		guard, err := queue.NewEpochGuard(binding.RecoverySchema, binding.RecoveryEpoch, binding.RecoveryRegion)
		if err != nil {
			t.Fatal(err)
		}
		prepared.runtime.store, err = delivery.NewStoreWithEpochGuard(identityPool, "users", delivery.Policy{Subjects: map[string]bool{auth.IdentityRevisionSubject: true}, EventTypes: map[string]bool{auth.IdentityRevisionEventType: true}}, guard.Assert)
		if err != nil {
			t.Fatal(err)
		}
		for _, edit := range []func(*gatewayRuntimeBinding){
			func(b *gatewayRuntimeBinding) { b.RecoverySchema = "bad-schema" },
			func(b *gatewayRuntimeBinding) { b.RecoveryEpoch = 0 },
			func(b *gatewayRuntimeBinding) { b.RecoveryRegion = "bad_region" },
		} {
			candidate := clone()
			edit(&candidate)
			if _, err := queue.NewEpochGuard(candidate.RecoverySchema, candidate.RecoveryEpoch, candidate.RecoveryRegion); err == nil {
				t.Fatal("changed epoch binding admitted")
			}
		}
		if err := prepared.runtime.configureAuditStores(identityPool, guard, prepared.secret); err != nil {
			t.Fatalf("composed audit stores: %v", err)
		}
		candidate := &gatewayConnectedRuntime{binding: clone()}
		candidate.binding.Audit.Durable = binding.Durable
		if err := candidate.configureAuditStores(identityPool, guard, prepared.secret); err == nil {
			t.Fatal("shared identity and audit durable admitted")
		}
		if err := validateGatewayBrokerURL(binding.NATSURL); err != nil {
			t.Fatal(err)
		}
		if err := validateGatewayBrokerURL(binding.NATSURL + "/"); err == nil {
			t.Fatal("broker URL with path admitted")
		}
		for _, item := range []struct{ stream, durable, subject string }{
			{binding.Stream, binding.Durable, auth.IdentityRevisionSubject},
			{binding.Audit.Stream, binding.Audit.Durable, binding.Audit.Subject},
		} {
			consumer := natsdriver.ConsumerConfig{Stream: item.stream, Durable: item.durable, Subject: item.subject, Batch: 32, MaxAckPending: 128, MaxWaiting: 4, MaxDeliver: 10, FetchWait: 500 * time.Millisecond, AckWait: 30 * time.Second}
			if _, err := consumer.BrokerConsumerConfig(); err != nil {
				t.Fatalf("composed durable: %v", err)
			}
			consumer.Durable = "bad durable"
			if _, err := consumer.BrokerConsumerConfig(); err == nil {
				t.Fatal("changed durable admitted")
			}
		}
	})
	t.Run("identity keys", func(t *testing.T) {
		identity, err := prepareProductionIdentity(ctx, cfg, newIdentityRuntimeMetrics())
		if err != nil || identity.tokens == nil {
			t.Fatalf("composed identity signing keys: %v", err)
		}
		if err := identity.check(ctx); err != nil {
			t.Fatal(err)
		}
		document := identity.binding
		identityConfig := auth.IdentityConfig{Pool: identityPool, Domain: document.Domain, URI: document.URI, AllowedOrigins: document.AllowedOrigins, Verifier: auth.Chain125ProofVerifier{}, LimiterKey: []byte(cfg.IdentityLimiterKey), Tokens: identity.tokens, SecurityAuditKey: prepared.runtime.auditKey, Events: &auth.IdentityEventDelivery{Store: prepared.runtime.store, Subject: auth.IdentityRevisionSubject, EventType: auth.IdentityRevisionEventType}}
		if _, err := auth.NewIdentity(identityConfig); err != nil {
			t.Fatalf("composed identity binding: %v", err)
		}
		identityConfig.Domain = "other.kindlelaunch.example"
		if _, err := auth.NewIdentity(identityConfig); err == nil {
			t.Fatal("changed identity domain admitted")
		}
		candidate := document
		candidate.ActiveKey = "absent"
		path := filepath.Join(out, "changed-identity.json")
		raw := marshal(candidate)
		write(path, raw)
		changed := cfg
		changed.IdentityConfigFile, changed.IdentityConfigSHA256 = path, digest(raw)
		if _, err := prepareProductionIdentity(ctx, changed, newIdentityRuntimeMetrics()); err == nil {
			t.Fatal("changed active identity key admitted")
		}
		t.Setenv(document.Keys[0].SecretReference, strings.Repeat("z", 64))
		if err := identity.check(ctx); err == nil {
			t.Fatal("changed identity signing secret admitted")
		}
	})
	if identityPool.Stat().TotalConns() != 0 || controlPool.Stat().TotalConns() != 0 {
		t.Fatal("document binding checks opened database connections")
	}
}
