package auth

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/google/uuid"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

func TestRealMTLSServiceAssertion(t *testing.T) {
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(34), Subject: pkix.Name{CommonName: "L34-private-fixture"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(cert)
	certificate := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	digest := sha256.Sum256(der)
	assertion := ServiceAssertion{1, "gateway", "account", Principal{goldenAddr, "session", []string{"owner"}, "trace"}, RequestDigest("PATCH", "/profile", nil), now.Unix(), now.Unix() + 30}
	data, _ := json.Marshal(assertion)
	signature := ed25519.Sign(key, data)
	port := os.Getenv("L34_MTLS_PORT")
	if port == "" {
		port = "23401"
	}
	address := net.JoinHostPort("127.0.0.1", port)
	listener, err := tls.Listen("tcp", address, &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{certificate}, ClientCAs: roots, ClientAuth: tls.RequireAndVerifyClientCert})
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := VerifyServiceAssertion(assertion, signature, pub, r.TLS, hex.EncodeToString(digest[:]), "gateway", "account", assertion.RequestDigest, time.Now())
		if err != nil {
			http.Error(w, "denied", 401)
			return
		}
		w.WriteHeader(204)
	})}
	defer server.Close()
	go server.Serve(listener)
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, Certificates: []tls.Certificate{certificate}}}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
	resp, err := client.Get("https://" + address)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 204 {
		t.Fatal("authenticated peer denied", resp.StatusCode)
	}
	noPeer := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots}}
	defer noPeer.CloseIdleConnections()
	if resp, err := (&http.Client{Transport: noPeer, Timeout: 2 * time.Second}).Get("https://" + address); err == nil {
		resp.Body.Close()
		t.Fatal("missing peer accepted")
	}
}

func TestContractRPCDeadlineNoEOAFallback(t *testing.T) {
	port := os.Getenv("L34_RPC_TIMEOUT_PORT")
	if port == "" {
		port = "23402"
	}
	address := net.JoinHostPort("127.0.0.1", port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() })}
	defer server.Close()
	go server.Serve(listener)
	client, err := ethclient.Dial("http://" + address)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	for _, kind := range []string{"erc1271", "eoa_eip191"} {
		t.Run(kind, func(t *testing.T) {
			var observations []string
			start := time.Now()
			_, err := VerifyProof(context.Background(), NewReportedHashBackend(client.Client()), WalletProof{Kind: kind, Wallet: goldenAddr, Signature: goldenSig}, goldenMsg, func(method, category string, duration time.Duration) {
				if duration <= 0 {
					t.Fatal("verification duration missing")
				}
				observations = append(observations, method+":"+category)
			})
			failure, ok := err.(*VerificationError)
			if !ok || !failure.Retryable || time.Since(start) > 3*time.Second {
				t.Fatal("deadline/retry", err, time.Since(start))
			}
			method := "eip1271"
			if kind == "eoa_eip191" {
				method = "eip191"
			}
			if len(observations) != 1 || observations[0] != method+":upstream" {
				t.Fatal("upstream observation", observations)
			}
			status, code, retryable := PublicCode(err)
			if status != 503 || code != "DEPENDENCY_UNAVAILABLE" || !retryable {
				t.Fatal(status, code, retryable)
			}
		})
	}
}

