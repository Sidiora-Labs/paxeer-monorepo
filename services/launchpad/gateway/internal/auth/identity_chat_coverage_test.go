package auth

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	sharedauth "github.com/Sidiora-Technologies/KindleLaunch/shared/auth"
)

func TestIdentityChatTicketPrivateRoute(t *testing.T) {
	f := ownerHTTPFixture(t)
	credentials, principal := f.loginHTTP(t)
	topics := []string{"private.chat." + principal.Wallet()}
	binding := sharedauth.StreamTicketBinding{SchemaMajor: 2, Topics: topics, FilterDigest: sharedauth.StreamFilterDigest(topics), Origin: "https://kindle.test", Endpoint: "/v2/stream"}
	if !chatBinding(binding) || chatBinding(sharedauth.StreamTicketBinding{Topics: []string{"market.price"}}) {
		t.Fatal("chat topic classification changed")
	}
	ticket, err := f.s.IssueChatStreamTicket(f.ctx, principal, binding)
	if err != nil || len(ticket) != 64 {
		t.Fatalf("issued chat ticket: length=%d err=%v", len(ticket), err)
	}
	request := map[string]any{"ticket": ticket, "binding": binding}
	path := "/internal/v2/identity/chat-ticket:consume"
	status, body, _ := f.request(t, http.MethodPost, path, credentials.AccessToken, "", request)
	if status != http.StatusUnauthorized {
		t.Fatalf("browser token reached private chat route: status=%d body=%s", status, body)
	}
	serviceToken := strings.Repeat("t", 32)
	status, body, headers := f.request(t, http.MethodPost, path, serviceToken, "", request)
	var got sharedauth.StreamPrincipal
	if status != http.StatusOK || json.Unmarshal(body, &got) != nil || got.SessionID != principal.SessionID() || got.Wallet != principal.Wallet() || headers.Get("Cache-Control") != "no-store" {
		t.Fatalf("private chat consume: status=%d principal=%+v body=%s", status, got, body)
	}
	status, body, _ = f.request(t, http.MethodPost, path, serviceToken, "", request)
	if status != http.StatusUnauthorized {
		t.Fatalf("replayed chat ticket: status=%d body=%s", status, body)
	}
	status, body, _ = f.request(t, http.MethodPost, path, serviceToken, "", map[string]any{"ticket": ticket, "binding": binding, "unknown": true})
	if status != http.StatusUnprocessableEntity {
		t.Fatalf("unknown chat field: status=%d body=%s", status, body)
	}
}
