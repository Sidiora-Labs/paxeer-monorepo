package app

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/pem"
	"errors"
	"io"
	"log/slog"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/metadata"
	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/process"
	sharedredis "github.com/Sidiora-Technologies/KindleLaunch/shared/redis"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth/ethconfig"
	"github.com/ethereum/go-ethereum/ethclient/simulated"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/params"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProductionWiringChecksRealMediaDependencies(t *testing.T) {
	ctx := context.Background()
	admin, dsn := internaltest.NewPostgresURL(t)
	if _, err := admin.Exec(ctx, "CREATE SCHEMA chat; CREATE SCHEMA IF NOT EXISTS metadata; CREATE TABLE chat.safety_visibility(target_type text NOT NULL,target_id text NOT NULL,status text NOT NULL,revision bigint NOT NULL,PRIMARY KEY(target_type,target_id)); CREATE VIEW chat.safety_visibility_v2 AS SELECT target_type,target_id,status,revision FROM chat.safety_visibility"); err != nil {
		t.Fatal(err)
	}
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatal(err)
	}
	poolConfig.ConnConfig.RuntimeParams["search_path"] = "metadata,pg_catalog"
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		t.Fatal(err)
	}
	redisURL := internaltest.NewRedisURL(t)
	rdb, err := sharedredis.NewClient(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	defer rdb.Close()
	objects := internaltest.NewMinIO(t, "metadata-production-wiring-approved")
	quarantine := internaltest.NewMinIO(t, "metadata-production-wiring-quarantine")
	host, port := internaltest.NewClamd(t)
	if host == "" || port == 0 {
		t.Fatal("production scanner endpoint is required")
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	chainPort := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	chainConfig := *params.AllDevChainProtocolChanges
	chainConfig.ChainID = big.NewInt(125)
	backend := simulated.NewBackend(types.GenesisAlloc{}, func(n *node.Config, e *ethconfig.Config) {
		n.HTTPHost = "127.0.0.1"
		n.HTTPPort = chainPort
		n.HTTPModules = []string{"eth"}
		e.Genesis.Config = &chainConfig
		e.NetworkId = 125
	})
	defer backend.Close()
	executable := filepath.Join(t.TempDir(), "metadatad")
	build := exec.Command("go", "build", "-o", executable, "./cmd/metadatad")
	build.Dir = "../.."
	build.Env = append(os.Environ(), "GOWORK=off", "GOFLAGS=")
	if output, err := build.CombinedOutput(); err != nil {
		t.Fatalf("build raster worker: %v %s", err, output)
	}
	cfg := config.Config{
		NodeEnv: "production", Ingress: config.IngressEdge,
		RasterIsolation: config.IsolationRlimit, RasterExecutable: executable,
		ModerationSource: config.ModerationDatabase, ModerationSchema: "chat",
		MetadataSchema: "metadata", CanonicalSchema: "indexer", CanonicalStreamID: "paxeer125-canonical",
		IdentityAudience: "https://metadata.sidiora.test", IdentityServiceURL: "https://app.sidiora.test", IdentityServiceToken: strings.Repeat("a", 32),
		QuarantineBucket: quarantine.Bucket, ApprovedBucket: objects.Bucket,
		ClamAVHost: host, ClamAVPort: port, PublicURL: "https://metadata.sidiora.test",
		RPCURL:     "http://127.0.0.1:" + strconv.Itoa(chainPort),
		S3Endpoint: objects.Endpoint, S3Region: "us-east-1", S3Bucket: objects.Bucket,
		S3AccessKeyID: objects.AccessKeyID, S3SecretAccessKey: objects.SecretAccessKey,
		DatabaseURL: dsn, RedisURL: redisURL, SecretsProvider: "env",
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	limitedConfig := pool.Config()
	limitedConfig.MaxConns = 2
	limitedPool, err := pgxpool.NewWithConfig(ctx, limitedConfig)
	if err != nil {
		t.Fatal(err)
	}
	owned := false
	_, secretStop := context.WithCancel(ctx)
	if instance, err := newRuntime(ctx, cfg, logger, secretStop, limitedPool, rdb, process.NewTracker(), nil, map[string]sharedhttp.Check{}, &owned); instance != nil || err == nil || !strings.Contains(err.Error(), "metadata_runtime_pool_requires_three_connections") || owned {
		t.Fatalf("limited runtime pool = %v, owned=%v", err, owned)
	}
	secretStop()
	limitedPool.Close()
	badRPC := cfg
	badRPC.RPCURL = "://"
	owned = false
	_, secretStop = context.WithCancel(ctx)
	if instance, err := newRuntime(ctx, badRPC, logger, secretStop, pool, rdb, process.NewTracker(), nil, map[string]sharedhttp.Check{}, &owned); instance != nil || err == nil || !strings.Contains(err.Error(), "metadata_rpc_unavailable") || owned {
		t.Fatalf("invalid chain endpoint = %v, owned=%v", err, owned)
	}
	secretStop()
	publicationDocument := []byte(`{"schemaVersion":3,"environment":"paxeer125-production","envelopeHeader":"X-Kindle-Operation","signatureHeader":"X-Kindle-Operation-Signature"}`)
	publicationDigest := sha256.Sum256(publicationDocument)
	bindPublication := func(c *config.Config) {
		c.PublicationConfigDocument = base64.StdEncoding.EncodeToString(publicationDocument)
		c.PublicationConfigSHA256 = hex.EncodeToString(publicationDigest[:])
		c.PublicationAuditKey = base64.StdEncoding.EncodeToString([]byte(strings.Repeat("a", 32)))
	}
	for _, tc := range []struct {
		name   string
		change func(*config.Config)
		want   string
	}{
		{name: "missing scanner", change: func(c *config.Config) { c.ClamAVPort = 1 }, want: "metadata_media_dependencies_unavailable"},
		{name: "missing public bucket", change: func(c *config.Config) { c.ApprovedBucket = "missing-public-bucket" }, want: "metadata_media_dependencies_unavailable"},
		{name: "invalid metadata schema", change: func(c *config.Config) { c.MetadataSchema = "Invalid" }, want: "invalid metadata"},
		{name: "invalid secret provider", change: func(c *config.Config) { c.SecretsProvider = "invalid" }, want: "secret"},
		{name: "unbound publication", change: func(*config.Config) {}, want: "runtime_document_read"},
		{name: "missing committed authority", change: bindPublication, want: "metadata_publication_committed_authority_unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			variant := cfg
			tc.change(&variant)
			owned := false
			_, secretStop := context.WithCancel(ctx)
			defer secretStop()
			instance, err := newRuntime(ctx, variant, logger, secretStop, pool, rdb, process.NewTracker(), nil, map[string]sharedhttp.Check{}, &owned)
			if instance != nil {
				instance.Close()
				t.Fatal("unbound production runtime admitted")
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) || owned {
				t.Fatalf("production dependency check = %v, owned=%v, want %q", err, owned, tc.want)
			}
		})
	}
	genesis, err := backend.Client().HeaderByNumber(ctx, big.NewInt(0))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "CREATE TABLE metadata.auth_committed_blocks(chain_id integer NOT NULL,height bigint NOT NULL,block_hash text NOT NULL,integrity text NOT NULL)"); err != nil {
		t.Fatal(err)
	}
	if _, err := admin.Exec(ctx, "INSERT INTO metadata.auth_committed_blocks(chain_id,height,block_hash,integrity) VALUES(125,0,$1,'healthy')", strings.ToLower(genesis.Hash().Hex())); err != nil {
		t.Fatal(err)
	}
	bound := cfg
	bindPublication(&bound)
	owned = false
	_, secretStop = context.WithCancel(ctx)
	defer secretStop()
	instance, err := newRuntime(ctx, bound, logger, secretStop, pool, rdb, process.NewTracker(), nil, map[string]sharedhttp.Check{}, &owned)
	if instance != nil {
		instance.Close()
		t.Fatal("production runtime admitted without publication storage")
	}
	if err == nil || !strings.Contains(err.Error(), "metadata_publication_storage_unavailable") || owned {
		t.Fatalf("production storage check = %v, owned=%v", err, owned)
	}
	store, err := metadata.NewProductionStore(pool, "metadata", nil)
	if err != nil {
		t.Fatal(err)
	}
	ledgerDDL := strings.ReplaceAll(sharedauth.LedgerDDL, "CREATE TABLE operation_ledger", "CREATE TABLE metadata.operation_ledger")
	ledgerDDL = strings.ReplaceAll(ledgerDDL, "CREATE TABLE security_audit_outbox", "CREATE TABLE metadata.security_audit_outbox")
	if _, err := admin.Exec(ctx, store.SchemaDDL()+store.V2SchemaDDL()+store.ConnectedSchemaDDL()+store.PublicationSchemaDDL()+ledgerDDL); err != nil {
		t.Fatal(err)
	}
	identityServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer identityServer.Close()
	certificate := identityServer.Certificate()
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw})
	keyDER, err := x509.MarshalPKCS8PrivateKey(identityServer.TLS.Certificates[0].PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	privateDir := t.TempDir()
	private := bound
	private.Ingress = config.IngressMTLS
	private.TLSCertFile = filepath.Join(privateDir, "server.pem")
	private.TLSKeyFile = filepath.Join(privateDir, "server.key")
	private.TLSClientCAFile = filepath.Join(privateDir, "clients.pem")
	for path, body := range map[string][]byte{private.TLSCertFile: certificatePEM, private.TLSKeyFile: keyPEM, private.TLSClientCAFile: certificatePEM} {
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	private.IdentityEndpoint = identityServer.URL
	identityPin := sha256.Sum256(certificate.Raw)
	private.IdentityPeerCertSHA256 = hex.EncodeToString(identityPin[:])
	_, signer, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	private.MetadataAssertionPrivateKeyHex = hex.EncodeToString(signer)
	badPrivate := private
	badPrivate.MetadataAssertionPrivateKeyHex = "invalid"
	owned = false
	if instance, err := newRuntime(ctx, badPrivate, logger, secretStop, pool, rdb, process.NewTracker(), nil, map[string]sharedhttp.Check{}, &owned); instance != nil || err == nil || !strings.Contains(err.Error(), "metadata_signer_unavailable") || owned {
		t.Fatalf("invalid private signer = %v, owned=%v", err, owned)
	}
	badTLS := private
	badTLS.TLSKeyFile = filepath.Join(privateDir, "missing.key")
	owned = false
	if instance, err := newRuntime(ctx, badTLS, logger, secretStop, pool, rdb, process.NewTracker(), nil, map[string]sharedhttp.Check{}, &owned); instance != nil || err == nil || !strings.Contains(err.Error(), "tls_load") || owned {
		t.Fatalf("unavailable private certificate = %v, owned=%v", err, owned)
	}
	owned = false
	instance, err = newRuntime(ctx, private, logger, secretStop, pool, rdb, process.NewTracker(), nil, map[string]sharedhttp.Check{}, &owned)
	if instance != nil {
		instance.Close()
		t.Fatal("private runtime admitted without identity guard")
	}
	if err == nil || !strings.Contains(err.Error(), "metadata_publication_identity_authority_unavailable") || owned {
		t.Fatalf("private identity check = %v, owned=%v", err, owned)
	}
	badTransport := private
	badTransport.IdentityEndpoint = "://"
	owned = false
	instance, err = newRuntime(ctx, badTransport, logger, secretStop, pool, rdb, process.NewTracker(), nil, map[string]sharedhttp.Check{}, &owned)
	if instance != nil {
		instance.Close()
		t.Fatal("invalid identity transport admitted")
	}
	if err == nil || !strings.Contains(err.Error(), "metadata_identity_transport_unavailable") || owned {
		t.Fatalf("identity transport check = %v, owned=%v", err, owned)
	}
	missingModeration := bound
	missingModeration.ModerationSchema = "missing_visibility"
	owned = false
	instance, err = newRuntime(ctx, missingModeration, logger, secretStop, pool, rdb, process.NewTracker(), nil, map[string]sharedhttp.Check{}, &owned)
	if instance != nil {
		instance.Close()
		t.Fatal("unavailable moderation schema admitted")
	}
	if err == nil || !strings.Contains(err.Error(), "metadata_moderation_authority_unavailable") || owned {
		t.Fatalf("moderation schema check = %v, owned=%v", err, owned)
	}
	serviceModeration := private
	serviceModeration.ModerationSource = config.ModerationService
	serviceModeration.ModerationEndpoint = identityServer.URL
	serviceModeration.ModerationPeerCertSHA256 = hex.EncodeToString(identityPin[:])
	serviceModeration.ModerationBearerKey = "METADATA_TEST_MODERATION_TOKEN"
	t.Setenv("METADATA_TEST_MODERATION_TOKEN", "")
	owned = false
	instance, err = newRuntime(ctx, serviceModeration, logger, secretStop, pool, rdb, process.NewTracker(), nil, map[string]sharedhttp.Check{}, &owned)
	if instance != nil {
		instance.Close()
		t.Fatal("moderation service admitted without its credential")
	}
	if err == nil || !strings.Contains(err.Error(), "metadata_moderation_identity_unavailable") || owned {
		t.Fatalf("moderation credential check = %v, owned=%v", err, owned)
	}
	t.Setenv("METADATA_TEST_MODERATION_TOKEN", strings.Repeat("m", 32))
	owned = false
	instance, err = newRuntime(ctx, serviceModeration, logger, secretStop, pool, rdb, process.NewTracker(), nil, map[string]sharedhttp.Check{}, &owned)
	if instance != nil {
		instance.Close()
		t.Fatal("moderation identity accepted an unrelated session")
	}
	if err == nil || !strings.Contains(err.Error(), "metadata_moderation_identity_unavailable") || owned {
		t.Fatalf("moderation identity check = %v, owned=%v", err, owned)
	}
	badProxies := bound
	badProxies.TrustedProxyCIDRs = "0.0.0.0/0"
	owned = false
	instance, err = newRuntime(ctx, badProxies, logger, secretStop, pool, rdb, process.NewTracker(), nil, map[string]sharedhttp.Check{}, &owned)
	if instance != nil {
		instance.Close()
		t.Fatal("unbounded trusted proxy admitted")
	}
	if err == nil || !strings.Contains(err.Error(), "metadata_trusted_proxy_invalid") || owned {
		t.Fatalf("trusted proxy check = %v, owned=%v", err, owned)
	}
	owned = false
	checks := map[string]sharedhttp.Check{}
	instance, err = newRuntime(ctx, bound, logger, secretStop, pool, rdb, process.NewTracker(), nil, checks, &owned)
	if err != nil || instance == nil || !owned {
		t.Fatalf("production runtime = %v, owned=%v", err, owned)
	}
	for _, name := range []string{"media_retention", "media_objects", "media_scanner", "media_worker", "media_moderation", "media_rpc", "metadata_publication"} {
		check := checks[name]
		if check == nil || check(ctx) != nil {
			t.Fatalf("%s readiness unavailable", name)
		}
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if err := checks["media_retention"](canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled media maintenance readiness = %v", err)
	}
	if err := checks["media_worker"](canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled worker readiness = %v", err)
	}
	if err := checks["media_rpc"](canceled); err == nil {
		t.Fatal("canceled chain request reported ready")
	}
	instance.Close()
	if _, err := admin.Exec(ctx, "CREATE SCHEMA users; CREATE FUNCTION users.identity_guard_v2(uuid,text) RETURNS jsonb LANGUAGE sql AS 'SELECT ''{}''::jsonb'"); err != nil {
		t.Fatal(err)
	}
	privatePool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer privatePool.Close()
	privateRedis, err := sharedredis.NewClient(redisURL)
	if err != nil {
		t.Fatal(err)
	}
	defer privateRedis.Close()
	privateChecks := map[string]sharedhttp.Check{}
	owned = false
	privateInstance, err := newRuntime(ctx, private, logger, secretStop, privatePool, privateRedis, process.NewTracker(), nil, privateChecks, &owned)
	if err != nil || privateInstance == nil || !owned {
		t.Fatalf("private runtime = %v, owned=%v", err, owned)
	}
	if err := privateChecks["metadata_publication"](ctx); err != nil {
		t.Fatalf("private publication readiness = %v", err)
	}
	privateInstance.Close()
}
