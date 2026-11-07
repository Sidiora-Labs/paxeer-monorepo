// Proves: productionUnservedOperations and App.registerDiscovery -> App.Router over net/http -> v2.DecodeError.
//
// The production registration entry point is shared with New; the stateless
// unavailable and unauthenticated paths need no database or restart. Every
// request uses a real HTTP connection, the gateway middleware and the generated
// consumer decoder. Admission counters and fresh request IDs prove two reads
// reach the producer without a cached shortcut.
//
// Repairs production_unserved.go: registerUnservedOperations previously omitted
// the private cache directive for cookie-only failures on private operations.
// WriteV2NotConfiguredError now preserves private, no-store for those routes.
//
// Discovery sanitizes malformed X-Request-Id values before writing its error
// envelope. The consumer must accept the resulting UUID and the malformed
// input must appear in neither the response body nor its metadata.
// The unavailable response is the successful contract result for operations
// whose backing authority is unconfigured; malformed queries must be refused
// before admission rather than reaching that producer.
package app

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/rest"
	sharedhttp "github.com/Sidiora-Technologies/KindleLaunch/shared/http"
	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	sharedprocess "github.com/Sidiora-Technologies/KindleLaunch/shared/process"
	"github.com/google/uuid"
)

func proofLaunchpadAPIServer(t *testing.T) (*httptest.Server, *rest.Admission) {
	t.Helper()
	admission, err := rest.NewAdmission(10, 1, time.Millisecond)
	if err != nil {
		t.Fatal(err)
	}
	router := sharedhttp.NewRouter(sharedhttp.ServerOptions{})
	a := &App{tracker: sharedprocess.NewTracker()}
	checks := map[string]sharedhttp.Check{}
	if err := a.registerProductionOperations(context.Background(), router, nil, admission, checks); err != nil {
		t.Fatal(err)
	}
	if checks["discovery_sources"] == nil || checks["notification_delivery"] == nil {
		t.Fatal("unconfigured discovery readiness checks missing")
	}
	for name, check := range checks {
		if err := check(context.Background()); err == nil {
			t.Fatalf("unconfigured dependency %s reported ready", name)
		}
	}
	a.Router = runtimeHandler(a.tracker, router, nil)
	if err := a.tracker.SetReady(); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(a.Router)
	t.Cleanup(server.Close)
	return server, admission
}

func proofLaunchpadAPIRequest(t *testing.T, server *httptest.Server, method, path, requestID string) (*http.Response, []byte) {
	t.Helper()
	req, err := http.NewRequest(method, server.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	// Cookie-only requests exercise the private error cache policy.
	req.AddCookie(&http.Cookie{Name: "session", Value: "unrecognized"})
	if requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}
	client := server.Client()
	client.Timeout = 2 * time.Second
	response, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 16385))
	if err != nil {
		t.Fatal(err)
	}
	if len(body) > 16384 {
		t.Fatal("unbounded error response")
	}
	if response.Header.Get("X-Schema-Version") != "2" || response.Header.Get("Content-Type") != "application/json" {
		t.Fatalf("missing v2 JSON metadata: %v; body=%s", response.Header, body)
	}
	if _, err := uuid.Parse(response.Header.Get("X-Request-ID")); err != nil {
		t.Fatalf("invalid response request ID: %v", err)
	}
	return response, body
}

func proofLaunchpadAPIDecode(t *testing.T, response *http.Response, body []byte, status int, code string) v2.Error {
	t.Helper()
	if response.StatusCode != status {
		t.Fatalf("status=%d, want %d; body=%s", response.StatusCode, status, body)
	}
	failure, err := v2.DecodeError(body)
	if err != nil {
		t.Fatalf("consumer refused producer error: %v; body=%s", err, body)
	}
	if failure.Code != code || failure.Message == "" || failure.Retryable || failure.RetryAfterMs != nil || failure.FieldErrors == nil || len(failure.FieldErrors) != 0 || failure.Details != nil {
		t.Fatalf("unexpected consumer result: %+v", failure)
	}
	if response.Header.Get("Retry-After") != "" {
		t.Fatalf("non-retryable failure carries Retry-After: %v", response.Header)
	}
	return failure
}

