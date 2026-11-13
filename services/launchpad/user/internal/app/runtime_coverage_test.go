package app

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/account"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/nats-io/nats.go"
)

func TestAccountRuntimeHealthTracksWorkerCycles(t *testing.T) {
	health := &accountRuntimeHealth{}
	if err := health.Check(context.Background()); err == nil || err.Error() != "account_worker_not_ready" {
		t.Fatalf("health before worker cycle = %v", err)
	}
	health.record(nil)
	if err := health.Check(context.Background()); err != nil {
		t.Fatalf("healthy worker cycle = %v", err)
	}
	want := errors.New("account broker unavailable")
	health.record(want)
	if err := health.Check(context.Background()); !errors.Is(err, want) {
		t.Fatalf("unhealthy worker cycle = %v", err)
	}
	canceled, stop := context.WithCancel(context.Background())
	stop()
	if err := health.Check(canceled); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled health probe = %v", err)
	}
}

func TestAccountWorkersReportDisconnectedBroker(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan struct{})
	health := &accountRuntimeHealth{}
	worker := account.Worker{Store: account.Store{DB: pool}, Runtime: &account.Runtime{}, Owner: "user-worker", LeaseDuration: time.Minute}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	go runAccountWorkers(ctx, worker, delivery.Relay{}, &nats.Conn{}, health, done, logger)
	deadline := time.After(4 * time.Second)
	for {
		err := health.Check(context.Background())
		if err != nil && strings.Contains(err.Error(), "account_broker_disconnected") && strings.Contains(err.Error(), "invalid relay configuration") {
			break
		}
		select {
		case <-deadline:
			stop()
			<-done
			t.Fatalf("worker health = %v", err)
		case <-time.After(20 * time.Millisecond):
		}
	}
	stop()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("worker did not stop after cancellation")
	}
}

func TestPinnedAccountClientChecksPeerCertificate(t *testing.T) {
	pki := testpki.New(t)
	leaf := pki.Leaf("identity", "identity.test")
	root := pki.Roots()
	cfg := &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: root, Certificates: []tls.Certificate{leaf.Pair}}
	if _, err := pinnedAccountClient(nil, leaf.SHA256); err == nil {
		t.Fatal("missing TLS configuration accepted")
	}
	if _, err := pinnedAccountClient(cfg, "invalid"); err == nil {
		t.Fatal("invalid peer pin accepted")
	}
	client, err := pinnedAccountClient(cfg, leaf.SHA256)
	if err != nil {
		t.Fatal(err)
	}
	if client.Timeout != 10*time.Second || client.CheckRedirect == nil {
		t.Fatalf("pinned client policy = %+v", client)
	}
	if err := client.CheckRedirect(nil, nil); !errors.Is(err, http.ErrUseLastResponse) {
		t.Fatalf("pinned client redirect policy = %v", err)
	}
	transport, ok := client.Transport.(*http.Transport)
	if !ok || transport.TLSClientConfig.VerifyConnection == nil {
		t.Fatal("peer verification is missing")
	}
	peer, err := x509.ParseCertificate(leaf.Pair.Certificate[0])
	if err != nil {
		t.Fatal(err)
	}
	state := tls.ConnectionState{VerifiedChains: [][]*x509.Certificate{{peer}}, PeerCertificates: []*x509.Certificate{peer}}
	if err := transport.TLSClientConfig.VerifyConnection(state); err != nil {
		t.Fatalf("pinned peer rejected: %v", err)
	}
	if err := transport.TLSClientConfig.VerifyConnection(tls.ConnectionState{}); err == nil {
		t.Fatal("unverified peer accepted")
	}
	other := sha256.Sum256([]byte("another certificate"))
	wrong, err := pinnedAccountClient(cfg, hex.EncodeToString(other[:]))
	if err != nil {
		t.Fatal(err)
	}
	if err := wrong.Transport.(*http.Transport).TLSClientConfig.VerifyConnection(state); err == nil {
		t.Fatal("certificate outside the pin accepted")
	}
}

func TestAccountTLSRejectsIncompleteMaterial(t *testing.T) {
	pki := testpki.New(t)
	leaf := pki.Leaf("user", "user.test")
	ca, err := os.ReadFile(pki.CAFile)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, cert, key, roots, want string
	}{
		{"certificate", "", string(leaf.KeyPEM), string(ca), "account_tls_certificate_unavailable"},
		{"key", string(leaf.CertPEM), "", string(ca), "account_tls_key_unavailable"},
		{"key pair", string(leaf.CertPEM), "invalid", string(ca), "failed"},
		{"roots", string(leaf.CertPEM), string(leaf.KeyPEM), "", "account_tls_roots_unavailable"},
		{"invalid roots", string(leaf.CertPEM), string(leaf.KeyPEM), "invalid", "account_tls_roots_invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, _, err := materialTLS("", tc.cert, "", tc.key, "", tc.roots)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("TLS material %s = %v", tc.name, err)
			}
		})
	}
}

func TestRunDevelopmentDrainsAfterCancellation(t *testing.T) {
	pool := internaltest.NewPostgres(t)
	redisURL := internaltest.NewRedisURL(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	for name, value := range map[string]string{
		"NODE_ENV":             "development",
		"DATABASE_URL":         pool.Config().ConnString(),
		"REDIS_URL":            redisURL,
		"S3_ENDPOINT":          os.Getenv("USER_TEST_MINIO_ENDPOINT"),
		"S3_BUCKET":            "user-assets",
		"S3_ACCESS_KEY_ID":     os.Getenv("USER_TEST_MINIO_ACCESS_KEY"),
		"S3_SECRET_ACCESS_KEY": os.Getenv("USER_TEST_MINIO_SECRET_KEY"),
		"BIND_ADDRESS":         "127.0.0.1",
		"PORT":                 strconv.Itoa(port),
	} {
		t.Setenv(name, value)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := Run(ctx); err != nil {
		t.Fatalf("development service shutdown: %v", err)
	}
}
