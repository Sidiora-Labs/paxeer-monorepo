package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestIdentityUnavailableEnvelopeAndMeasurementProjection(t *testing.T) {
	w := httptest.NewRecorder()
	w.Header().Set("X-Request-Id", "11111111-1111-4111-8111-111111111111")
	WriteNotServed(w)
	var answer PublicError
	if err := json.Unmarshal(w.Body.Bytes(), &answer); err != nil {
		t.Fatal(err)
	}
	if w.Code != http.StatusServiceUnavailable || answer.Code != "DEPENDENCY_UNAVAILABLE" || answer.Retryable || answer.RequestID != "11111111-1111-4111-8111-111111111111" || w.Header().Get("Retry-After") != "" {
		t.Fatalf("unavailable response: status=%d body=%s", w.Code, w.Body.String())
	}
	if answer.Error() != answer.Code || notServedError().Status != http.StatusServiceUnavailable {
		t.Fatal("unavailable public error changed")
	}
	for _, tc := range []struct {
		value string
		valid bool
	}{
		{strings.Repeat("a", 64), true},
		{strings.Repeat("A", 64), false},
		{strings.Repeat("g", 64), false},
		{"a0", false},
	} {
		if got := validMeasurementDigest(tc.value); got != tc.valid {
			t.Fatalf("digest %q valid=%t, want %t", tc.value, got, tc.valid)
		}
	}
	phase := IdentityMeasurementPhase{Name: "steady"}
	recorder := &IdentityMeasurementRecorder{phaseFailures: map[string]int64{}}
	recorder.recordFailure(phase, "invalid response")
	if recorder.summary.UnexpectedFailures != 1 || recorder.phaseFailures["steady"] != 1 || len(recorder.summary.Failures) != 1 {
		t.Fatalf("failed measured request: %+v", recorder.summary)
	}
	phase.Warmup = true
	recorder.recordFailure(phase, "warmup response")
	if recorder.summary.UnexpectedFailures != 1 || recorder.phaseFailures["steady"] != 1 || len(recorder.summary.Failures) != 2 {
		t.Fatalf("warmup failure accounting: %+v", recorder.summary)
	}
	if got := recorder.summary.String(); !strings.Contains(got, "requests=0 success=0 failures=2 final_acceptance=false") {
		t.Fatalf("measurement summary: %s", got)
	}
}

func TestIdentityWalletLinksReadUsesCurrentPrincipal(t *testing.T) {
	f := ownerHTTPFixture(t)
	credentials, principal := ownerCredentials(t, f)
	if credentials.AccessToken == "" {
		t.Fatal("identity access token absent")
	}
	links, err := f.s.WalletLinks(context.Background(), principal)
	if err != nil || len(links) != 1 || links[0].Wallet != principal.Wallet() {
		t.Fatalf("wallet links: links=%+v err=%v", links, err)
	}
	proofAt := time.Now().UTC()
	principal.reauthenticatedAt = proofAt
	if !principal.ProofAt().Equal(proofAt) {
		t.Fatal("principal proof time changed")
	}
}
