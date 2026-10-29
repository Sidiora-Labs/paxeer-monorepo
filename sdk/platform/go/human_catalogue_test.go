package layerx

import (
	"encoding/json"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func humanCatalogueConstants(t *testing.T, typeName string) map[string]string {
	t.Helper()
	source, err := parser.ParseFile(token.NewFileSet(), "generated.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	constants := make(map[string]string)
	for _, declaration := range source.Decls {
		group, ok := declaration.(*ast.GenDecl)
		if !ok || group.Tok != token.CONST {
			continue
		}
		for _, specification := range group.Specs {
			value := specification.(*ast.ValueSpec)
			kind, ok := value.Type.(*ast.Ident)
			if !ok || kind.Name != typeName {
				continue
			}
			for i, name := range value.Names {
				literal, ok := value.Values[i].(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					t.Fatalf("%s is not a string constant", name.Name)
				}
				decoded, err := strconv.Unquote(literal.Value)
				if err != nil {
					t.Fatal(err)
				}
				constants[name.Name] = decoded
			}
		}
	}
	if len(constants) == 0 {
		t.Fatalf("no generated constants for %s", typeName)
	}
	return constants
}

func TestHumanVerificationLevelsUseBlockCommitted(t *testing.T) {
	want := map[string]HumanVerificationLevel{
		"HumanVerificationUnverified":      HumanVerificationUnverified,
		"HumanVerificationReceiptVerified": HumanVerificationReceiptVerified,
		"HumanVerificationBlockCommitted":  HumanVerificationBlockCommitted,
	}
	if HumanVerificationBlockCommitted != "block-committed" {
		t.Fatalf("block commitment level = %q", HumanVerificationBlockCommitted)
	}
	constants := humanCatalogueConstants(t, "HumanVerificationLevel")
	if len(constants) != len(want) {
		t.Errorf("verification constants = %v, want %v", constants, want)
	}
	for name, level := range want {
		if value, ok := constants[name]; !ok || value != string(level) || !level.Valid() {
			t.Errorf("verification level %s = %q (declared %t, valid %t)", name, value, ok, level.Valid())
		}
	}
	for _, level := range []HumanVerificationLevel{"checkpoint-finalised", "paxeer-finalised", "settlement-anchored", ""} {
		if level.Valid() {
			t.Errorf("obsolete or invalid verification level %q is accepted", level)
		}
	}
}

func TestHumanCatalogueHasNoRetiredJourneys(t *testing.T) {
	retired := func(name string) bool {
		switch strings.SplitN(name, ".", 2)[0] {
		case "binding", "deposit", "exit", "withdraw":
			return true
		default:
			return false
		}
	}
	for name, value := range humanCatalogueConstants(t, "HumanOperation") {
		if retired(value) {
			t.Errorf("retired operation constant %s = %q", name, value)
		}
	}
	operations := AllHumanOperations()
	if len(operations) == 0 {
		t.Fatal("human operation catalogue is empty")
	}
	for _, operation := range operations {
		if retired(string(operation)) || !operation.Valid() {
			t.Errorf("catalogue contains retired or invalid operation %q", operation)
		}
	}
	for _, operation := range []HumanOperation{
		"binding.rebind", "binding.rebind.action", "binding.statement", "binding.status", "binding.submit",
		"deposit.confirm", "deposit.start", "exit.eligibility", "exit.start", "withdraw.claim", "withdraw.start",
	} {
		if operation.Valid() || operation.RequiresIdempotency() {
			t.Errorf("retired operation %q is accepted", operation)
		}
		if _, ok := operation.Metadata(); ok {
			t.Errorf("retired operation %q has metadata", operation)
		}
	}
	encoded, err := os.ReadFile("../conformance/operations.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalogue struct {
		Schema     int `json:"schema"`
		Operations map[string][]struct {
			Name string `json:"name"`
		} `json:"operations"`
	}
	if err := json.Unmarshal(encoded, &catalogue); err != nil {
		t.Fatal(err)
	}
	if catalogue.Schema != 1 || len(catalogue.Operations) == 0 {
		t.Fatalf("invalid JSON operation catalogue: %+v", catalogue)
	}
	for id, entries := range catalogue.Operations {
		if len(entries) == 0 {
			t.Errorf("JSON catalogue contract %s has no operations", id)
		}
		for _, entry := range entries {
			if entry.Name == "" || retired(entry.Name) {
				t.Errorf("JSON catalogue contains retired or unnamed operation %q", entry.Name)
			}
		}
	}
	for _, kind := range []JourneyKind{"wallet-binding", "deposit", "withdraw", "exit"} {
		if kind.Valid() {
			t.Errorf("retired journey kind %q is accepted", kind)
		}
		for name, value := range humanCatalogueConstants(t, "JourneyKind") {
			if value == string(kind) {
				t.Errorf("retired journey constant %s = %q", name, value)
			}
		}
	}
	for _, kind := range []JourneyKind{JourneyOnboarding, JourneyMove, JourneyAgentCreate, JourneyAgentFund, JourneyAgentPause, JourneyAgentRetire} {
		if !kind.Valid() {
			t.Errorf("active journey kind %q is rejected", kind)
		}
	}
	for _, code := range []HumanErrorCode{"wallet-not-bound", "exit-unavailable"} {
		if code.Valid() {
			t.Errorf("retired error code %q is accepted", code)
		}
		for name, value := range humanCatalogueConstants(t, "HumanErrorCode") {
			if value == string(code) {
				t.Errorf("retired error constant %s = %q", name, value)
			}
		}
	}
	if HumanErrorOperationRetired != "operation-retired" || !HumanErrorOperationRetired.Valid() {
		t.Errorf("operation-retired error code %q is invalid", HumanErrorOperationRetired)
	}
}
