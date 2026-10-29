package layerx

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"
)

func TestAgentEnvelopeEncodesVersionTwo(t *testing.T) {
	token, err := NewSecretBytes([]byte(strings.Repeat("ab", 32)))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(token.Destroy)
	credential, err := NewAgentSessionCredential("tenant-a", strings.Repeat("cd", 32), token, 7)
	if err != nil {
		t.Fatal(err)
	}
	transport, err := NewAgentDaemonEnvelopeHTTPTransport("https://localhost", nil, credential)
	if err != nil {
		t.Fatal(err)
	}
	key, err := NewIdempotencyKey(strings.Repeat("12", 32))
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range AllAgentOperations() {
		t.Run(string(operation), func(t *testing.T) {
			call := TransportCall{Plane: PlaneAgent, Operation: string(operation), Request: json.RawMessage(`{}`)}
			if operation.RequiresIdempotency() {
				call.IdempotencyKey = key
			}
			encoded, err := transport.encode(call, "42")
			if err != nil {
				t.Fatal(err)
			}
			defer clear(encoded)
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			if string(fields["version"]) != "2" {
				t.Fatalf("encoded version = %s, want literal 2", fields["version"])
			}
			var encodedOperation string
			if json.Unmarshal(fields["operation"], &encodedOperation) != nil || encodedOperation != string(operation) {
				t.Fatal("encoded envelope does not identify the requested operation")
			}
		})
	}
}

func TestAgentEnvelopeRefusesVersionOneResponse(t *testing.T) {
	responses := []struct {
		name   string
		status int
		fields string
		code   ErrorCode
	}{
		{"success", http.StatusOK, `"request_id":"42","value":{"account":"ready"},"verification_status":{"state":"achieved","level":"SequencerSigned"}`, ""},
		{"refusal", http.StatusForbidden, `"class":"PolicyRefusal","protocol_result_code":null,"retriability":"Terminal","request_id":"42","reason":"policy.refused"`, ErrorPolicyRefusal},
	}
	versions := []struct {
		name  string
		field string
		valid bool
	}{
		{"one", `"version":1,`, false},
		{"two", `"version":2,`, true},
		{"zero", `"version":0,`, false},
		{"future", `"version":3,`, false},
		{"negative", `"version":-1,`, false},
		{"string", `"version":"2",`, false},
		{"fraction", `"version":2.5,`, false},
		{"null", `"version":null,`, false},
		{"missing", "", false},
	}
	for _, response := range responses {
		for _, candidate := range versions {
			t.Run(response.name+"/"+candidate.name, func(t *testing.T) {
				encoded := []byte("{" + candidate.field + response.fields + "}")
				value, err := decodeAgentEnvelopeResponse(response.status, encoded, "42")
				if !candidate.valid {
					if value != nil || err == nil || err.Code != ErrorProtocolIncompatible || err.Retry != RetryNever || err.ServiceCode != "envelope.version" || !errors.Is(err, ErrIncompatibleService) {
						t.Fatalf("incompatible envelope: value = %s, error = %#v; want no result and a terminal version error", value, err)
					}
					return
				}
				if response.code != "" {
					if value != nil || err == nil || err.Code != response.code || err.Retry != RetryNever || err.ServiceCode != "policy.refused" {
						t.Fatalf("version-2 refusal: value = %s, error = %#v", value, err)
					}
					return
				}
				if err != nil || string(value) != `{"account":"ready"}` {
					t.Fatalf("version-2 success: value = %s, error = %v", value, err)
				}
			})
		}
	}
}
