package production

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v1"
)

func TestRepairM_957_FrozenFixtureProvenance(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "protocol", "contracts", "v1", "examples.json"))
	if err != nil {
		t.Fatal(err)
	}
	const frozenSHA256 = "89711be034e3cbcc2d80a6a23ca8cab041bfc4aa489ce207a6d10c4520a98850"
	if got := Checksum(raw); got != frozenSHA256 {
		t.Fatalf("frozen generated examples changed: got %s want %s", got, frozenSHA256)
	}
	var examples struct {
		Label     string                     `json:"label"`
		Inputs    map[string]string          `json:"inputs"`
		Endpoints map[string]json.RawMessage `json:"endpoints"`
	}
	if err := json.Unmarshal(raw, &examples); err != nil {
		t.Fatal(err)
	}
	if examples.Label != "SYNTHETIC_SCHEMA_EXAMPLES_NOT_HANDLER_OR_RECEIPT_PROOF" {
		t.Fatal("frozen examples must retain their schema-only evidence label")
	}
	const schemaSHA256 = "a2379b5862a29881c7fd9e764cd7c64ef3b92dc033986300fcca92606e8ff847"
	if examples.Inputs["schema-definitions.json"] != schemaSHA256 {
		t.Fatal("examples were not generated from the frozen v1 schema")
	}
	if len(examples.Endpoints) != 108 {
		t.Fatalf("frozen endpoint coverage: got %d want 108", len(examples.Endpoints))
	}
}

func TestRepairM_957_CompatibilityFixture(t *testing.T) {
	wire, oldPool := phase2FrozenPool(t)
	newPool, err := interfaces.DecodePool(wire)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(oldPool, newPool) {
		t.Fatal("validated legacy reader and generated reader disagree on frozen Pool")
	}
	oldRepresentation, err := json.Marshal([]interfaces.Pool{oldPool})
	if err != nil {
		t.Fatal(err)
	}
	newRepresentation, err := json.Marshal([]interfaces.Pool{newPool})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(oldRepresentation, newRepresentation) {
		t.Fatal("frozen Pool representation changed between readers")
	}
	if _, err := interfaces.DecodePool(newRepresentation[1 : len(newRepresentation)-1]); err != nil {
		t.Fatalf("reader representation violates frozen Pool contract: %v", err)
	}
	var original, encoded map[string]json.RawMessage
	if err := json.Unmarshal(wire, &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(newRepresentation[1:len(newRepresentation)-1], &encoded); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"effectivePricingUsdlReserveRaw", "realUsdlBalanceRaw", "tokenReserveRaw", "virtualUsdlReserveRaw"} {
		var amount string
		if err := json.Unmarshal(original[field], &amount); err != nil {
			t.Fatalf("frozen %s is not a decimal string: %v", field, err)
		}
		var roundtrip string
		if err := json.Unmarshal(encoded[field], &roundtrip); err != nil {
			t.Fatalf("reader %s lost decimal-string serialization: %v", field, err)
		}
		if roundtrip != amount {
			t.Fatalf("reader changed %s: got %q want %q", field, roundtrip, amount)
		}
	}
	t.Logf("frozen_pool_sha256=%s representation_sha256=%s", Checksum(wire), Checksum(newRepresentation))
}
