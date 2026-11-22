package app

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/ranker"
)

func lifecycleAddress(t *testing.T) (string, net.Listener) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = listener.Close() })
	return listener.Addr().String(), listener
}

func lifecycleEnvironment(t *testing.T, dsn, redisURL, address string) {
	t.Helper()
	contractEnv(t)
	_, port, err := net.SplitHostPort(address)
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{
		"NODE_ENV": "test", "DATABASE_URL": dsn, "REDIS_URL": redisURL, "REDIS_BULL_URL": redisURL,
		"RPC_URL": "https://rpc.example.com/rpc", "BIND_ADDRESS": "127.0.0.1", "PORT": port,
		"LOG_LEVEL": "error", "ZONE": "zone-a", "METRICS_ENVIRONMENT": "production",
		"RANKING_TICK_INTERVAL_MS": "30000", "RANKING_NEW_TICK_INTERVAL_MS": "10000",
	} {
		t.Setenv(key, value)
	}
}

func lifecycleRun(t *testing.T) (context.CancelFunc, <-chan error) {
	t.Helper()
	previous := slog.Default()
	t.Cleanup(func() { slog.SetDefault(previous) })
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- Run(ctx); close(done) }()
	t.Cleanup(func() {
		cancel()
		select {
		case <-done:
		case <-time.After(40 * time.Second):
			t.Error("service did not finish shutdown")
		}
	})
	return cancel, done
}

