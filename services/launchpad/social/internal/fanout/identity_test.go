package fanout

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
)

func TestRawForgedActorHeaderNeverAuthenticates(t *testing.T) {
	h := &Hub{}
	req := httptest.NewRequest("GET", "/ws", nil)
	req.Header.Set("X-Actor-Wallet", "0x00000000000000000000000000000000000000aa")
	req.Header.Set("X-Admin", "true")
	out := httptest.NewRecorder()
	h.ServeWS(out, req)
	if out.Code != 401 {
		t.Fatal(out.Code)
	}
}
func TestPrincipalExpiryAndRevocationFailClosed(t *testing.T) {
	now := time.Now()
	p := Principal{Wallet: "0x00000000000000000000000000000000000000aa", SessionID: "fixture", ExpiresAt: now.Add(time.Minute), Validate: func(context.Context) error { return nil }}
	if !p.valid(context.Background(), now) {
		t.Fatal("valid fixture rejected")
	}
	if p.valid(context.Background(), now.Add(time.Minute)) {
		t.Fatal("expired fixture accepted")
	}
	p.Validate = func(context.Context) error { return errors.New("revoked") }
	if p.valid(context.Background(), now) {
		t.Fatal("revoked fixture accepted")
	}
	p.Validate = nil
	if p.valid(context.Background(), now) {
		t.Fatal("missing validator accepted")
	}
}

func TestContextPrincipalRequiresLiveSession(t *testing.T) {
	if _, ok := PrincipalFromContext(nil); ok {
		t.Fatal("nil context exposed a stream principal")
	}
	if _, ok := PrincipalFromContext(context.Background()); ok {
		t.Fatal("anonymous context exposed a stream principal")
	}
	p := Principal{Wallet: "0x00000000000000000000000000000000000000aa", SessionID: "stream-context", ExpiresAt: time.Now().Add(time.Minute), Validate: func(context.Context) error { return nil }}
	if got, ok := PrincipalFromContext(WithPrincipal(context.Background(), p)); !ok || got.Wallet != p.Wallet || got.SessionID != p.SessionID {
		t.Fatalf("verified stream principal = %+v, present %t", got, ok)
	}
	p.Validate = func(context.Context) error { return errors.New("revoked") }
	if _, ok := PrincipalFromContext(WithPrincipal(context.Background(), p)); ok {
		t.Fatal("revoked stream principal remained active")
	}
	request := httptest.NewRequest("GET", "/v2/stream", nil)
	if hasSessionCredential(request) {
		t.Fatal("anonymous stream carried a session credential")
	}
	request.Header.Set("Authorization", "Bearer verified")
	if !hasSessionCredential(request) {
		t.Fatal("authorization credential was not recognized")
	}
	request.Header.Del("Authorization")
	request.AddCookie(&http.Cookie{Name: "__Host-kindle_access", Value: "verified"})
	if !hasSessionCredential(request) {
		t.Fatal("host access cookie was not recognized")
	}
}

func TestVerifiedRequestAuthenticatorRejectsUnboundIdentityAuthority(t *testing.T) {
	authenticate := VerifiedRequestAuthenticator(sharedauth.HTTPIdentityAuthority{})
	request := httptest.NewRequest(http.MethodGet, "/v2/chat/history", nil)
	request.Header.Set("Authorization", "Bearer unbound-session")
	if _, err := authenticate(request); err == nil {
		t.Fatal("unbound identity authority accepted a chat request")
	}
}
