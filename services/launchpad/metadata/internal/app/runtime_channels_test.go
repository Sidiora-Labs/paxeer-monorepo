package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/config"
)

func TestPrivateRuntimeChannels(t *testing.T) {
	served := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/health/ready" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer served.Close()
	certificate := served.Certificate()
	certificatePEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw})
	keyDER, err := x509.MarshalPKCS8PrivateKey(served.TLS.Certificates[0].PrivateKey)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	dir := t.TempDir()
	cfg := config.Config{TLSCertFile: filepath.Join(dir, "server.pem"), TLSKeyFile: filepath.Join(dir, "server.key"), TLSClientCAFile: filepath.Join(dir, "clients.pem")}
	for path, body := range map[string][]byte{cfg.TLSCertFile: certificatePEM, cfg.TLSKeyFile: keyPEM, cfg.TLSClientCAFile: certificatePEM} {
		if err := os.WriteFile(path, body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	serverTLS, err := metadataServerTLS(cfg)
	if err != nil || serverTLS.MinVersion != tls.VersionTLS13 || serverTLS.ClientAuth != tls.RequireAndVerifyClientCert {
		t.Fatalf("server TLS = %+v, %v", serverTLS, err)
	}
	if err := os.Chmod(cfg.TLSKeyFile, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := metadataServerTLS(cfg); err == nil {
		t.Fatal("readable private key accepted")
	}
	roots := x509.NewCertPool()
	roots.AddCert(certificate)
	pin := sha256.Sum256(certificate.Raw)
	client := metadataPinnedClient(&tls.Config{RootCAs: roots}, hex.EncodeToString(pin[:]))
	response, err := client.Get(served.URL)
	if err != nil || response.StatusCode != http.StatusNoContent {
		t.Fatalf("pinned service read = %v, %v", response, err)
	}
	response.Body.Close()
	if err := probeServiceReady(context.Background(), served.URL, client); err != nil {
		t.Fatalf("service readiness: %v", err)
	}
	if _, err := metadataPinnedClient(&tls.Config{RootCAs: roots}, strings.Repeat("0", 64)).Get(served.URL); err == nil {
		t.Fatal("unmatched peer certificate accepted")
	}
	if err := probeServiceReady(context.Background(), served.URL+"/missing", client); err == nil {
		t.Fatal("missing service readiness accepted")
	}

	cfg.GatewayPeerCertSHA256 = hex.EncodeToString(pin[:])
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { called = true; w.WriteHeader(http.StatusNoContent) })
	channel := metadataPrivateChannel(cfg, nil)(next)
	request := httptest.NewRequest(http.MethodGet, "/v2/tokens/example/metadata", nil)
	request.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{certificate}, VerifiedChains: [][]*x509.Certificate{{certificate}}}
	recorder := httptest.NewRecorder()
	channel.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNoContent || !called {
		t.Fatalf("public metadata read = %d", recorder.Code)
	}
	called = false
	request = httptest.NewRequest(http.MethodPost, "/v3/tokens/example/metadata-publications", bytes.NewReader([]byte("{}")))
	request.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{certificate}, VerifiedChains: [][]*x509.Certificate{{certificate}}}
	recorder = httptest.NewRecorder()
	channel.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || called {
		t.Fatalf("unsigned private write = %d", recorder.Code)
	}
	request = httptest.NewRequest(http.MethodGet, "/v2/tokens/example/metadata", nil)
	recorder = httptest.NewRecorder()
	channel.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusForbidden || called {
		t.Fatalf("unverified peer = %d", recorder.Code)
	}
	accountChannel := metadataAccountChannel(config.Config{})(next)
	recorder = httptest.NewRecorder()
	accountChannel.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/internal/v2/accounts/example/assets", nil))
	if recorder.Code != http.StatusForbidden || called {
		t.Fatalf("unsigned account request = %d", recorder.Code)
	}
	recorder = httptest.NewRecorder()
	accountChannel.ServeHTTP(recorder, httptest.NewRequest(http.MethodDelete, "/internal/v2/accounts/example/assets", strings.NewReader(strings.Repeat("a", 16385))))
	if recorder.Code != http.StatusRequestEntityTooLarge || called {
		t.Fatalf("oversized account request = %d", recorder.Code)
	}
}
