package bindings

import (
	"bytes"
	"os"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

func TestQuoterBindingMatchesRoleABI(t *testing.T) {
	roleJSON, err := os.ReadFile("../../../../precompiles/launchpad/abi/Quoter.json")
	if err != nil {
		t.Fatalf("read Quoter role ABI: %v", err)
	}
	roleABI, err := abi.JSON(bytes.NewReader(roleJSON))
	if err != nil {
		t.Fatalf("parse Quoter role ABI: %v", err)
	}
	bindingABI, err := QuoterMetaData.GetAbi()
	if err != nil {
		t.Fatalf("parse Quoter binding ABI: %v", err)
	}
	if bindingABI == nil {
		t.Fatal("Quoter binding ABI is nil")
	}

	signatures := func(contractABI *abi.ABI) map[string]map[string]struct{} {
		sets := map[string]map[string]struct{}{
			"methods": {},
			"events":  {},
			"errors":  {},
		}
		for _, method := range contractABI.Methods {
			sets["methods"][method.Sig] = struct{}{}
		}
		for _, event := range contractABI.Events {
			sets["events"][event.Sig] = struct{}{}
		}
		for _, contractError := range contractABI.Errors {
			sets["errors"][contractError.Sig] = struct{}{}
		}
		return sets
	}
	want, got := signatures(&roleABI), signatures(bindingABI)
	for _, kind := range []string{"methods", "events", "errors"} {
		t.Run(kind, func(t *testing.T) {
			if !reflect.DeepEqual(got[kind], want[kind]) {
				t.Errorf("Quoter %s signatures differ: binding = %v, role = %v", kind, got[kind], want[kind])
			}
		})
	}
}
