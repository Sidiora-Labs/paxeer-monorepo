package bindings

import (
	"bytes"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

func TestSidioraFactoryBindingMatchesRoleABI(t *testing.T) {
	data, err := os.ReadFile("../../../../precompiles/launchpad/abi/SidioraFactory.json")
	if err != nil {
		t.Fatalf("read role ABI: %v", err)
	}
	roleABI, err := abi.JSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse role ABI: %v", err)
	}
	bindingABI, err := SidioraFactoryMetaData.GetAbi()
	if err != nil {
		t.Fatalf("parse binding ABI: %v", err)
	}
	if bindingABI == nil {
		t.Fatal("binding ABI is nil")
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
		for _, abiError := range contractABI.Errors {
			sets["errors"][abiError.Sig] = struct{}{}
		}
		return sets
	}

	want, got := signatures(&roleABI), signatures(bindingABI)
	for _, kind := range []string{"methods", "events", "errors"} {
		t.Run(kind, func(t *testing.T) {
			for signature := range want[kind] {
				if _, ok := got[kind][signature]; !ok {
					t.Errorf("binding is missing %s signature %q", kind, signature)
				}
			}
			for signature := range got[kind] {
				if _, ok := want[kind][signature]; !ok {
					t.Errorf("binding has unexpected %s signature %q", kind, signature)
				}
			}
		})
	}
}
