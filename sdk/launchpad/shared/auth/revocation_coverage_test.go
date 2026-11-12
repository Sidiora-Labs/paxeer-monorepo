package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
)

func TestLogoutRevocationAcrossPinnedIdentityChannel(t *testing.T) {
	pki := testpki.New(t)
	serverLeaf := pki.Leaf("identity", "localhost")
	clientLeaf := pki.Leaf("consumer", "localhost")
	key := testpki.Ed25519(t)
	principal := StreamPrincipal{AccountID: "6f9b1a52-7c0e-4b8e-9d4a-1f2e3d4c5b6a", SessionID: "0d7c2e41-5a9b-4c3d-8e1f-2a3b4c5d6e7f", Wallet: "0x1dae78b53c9f39930a2ff2b6b7aac12ec75fb154", AuthEpoch: 3}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{serverLeaf.Pair}, ClientCAs: pki.Roots(), ClientAuth: tls.RequireAndVerifyClientCert})
	if err != nil {
		t.Fatal(err)
	}
	var checks, logouts atomic.Int32
	var active atomic.Bool
	active.Store(true)
	verifier := ServiceRequestVerifier{Issuer: "consumer", Audience: "identity", Key: key.Private.Public().(ed25519.PublicKey), PeerCertSHA256: clientLeaf.SHA256, Transport: AssertionTransport{IdentityAssertionHeader, IdentitySignatureHeader}}
	server := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/internal/v2/identity/session:check":
			raw, err := io.ReadAll(io.LimitReader(r.Body, 16385))
			if err != nil || verifier.Authenticate(r, raw) != nil {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			var got StreamPrincipal
			if json.Unmarshal(raw, &got) != nil || got != principal {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			checks.Add(1)
			if !active.Load() {
				w.WriteHeader(http.StatusUnauthorized)
			}
		case "/v2/auth/sessions:logout-all":
			if r.Header.Get("Authorization") != "Bearer session-token" || r.Method != http.MethodPost {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			logouts.Add(1)
			active.Store(false)
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	})}
	go func() { _ = server.Serve(listener) }()
	t.Cleanup(func() { _ = server.Close() })
	ca, err := os.ReadFile(pki.CAFile)
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewMTLSIdentityClient(MTLSIdentityClientConfig{Endpoint: "https://" + listener.Addr().String(), PeerCertSHA256: serverLeaf.SHA256, CAPEM: string(ca), ClientCertPEM: string(clientLeaf.CertPEM), ClientKeyPEM: string(clientLeaf.KeyPEM), Issuer: "consumer", SigningKey: key.StdBase64})
	if err != nil {
		t.Fatal(err)
	}
	defer authority.Client.Transport.(*http.Transport).CloseIdleConnections()
	if address := listener.Addr().(*net.TCPAddr); !address.IP.IsLoopback() {
		t.Fatalf("private identity channel left loopback: %s", address)
	}
	endpoint := "https://" + listener.Addr().String() + "/v2/auth/sessions:logout-all"
	duration, err := MeasureLogoutRevocation(context.Background(), authority, principal, authority.Client, endpoint, "session-token")
	if err != nil || duration < 0 || duration > 5*time.Second || checks.Load() < 2 || logouts.Load() != 1 {
		t.Fatalf("bound logout revocation: duration=%s checks=%d logouts=%d err=%v", duration, checks.Load(), logouts.Load(), err)
	}
	if _, err := MeasureIdentityRevocation(context.Background(), authority, principal, time.Now()); err != nil {
		t.Fatalf("committed session remains revoked: %v", err)
	}
	if _, err := MeasureLogoutRevocation(context.Background(), authority, principal, authority.Client, endpoint, "session-token"); err == nil || logouts.Load() != 1 {
		t.Fatalf("inactive session initiated logout: %v", err)
	}
}
