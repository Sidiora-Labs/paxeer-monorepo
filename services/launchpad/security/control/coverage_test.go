package control

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestLocalCredentialLifecycleAndIsolation(t *testing.T) {
	authority := NewLocalCredentialAuthority()
	reader := authority.Reader()
	credential := strings.Repeat("c", 32)
	principal := Principal{ID: id(), CredentialID: id(), Role: "infrastructure", Scopes: []string{"service"}}
	expires := time.Now().Add(time.Hour)
	if err := authority.Grant(credential, principal, expires); err != nil {
		t.Fatal(err)
	}
	principal.Scopes[0] = "other"
	resolved, err := reader.Resolve(context.Background(), credential)
	if err != nil || len(resolved.Scopes) != 1 || resolved.Scopes[0] != "service" {
		t.Fatalf("credential scope changed after grant: %+v, %v", resolved, err)
	}
	resolved.Scopes[0] = "other"
	resolved, err = reader.Resolve(context.Background(), credential)
	if err != nil || resolved.Scopes[0] != "service" || !reader.Active(context.Background(), resolved) {
		t.Fatalf("credential scope changed after read: %+v, %v", resolved, err)
	}
	if err := authority.Grant(credential, resolved, expires); !errors.Is(err, Conflict) {
		t.Fatalf("duplicate credential: %v", err)
	}
	if err := reader.RevokeIdentity(context.Background(), resolved.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Resolve(context.Background(), credential); !errors.Is(err, Denied) || reader.Active(context.Background(), resolved) {
		t.Fatalf("revoked credential remained active: %v", err)
	}
	if err := authority.Grant(strings.Repeat("d", 32), resolved, expires); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.Resolve(context.Background(), strings.Repeat("d", 32)); err != nil {
		t.Fatalf("new credential after revocation: %v", err)
	}
}

func TestLocalCredentialRejectsInvalidAuthority(t *testing.T) {
	authority := NewLocalCredentialAuthority()
	valid := Principal{ID: id(), CredentialID: id(), Role: "support", Scopes: []string{"service"}}
	for _, sample := range []struct {
		name       string
		credential string
		principal  Principal
		expires    time.Time
	}{
		{"short credential", "short", valid, time.Now().Add(time.Hour)},
		{"expired", strings.Repeat("a", 32), valid, time.Now().Add(-time.Hour)},
		{"unknown role", strings.Repeat("a", 32), Principal{ID: id(), CredentialID: id(), Role: "owner", Scopes: []string{"service"}}, time.Now().Add(time.Hour)},
		{"invalid scope", strings.Repeat("a", 32), Principal{ID: id(), CredentialID: id(), Role: "support", Scopes: []string{"bad scope"}}, time.Now().Add(time.Hour)},
		{"user without session", strings.Repeat("a", 32), Principal{ID: id(), CredentialID: id(), Role: "user", Scopes: []string{"service"}}, time.Now().Add(time.Hour)},
	} {
		t.Run(sample.name, func(t *testing.T) {
			if err := authority.Grant(sample.credential, sample.principal, sample.expires); !errors.Is(err, Invalid) {
				t.Fatalf("invalid grant: %v", err)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := authority.Reader().Resolve(ctx, strings.Repeat("a", 32)); !errors.Is(err, Denied) {
		t.Fatalf("cancelled resolve: %v", err)
	}
	if authority.Reader().Active(ctx, valid) {
		t.Fatal("cancelled active check succeeded")
	}
	if err := authority.Reader().RevokeIdentity(ctx, valid.ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled revoke: %v", err)
	}
}

func TestControlHTTPRejectsAmbiguousRequests(t *testing.T) {
	for _, sample := range []struct {
		name, contentType, body string
	}{
		{"missing content type", "", `{"key":"first"}`},
		{"oversized body", "application/json", strings.Repeat("x", 65537)},
	} {
		request := httptest.NewRequest(http.MethodPost, "/v2/control/actions", strings.NewReader(sample.body))
		request.Header.Set("Content-Type", sample.contentType)
		var action ActionRequest
		if err := decodeRequest(request, &action); !errors.Is(err, Invalid) {
			t.Fatalf("%s: %v", sample.name, err)
		}
	}
	for _, body := range []string{`{"key":"first","key":"second"}`, `{"unknown":true}`, `{"key":"first"} {"key":"second"}`} {
		request := httptest.NewRequest(http.MethodPost, "/v2/control/actions", strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		var action ActionRequest
		if err := decodeRequest(request, &action); !errors.Is(err, Invalid) {
			t.Fatalf("ambiguous request %q: %v", body, err)
		}
	}
	request := httptest.NewRequest(http.MethodPost, "/v2/control/actions", strings.NewReader(`{"key":"first"}`))
	request.Header.Set("Content-Type", "application/json")
	var action ActionRequest
	if err := decodeRequest(request, &action); err != nil || action.Key != "first" {
		t.Fatalf("canonical request: %+v, %v", action, err)
	}
	for _, authorization := range []string{"", "Basic opaque", "Bearer one two", "Bearer " + strings.Repeat("x", 4097)} {
		request.Header.Set("Authorization", authorization)
		if _, err := credential(request); !errors.Is(err, Denied) {
			t.Fatalf("authorization %q: %v", authorization, err)
		}
	}
	request.Header.Set("Authorization", "Bearer opaque")
	if value, err := credential(request); err != nil || value != "opaque" {
		t.Fatalf("bearer credential: %q, %v", value, err)
	}
}