func lifecycleGet(t *testing.T, client *http.Client, endpoint string) (int, []byte) {
	t.Helper()
	response, err := client.Get(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	raw, readErr := io.ReadAll(response.Body)
	closeErr := response.Body.Close()
	if readErr != nil || closeErr != nil {
		t.Fatalf("read response: %v, close: %v", readErr, closeErr)
	}
	return response.StatusCode, raw
}

func lifecycleWait(t *testing.T, client *http.Client, endpoint string, want int) {
	t.Helper()
	deadline := time.Now().Add(30 * time.Second)
	for {
		response, err := client.Get(endpoint)
		if err == nil {
			_ = response.Body.Close()
			if response.StatusCode == want {
				return
			}
		}
		if time.Now().After(deadline) {
			t.Fatalf("service did not answer %d at %s: %v", want, endpoint, err)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func lifecycleStopped(t *testing.T, done <-chan error) error {
	t.Helper()
	select {
	case err := <-done:
		return err
	case <-time.After(40 * time.Second):
		t.Fatal("service did not return within its shutdown deadline")
		return nil
	}
}

func TestRunDevelopmentServesAndDrains(t *testing.T) {
	dsn, _ := internaltest.NewPostgresWithDSN(t)
	redisURL := internaltest.NewRedisURL(t)
	address, reserved := lifecycleAddress(t)
	lifecycleEnvironment(t, dsn, redisURL, address)
	if err := reserved.Close(); err != nil {
		t.Fatal(err)
	}
	cancel, done := lifecycleRun(t)
	client := &http.Client{Timeout: time.Second}
	defer client.CloseIdleConnections()
	lifecycleWait(t, client, "http://"+address+healthReadyPath, http.StatusOK)
	code, body := lifecycleGet(t, client, "http://"+address+"/rankings/new")
	if code != http.StatusOK || !strings.Contains(string(body), `"items":[]`) {
		t.Fatalf("empty development rankings: %d %s", code, body)
	}
	cancel()
	lifecycleWait(t, client, "http://"+address+healthReadyPath, http.StatusServiceUnavailable)
	if code, _ := lifecycleGet(t, client, "http://"+address+healthLivePath); code != http.StatusOK {
		t.Fatalf("draining liveness = %d", code)
	}
	if code, _ := lifecycleGet(t, client, "http://"+address+"/rankings/new"); code != http.StatusServiceUnavailable {
		t.Fatalf("draining rankings = %d", code)
	}
	if err := lifecycleStopped(t, done); err != nil {
		t.Fatalf("development shutdown: %v", err)
	}
}

func TestRunReportsStartupAndListenerFailures(t *testing.T) {
	dsn, pool := internaltest.NewPostgresWithDSN(t)
	redisURL := internaltest.NewRedisURL(t)
	address, _ := lifecycleAddress(t)
	lifecycleEnvironment(t, dsn, redisURL, address)
	t.Run("invalid configuration", func(t *testing.T) {
		t.Setenv("RANKING_TICK_INTERVAL_MS", "0")
		if err := Run(context.Background()); err == nil || !strings.Contains(err.Error(), "RANKING_TICK_INTERVAL_MS must be positive") {
			t.Fatalf("invalid cadence: %v", err)
		}
	})
	t.Run("database configuration", func(t *testing.T) {
		t.Setenv("DATABASE_URL", "postgres://127.0.0.1:1/unavailable?connect_timeout=1")
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if err := Run(ctx); err == nil {
			t.Fatal("unavailable database admitted")
		}
	})
	t.Run("initial computation and listener", func(t *testing.T) {
		if _, err := pool.Exec(context.Background(), "DROP SCHEMA stats CASCADE; DROP SCHEMA indexer CASCADE"); err != nil {
			t.Fatal(err)
		}
		output, err := os.CreateTemp(t.TempDir(), "ranking-output-")
		if err != nil {
			t.Fatal(err)
		}
		stdoutFD := int(os.Stdout.Fd())
		previousFD, err := syscall.Dup(stdoutFD)
		if err != nil {
			t.Fatal(err)
		}
		if err := syscall.Dup2(int(output.Fd()), stdoutFD); err != nil {
			_ = syscall.Close(previousFD)
			t.Fatal(err)
		}
		defer func() {
			if err := syscall.Dup2(previousFD, stdoutFD); err != nil {
				t.Error(err)
			}
			_ = syscall.Close(previousFD)
			_ = output.Close()
		}()
		_, done := lifecycleRun(t)
		err = lifecycleStopped(t, done)
		var op *net.OpError
		if !errors.As(err, &op) || op.Op != "listen" {
			t.Fatalf("occupied listener: %v", err)
		}
		raw, err := os.ReadFile(output.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, message := range []string{"initial runAll failed", "initial runNew failed"} {
			if !strings.Contains(string(raw), message) {
				t.Fatalf("missing computation diagnostic %q: %s", message, raw)
			}
		}
	})
}

func TestNewRejectsDependencies(t *testing.T) {
	dsn, _ := internaltest.NewPostgresWithDSN(t)
	for _, tc := range []struct {
		name string
		cfg  config.Config
		want string
	}{
		{"environment", config.Config{}, "environment_invalid"},
		{"metrics identity", config.Config{RuntimeMetricsEnvironment: "production", RuntimeZone: "unknown"}, "telemetry_identity_outside_inventory"},
		{"redis address", config.Config{}, "redis"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := tc.cfg
			switch tc.name {
			case "environment":
				cfg.NodeEnv = "unknown"
			case "metrics identity":
				cfg.NodeEnv = "production"
				cfg.SecretsProvider, cfg.SecretsRefreshSeconds = "env", 30
				t.Setenv("DATABASE_URL", dsn)
				t.Setenv("REDIS_URL", "redis://localhost:6379")
				t.Setenv("RPC_URL", "https://rpc.example.com/rpc")
			case "redis address":
				cfg.NodeEnv, cfg.DatabaseURL, cfg.RedisURL = "test", dsn, "invalid"
			}
			instance, err := New(context.Background(), cfg, quietLogger())
			if instance != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("dependency refusal = %v, %v", instance, err)
			}
		})
	}
}

func TestRunNativeServesPublishesAndStops(t *testing.T) {
	database := internaltest.NewRankingRuntimeDatabase(t)
	redisURL := internaltest.NewRedisURL(t)
	admin := publicationRecoveryOpenPool(t, database.AdminDSN)
	defer admin.Close()
	ctx := context.Background()
	for _, sql := range []string{
		`INSERT INTO indexer.indexer_ingest_leases(chain_id,stream_id,owner_instance,fence_token,expires_at) VALUES(125,'chain125-canonical','indexer-1',1,clock_timestamp()+interval '10 minutes')`,
		`INSERT INTO indexer.indexer_ingest_watermarks(chain_id,stream_id,raw_complete_height,typed_complete_height) VALUES(125,'chain125-canonical',200,200)`,
		`INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete,completed_at) VALUES(125,'chain125-canonical',200,'0x` + strings.Repeat("c8", 32) + `',0,true,true,clock_timestamp())`,
	} {
		if _, err := admin.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	pki := newPKI(t)
	server := pki.issue(t, rankingDNS, x509.ExtKeyUsageServerAuth)
	clientLeaf := pki.issue(t, rankingDNS, x509.ExtKeyUsageClientAuth)
	gateway := pki.issue(t, gatewayDNS, x509.ExtKeyUsageClientAuth)
	social := pki.issue(t, visibilityDNS, x509.ExtKeyUsageServerAuth)
	peer, authority := publicationRecoveryVisibilityPeer(t, pki, social, clientLeaf, map[string]bool{})
	dir := t.TempDir()
	evidence := rankingEvidencePeer(t, map[string]bool{})
	binding := config.RankingRuntimeBinding{
		Version: 2, RankingSchema: "ranking", StatsSchema: "stats", SourceSchema: "indexer", SourceStream: sourceStream, ProjectionVersion: projection,
		CacheNamespace: "snapshots", CursorKeyReference: "TEST_RANKING_CURSOR_KEY", RecoverySchema: "recovery", RecoveryEpoch: database.Epoch, RecoveryRegion: database.Region,
		EvidenceEndpoint:   evidence.URL + "/internal/ranking-evidence",
		VisibilityEndpoint: peer.URL + "/internal/v2/social/ranking-visibility", VisibilityDNS: visibilityDNS, VisibilityCertSHA256: social.pin(), GatewayDNS: gatewayDNS, GatewayCertSHA256: gateway.pin(),
		CAFile: filepath.Join(dir, "ca.pem"), ClientCertFile: filepath.Join(dir, "client.pem"), ClientKeyReference: "TEST_RANKING_CLIENT_KEY_PEM", ServerCertFile: filepath.Join(dir, "server.pem"), ServerKeyReference: "TEST_RANKING_SERVER_KEY_PEM", MaxAttempts: 10, MaximumReads: 8,
	}
	binding.CASHA256 = writeBound(t, binding.CAFile, pki.caPEM)
	binding.ClientCertSHA256 = writeBound(t, binding.ClientCertFile, clientLeaf.certPEM)
	binding.ServerCertSHA256 = writeBound(t, binding.ServerCertFile, server.certPEM)
	t.Setenv(binding.ClientKeyReference, string(clientLeaf.keyPEM))
	t.Setenv(binding.ServerKeyReference, string(server.keyPEM))
	t.Setenv(binding.CursorKeyReference, base64.RawURLEncoding.EncodeToString([]byte(strings.Repeat("c", 32))))
	address, reserved := lifecycleAddress(t)
	privateAddress, privateReserved := lifecycleAddress(t)
	lifecycleEnvironment(t, database.RuntimeDSN, redisURL, address)
	t.Setenv("NODE_ENV", "production")
	t.Setenv("RUNTIME_PROFILE", "owned_local")
	t.Setenv("SECRETS_PROVIDER", "env")
	t.Setenv("SECRETS_REFRESH_SECONDS", "30")
	t.Setenv("RANKING_PRIVATE_LISTEN_ADDRESS", privateAddress)
	t.Setenv("INSTANCE_ID", "ranking-lifecycle")
	var maxConnections int64
	if err := admin.QueryRow(ctx, "SELECT setting::bigint FROM pg_settings WHERE name='max_connections'").Scan(&maxConnections); err != nil {
		t.Fatal(err)
	}
	composeStartup(t, database, binding, maxConnections)
	// The composed contract supplies container defaults; this listener is owned locally.
	_, port, _ := net.SplitHostPort(address)
	t.Setenv("BIND_ADDRESS", "127.0.0.1")
	t.Setenv("PORT", port)
	t.Setenv("RUNTIME_PROFILE", "owned_local")
	t.Setenv("RANKING_PRIVATE_LISTEN_ADDRESS", privateAddress)
	if err := reserved.Close(); err != nil {
		t.Fatal(err)
	}
	if err := privateReserved.Close(); err != nil {
		t.Fatal(err)
	}
	cancel, done := lifecycleRun(t)
	health := &http.Client{Timeout: time.Second}
	defer health.CloseIdleConnections()
	lifecycleWait(t, health, "http://"+address+healthLivePath, http.StatusOK)
	lifecycleWait(t, health, "http://"+address+healthReadyPath, http.StatusOK)
	authority.Store(false)
	if code, _ := lifecycleGet(t, health, "http://"+address+healthReadyPath); code != http.StatusServiceUnavailable {
		t.Fatalf("readiness without visibility authority = %d", code)
	}
	roots := x509.NewCertPool()
	roots.AddCert(pki.caCert)
	client := &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, ServerName: rankingDNS, Certificates: []tls.Certificate{gateway.pair(t)}}}}
	defer client.CloseIdleConnections()
	base := "https://" + privateAddress
	if code, body := lifecycleGet(t, client, base+"/internal/v2/rankings/inputs"); code != http.StatusServiceUnavailable || !strings.Contains(string(body), "RANKING_INPUT_UNAVAILABLE") {
		t.Fatalf("inputs without visibility: %d %s", code, body)
	}
	if code, body := lifecycleGet(t, client, base+"/rankings/unknown"); code != http.StatusBadRequest || !strings.Contains(string(body), "INVALID_CATEGORY") {
		t.Fatalf("invalid category: %d %s", code, body)
	}
	authority.Store(true)
	lifecycleWait(t, health, "http://"+address+healthReadyPath, http.StatusOK)
	code, body := lifecycleGet(t, client, base+"/internal/v2/rankings/inputs")
	var frozen ranker.InputSnapshot
	if code != http.StatusOK || json.Unmarshal(body, &frozen) != nil || !frozen.Complete || frozen.Candidates == nil || len(frozen.Candidates) != 0 {
		t.Fatalf("complete empty native inputs: %d %s", code, body)
	}
	var published int
	publicationDeadline := time.Now().Add(5 * time.Second)
	for {
		if err := admin.QueryRow(ctx, "SELECT count(*) FROM ranking.publication_outbox WHERE published_at IS NOT NULL").Scan(&published); err != nil {
			t.Fatal(err)
		}
		if published == len(ranker.Categories) {
			break
		}
		if time.Now().After(publicationDeadline) {
			t.Fatalf("published categories = %d, want %d", published, len(ranker.Categories))
		}
		time.Sleep(20 * time.Millisecond)
	}
	if code, body := lifecycleGet(t, client, base+metricsPath); code != http.StatusOK || !strings.Contains(string(body), "sli_read_eligible_requests_total{") {
		t.Fatalf("native metrics: %d %s", code, body)
	}
	cancel()
	if err := lifecycleStopped(t, done); err != nil {
		t.Fatalf("native shutdown: %v", err)
	}
	for _, endpoint := range []string{address, privateAddress} {
		connection, err := net.DialTimeout("tcp", endpoint, time.Second)
		if err == nil {
			_ = connection.Close()
			t.Fatalf("listener still accepts connections after shutdown: %s", endpoint)
		}
	}
	for _, occupied := range []string{address, privateAddress} {
		t.Run("occupied "+occupied, func(t *testing.T) {
			listener, err := net.Listen("tcp", occupied)
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			_, failed := lifecycleRun(t)
			err = lifecycleStopped(t, failed)
			var op *net.OpError
			if !errors.As(err, &op) || op.Op != "listen" || op.Addr.String() != occupied {
				t.Fatalf("native listener failure: %v", err)
			}
		})
	}

}
