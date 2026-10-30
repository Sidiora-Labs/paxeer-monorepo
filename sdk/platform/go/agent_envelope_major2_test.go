package layerx

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAgentEnvelopeEncodesDaemonRequestGolden(t *testing.T) {
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
	golden, err := os.ReadFile(filepath.Join("..", "..", "..", "services", "agent", "schema", "agent-api", "golden", "agent-http-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, operation := range AllAgentOperations() {
		t.Run(string(operation), func(t *testing.T) {
			call := TransportCall{Plane: PlaneAgent, Operation: string(operation), Request: json.RawMessage(`{}`)}
			if operation.RequiresIdempotency() {
				call.IdempotencyKey = key
			}
			encoded, err := transport.encode(call, "7")
			if err != nil {
				t.Fatal(err)
			}
			defer clear(encoded)
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			if string(fields["version"]) != "1" {
				t.Fatalf("encoded version = %s, want literal 1", fields["version"])
			}
			var encodedOperation string
			if json.Unmarshal(fields["operation"], &encodedOperation) != nil || encodedOperation != string(operation) {
				t.Fatal("encoded envelope does not identify the requested operation")
			}
		})
	}
	encoded, err := transport.encode(TransportCall{Plane: PlaneAgent, Operation: string(AgentOperationTenantReadiness), Request: json.RawMessage(`{}`)}, "7")
	if err != nil {
		t.Fatal(err)
	}
	defer clear(encoded)
	if !bytes.Equal(encoded, golden) {
		t.Fatalf("encoded readiness request = %s, want %s", encoded, golden)
	}
}

func TestAgentEnvelopeDecodesDaemonResponseGoldens(t *testing.T) {
	readGolden := func(name string) []byte {
		t.Helper()
		encoded, err := os.ReadFile(filepath.Join("..", "..", "..", "services", "agent", "schema", "agent-api", "golden", "agent-http-"+name+".json"))
		if err != nil {
			t.Fatal(err)
		}
		return encoded
	}
	success := readGolden("success")
	refusal := readGolden("refusal")
	value, decodeErr := decodeAgentEnvelopeResponse(http.StatusOK, success, "7")
	if decodeErr != nil || string(value) != `{"account":"ready"}` {
		t.Fatalf("daemon success: value = %s, error = %#v", value, decodeErr)
	}
	value, decodeErr = decodeAgentEnvelopeResponse(http.StatusForbidden, refusal, "7")
	if value != nil || decodeErr == nil || decodeErr.Code != ErrorPolicyRefusal || decodeErr.Retry != RetryNever || decodeErr.ServiceCode != "policy.refused" || decodeErr.RequestID != "7" || decodeErr.ProtocolResultCode != nil {
		t.Fatalf("daemon refusal: value = %s, error = %#v", value, decodeErr)
	}

	for _, response := range []struct {
		name    string
		status  int
		encoded []byte
	}{
		{"success", http.StatusOK, success},
		{"refusal", http.StatusForbidden, refusal},
	} {
		var fields map[string]json.RawMessage
		if err := json.Unmarshal(response.encoded, &fields); err != nil {
			t.Fatal(err)
		}
		for _, field := range []string{"version", "unknown"} {
			t.Run(response.name+"/unexpected_"+field, func(t *testing.T) {
				fields[field] = json.RawMessage(`2`)
				encoded, err := json.Marshal(fields)
				if err != nil {
					t.Fatal(err)
				}
				delete(fields, field)
				value, errSDK := decodeAgentEnvelopeResponse(response.status, encoded, "7")
				if value != nil || errSDK == nil || errSDK.Code != ErrorDecodeFailure || errSDK.Retry != RetryNever {
					t.Fatalf("unknown response field accepted: value = %s, error = %#v", value, errSDK)
				}
			})
		}
		for field := range fields {
			t.Run(response.name+"/missing_"+field, func(t *testing.T) {
				incomplete := make(map[string]json.RawMessage, len(fields)-1)
				for name, value := range fields {
					if name != field {
						incomplete[name] = value
					}
				}
				encoded, err := json.Marshal(incomplete)
				if err != nil {
					t.Fatal(err)
				}
				value, errSDK := decodeAgentEnvelopeResponse(response.status, encoded, "7")
				if value != nil || errSDK == nil || errSDK.Code != ErrorDecodeFailure || errSDK.Retry != RetryNever {
					t.Fatalf("missing response field accepted: value = %s, error = %#v", value, errSDK)
				}
			})
		}
	}

	var fields map[string]json.RawMessage
	if err := json.Unmarshal(success, &fields); err != nil {
		t.Fatal(err)
	}
	checkVerification := func(t *testing.T, verification any, accepted bool) {
		t.Helper()
		encodedVerification, err := json.Marshal(verification)
		if err != nil {
			t.Fatal(err)
		}
		fields["verification_status"] = encodedVerification
		encoded, err := json.Marshal(fields)
		if err != nil {
			t.Fatal(err)
		}
		value, errSDK := decodeAgentEnvelopeResponse(http.StatusOK, encoded, "7")
		if accepted {
			if errSDK != nil || string(value) != `{"account":"ready"}` {
				t.Fatalf("valid verification refused: value = %s, error = %#v", value, errSDK)
			}
		} else if value != nil || errSDK == nil || errSDK.Code != ErrorVerificationFailure || errSDK.Retry != RetryNever {
			t.Fatalf("invalid verification accepted: value = %s, error = %#v", value, errSDK)
		}
	}
	rungs := []string{"submitted", "receipted", "committed"}
	for requestedIndex, requested := range rungs {
		t.Run("achieved/"+requested, func(t *testing.T) {
			checkVerification(t, map[string]string{"state": "achieved", "rung": requested}, true)
		})
		for achievedIndex, achieved := range rungs {
			t.Run("unverified/"+requested+"/"+achieved, func(t *testing.T) {
				checkVerification(t, map[string]string{"state": "unverified", "requested": requested, "achieved": achieved, "reason": "owner_payload_carries_no_level"}, achievedIndex < requestedIndex)
			})
		}
	}
	for name, verification := range map[string]string{
		"unknown_rung":             `{"state":"achieved","rung":"finalised"}`,
		"old_level":                `{"state":"achieved","level":"SequencerSigned"}`,
		"unknown_field":            `{"state":"achieved","rung":"submitted","extra":true}`,
		"missing_rung":             `{"state":"achieved"}`,
		"null_rung":                `{"state":"achieved","rung":null}`,
		"unknown_state":            `{"state":"pending","rung":"submitted"}`,
		"unknown_requested":        `{"state":"unverified","requested":"finalised","achieved":"submitted","reason":"pending"}`,
		"unknown_achieved":         `{"state":"unverified","requested":"committed","achieved":"Unverified","reason":"pending"}`,
		"missing_reason":           `{"state":"unverified","requested":"committed","achieved":"submitted"}`,
		"empty_reason":             `{"state":"unverified","requested":"committed","achieved":"submitted","reason":""}`,
		"null_reason":              `{"state":"unverified","requested":"committed","achieved":"submitted","reason":null}`,
		"unverified_unknown_field": `{"state":"unverified","requested":"committed","achieved":"submitted","reason":"pending","extra":true}`,
		"null":                     `null`,
	} {
		t.Run(name, func(t *testing.T) { checkVerification(t, json.RawMessage(verification), false) })
	}
	t.Run("oversized_reason", func(t *testing.T) {
		checkVerification(t, map[string]string{"state": "unverified", "requested": "committed", "achieved": "submitted", "reason": strings.Repeat("r", 257)}, false)
	})

	var request map[string]json.RawMessage
	if err := json.Unmarshal(readGolden("request"), &request); err != nil {
		t.Fatal(err)
	}
	request["version"] = json.RawMessage(`2`)
	encoded, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	value, decodeErr = decodeAgentEnvelopeResponse(http.StatusOK, encoded, "7")
	if value != nil || decodeErr == nil || decodeErr.Code != ErrorDecodeFailure {
		t.Fatalf("version-2 request accepted as a response: value = %s, error = %#v", value, decodeErr)
	}
	versionRefusal := []byte(`{"class":"ProtocolIncompatibility","protocol_result_code":null,"retriability":"Terminal","request_id":"7","reason":"envelope.version"}`)
	value, decodeErr = decodeAgentEnvelopeResponse(http.StatusBadRequest, versionRefusal, "7")
	if value != nil || decodeErr == nil || decodeErr.Code != ErrorProtocolIncompatible || decodeErr.Retry != RetryNever || decodeErr.ServiceCode != "envelope.version" || !errors.Is(decodeErr, ErrIncompatibleService) {
		t.Fatalf("version refusal lost: value = %s, error = %#v", value, decodeErr)
	}
}
