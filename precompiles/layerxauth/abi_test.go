package layerxauth

import (
	"encoding/json"
	"fmt"
	"slices"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

func TestAuthABIMethods(t *testing.T) {
	authABI := GetABI()
	tests := []struct {
		name       string
		signature  string
		selector   string
		mutability string
		inputs     []string
		outputs    []string
	}{
		{
			"grantSession", "grantSession(bytes,uint64,bytes32)", "0x1107b49e", "nonpayable",
			[]string{"grant:bytes", "expirySequence:uint64", "actionKey:bytes32"},
			[]string{"grantId:bytes32"},
		},
		{
			"grantSessionSuccessor", "grantSessionSuccessor(bytes,uint64,bytes32,bytes32,bytes32)", "0x0c49a046", "nonpayable",
			[]string{"grant:bytes", "expirySequence:uint64", "actionKey:bytes32", "predecessorId:bytes32", "expectedCommitment:bytes32"},
			[]string{"grantId:bytes32"},
		},
		{
			"grantCapability", "grantCapability(bytes)", "0x0cb291d1", "nonpayable",
			[]string{"grant:bytes"}, []string{"grantId:bytes32"},
		},
		{
			"revokeGrant", "revokeGrant(bytes32,uint8,uint64)", "0x859615ab", "nonpayable",
			[]string{"grantId:bytes32", "reason:uint8", "sequence:uint64"}, nil,
		},
		{
			"resultByIdempotency", "resultByIdempotency(bytes32,bytes32)", "0x2eda9f2b", "view",
			[]string{"didId:bytes32", "idempotencyKey:bytes32"}, []string{"receipt:bytes"},
		},
		{
			"getGrant", "getGrant(bytes32)", "0xe483ac04", "view",
			[]string{"grantId:bytes32"}, []string{"grant:bytes", "revoked:bool", "revokedAt:uint64"},
		},
		{
			"isLive", "isLive(bytes32,uint64,uint64)", "0xd4a03b3d", "view",
			[]string{"grantId:bytes32", "sequence:uint64", "timestampMs:uint64"}, []string{"live:bool", "code:string"},
		},
		{
			"sessionKey", "sessionKey(bytes32)", "0x55eb131c", "view",
			[]string{"grantId:bytes32"}, []string{"summary:bytes"},
		},
		{
			"capabilityScope", "capabilityScope(bytes32)", "0xd05b2513", "view",
			[]string{"grantId:bytes32"},
			[]string{
				"moduleMask:uint64", "ordinalMin:uint16", "ordinalMax:uint16", "assetId:bytes32",
				"maxPerActivity:uint128", "maxTotal:uint128", "spentTotal:uint128", "periodLength:uint64",
				"maxPerPeriod:uint128", "spentThisPeriod:uint128", "periodStart:uint64", "purposeHash:bytes32",
			},
		},
		{
			"checkActivity", "checkActivity(bytes32,uint32,bytes32,uint128)", "0xd825990c", "view",
			[]string{"grantId:bytes32", "activityType:uint32", "assetId:bytes32", "amount:uint128"}, []string{"code:string"},
		},
	}
	assertAuthABIDeclarations(t, "function", len(tests))
	if len(authABI.Methods) != len(tests) {
		t.Fatalf("method count = %d, want %d", len(authABI.Methods), len(tests))
	}
	if len(authABI.Errors) != 0 || authABI.HasFallback() || authABI.HasReceive() || len(authABI.Constructor.Inputs) != 0 {
		t.Fatal("unexpected ABI errors, fallback, receive or constructor inputs")
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			method, ok := authABI.Methods[tt.name]
			if !ok {
				t.Fatalf("missing method %s", tt.name)
			}
			if method.Type != abi.Function || method.RawName != tt.name || method.Sig != tt.signature {
				t.Errorf("method = %s (%s), want function %s", method.RawName, method.Sig, tt.signature)
			}
			if selector := fmt.Sprintf("0x%x", method.ID); selector != tt.selector {
				t.Errorf("selector = %s, want %s", selector, tt.selector)
			}
			if method.StateMutability != tt.mutability || method.IsPayable() || method.IsConstant() != (tt.mutability == "view") {
				t.Errorf("mutability = %s, want %s", method.StateMutability, tt.mutability)
			}
			assertAuthABIArguments(t, method.Inputs, tt.inputs)
			assertAuthABIArguments(t, method.Outputs, tt.outputs)
			resolved, err := authABI.MethodById(method.ID)
			if err != nil {
				t.Fatalf("resolve selector: %v", err)
			}
			if resolved.Sig != tt.signature {
				t.Errorf("resolved method = %s, want %s", resolved.Sig, tt.signature)
			}
		})
	}
}

