package policy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/redis/go-redis/v9"
)

func TestMeasuredAccountAdmissionUsesSignedPrivateIdentity(t *testing.T) {
	pool, schema := realSecurityDatabase(t)
	socket := localSecurityRedis(t)
	client := redis.NewClient(&redis.Options{Network: "unix", Addr: socket})
	t.Cleanup(func() {
		if err := client.Close(); err != nil {
			t.Error(err)
		}
	})
	var output bytes.Buffer
	capture := &MiddlewareCapture{Writer: &output, Key: []byte(strings.Repeat("k", 32)), RunID: "private-run", ReplicaID: "account-a", CandidateSHA256: strings.Repeat("a", 64), PolicyVersion: "v1"}
	var served atomic.Int32
	server, httpClient, key := realPrivateOperatorServer(t, pool, schema, func(verifier WorkloadVerifier) http.Handler {
		handler, err := MeasuredAdmission(AdmissionOptions{Redis: client, Namespace: "measured-account", Class: classAccountWrite, Cost: 1, Verifier: verifier}, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			served.Add(1)
			w.WriteHeader(http.StatusNoContent)
		}), capture)
		if err != nil {
			t.Fatal(err)
		}
		return handler
	})
	request, err := http.NewRequest(http.MethodPost, server.URL+"/account", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("X-Request-ID", "unsigned-account")
	response, err := httpClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized || served.Load() != 0 {
		t.Fatalf("unsigned account request: %d, %d", response.StatusCode, served.Load())
	}
	request, err = http.NewRequest(http.MethodPost, server.URL+"/account", nil)
	if err != nil {
		t.Fatal(err)
	}
	signRealOperatorRequest(t, request, key, "ordinary-session")
	response, err = httpClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusNoContent || served.Load() != 1 {
		t.Fatalf("signed account request: %d, %d", response.StatusCode, served.Load())
	}
	lines := bytes.Split(bytes.TrimSpace(output.Bytes()), []byte{'\n'})
	if len(lines) != 3 {
		t.Fatalf("authorization and limiter records: %d", len(lines))
	}
	for index, expected := range []struct {
		stage   string
		allowed bool
	}{{"authorization", false}, {"authorization", true}, {"limiter", true}} {
		var record MiddlewareRecord
		if err := json.Unmarshal(lines[index], &record); err != nil || record.Stage != expected.stage || record.Allowed != expected.allowed {
			t.Fatalf("record %d: %+v, %v", index, record, err)
		}
	}
}

func TestAuthorizationRequiresDurableDecisionAndCapture(t *testing.T) {
	pool, schema := realSecurityDatabase(t)
	var served atomic.Int32
	server, client, key := realPrivateOperatorServer(t, pool, schema, func(verifier WorkloadVerifier) http.Handler {
		journal := PostgresJournal{Pool: pool, Schema: "invalid schema"}
		return Authorize(verifier, journal, "indexer", "replay", "v1", http.HandlerFunc(func(http.ResponseWriter, *http.Request) { served.Add(1) }))
	})
	for _, signed := range []bool{false, true} {
		request, err := http.NewRequest(http.MethodPost, server.URL+"/operator/replay", nil)
		if err != nil {
			t.Fatal(err)
		}
		if signed {
			signRealOperatorRequest(t, request, key, "ordinary-session")
		}
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusServiceUnavailable || served.Load() != 0 {
			t.Fatalf("unrecorded authorization: %d, %d", response.StatusCode, served.Load())
		}
	}
	handler := MeasuredAuthorize(WorkloadVerifier{Pool: pool, Schema: schema}, PostgresJournal{Pool: pool, Schema: schema}, "indexer", "replay", "v1", http.NotFoundHandler(), nil)
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/operator/replay", nil))
	if response.Code != http.StatusServiceUnavailable || !strings.Contains(response.Body.String(), "capture_unavailable") {
		t.Fatalf("unmeasured authorization: %d %s", response.Code, response.Body.String())
	}
}

func TestCanonicalMutationDestinationRejectsAmbiguousShapes(t *testing.T) {
	pool, _ := realSecurityDatabase(t)
	for _, sample := range []struct {
		name, body  string
		destination func() any
	}{
		{"nil destination", `{"name":"value"}`, func() any { return nil }},
		{"nonpointer destination", `{"name":"value"}`, func() any { return struct{ Name string }{} }},
		{"unknown field", `{"other":"value"}`, func() any { return new(struct{ Name string }) }},
		{"noncanonical field", `{"name":"value"}`, func() any { return new(struct{ Name string }) }},
		{"array shape", `["value"]`, func() any { return new(struct{ Name string }) }},
		{"trailing value", `{"Name":"value"} true`, func() any { return new(struct{ Name string }) }},
	} {
		t.Run(sample.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/mutation", strings.NewReader(sample.body))
			if err := DecodeMutation(request, sample.destination(), 1024); err == nil {
				t.Fatalf("ambiguous mutation %s admitted", sample.name)
			}
		})
	}
	var destination struct{ Name string }
	request := httptest.NewRequest(http.MethodPost, "/mutation", strings.NewReader(`{"Name":"value"}`))
	if err := DecodeMutation(request, &destination, 1024); err != nil || destination.Name != "value" {
		t.Fatalf("canonical mutation: %+v, %v", destination, err)
	}
	request = httptest.NewRequest(http.MethodPost, "/mutation", strings.NewReader(`{"Name":"another"}`))
	if err := DecodeMutation(request, &destination, 1024); err != nil || destination.Name != "another" {
		t.Fatalf("updated mutation: %+v, %v", destination, err)
	}
	if err := ValidateDatabaseRole(context.Background(), nil, "audit_schema"); !errors.Is(err, ErrDenied) {
		t.Fatalf("missing role database: %v", err)
	}
	if err := ValidateDatabaseRole(context.Background(), pool, "invalid schema"); !errors.Is(err, ErrDenied) {
		t.Fatalf("invalid owned schema: %v", err)
	}
}
