package bindings

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

func TestSidioraNFTBindingMatchesRoleABI(t *testing.T) {
	roleJSON, err := os.ReadFile("../../../../precompiles/launchpad/abi/SidioraNFT.json")
	if err != nil {
		t.Fatalf("read SidioraNFT role ABI: %v", err)
	}
	roleABI, err := abi.JSON(strings.NewReader(string(roleJSON)))
	if err != nil {
		t.Fatalf("parse SidioraNFT role ABI: %v", err)
	}
	bindingABI, err := SidioraNFTMetaData.GetAbi()
	if err != nil {
		t.Fatalf("parse SidioraNFT binding ABI: %v", err)
	}

	signatureSets := func(contractABI abi.ABI) map[string][]string {
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
		result := make(map[string][]string, len(sets))
		for kind, signatures := range sets {
			for signature := range signatures {
				result[kind] = append(result[kind], signature)
			}
			slices.Sort(result[kind])
		}
		return result
	}

	want := signatureSets(roleABI)
	got := signatureSets(*bindingABI)
	for _, kind := range []string{"methods", "events", "errors"} {
		t.Run(kind, func(t *testing.T) {
			if !slices.Equal(got[kind], want[kind]) {
				t.Errorf("SidioraNFT %s signatures differ:\n binding: %v\n role ABI: %v", kind, got[kind], want[kind])
			}
		})
	}
}