func TestRealMTLSServiceRequestAssertion(t *testing.T) {
	public, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	template := &x509.Certificate{SerialNumber: big.NewInt(23434), Subject: pkix.Name{CommonName: "identity-service-request"}, NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth}, IPAddresses: []net.IP{net.ParseIP("127.0.0.1")}}
	der, err := x509.CreateCertificate(rand.Reader, template, template, public, key)
	if err != nil {
		t.Fatal(err)
	}
	certificate, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(certificate)
	cert := tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
	hash := sha256.Sum256(der)
	pin := hex.EncodeToString(hash[:])
	transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, Certificates: []tls.Certificate{cert}}}
	defer transport.CloseIdleConnections()
	signer := ServiceRequestIssuer{Issuer: "social", Audience: "identity", Key: key, Transport: AssertionTransport{IdentityAssertionHeader, IdentitySignatureHeader}}
	listener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}, ClientCAs: roots, ClientAuth: tls.RequireAndVerifyClientCert})
	if err != nil {
		t.Fatal(err)
	}
	verifier := ServiceRequestVerifier{Issuer: "social", Audience: "identity", Key: public, PeerCertSHA256: pin, Transport: signer.Transport}
	identity := IdentityPrincipal{AccountID: uuid.NewString(), SessionID: uuid.NewString(), Wallet: strings.ToLower(goldenAddr), AuthEpoch: 2, ProofAt: time.Now().Add(-time.Second).UTC(), AccessExpiresAt: time.Now().Add(time.Hour).UTC()}
	principal := identity.StreamPrincipal()
	binding := StreamTicketBinding{SchemaMajor: 2, Topics: []string{"market.rankings.trending"}, Origin: "https://app.example", Endpoint: "/v2/stream"}
	binding.FilterDigest = StreamFilterDigest(binding.Topics)
	chatBinding := StreamTicketBinding{SchemaMajor: 2, Topics: []string{"private.chat." + principal.Wallet}, Origin: "https://app.example", Endpoint: "/v2/stream"}
	chatBinding.FilterDigest = StreamFilterDigest(chatBinding.Topics)
	server := &http.Server{ReadHeaderTimeout: time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(io.LimitReader(r.Body, (1<<20)+1))
		if err != nil || verifier.Authenticate(r, body) != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/internal/v2/identity/request:authenticate":
			var request IdentityAuthenticationRequest
			if json.Unmarshal(body, &request) != nil || request.Authorization != "Bearer access-token" || request.Method != http.MethodGet {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			_ = json.NewEncoder(w).Encode(identity)
		case "/internal/v2/identity/ticket:consume":
			var request struct {
				Ticket  string              `json:"ticket"`
				Binding StreamTicketBinding `json:"binding"`
			}
			if json.Unmarshal(body, &request) != nil || request.Ticket != strings.Repeat("a", 64) || ValidateStreamBinding(principal, request.Binding) != nil {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			_ = json.NewEncoder(w).Encode(principal)
		case "/internal/v2/identity/chat-ticket:consume":
			var request struct {
				Ticket  string              `json:"ticket"`
				Binding StreamTicketBinding `json:"binding"`
			}
			if json.Unmarshal(body, &request) != nil || request.Ticket != strings.Repeat("ab", 32) || ValidateChatStreamBinding(principal, request.Binding) != nil {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			_ = json.NewEncoder(w).Encode(principal)
		case "/internal/v2/identity/session:check":
			if string(body) == "[]" {
				w.WriteHeader(http.StatusUnprocessableEntity)
				return
			}
			var got StreamPrincipal
			if json.Unmarshal(body, &got) != nil || got != principal {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	})}
	go server.Serve(listener)
	defer server.Close()
	endpoint := "https://" + listener.Addr().String()
	client := &http.Client{Transport: transport, Timeout: 2 * time.Second}
	authority, err := NewMTLSIdentityAuthority(endpoint, pin, client, signer)
	if err != nil {
		t.Fatal(err)
	}
	signedBody := []byte(`{"action":"verify_service_request"}`)
	for _, c := range []struct {
		name, path string
		body       []byte
		sign       bool
		status     int
	}{{"matching", "/internal/v2/identity/test:verify", signedBody, true, 204}, {"altered_body", "/internal/v2/identity/test:verify", []byte(`{"action":"altered"}`), true, 401}, {"altered_path", "/internal/v2/identity/test:other", signedBody, true, 401}, {"unsigned_header_spoof", "/internal/v2/identity/test:verify", signedBody, false, 401}} {
		t.Run(c.name, func(t *testing.T) {
			request, err := http.NewRequest(http.MethodPost, endpoint+"/internal/v2/identity/test:verify", bytes.NewReader(signedBody))
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("X-Request-ID", uuid.NewString())
			if c.sign {
				if err = signer.SignRequest(request, signedBody); err != nil {
					t.Fatal(err)
				}
			} else {
				request.Header.Set("X-Actor-Wallet", goldenAddr)
				request.Header.Set("X-API-Key", "untrusted")
			}
			request.URL.Path = c.path
			request.Body = io.NopCloser(bytes.NewReader(c.body))
			request.ContentLength = int64(len(c.body))
			response, err := client.Do(request)
			if err != nil {
				t.Fatal(err)
			}
			response.Body.Close()
			if response.StatusCode != c.status {
				t.Fatal(response.StatusCode, c.status)
			}
		})
	}
	if err := authority.Probe(context.Background()); err != nil {
		t.Fatalf("authenticated identity channel: %v", err)
	}
	request, err := http.NewRequest(http.MethodGet, "https://app.example/v2/profile", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer access-token")
	gotIdentity, err := authority.AuthenticateRequest(context.Background(), request)
	if err != nil || gotIdentity != identity {
		t.Fatalf("authenticated identity = %+v, %v", gotIdentity, err)
	}
	for _, tc := range []struct {
		name string
		edit func(*http.Request)
	}{
		{"query credential", func(r *http.Request) { r.URL.RawQuery = "accessToken=secret" }},
		{"malformed query", func(r *http.Request) { r.URL.RawQuery = "%ZZ" }},
		{"duplicate authorization", func(r *http.Request) { r.Header.Add("Authorization", "Bearer another") }},
		{"duplicate cookie", func(r *http.Request) {
			r.AddCookie(&http.Cookie{Name: "__Host-kindle_access", Value: "first"})
			r.AddCookie(&http.Cookie{Name: "__Host-kindle_access", Value: "second"})
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := request.Clone(context.Background())
			tc.edit(candidate)
			if _, err := authority.AuthenticateRequest(context.Background(), candidate); !errors.Is(err, ErrScopeDenied) {
				t.Fatalf("request scope: %v", err)
			}
		})
	}
	stream, err := authority.AuthenticateStreamHTTP(context.Background(), "access-token")
	if err != nil || stream != principal {
		t.Fatalf("authenticated stream = %+v, %v", stream, err)
	}
	consumed, err := authority.ConsumeStreamTicket(context.Background(), strings.Repeat("a", 64), binding)
	if err != nil || consumed != principal {
		t.Fatalf("bound ticket = %+v, %v", consumed, err)
	}
	for _, malformed := range []string{"", strings.Repeat("A", 64), strings.Repeat("z", 64)} {
		if _, err := authority.ConsumeStreamTicket(context.Background(), malformed, binding); !errors.Is(err, ErrScopeDenied) {
			t.Fatalf("malformed stream ticket %q: %v", malformed, err)
		}
	}
	if err := authority.CheckStreamSession(context.Background(), principal); err != nil {
		t.Fatalf("active stream session: %v", err)
	}
	invalidPrincipal := principal
	invalidPrincipal.AccountID = "invalid"
	if err := authority.CheckStreamSession(context.Background(), invalidPrincipal); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("invalid stream account: %v", err)
	}
	chat, err := authority.ConsumeChatStreamTicket(context.Background(), strings.Repeat("ab", 32), chatBinding)
	if err != nil || chat != principal {
		t.Fatalf("bound chat ticket = %+v, %v", chat, err)
	}
	for _, malformed := range []string{"", strings.Repeat("AB", 32), strings.Repeat("zz", 32)} {
		if _, err := authority.ConsumeChatStreamTicket(context.Background(), malformed, chatBinding); !errors.Is(err, ErrScopeDenied) {
			t.Fatalf("malformed chat ticket %q: %v", malformed, err)
		}
	}
}
