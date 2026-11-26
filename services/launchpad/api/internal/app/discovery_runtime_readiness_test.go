package app

import (
	"bytes"
	"context"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/discovery"
	"github.com/Sidiora-Technologies/KindleLaunch/core/notifications"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	goredis "github.com/redis/go-redis/v9"
)

func TestDiscoveryRuntimeReadinessRecoversAfterFailedPass(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	dsn, _ := internaltest.NewPostgresWithDSN(t)
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.ExecContext(ctx, discovery.Schema+notifications.Schema); err != nil {
		t.Fatal(err)
	}
	pki := testpki.New(t)
	peer := pki.Leaf("api", "api.example")
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), Certificates: []tls.Certificate{peer.Pair}}}}
	defer client.CloseIdleConnections()
	for _, name := range []string{"DISCOVERY_READINESS_D11", "DISCOVERY_READINESS_D12", "DISCOVERY_READINESS_INTERNAL"} {
		t.Setenv(name, "runtime-value")
	}
	t.Setenv("DISCOVERY_READINESS_CURSOR", strings.Repeat("c", 32))
	runtimeConfig := notifications.RuntimeConfig{
		Principal: notificationIdentityPrincipalForReadiness,
		Secrets:   secrets.Options{Provider: secrets.ProviderEnv}, Client: client,
		D11Base: "https://accounts.example", D12Base: "https://social.example",
		D11TokenRef: "DISCOVERY_READINESS_D11", D12TokenRef: "DISCOVERY_READINESS_D12", CursorKeyRef: "DISCOVERY_READINESS_CURSOR", InternalTokenRef: "DISCOVERY_READINESS_INTERNAL",
		DeletePeerDNS: "delete.example", ChatPeerDNS: "chat.example",
		Pools: notifications.CanonicalPools{DB: db}, Intervals: discovery.SQLIntervalAuthority{DB: db},
	}
	runtime, err := notifications.NewHTTPRuntime(ctx, db, func(r *http.Request) (string, error) {
		principal, err := runtimeConfig.Principal(r)
		return principal.AccountID, err
	}, runtimeConfig)
	if err != nil {
		t.Fatal(err)
	}
	redisOptions, err := goredis.ParseURL(internaltest.NewRedisURL(t))
	if err != nil {
		t.Fatal(err)
	}
	redis := goredis.NewClient(redisOptions)
	defer redis.Close()
	app := &App{cfg: config.Config{NodeEnv: literalProduction, APIRole: literalRest, DiscoveryRuntimeDocument: "configured"}, discoveryRuntime: runtime}
	router := app.newRouter(sharedhttp.HealthDeps{DB: db.PingContext, Redis: func(ctx context.Context) error { return redis.Ping(ctx).Err() }, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	server := httptest.NewServer(router)
	defer server.Close()
	check := func() (int, string) {
		t.Helper()
		response, err := server.Client().Get(server.URL + "/health/ready")
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var body struct {
			Checks map[string]string `json:"checks"`
		}
		if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, body.Checks["discovery_notifications"]
	}
	if status, result := check(); status != http.StatusServiceUnavailable || result != "failed" {
		t.Fatalf("runtime before start: %d %s", status, result)
	}
	t.Setenv("DISCOVERY_READINESS_D11", "")
	done := make(chan error, 1)
	go func() { done <- runtime.Run(ctx) }()
	defer func() {
		cancel()
		select {
		case err := <-done:
			if !errors.Is(err, context.Canceled) {
				t.Errorf("runtime shutdown: %v", err)
			}
		case <-time.After(time.Second):
			t.Error("runtime did not stop")
		}
	}()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if err := runtime.Credentials.Ready(ctx); err != nil {
			t.Fatalf("last good credentials unavailable: %v", err)
		}
		if status, result := check(); status != http.StatusServiceUnavailable || result != "failed" {
			t.Fatalf("failed runtime pass: %d %s", status, result)
		}
		time.Sleep(10 * time.Millisecond)
	}
	select {
	case err := <-done:
		t.Fatalf("runtime stopped after credential error: %v", err)
	default:
	}
	t.Setenv("DISCOVERY_READINESS_D11", "runtime-value")
	deadline = time.Now().Add(5 * time.Second)
	for {
		status, result := check()
		if status == http.StatusOK && result == "ok" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("runtime did not restore API readiness: %d %s", status, result)
		}
		time.Sleep(10 * time.Millisecond)
	}
	// Exhausting the real pool proves the discovery check has its own deadline.
	db.SetMaxOpenConns(1)
	connection, err := db.Conn(ctx)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	err = app.discoveryReady(context.Background())
	connection.Close()
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 1500*time.Millisecond {
		t.Fatalf("discovery readiness deadline: %v after %v", err, time.Since(started))
	}
}

func notificationIdentityPrincipalForReadiness(r *http.Request) (notifications.Principal, error) {
	if r.TLS == nil || len(r.TLS.VerifiedChains) == 0 {
		return notifications.Principal{}, notifications.ErrForbidden
	}
	return notifications.Principal{AccountID: r.TLS.PeerCertificates[0].Subject.CommonName}, nil
}

func TestDiscoveryRuntimeRestartsSequentiallyAndStopsOnCancellation(t *testing.T) {
	var output bytes.Buffer
	app := &App{discoveryRuntime: &notifications.HTTPRuntime{}, logger: slog.New(slog.NewJSONHandler(&output, nil))}
	ctx, cancel := context.WithTimeout(context.Background(), 1200*time.Millisecond)
	defer cancel()
	app.runDiscovery(ctx)
	if got := strings.Count(output.String(), "discovery runtime stopped"); got != 2 {
		t.Fatalf("runtime restarts: %d, want 2: %s", got, output.String())
	}
}
