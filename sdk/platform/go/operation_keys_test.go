package layerx

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"regexp"
	"testing"
)

func TestOperationsAreKeyedByContractId(t *testing.T) {
	encoded, err := os.ReadFile("../conformance/operations.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Schema     int             `json:"schema"`
		Operations json.RawMessage `json:"operations"`
	}
	if err := json.Unmarshal(encoded, &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Schema != 1 {
		t.Fatalf("operation catalogue schema = %d, want 1", catalog.Schema)
	}
	decoder := json.NewDecoder(bytes.NewReader(catalog.Operations))
	opening, err := decoder.Token()
	if err != nil || opening != json.Delim('{') {
		t.Fatalf("operations must be an object keyed by contract ID: token %v, error %v", opening, err)
	}
	validID := regexp.MustCompile(`^O([1-9]|[1-7][0-9])$`)
	ids := make(map[string]bool)
	entries := make(map[string]string)
	for decoder.More() {
		token, err := decoder.Token()
		if err != nil {
			t.Fatal(err)
		}
		id, ok := token.(string)
		if !ok || !validID.MatchString(id) {
			t.Fatalf("operation key %v is not O1 through O79", token)
		}
		if ids[id] {
			t.Fatalf("duplicate operation key %s", id)
		}
		ids[id] = true
		var descriptors []struct {
			Plane string `json:"plane"`
			Name  string `json:"name"`
		}
		if err := decoder.Decode(&descriptors); err != nil {
			t.Fatalf("%s: %v", id, err)
		}
		if len(descriptors) == 0 {
			t.Fatalf("%s has no named operations", id)
		}
		for _, entry := range descriptors {
			if entry.Name == "" {
				t.Fatalf("%s lost its operation name", id)
			}
			key := entry.Plane + ":" + entry.Name
			if previous, exists := entries[key]; exists {
				t.Fatalf("operation %s repeats under %s and %s", key, previous, id)
			}
			entries[key] = id
			switch entry.Plane {
			case "agent":
				if !AgentOperation(entry.Name).Valid() {
					t.Fatalf("%s: unknown agent operation %s", id, entry.Name)
				}
			case "human":
				if !HumanOperation(entry.Name).Valid() {
					t.Fatalf("%s: unknown human operation %s", id, entry.Name)
				}
			default:
				t.Fatalf("%s: unknown plane %q", id, entry.Plane)
			}
		}
	}
	if closing, err := decoder.Token(); err != nil || closing != json.Delim('}') {
		t.Fatalf("invalid operation object ending: token %v, error %v", closing, err)
	}
	if token, err := decoder.Token(); err != io.EOF {
		t.Fatalf("unexpected data after operations: token %v, error %v", token, err)
	}
	if len(ids) == 0 {
		t.Fatal("operation catalogue is empty")
	}
	for _, operation := range AllAgentOperations() {
		if _, exists := entries["agent:"+string(operation)]; !exists {
			t.Errorf("agent operation %s lost its contract ID", operation)
		}
	}
	for _, operation := range AllHumanOperations() {
		if _, exists := entries["human:"+string(operation)]; !exists {
			t.Errorf("human operation %s lost its contract ID", operation)
		}
	}
	if want := len(AllAgentOperations()) + len(AllHumanOperations()); len(entries) != want {
		t.Errorf("catalogue has %d named SDK operations, want %d", len(entries), want)
	}
	for key, want := range map[string]string{
		"agent:prepare":           "O1",
		"agent:sign":              "O3",
		"agent:submit":            "O5",
		"agent:read.account":      "O12",
		"agent:read.balance":      "O12",
		"agent:program.call":      "O30",
		"agent:program.simulate":  "O30",
		"agent:program.deploy":    "O31",
		"agent:program.upgrade":   "O31",
		"agent:program.wind-down": "O31",
		"agent:program.discover":  "O32",
		"human:intent.plan":       "O57",
		"human:intent.submit":     "O57",
	} {
		if got := entries[key]; got != want {
			t.Errorf("operation %s has contract ID %s, want %s", key, got, want)
		}
	}
}
