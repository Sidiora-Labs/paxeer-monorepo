package app

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/google/uuid"

	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/media/metadata/internal/metadata"
)

func TestPrivateChannelBindsGatewayAssertionToSession(t *testing.T) {
	ctx := context.Background()
	_, dsn := internaltest.NewPostgresURL(t)
	redisURL := internaltest.NewRedisURL(t)
	objects := internaltest.NewMinIO(t, "metadata-private-channel")
	gateway := internaltest.NewGateway(t, dsn, redisURL, objects, "https://app.sidiora.test")
	wallet, bearer := gateway.SignIn(t)
	identity := sharedauth.HTTPIdentityAuthority{Endpoint: gateway.URL, ServiceToken: gateway.ServiceToken, AllowPlainHTTP: true}
	authentication := httptest.NewRequest(http.MethodGet, "/v2/tokens/example/metadata", nil)
	authentication.Header.Set("Authorization", "Bearer "+bearer)
	authentication.Header.Set("Origin", "https://app.sidiora.test")
	principal, err := identity.AuthenticateRequest(ctx, authentication)
	if err != nil || principal.Wallet != wallet {
		t.Fatalf("gateway session: %+v %v", principal, err)
	}
	peer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	defer peer.Close()
	certificate := peer.Certificate()
	certHash := sha256.Sum256(certificate.Raw)
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	cfg := config.Config{GatewayPublicKeyHex: hex.EncodeToString(publicKey), GatewayPeerCertSHA256: hex.EncodeToString(certHash[:])}
	called := 0
	channel := metadataPrivateChannel(cfg, identity)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		called++
		w.WriteHeader(http.StatusNoContent)
	}))
	issuer := sharedauth.ServiceAssertionIssuer{Issuer: "gateway", Audience: "metadata", Key: privateKey, Transport: sharedauth.AssertionTransport{AssertionHeader: "X-Internal-Assertion", SignatureHeader: "X-Internal-Signature"}}
	path := "/v3/tokens/0x" + "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" + "/metadata-publications"
	request := func(body []byte) *http.Request {
		t.Helper()
		r := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		r.Header.Set("Authorization", "Bearer "+bearer)
		r.Header.Set("Origin", "https://app.sidiora.test")
		r.Header.Set("X-Request-ID", uuid.NewString())
		r.TLS = &tls.ConnectionState{Version: tls.VersionTLS13, PeerCertificates: []*x509.Certificate{certificate}, VerifiedChains: [][]*x509.Certificate{{certificate}}}
		if err := issuer.SignRequest(r, principal, nil, body); err != nil {
			t.Fatal(err)
		}
		return r
	}
	valid := httptest.NewRecorder()
	channel.ServeHTTP(valid, request([]byte(`{"draft":1}`)))
	if valid.Code != http.StatusNoContent || called != 1 {
		t.Fatalf("signed gateway session: %d called %d %s", valid.Code, called, valid.Body.String())
	}
	changed := request([]byte(`{"draft":1}`))
	changed.Body = io.NopCloser(bytes.NewReader([]byte(`{"draft":2}`)))
	invalid := httptest.NewRecorder()
	channel.ServeHTTP(invalid, changed)
	if invalid.Code != http.StatusForbidden || called != 1 {
		t.Fatalf("changed body: %d called %d", invalid.Code, called)
	}
	missingSession := request([]byte(`{"draft":1}`))
	missingSession.Header.Set("Authorization", "Bearer unavailable")
	unavailable := httptest.NewRecorder()
	channel.ServeHTTP(unavailable, missingSession)
	if unavailable.Code == http.StatusNoContent || called != 1 {
		t.Fatalf("unavailable session: %d called %d", unavailable.Code, called)
	}
	for _, tc := range []struct {
		name   string
		change func(*http.Request)
		want   int
	}{
		{"missing assertion", func(r *http.Request) { r.Header.Del("X-Internal-Assertion") }, http.StatusForbidden},
		{"invalid assertion encoding", func(r *http.Request) { r.Header.Set("X-Internal-Assertion", "!") }, http.StatusForbidden},
		{"invalid assertion document", func(r *http.Request) {
			r.Header.Set("X-Internal-Assertion", base64.RawURLEncoding.EncodeToString([]byte("{")))
		}, http.StatusForbidden},
		{"noncanonical assertion", func(r *http.Request) {
			raw, err := base64.RawURLEncoding.DecodeString(r.Header.Get("X-Internal-Assertion"))
			if err != nil {
				t.Fatal(err)
			}
			r.Header.Set("X-Internal-Assertion", base64.RawURLEncoding.EncodeToString(append(raw, ' ')))
		}, http.StatusForbidden},
		{"invalid signature encoding", func(r *http.Request) { r.Header.Set("X-Internal-Signature", "!") }, http.StatusForbidden},
		{"changed signature", func(r *http.Request) {
			r.Header.Set("X-Internal-Signature", base64.RawURLEncoding.EncodeToString(bytes.Repeat([]byte{0}, ed25519.SignatureSize)))
		}, http.StatusForbidden},
		{"publication body limit", func(r *http.Request) {
			r.Body = io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("x"), metadata.PublicationBodyLimit+1)))
		}, http.StatusRequestEntityTooLarge},
		{"asset read protected", func(r *http.Request) {
			r.Method = http.MethodGet
			r.URL.Path = "/v2/tokens/example/assets/" + uuid.NewString()
			r.Header.Del("X-Internal-Assertion")
		}, http.StatusForbidden},
		{"asset body limit", func(r *http.Request) {
			r.Method = http.MethodPut
			r.URL.Path = "/v2/tokens/example/assets/" + uuid.NewString()
			r.Body = io.NopCloser(bytes.NewReader(bytes.Repeat([]byte("x"), (6<<20)+1)))
		}, http.StatusRequestEntityTooLarge},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := request([]byte(`{"draft":1}`))
			tc.change(r)
			response := httptest.NewRecorder()
			channel.ServeHTTP(response, r)
			if response.Code != tc.want || !strings.Contains(response.Body.String(), "media_channel_unavailable") || called != 1 {
				t.Fatalf("private channel refusal = %d, called %d, body %s", response.Code, called, response.Body.String())
			}
		})
	}
	otherSession := request([]byte(`{"draft":1}`))
	changedPrincipal := principal
	changedPrincipal.SessionID = uuid.NewString()
	if err := issuer.SignRequest(otherSession, changedPrincipal, nil, []byte(`{"draft":1}`)); err != nil {
		t.Fatal(err)
	}
	otherSessionResponse := httptest.NewRecorder()
	channel.ServeHTTP(otherSessionResponse, otherSession)
	if otherSessionResponse.Code != http.StatusForbidden || called != 1 {
		t.Fatalf("session mismatch = %d, called %d", otherSessionResponse.Code, called)
	}
	wrongPeer := cfg
	wrongPeer.GatewayPeerCertSHA256 = strings.Repeat("0", 64)
	wrongPeerResponse := httptest.NewRecorder()
	metadataPrivateChannel(wrongPeer, identity)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called++ })).ServeHTTP(wrongPeerResponse, request([]byte(`{"draft":1}`)))
	if wrongPeerResponse.Code != http.StatusForbidden || called != 1 {
		t.Fatalf("peer pin mismatch = %d, called %d", wrongPeerResponse.Code, called)
	}
}