func TestAuthABIEvents(t *testing.T) {
	authABI := GetABI()
	tests := []struct {
		name      string
		signature string
		topic     string
		inputs    []string
	}{
		{
			"SessionGranted", "SessionGranted(bytes32,bytes)",
			"0xcd2f11433d26596e4c5d214c177bd03c817c9e19d9850ee61bda3442aa0e3b9a",
			[]string{"grantId:bytes32", "summary:bytes"},
		},
		{
			"GrantBody", "GrantBody(bytes32,uint8,uint8,bytes)",
			"0x882911a9b00b6db8640402713cdfb0ec23798ce72304a969c61288652ef2ac26",
			[]string{"grantId:bytes32", "sub:uint8", "chunk:uint8", "body:bytes"},
		},
		{
			"CapabilityGranted", "CapabilityGranted(bytes32)",
			"0x04898c328b1814096864087e5d20b9e11dae070d4f1820ca72f13978d3550108",
			[]string{"grantId:bytes32"},
		},
		{
			"GrantRevoked", "GrantRevoked(bytes32,bytes)",
			"0x56ae84eff6bf7f5e201e33f3e37e1e3bf3f3470dacdb55801842589a3d156419",
			[]string{"grantId:bytes32", "record:bytes"},
		},
		{
			"IdentityStateUpdated", "IdentityStateUpdated(bytes32,bytes)",
			"0x3c6016beeec8cc4427dbd9648f9f4e4cac68c008d10c9fa39e06e677b7ddbe1a",
			[]string{"didId:bytes32", "state:bytes"},
		},
	}
	assertAuthABIDeclarations(t, "event", len(tests))
	if len(authABI.Events) != len(tests) {
		t.Fatalf("event count = %d, want %d", len(authABI.Events), len(tests))
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event, ok := authABI.Events[tt.name]
			if !ok {
				t.Fatalf("missing event %s", tt.name)
			}
			if event.RawName != tt.name || event.Sig != tt.signature || event.Anonymous {
				t.Errorf("event = %s (%s), anonymous = %t, want non-anonymous %s", event.RawName, event.Sig, event.Anonymous, tt.signature)
			}
			if event.ID.Hex() != tt.topic {
				t.Errorf("topic0 = %s, want %s", event.ID.Hex(), tt.topic)
			}
			assertAuthABIArguments(t, event.Inputs, tt.inputs)
			for i, input := range event.Inputs {
				if input.Indexed != (i == 0) {
					t.Errorf("argument %d (%s) indexed = %t, want %t", i, input.Name, input.Indexed, i == 0)
				}
			}
			resolved, err := authABI.EventByID(event.ID)
			if err != nil {
				t.Fatalf("resolve topic0: %v", err)
			}
			if resolved.Sig != tt.signature {
				t.Errorf("resolved event = %s, want %s", resolved.Sig, tt.signature)
			}
		})
	}
}

func assertAuthABIArguments(t *testing.T, args abi.Arguments, want []string) {
	t.Helper()
	got := make([]string, len(args))
	for i, arg := range args {
		got[i] = arg.Name + ":" + arg.Type.String()
	}
	if !slices.Equal(got, want) {
		t.Errorf("arguments = %v, want %v", got, want)
	}
}

func assertAuthABIDeclarations(t *testing.T, kind string, want int) {
	t.Helper()
	data, err := f.ReadFile("abi.json")
	if err != nil {
		t.Fatal(err)
	}
	var entries []struct {
		Type string `json:"type"`
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &entries); err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	count := 0
	for _, entry := range entries {
		if entry.Type != "function" && entry.Type != "event" {
			t.Errorf("unexpected ABI declaration %s %s", entry.Type, entry.Name)
		}
		if entry.Type != kind {
			continue
		}
		count++
		if seen[entry.Name] {
			t.Errorf("duplicate %s %s", kind, entry.Name)
		}
		seen[entry.Name] = true
	}
	if count != want {
		t.Errorf("%s declarations = %d, want %d", kind, count, want)
	}
}
