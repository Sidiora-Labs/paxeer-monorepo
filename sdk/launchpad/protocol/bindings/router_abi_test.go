package bindings

import (
	"os"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

func TestRouterBindingMatchesRoleABI(t *testing.T) {
	data, err := os.ReadFile("../../../../precompiles/launchpad/abi/Router.json")
	if err != nil {
		t.Fatalf("read Router role ABI: %v", err)
	}
	roleABI, err := abi.JSON(strings.NewReader(string(data)))
	if err != nil {
		t.Fatalf("parse Router role ABI: %v", err)
	}
	bindingABI, err := RouterMetaData.GetAbi()
	if err != nil {
		t.Fatalf("parse Router binding ABI: %v", err)
	}
	if bindingABI == nil {
		t.Fatal("Router binding ABI is nil")
	}

	signatures := func(contract abi.ABI) map[string]map[string]struct{} {
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
	want := signatures(roleABI)
	got := signatures(*bindingABI)
	for _, kind := range []string{"methods", "events", "errors"} {
		t.Run(kind, func(t *testing.T) {
			for signature := range want[kind] {
				if _, ok := got[kind][signature]; !ok {
					t.Errorf("Router binding is missing %s signature %s", kind, signature)
				}
			}
			for signature := range got[kind] {
				if _, ok := want[kind][signature]; !ok {
					t.Errorf("Router binding has unexpected %s signature %s", kind, signature)
				}
			}
		})
	}
}
