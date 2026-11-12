package auth

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestHTTPAssertionAdmissionClassifiesBoundInputs(t *testing.T) {
	for _, raw := range [][]byte{nil, []byte(`{"issuer":"one","issuer":"two"}`), []byte(`{} {}`)} {
		if err := decodeAuthorizationJSON(raw, new(ServiceAssertion)); !errors.Is(err, ErrScopeDenied) {
			t.Fatalf("unbound authorization JSON = %v", err)
		}
	}
	if value, err := authorizationHeader(nil, "X-Assertion"); value != "" || !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("nil authorization request = %q, %v", value, err)
	}
	request := &http.Request{Header: make(http.Header), URL: &url.URL{Path: "/v1/action"}, TLS: &tls.ConnectionState{Version: tls.VersionTLS13}, Method: http.MethodPost}
	if value, err := authorizationHeader(request, "X-Assertion"); value != "" || !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("missing authorization header = %q, %v", value, err)
	}
	if value, err := authorizationHeader(request, "bad header"); value != "" || !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("invalid authorization header name = %q, %v", value, err)
	}
	public, private, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	categories := []string{}
	verifier := ServiceAssertionVerifier{Transport: AssertionTransport{"X-Assertion", "X-Signature"}, Issuer: "gateway", Audience: "account", Key: public, Observe: func(category string) { categories = append(categories, category) }}
	for _, tc := range []struct {
		name, issuer, audience string
		expiry                 int64
		category               string
	}{
		{"issuer", "other", "account", time.Now().Add(time.Minute).Unix(), "assertion_issuer"},
		{"audience", "gateway", "other", time.Now().Add(time.Minute).Unix(), "assertion_audience"},
		{"expiry", "gateway", "account", time.Now().Add(-time.Minute).Unix(), "assertion_expired"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assertion := ServiceAssertion{Version: 1, Issuer: tc.issuer, Audience: tc.audience, ExpiresAt: tc.expiry}
			raw, err := json.Marshal(assertion)
			if err != nil {
				t.Fatal(err)
			}
			request.Header.Set("X-Assertion", base64.RawURLEncoding.EncodeToString(raw))
			request.Header.Set("X-Signature", base64.RawURLEncoding.EncodeToString(ed25519.Sign(private, raw)))
			if principal, err := verifier.Authenticate(request, nil); principal.Wallet != "" || !errors.Is(err, ErrScopeDenied) || categories[len(categories)-1] != tc.category {
				t.Fatalf("assertion %s = %+v, %v, categories %v", tc.name, principal, err, categories)
			}
		})
	}
	request.Header.Set("X-Assertion", "not-base64")
	if _, err := verifier.Authenticate(request, nil); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("invalid assertion encoding = %v", err)
	}
	request.TLS = nil
	if _, err := verifier.Authenticate(request, nil); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("unencrypted assertion = %v", err)
	}
}

func TestOperationHTTPRouteAndIdentityAdmission(t *testing.T) {
	for _, tc := range []struct {
		template, path string
		want           bool
	}{
		{"/v1/profiles/{wallet}", "/v1/profiles/abc", true},
		{"", "/v1/profiles/abc", false},
		{"/v1/profiles/{wallet}", "/v1/profiles/abc?token=x", false},
		{"/v1/profiles/{wallet}", "/v1/other/abc", false},
		{"/v1/profiles/{wallet}", "/v1/profiles/", false},
		{"/v1/profiles/{wallet}/metadata", "/v1/profiles/abc", false},
	} {
		if got := operationRouteMatches(tc.template, tc.path); got != tc.want {
			t.Fatalf("route %s matches %s = %t", tc.template, tc.path, got)
		}
	}
	categories := []string{}
	policies, routes := AccountOwnerPoliciesV2()
	authorizer := OperationAuthorizer{Ledger: Ledger{Environment: "test"}, Verifier: CommittedProofVerifier{}, Policies: policies, Routes: routes, Transport: OperationTransport{"X-Operation", "X-Signature"}, Observe: func(category string) { categories = append(categories, category) }}
	request, err := http.NewRequest(http.MethodPatch, "https://example.invalid/v1/profiles/0x0000000000000000000000000000000000000001", nil)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := authorizer.AuthorizeRequest(ctx, request, IdentityPrincipal{}, "profile.update", "", nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled operation = %v", err)
	}
	if _, err := authorizer.AuthorizeRequest(context.Background(), nil, IdentityPrincipal{}, "profile.update", "", nil); !errors.Is(err, ErrScopeDenied) {
		t.Fatalf("missing operation request = %v", err)
	}
	identity := IdentityPrincipal{AccountID: uuid.NewString(), SessionID: uuid.NewString(), Wallet: strings.ToLower("0x0000000000000000000000000000000000000001"), ProofAt: time.Now(), AccessExpiresAt: time.Now().Add(time.Minute)}
	if _, err := authorizer.AuthorizeRequest(context.Background(), request, identity, "unknown", "", nil); !errors.Is(err, ErrScopeDenied) || categories[len(categories)-1] != "action_mismatch" {
		t.Fatalf("unregistered operation = %v, categories %v", err, categories)
	}
}