func TestProofLaunchpadApiProductionOperations(t *testing.T) {
	server, admission := proofLaunchpadAPIServer(t)
	catalog := map[string]internaltest.FrozenOperation{}
	for _, op := range internaltest.FrozenV2Operations(t) {
		if op.Service == "api" {
			catalog[op.Method+" "+op.Path] = op
		}
	}
	if len(productionUnservedOperations) != 21 {
		t.Fatalf("producer inventory=%d, want 21", len(productionUnservedOperations))
	}
	parameter := regexp.MustCompile(`\{[^}]+\}`)
	seen := map[string]bool{}
	for _, op := range productionUnservedOperations {
		key := op.method + " " + op.path
		if seen[key] || catalog[key].ID == "" {
			t.Fatalf("duplicate or non-contract producer: %s", key)
		}
		seen[key] = true
		t.Run(key, func(t *testing.T) {
			path := parameter.ReplaceAllString(op.path, "11111111-1111-4111-8111-111111111111")
			before := admission.Stats()
			var previous string
			for range 2 {
				response, body := proofLaunchpadAPIRequest(t, server, op.method, path, "")
				failure := proofLaunchpadAPIDecode(t, response, body, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
				if failure.RequestId != response.Header.Get("X-Request-ID") || failure.RequestId == previous {
					t.Fatalf("response reused or lost request identity: %+v", failure)
				}
				previous = failure.RequestId
				wantCache := "no-store"
				if strings.HasPrefix(op.path, "/v2/me/") || strings.HasPrefix(op.path, "/v2/admin/") || strings.HasPrefix(op.path, "/v2/operator/") {
					wantCache = "private, no-store"
				}
				if response.Header.Get("Cache-Control") != wantCache {
					t.Fatalf("cache policy=%q, want %q", response.Header.Get("Cache-Control"), wantCache)
				}
			}
			after := admission.Stats()
			if after.Admitted != before.Admitted+2 || after.Used != 0 || after.PublicUsed != 0 || after.Waiting != 0 {
				t.Fatalf("requests did not traverse and release admission twice: before=%+v after=%+v", before, after)
			}
			response, body := proofLaunchpadAPIRequest(t, server, op.method, path+"?limit=1&limit=2", "")
			failure := proofLaunchpadAPIDecode(t, response, body, http.StatusBadRequest, "QUERY_INVALID")
			if failure.RequestId != response.Header.Get("X-Request-ID") || admission.Stats().Admitted != after.Admitted {
				t.Fatal("malformed input reached admission or lost error correlation")
			}
		})
	}
	for _, required := range []string{
		"GET /v2/admin/reports",
		"GET /v2/automation-campaigns/{campaignId}/disclosure",
		"GET /v2/me/automation-campaigns/{campaignId}",
		"GET /v2/me/conversations",
		"GET /v2/me/funding/{fundingId}",
		"GET /v2/me/funding/withdrawals/{preparationId}",
	} {
		if !seen[required] {
			t.Fatalf("required producer missing: %s", required)
		}
	}
}

func TestProofLaunchpadApiDiscoveryAndMalformedProducer(t *testing.T) {
	server, admission := proofLaunchpadAPIServer(t)
	const id = "11111111-1111-4111-8111-111111111111"
	for _, op := range []struct{ method, path string }{
		{http.MethodDelete, "/v2/me/alerts/" + id},
		{http.MethodPost, "/v2/me/alerts"},
		{http.MethodGet, "/v2/me/notifications"},
		{http.MethodPut, "/v2/me/notifications/" + id + "/read"},
		{http.MethodGet, "/v2/squads/" + id},
		{http.MethodPost, "/v2/squads"},
		{http.MethodPut, "/v2/squads/" + id + "/members/" + id},
	} {
		t.Run(op.method+" "+op.path, func(t *testing.T) {
			before := admission.Stats()
			var previous string
			for range 2 {
				response, body := proofLaunchpadAPIRequest(t, server, op.method, op.path, "")
				failure := proofLaunchpadAPIDecode(t, response, body, http.StatusUnauthorized, "AUTH_REQUIRED")
				if failure.RequestId == previous {
					t.Fatal("discovery reused an earlier producer result")
				}
				previous = failure.RequestId
				if response.Header.Get("Cache-Control") != "private, no-store" {
					t.Fatalf("private discovery failure can be cached: %v", response.Header)
				}
			}
			after := admission.Stats()
			if after.Admitted != before.Admitted+2 || after.Used != 0 || after.PublicUsed != 0 || after.Waiting != 0 {
				t.Fatalf("discovery bypassed or leaked admission: before=%+v after=%+v", before, after)
			}
		})
	}
	// Exercise malformed input through the real discovery producer and decoder.
	before := admission.Stats()
	response, body := proofLaunchpadAPIRequest(t, server, http.MethodDelete, "/v2/me/alerts/"+id, "not-a-uuid")
	failure := proofLaunchpadAPIDecode(t, response, body, http.StatusUnauthorized, "AUTH_REQUIRED")
	if strings.Contains(string(body), "not-a-uuid") || response.Header.Get("X-Request-ID") == "not-a-uuid" {
		t.Fatal("discovery accepted the malformed request identity")
	}
	if _, err := uuid.Parse(failure.RequestId); err != nil {
		t.Fatalf("discovery did not sanitize the request identity: %v", err)
	}
	if response.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatalf("private discovery failure can be cached: %v", response.Header)
	}
	if admission.Stats().Admitted != before.Admitted+1 || admission.Stats().Used != 0 {
		t.Fatal("malformed request identity did not traverse real admission")
	}
}
