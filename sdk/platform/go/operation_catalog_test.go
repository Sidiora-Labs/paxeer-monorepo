package layerx

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

type catalogOperation struct {
	Plane       string `json:"plane"`
	Name        string `json:"name"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Transport   string `json:"transport"`
	To          string `json:"to"`
	Selector    string `json:"selector"`
	PerLeg      bool   `json:"per_leg"`
	Idempotency bool   `json:"idempotency"`
}

func loadOperationCatalog(t *testing.T) []catalogOperation {
	t.Helper()
	encoded, err := os.ReadFile("../conformance/operations.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct {
		Schema     int                `json:"schema"`
		Operations []catalogOperation `json:"operations"`
	}
	if err := json.Unmarshal(encoded, &catalog); err != nil {
		t.Fatal(err)
	}
	if catalog.Schema != 1 || len(catalog.Operations) == 0 {
		t.Fatalf("invalid operation catalogue: schema %d, %d operations", catalog.Schema, len(catalog.Operations))
	}
	seen := make(map[string]bool)
	for _, operation := range catalog.Operations {
		key := operation.Plane + ":" + operation.Name
		if seen[key] {
			t.Fatalf("duplicate operation %s", key)
		}
		seen[key] = true
	}
	return catalog.Operations
}

func TestProgramOperationsHaveNoRestRoute(t *testing.T) {
	const programs = "0x000000000000000000000000000000000000101d"
	const auth = "0x000000000000000000000000000000000000101c"
	expected := map[AgentOperation]EVMOperationMetadata{
		AgentOperationProgramActivity:  {Method: "eth_getTransactionByHash"},
		AgentOperationProgramCall:      {Method: "eth_sendRawTransaction", To: programs, Selector: "call(bytes32,uint16,string,bytes,bytes,bytes,uint32,uint64[7],uint128)"},
		AgentOperationProgramDeploy:    {Method: "eth_sendRawTransaction", To: programs, Selector: "deploy(bytes32,uint16,uint8,bytes32,bytes32,bytes,bytes)"},
		AgentOperationProgramDiscover:  {Method: "eth_call", To: programs, Selector: "getProgram(bytes32)"},
		AgentOperationProgramInterface: {Method: "eth_call", To: programs, Selector: "getInterface(bytes32)"},
		AgentOperationProgramReceipt:   {Method: "eth_call", To: auth, Selector: "resultByIdempotency(bytes32,bytes32)"},
		AgentOperationProgramSimulate:  {Method: "eth_call", To: programs, Selector: "simulateCall(bytes32,uint16,string,bytes,bytes,bytes,uint32,uint64[7],uint128)"},
		AgentOperationProgramUpgrade:   {Method: "eth_sendRawTransaction", To: programs, Selector: "upgrade(bytes32,uint16,uint8,bytes32,bytes32,bytes,bytes,bytes,uint128)"},
		AgentOperationProgramWindDown:  {Method: "eth_sendRawTransaction", To: programs, Selector: "deprecate(bytes32,bytes32,uint64)"},
	}
	listed := make(map[AgentOperation]bool)
	for _, operation := range AllAgentOperations() {
		listed[operation] = true
		if strings.HasPrefix(string(operation), "program.") {
			if _, ok := expected[operation]; !ok {
				t.Fatalf("generated catalogue has an untested program operation %s", operation)
			}
		}
	}
	seen := make(map[AgentOperation]bool)
	for _, entry := range loadOperationCatalog(t) {
		if !strings.HasPrefix(entry.Name, "program.") {
			continue
		}
		operation := AgentOperation(entry.Name)
		want, exists := expected[operation]
		if !exists || entry.Plane != "agent" || seen[operation] {
			t.Fatalf("unexpected program entry: %+v", entry)
		}
		seen[operation] = true
		t.Run(entry.Name, func(t *testing.T) {
			if entry.Path != "" || entry.Transport != "evm" {
				t.Fatalf("program operation retains a REST route or lacks EVM transport: %+v", entry)
			}
			got := EVMOperationMetadata{Method: entry.Method, To: entry.To, Selector: entry.Selector, PerLeg: entry.PerLeg}
			if got != want {
				t.Fatalf("JSON route = %+v, want %+v", got, want)
			}
			generated, ok := operation.EVMMetadata()
			if !ok || generated != want || !operation.Valid() || !listed[operation] {
				t.Fatalf("generated program catalogue diverges: %+v (present %t)", generated, ok)
			}
			mutating := want.Method == "eth_sendRawTransaction"
			if entry.Idempotency != mutating || operation.RequiresIdempotency() != mutating {
				t.Fatalf("%s idempotency does not match transaction semantics", operation)
			}
		})
	}
	if len(seen) != len(expected) {
		t.Fatalf("found %d program operations, want %d", len(seen), len(expected))
	}
	if _, ok := AgentOperation("program.unknown").EVMMetadata(); ok {
		t.Fatal("unknown program operation has EVM metadata")
	}
}

func TestIntentOperationsArePerLegCalls(t *testing.T) {
	expected := map[HumanOperation]string{
		HumanOperationIntentPlan:   "eth_call",
		HumanOperationIntentSubmit: "eth_sendRawTransaction",
	}
	listed := make(map[HumanOperation]bool)
	for _, operation := range AllHumanOperations() {
		listed[operation] = true
	}
	seen := make(map[HumanOperation]bool)
	for _, entry := range loadOperationCatalog(t) {
		if !strings.HasPrefix(entry.Name, "intent.") {
			continue
		}
		operation := HumanOperation(entry.Name)
		method, exists := expected[operation]
		if !exists || entry.Plane != "human" || seen[operation] {
			t.Fatalf("unexpected intent entry: %+v", entry)
		}
		seen[operation] = true
		t.Run(entry.Name, func(t *testing.T) {
			want := EVMOperationMetadata{Method: method, PerLeg: true}
			got := EVMOperationMetadata{Method: entry.Method, To: entry.To, Selector: entry.Selector, PerLeg: entry.PerLeg}
			if entry.Path != "" || entry.Transport != "evm" || got != want {
				t.Fatalf("intent must use per-leg EVM calls: %+v", entry)
			}
			generated, ok := operation.EVMMetadata()
			if !ok || generated != want || !operation.Valid() || !listed[operation] {
				t.Fatalf("generated intent catalogue diverges: %+v (present %t)", generated, ok)
			}
			metadata, ok := operation.Metadata()
			if !ok || metadata.Path != "" || metadata.Method != method || !metadata.PerLeg {
				t.Fatalf("intent metadata retains a REST route or lacks per-leg execution: %+v", metadata)
			}
			mutating := operation == HumanOperationIntentSubmit
			if entry.Idempotency != mutating || operation.RequiresIdempotency() != mutating {
				t.Fatalf("%s idempotency does not match per-leg transaction semantics", operation)
			}
		})
	}
	if len(seen) != len(expected) {
		t.Fatalf("found %d intent operations, want %d", len(seen), len(expected))
	}
	if _, ok := HumanOperation("intent.unknown").EVMMetadata(); ok {
		t.Fatal("unknown intent operation has EVM metadata")
	}
}
