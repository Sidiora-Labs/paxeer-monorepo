package bindings

import (
	"bytes"
	"maps"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

func TestPoolRegistryBindingMatchesRoleABI(t *testing.T) {
	roleJSON, err := os.ReadFile("../../../../precompiles/launchpad/abi/PoolRegistry.json")
	if err != nil {
		t.Fatalf("read PoolRegistry role ABI: %v", err)
	}
	roleABI, err := abi.JSON(bytes.NewReader(roleJSON))
	if err != nil {
		t.Fatalf("parse PoolRegistry role ABI: %v", err)
	}
	bindingABI, err := PoolRegistryMetaData.GetAbi()
	if err != nil {
		t.Fatalf("parse PoolRegistry binding ABI: %v", err)
	}
	if bindingABI == nil {
		t.Fatal("PoolRegistry binding ABI is nil")
	}

	signatureSets := func(contract abi.ABI) map[string]map[string]struct{} {
		sets := map[string]map[string]struct{}{
			"methods": {},
			"events":  {},
			"errors":  {},
		}
		for _, method := range contract.Methods {
			sets["methods"][method.Sig] = struct{}{}
		}
		for _, event := range contract.Events {
			sets["events"][event.Sig] = struct{}{}
		}
		for _, abiError := range contract.Errors {
			sets["errors"][abiError.Sig] = struct{}{}
		}
		return sets
	}
	want := signatureSets(roleABI)
	got := signatureSets(*bindingABI)
	for _, kind := range []string{"methods", "events", "errors"} {
		t.Run(kind, func(t *testing.T) {
			if !maps.Equal(got[kind], want[kind]) {
				t.Errorf("PoolRegistry %s signature set mismatch:\n binding: %v\n role ABI: %v", kind, got[kind], want[kind])
			}
		})
	}
}
