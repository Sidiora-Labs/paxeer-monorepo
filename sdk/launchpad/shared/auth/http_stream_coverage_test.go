package auth

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPStreamAuthorityConsumesBoundTicket(t *testing.T) {
	principal := StreamPrincipal{AccountID: "6f9b1a52-7c0e-4b8e-9d4a-1f2e3d4c5b6a", SessionID: "0d7c2e41-5a9b-4c3d-8e1f-2a3b4c5d6e7f", Wallet: "0x1dae78b53c9f39930a2ff2b6b7aac12ec75fb154", AuthEpoch: 3}
	identity := IdentityPrincipal{AccountID: principal.AccountID, SessionID: principal.SessionID, Wallet: principal.Wallet, AuthEpoch: principal.AuthEpoch, ProofAt: time.Now().Add(-time.Second), AccessExpiresAt: time.Now().Add(time.Minute)}
	binding := StreamTicketBinding{SchemaMajor: 2, Topics: []string{"private.notifications." + principal.Wallet}, Origin: "https://app.example", Endpoint: "/v2/stream"}
	binding.FilterDigest = StreamFilterDigest(binding.Topics)
	var consumed, checked, authenticated atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Header.Get("Authorization") != "Bearer "+strings.Repeat("s", 32) || r.Header.Get("Content-Type") != "application/json" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		switch r.URL.Path {
		case "/internal/v2/identity/request:authenticate":
			var request IdentityAuthenticationRequest
			if json.NewDecoder(r.Body).Decode(&request) != nil || request.Authorization != "Bearer access-token" || request.Method != http.MethodGet {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			authenticated.Add(1)
			_ = json.NewEncoder(w).Encode(identity)
		case "/internal/v2/identity/ticket:consume":
			var request struct {
				Ticket  string              `json:"ticket"`
				Binding StreamTicketBinding `json:"binding"`
			}
			if json.NewDecoder(r.Body).Decode(&request) != nil || request.Ticket != strings.Repeat("a", 64) || ValidateStreamBinding(principal, request.Binding) != nil {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			consumed.Add(1)
			_ = json.NewEncoder(w).Encode(principal)
		case "/internal/v2/identity/session:check":
			var got StreamPrincipal
			if json.NewDecoder(r.Body).Decode(&got) != nil || got != principal {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			checked.Add(1)
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()
	authority := HTTPStreamAuthority{Endpoint: server.URL, ServiceToken: strings.Repeat("s", 32), Client: server.Client()}
	actual, err := authority.ConsumeStreamTicket(context.Background(), strings.Repeat("a", 64), binding)
	if err != nil || actual != principal || consumed.Load() != 1 {
		t.Fatalf("bound stream ticket: %+v, %d, %v", actual, consumed.Load(), err)
	}
	if err := authority.CheckStreamSession(context.Background(), actual); err != nil || checked.Load() != 1 {
		t.Fatalf("live stream session: %d, %v", checked.Load(), err)
	}
	request := httptest.NewRequest(http.MethodGet, "https://app.example/v2/profile", nil)
	request.Header.Set("Authorization", "Bearer access-token")
	bound, err := HTTPIdentityAuthority(authority).AuthenticateRequest(context.Background(), request)
	if err != nil || bound.AccountID != identity.AccountID || bound.StreamPrincipal() != principal || authenticated.Load() != 1 {
		t.Fatalf("authenticated request: %+v, %d, %v", bound, authenticated.Load(), err)
	}
	stream, err := authority.AuthenticateStreamHTTP(context.Background(), "access-token")
	if err != nil || stream != principal || authenticated.Load() != 2 {
		t.Fatalf("authenticated stream: %+v, %d, %v", stream, authenticated.Load(), err)
	}
	queryToken := httptest.NewRequest(http.MethodGet, "https://app.example/v2/profile?accessToken=untrusted", nil)
	if _, err := HTTPIdentityAuthority(authority).AuthenticateRequest(context.Background(), queryToken); !errors.Is(err, ErrScopeDenied) || authenticated.Load() != 2 {
		t.Fatalf("query token reached identity authority: %d, %v", authenticated.Load(), err)
	}
	changed := binding
	changed.Topics = []string{"private.orders.0x1111111111111111111111111111111111111111"}
	if _, err := authority.ConsumeStreamTicket(context.Background(), strings.Repeat("a", 64), changed); err == nil || consumed.Load() != 1 {
		t.Fatalf("unbound topic consumed a ticket: %d, %v", consumed.Load(), err)
	}
	denied := authority
	denied.ServiceToken = strings.Repeat("x", 32)
	if err := denied.CheckStreamSession(context.Background(), principal); err == nil || !errors.Is(err, ErrScopeDenied) || checked.Load() != 1 {
		t.Fatalf("unauthorized stream check: %d, %v", checked.Load(), err)
	}
}
