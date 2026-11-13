package account

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/google/uuid"
)

func TestGatewayOwnerReadUsesSignedIdentity(t *testing.T) {
	s := connectedFixture(t)
	owner := owner(t, s, 8361)
	wallet := "0x00000000000000000000000000000000000020a9"
	session := uuid.New()
	ctx := context.Background()
	if _, err := s.DB.Exec(ctx, `INSERT INTO identity_accounts VALUES($1,1,clock_timestamp())`, owner.AccountID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO identity_wallet_links VALUES($1,$2,125,$3,'private','verified',clock_timestamp())`, uuid.New(), owner.AccountID, wallet); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO identity_sessions(session_id,account_id,wallet,auth_epoch,access_hash,csrf_hash,issued_at,access_expires_at,refresh_expires_at,reauthenticated_at,last_seen_at,device_label)
		VALUES($1,$2,$3,1,$4,$5,clock_timestamp(),clock_timestamp()+interval '1 hour',clock_timestamp()+interval '1 day',clock_timestamp(),clock_timestamp(),'gateway')`, session, owner.AccountID, wallet, []byte(uuid.NewString()), []byte(uuid.NewString())); err != nil {
		t.Fatal(err)
	}
	resolver := sharedauth.PostgresIdentityResolver{Pool: s.DB}
	identity, err := resolver.Resolve(ctx, sharedauth.Principal{Wallet: wallet, SessionID: session.String()})
	if err != nil {
		t.Fatal(err)
	}
	const identityToken = "identity-service-token-for-account-gateway"
	identityServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/internal/v2/identity/request:authenticate" || r.Header.Get("Authorization") != "Bearer "+identityToken {
			http.NotFound(w, r)
			return
		}
		_ = json.NewEncoder(w).Encode(identity)
	}))
	defer identityServer.Close()
	pki := testpki.New(t)
	serverLeaf := pki.Leaf("user", "user.test")
	clientLeaf := pki.Leaf("gateway", "gateway.test")
	publicKey, privateKey, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	transport := sharedauth.AssertionTransport{AssertionHeader: "X-Test-Assertion", SignatureHeader: "X-Test-Signature"}
	issuer := sharedauth.ServiceAssertionIssuer{Issuer: "gateway", Audience: "user", Key: privateKey, Transport: transport}
	verifier := sharedauth.ServiceAssertionVerifier{Issuer: "gateway", Audience: "user", PeerCertSHA256: clientLeaf.SHA256, Key: publicKey, Transport: transport}
	runtime := &Runtime{IdentityAuthority: sharedauth.HTTPIdentityAuthority{Endpoint: identityServer.URL, ServiceToken: identityToken, AllowPlainHTTP: true}}
	server := httptest.NewUnstartedServer((HTTP{Store: s, Runtime: runtime}).GatewayHandler(verifier, resolver, sharedauth.OperationAuthorizer{}))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{serverLeaf.Pair}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.Roots()}
	server.StartTLS()
	defer server.Close()
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: pki.Roots(), Certificates: []tls.Certificate{clientLeaf.Pair}}}}
	defer client.CloseIdleConnections()
	request := func(method, path, body string, roles []string) (int, []byte) {
		t.Helper()
		r, err := http.NewRequestWithContext(ctx, method, server.URL+path, strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		r.Header.Set("X-Request-ID", uuid.NewString())
		r.Header.Set("Authorization", "Bearer browser-session")
		if err := issuer.SignRequest(r, identity, roles, []byte(body)); err != nil {
			t.Fatal(err)
		}
		response, err := client.Do(r)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		raw, err := io.ReadAll(response.Body)
		if err != nil {
			t.Fatal(err)
		}
		return response.StatusCode, raw
	}
	if status, raw := request(http.MethodGet, "/v1/me/preferences", "", []string{"owner"}); status != http.StatusOK || !strings.Contains(string(raw), `"preferences"`) {
		t.Fatalf("authenticated preferences read = %d %s", status, raw)
	}
	if status, raw := request(http.MethodGet, "/v1/me/preferences", "", []string{"viewer"}); status != http.StatusForbidden {
		t.Fatalf("non-owner preferences read = %d %s", status, raw)
	}
	if status, raw := request(http.MethodPatch, "/v1/me/preferences", `{"expectedRevision":"0","locale":"fr"}`, []string{"owner"}); status != http.StatusForbidden {
		t.Fatalf("preference write without operation proof = %d %s", status, raw)
	}
	service := (HTTP{Store: s, Runtime: runtime}).ServiceHandler(runtime.IdentityAuthority)
	serviceRequest := httptest.NewRequest(http.MethodGet, "/v1/me/preferences", nil)
	serviceRequest.Header.Set("Authorization", "Bearer browser-session")
	serviceResponse := httptest.NewRecorder()
	service.ServeHTTP(serviceResponse, serviceRequest)
	if serviceResponse.Code != http.StatusOK || !strings.Contains(serviceResponse.Body.String(), `"preferences"`) {
		t.Fatalf("identity service preferences read = %d %s", serviceResponse.Code, serviceResponse.Body.String())
	}
	withoutIdentity := (HTTP{Store: s}).ServiceHandler(nil)
	private := httptest.NewRecorder()
	withoutIdentity.ServeHTTP(private, httptest.NewRequest(http.MethodGet, "/v1/me/preferences", nil))
	if private.Code != http.StatusServiceUnavailable {
		t.Fatalf("private read without identity authority = %d %s", private.Code, private.Body.String())
	}
	public := httptest.NewRecorder()
	withoutIdentity.ServeHTTP(public, httptest.NewRequest(http.MethodGet, "/v1/profiles/"+wallet, nil))
	if public.Code != http.StatusOK || !strings.Contains(public.Body.String(), wallet) {
		t.Fatalf("public profile read = %d %s", public.Code, public.Body.String())
	}
	if status, raw := request(http.MethodPost, "/v1/me/unknown", `{}`, []string{"owner"}); status != http.StatusForbidden {
		t.Fatalf("unknown owner action = %d %s", status, raw)
	}
	identityServer.Close()
	if status, raw := request(http.MethodGet, "/v1/me/preferences", "", []string{"owner"}); status != http.StatusServiceUnavailable {
		t.Fatalf("identity authority unavailable = %d %s", status, raw)
	}
}
