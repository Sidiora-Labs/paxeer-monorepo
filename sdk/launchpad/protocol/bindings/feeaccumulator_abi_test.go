package bindings

import (
	"bytes"
	"maps"
	"os"
	"slices"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

func TestFeeAccumulatorBindingMatchesRoleABI(t *testing.T) {
	roleJSON, err := os.ReadFile("../../../../precompiles/launchpad/abi/FeeAccumulator.json")
	if err != nil {
		t.Fatalf("read FeeAccumulator role ABI: %v", err)
	}
	roleABI, err := abi.JSON(bytes.NewReader(roleJSON))
	if err != nil {
		t.Fatalf("parse FeeAccumulator role ABI: %v", err)
	}
	bindingABI, err := FeeAccumulatorMetaData.GetAbi()
	if err != nil {
		t.Fatalf("parse FeeAccumulator binding ABI: %v", err)
	}
	if bindingABI == nil {
		t.Fatal("FeeAccumulator binding ABI is nil")
	}

	signatures := func(contractABI abi.ABI) map[string]map[string]struct{} {
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

	want := signatures(roleABI)
	got := signatures(*bindingABI)
	for _, kind := range []string{"methods", "events", "errors"} {
		t.Run(kind, func(t *testing.T) {
			if maps.Equal(got[kind], want[kind]) {
				return
			}
			var missing, unexpected []string
			for signature := range want[kind] {
				if _, ok := got[kind][signature]; !ok {
					missing = append(missing, signature)
				}
			}
			for signature := range got[kind] {
				if _, ok := want[kind][signature]; !ok {
					unexpected = append(unexpected, signature)
				}
			}
			slices.Sort(missing)
			slices.Sort(unexpected)
			t.Errorf("binding %s differ from role ABI: missing %v; unexpected %v", kind, missing, unexpected)
		})
	}
}
