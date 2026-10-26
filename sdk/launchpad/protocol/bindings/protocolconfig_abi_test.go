package bindings

import (
	"bytes"
	"os"
	"slices"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

func TestProtocolConfigBindingMatchesRoleABI(t *testing.T) {
	data, err := os.ReadFile("../../../../precompiles/launchpad/abi/ProtocolConfig.json")
	if err != nil {
		t.Fatalf("read ProtocolConfig role ABI: %v", err)
	}
	roleABI, err := abi.JSON(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("parse ProtocolConfig role ABI: %v", err)
	}
	bindingABI, err := ProtocolConfigMetaData.GetAbi()
	if err != nil {
		t.Fatalf("parse ProtocolConfig binding ABI: %v", err)
	}
	if bindingABI == nil {
		t.Fatal("ProtocolConfig binding ABI is nil")
	}

	signatures := func(parsed *abi.ABI) map[string][]string {
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
		result := make(map[string][]string, len(sets))
		for kind, set := range sets {
			for signature := range set {
				result[kind] = append(result[kind], signature)
			}
			slices.Sort(result[kind])
		}
		return result
	}

	want, got := signatures(&roleABI), signatures(bindingABI)
	for _, kind := range []string{"methods", "events", "errors"} {
		t.Run(kind, func(t *testing.T) {
			if !slices.Equal(got[kind], want[kind]) {
				t.Errorf("ProtocolConfig %s signatures differ:\n got: %v\nwant: %v", kind, got[kind], want[kind])
			}
		})
	}
}
