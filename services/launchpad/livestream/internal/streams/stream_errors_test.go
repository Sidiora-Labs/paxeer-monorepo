package streams

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"sort"
	"strings"
	"testing"

	v2 "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/go-chi/chi/v5"
)

func TestV2StreamRoutesWithoutLifecycleAnswerFrozenError(t *testing.T) {
	const streamID = "11111111-1111-4111-8111-111111111111"
	const sessionID = "22222222-2222-4222-8222-222222222222"
	const poolAddress = "0x1111111111111111111111111111111111111111"
	router := chi.NewRouter()
	New(Deps{}).RegisterRoutes(router)
	check := func(t *testing.T, method, target, body string, status int, code string) {
		t.Helper()
		request := httptest.NewRequest(method, target, strings.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != status {
			t.Errorf("%s %s: status = %d, want %d: %s", method, target, response.Code, status, response.Body.String())
		}
		if response.Header().Get("Content-Type") != "application/json" {
			t.Errorf("%s %s: missing JSON content type: %v", method, target, response.Header())
		}
		if err := v2.ValidateJSON("Error", response.Body.Bytes()); err != nil {
			t.Fatalf("%s %s: invalid frozen Error: %v: %s", method, target, err, response.Body.String())
		}
		var envelope v2.Error
		if err := json.Unmarshal(response.Body.Bytes(), &envelope); err != nil {
			t.Fatal(err)
		}
		if envelope.Code != code || envelope.Retryable != (status == http.StatusServiceUnavailable) || envelope.RetryAfterMs != nil {
			t.Errorf("%s %s: unexpected error: %+v; want code %s, retryable %t, no retry delay", method, target, envelope, code, status == http.StatusServiceUnavailable)
		}
	}

	catalog, err := os.ReadFile("../../../../../sdk/launchpad/packages/sdk/src/operations.gen.ts")
	if err != nil {
		t.Fatal(err)
	}
	_, definitions, ok := bytes.Cut(catalog, []byte("export const operationDefinitions = "))
	if !ok {
		t.Fatal("frozen operation definitions missing")
	}
	var operations map[string]struct {
		Method string `json:"method"`
		Path   string `json:"path"`
	}
	if err := json.NewDecoder(bytes.NewReader(definitions)).Decode(&operations); err != nil {
		t.Fatal(err)
	}
	bodies := map[string]string{
		"IF_LIVE_WEBHOOK":             `{"eventId":"event-1","streamId":"` + streamID + `","event":"active","occurredAt":"2026-10-26T00:00:00.000Z"}`,
		"IF_STREAM_CREATE":            `{"poolAddress":"` + poolAddress + `","title":"Frozen error contract"}`,
		"IF_STREAM_END":               `{"expectedRevision":"1"}`,
		"IF_STREAM_HEARTBEAT":         `{"viewerToken":"valid-shape","playbackSessionId":"` + sessionID + `"}`,
		"IF_STREAM_INGEST_CREDENTIAL": `{}`,
		"IF_STREAM_LIST":              "",
		"IF_STREAM_OPERATION_GET":     "",
		"IF_STREAM_START":             `{"expectedRevision":"1"}`,
	}
	var names []string
	for name, operation := range operations {
		if name != "IF_STREAM_TICKET_ISSUE" && (strings.HasPrefix(operation.Path, "/v2/streams") || strings.HasPrefix(operation.Path, "/v2/provider-webhooks/")) {
			names = append(names, name)
		}
	}
	if len(names) == 0 {
		t.Fatal("frozen stream catalog is empty")
	}
	sort.Strings(names)
	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			body, exists := bodies[name]
			if !exists {
				t.Fatalf("frozen route %s needs a valid request fixture", name)
			}
			operation := operations[name]
			target := strings.NewReplacer("{streamId}", streamID, "{operationId}", streamID).Replace(operation.Path)
			check(t, operation.Method, target, body, http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
		})
	}
	for name := range bodies {
		index := sort.SearchStrings(names, name)
		if index == len(names) || names[index] != name {
			t.Errorf("request fixture %s is absent from the frozen stream catalog", name)
		}
	}

	for _, query := range []string{
		"status=ended", "status=", "status=LIVE", "unknown=value",
		"poolAddress=", "poolAddress=invalid", "poolAddress=0x" + strings.Repeat("g", 40),
		"limit=", "limit=0", "limit=101", "limit=-1", "limit=1.5", "limit=01", "limit=%2B1", "limit=9999999999999999999999999",
		"cursor=", "cursor=" + strings.Repeat("a", 257),
		"status=live&status=ended", "limit=1&limit=2", "cursor=one&cursor=two",
		"poolAddress=" + poolAddress + "&poolAddress=" + poolAddress,
		"status=%zz", "status=live;unknown=value",
	} {
		t.Run("invalid_query/"+query, func(t *testing.T) {
			check(t, http.MethodGet, "/v2/streams?"+query, "", http.StatusBadRequest, "INVALID_REQUEST")
		})
	}
	for _, query := range []string{
		"status=live", "limit=1", "limit=100", "poolAddress=" + poolAddress,
		"status=live&poolAddress=" + poolAddress + "&cursor=opaque&limit=50",
	} {
		t.Run("valid_query/"+query, func(t *testing.T) {
			check(t, http.MethodGet, "/v2/streams?"+query, "", http.StatusServiceUnavailable, "DEPENDENCY_UNAVAILABLE")
		})
	}
	for _, action := range []string{"start", "end"} {
		target := "/v2/streams/" + streamID + ":" + action
		for _, tc := range []struct {
			name, body, code string
		}{
			{"connected_body", `{"revision":"1"}`, "FIELD_INVALID"},
			{"unknown_field", `{"expectedRevision":"1","revision":"1"}`, "FIELD_INVALID"},
			{"malformed_json", `{"expectedRevision":`, "FIELD_INVALID"},
			{"missing_revision", `{}`, "INVALID_REQUEST"},
			{"null_body", `null`, "INVALID_REQUEST"},
			{"null_revision", `{"expectedRevision":null}`, "INVALID_REQUEST"},
			{"noncanonical_revision", `{"expectedRevision":"01"}`, "INVALID_REQUEST"},
		} {
			t.Run(action+"/"+tc.name, func(t *testing.T) {
				check(t, http.MethodPost, target, tc.body, http.StatusBadRequest, tc.code)
			})
		}
	}
	for _, tc := range []struct {
		name, body, code string
	}{
		{"connected_body", `{"sessionId":"` + sessionID + `","token":"connected"}`, "FIELD_INVALID"},
		{"unknown_field", `{"viewerToken":"token","playbackSessionId":"` + sessionID + `","token":"connected"}`, "FIELD_INVALID"},
		{"malformed_json", `{"viewerToken":`, "FIELD_INVALID"},
		{"missing_fields", `{}`, "INVALID_REQUEST"},
		{"null_body", `null`, "INVALID_REQUEST"},
		{"missing_session", `{"viewerToken":"token"}`, "INVALID_REQUEST"},
		{"missing_token", `{"playbackSessionId":"` + sessionID + `"}`, "INVALID_REQUEST"},
		{"empty_token", `{"viewerToken":"","playbackSessionId":"` + sessionID + `"}`, "INVALID_REQUEST"},
		{"null_token", `{"viewerToken":null,"playbackSessionId":"` + sessionID + `"}`, "INVALID_REQUEST"},
		{"invalid_session", `{"viewerToken":"token","playbackSessionId":"invalid"}`, "INVALID_REQUEST"},
		{"long_token", `{"viewerToken":"` + strings.Repeat("t", 4097) + `","playbackSessionId":"` + sessionID + `"}`, "INVALID_REQUEST"},
	} {
		t.Run("heartbeat/"+tc.name, func(t *testing.T) {
			check(t, http.MethodPost, "/v2/streams/"+streamID+"/viewers:heartbeat", tc.body, http.StatusBadRequest, tc.code)
		})
	}
}
