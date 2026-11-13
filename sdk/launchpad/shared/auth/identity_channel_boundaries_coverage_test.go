package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/tls"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/google/uuid"
)

func TestPinnedIdentityChannelResponseAdmission(t *testing.T) {
	pki := testpki.New(t)
	serverLeaf, clientLeaf := pki.Leaf("identity", "localhost"), pki.Leaf("consumer", "localhost")
	key := testpki.Ed25519(t)
	signer := ServiceRequestIssuer{Issuer: "consumer", Audience: "identity", Key: key.Private, Transport: AssertionTransport{IdentityAssertionHeader, IdentitySignatureHeader}}
	verifier := ServiceRequestVerifier{Issuer: signer.Issuer, Audience: signer.Audience, Key: key.Private.Public().(ed25519.PublicKey), PeerCertSHA256: clientLeaf.SHA256, Transport: signer.Transport}
	principal := IdentityPrincipal{AccountID: uuid.NewString(), SessionID: uuid.NewString(), Wallet: strings.ToLower(goldenAddr), AuthEpoch: 1, ProofAt: time.Now(), AccessExpiresAt: time.Now().Add(time.Hour)}
	valid, _ := json.Marshal(principal)
	stream, _ := json.Marshal(principal.StreamPrincipal())
	for _, tc := range []struct {
		name, wire string
		status     int
	}{
		{"valid", string(valid), 200}, {"stream", string(stream), 200},
		{"empty", "", 200}, {"duplicate", `{"accountId":"a","accountId":"b"}`, 200},
		{"unknown", `{"extra":true}`, 200}, {"wrong type", `[]`, 200}, {"trailing", string(valid) + ` {}`, 200},
		{"oversize", strings.Repeat(" ", 16385), 200}, {"invalid principal", `{}`, 200},
		{"bad request", `{}`, 400}, {"unauthorized", `{}`, 401}, {"forbidden", `{}`, 403}, {"rate limited", `{}`, 429}, {"unavailable", `{}`, 503}, {"unexpected", `{}`, 500},
	} {
		t.Run(tc.name, func(t *testing.T) {
			listener := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				raw, err := io.ReadAll(r.Body)
				if err != nil || verifier.Authenticate(r, raw) != nil {
					t.Error("identity request is not authenticated")
					w.WriteHeader(401)
					return
				}
				w.WriteHeader(tc.status)
				_, _ = io.WriteString(w, tc.wire)
			}))
			listener.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{serverLeaf.Pair}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.Roots()}
			listener.StartTLS()
			defer listener.Close()
			transport := &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), Certificates: []tls.Certificate{clientLeaf.Pair}}}
			defer transport.CloseIdleConnections()
			authority, err := NewMTLSIdentityAuthority(listener.URL, serverLeaf.SHA256, &http.Client{Transport: transport}, signer)
			if err != nil {
				t.Fatal(err)
			}
			ctx := context.Background()
			if tc.status != 200 {
				_, err := authority.AuthenticateStreamHTTP(ctx, "token")
				var denial *IdentityAuthorityError
				want := tc.status
				if want == 500 {
					want = 503
				}
				if !errors.As(err, &denial) || denial.StatusCode() != want || denial.Error() == "" {
					t.Fatalf("identity status: %v", err)
				}
				return
			}
			binding := StreamTicketBinding{SchemaMajor: 2, Endpoint: "/v2/stream", Origin: "https://app.example", Topics: []string{"private.chat." + principal.Wallet}}
			binding.FilterDigest = StreamFilterDigest(binding.Topics)
			if tc.name == "valid" {
				got, err := authority.AuthenticateStreamHTTP(ctx, "Bearer token")
				if err != nil || got != principal.StreamPrincipal() {
					t.Fatalf("identity principal: %+v %v", got, err)
				}
				req := httptest.NewRequest("GET", "https://app.example/", nil)
				req.AddCookie(&http.Cookie{Name: "__Host-kindle_access", Value: "access"})
				if got, err := authority.AuthenticateRequest(ctx, req); err != nil || got.AccountID != principal.AccountID {
					t.Fatalf("cookie identity: %+v %v", got, err)
				}
			} else if tc.name == "stream" {
				for index, consume := range []func(context.Context, string, StreamTicketBinding) (StreamPrincipal, error){authority.ConsumeStreamTicket, authority.ConsumeChatStreamTicket} {
					admitted := binding
					if index == 0 {
						admitted.Topics = []string{"private.notifications." + principal.Wallet}
						admitted.FilterDigest = StreamFilterDigest(admitted.Topics)
					}
					got, err := consume(ctx, strings.Repeat("a", 64), admitted)
					if err != nil || got != principal.StreamPrincipal() {
						t.Fatalf("ticket principal: %+v %v", got, err)
					}
				}
				if err := authority.CheckStreamSession(ctx, principal.StreamPrincipal()); err != nil {
					t.Fatal(err)
				}
			} else {
				if _, err := authority.AuthenticateStreamHTTP(ctx, "token"); !errors.Is(err, ErrScopeDenied) {
					t.Fatalf("identity response admitted: %v", err)
				}
				if _, err := authority.ConsumeStreamTicket(ctx, strings.Repeat("a", 64), binding); !errors.Is(err, ErrScopeDenied) {
					t.Fatalf("stream response admitted: %v", err)
				}
				if _, err := authority.ConsumeChatStreamTicket(ctx, strings.Repeat("a", 64), binding); !errors.Is(err, ErrScopeDenied) {
					t.Fatalf("chat response admitted: %v", err)
				}
			}
			if err := authority.call(ctx, "/internal/v2/identity/session:check", make(chan int), nil); err == nil {
				t.Fatal("non-JSON identity request admitted")
			}
			for _, path := range []string{"/other", "/internal/v2/identity/a?query", "/internal/v2/identity/a%2fb"} {
				if _, _, err := authority.post(ctx, path, []byte(`{}`)); !errors.Is(err, ErrScopeDenied) {
					t.Fatalf("identity path admitted: %v", err)
				}
			}
			denied := authority
			denied.PeerCertSHA256 = strings.Repeat("0", 64)
			if _, err := denied.AuthenticateStreamHTTP(ctx, "token"); err == nil {
				t.Fatal("unpinned identity server admitted")
			}
			prior := authority.Client.Transport.(*http.Transport).Clone()
			prior.TLSClientConfig = prior.TLSClientConfig.Clone()
			prior.TLSClientConfig.VerifyConnection = func(tls.ConnectionState) error { return ErrScopeDenied }
			denied.Client = &http.Client{Transport: prior}
			denied.PeerCertSHA256 = serverLeaf.SHA256
			if _, err := denied.AuthenticateStreamHTTP(ctx, "token"); err == nil {
				t.Fatal("rejected identity certificate admitted")
			}
		})
	}
}
