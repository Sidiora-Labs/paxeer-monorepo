package bindings

import (
	"bytes"
	"os"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

func TestFeesRouterBindingMatchesRoleABI(t *testing.T) {
	data, err := os.ReadFile("../../../../precompiles/launchpad/abi/FeesRouter.json")
	if err != nil {
		t.Fatalf("read FeesRouter role ABI: %v", err)
	}
	roleABI, err := abi.JSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse FeesRouter role ABI: %v", err)
	}
	bindingABI, err := FeesRouterMetaData.GetAbi()
	if err != nil {
		t.Fatalf("parse FeesRouter binding ABI: %v", err)
	}
	if bindingABI == nil {
		t.Fatal("FeesRouter binding ABI is nil")
	}

	signatures := func(parsed abi.ABI) map[string]map[string]struct{} {
		sets := map[string]map[string]struct{}{
			"methods": {},
			"events":  {},
			"errors":  {},
		}
		for _, method := range parsed.Methods {
			sets["methods"][method.Sig] = struct{}{}
		}
		for _, event := range parsed.Events {
			sets["events"][event.Sig] = struct{}{}
		}
		for _, abiError := range parsed.Errors {
			sets["errors"][abiError.Sig] = struct{}{}
		}
		return sets
	}
	want := signatures(roleABI)
	got := signatures(*bindingABI)
	for _, kind := range []string{"methods", "events", "errors"} {
		t.Run(kind, func(t *testing.T) {
			if !reflect.DeepEqual(got[kind], want[kind]) {
				t.Errorf("FeesRouter %s signatures differ:\n got: %v\nwant: %v", kind, got[kind], want[kind])
			}
		})
	}
}
