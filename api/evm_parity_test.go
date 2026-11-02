package api_test

import (
	"os"
	"path/filepath"
	"testing"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
)

type evmPointerDescriptor struct {
	evmtypes.PointerType
}

func (d evmPointerDescriptor) Descriptor() ([]byte, []int) {
	return d.EnumDescriptor()
}

func TestEvmDescriptorParity(t *testing.T) {
	files := []struct {
		name           string
		representative interface{ Descriptor() ([]byte, []int) }
	}{
		{"evm/config.proto", &evmtypes.ChainConfig{}},
		{"evm/enums.proto", evmPointerDescriptor{evmtypes.PointerType_ERC20}},
		{"evm/genesis.proto", &evmtypes.AddressAssociation{}},
		{"evm/gov.proto", &evmtypes.AddERCNativePointerProposal{}},
		{"evm/params.proto", &evmtypes.Params{}},
		{"evm/query.proto", &evmtypes.QueryPaxAddressByEVMAddressRequest{}},
		{"evm/receipt.proto", &evmtypes.Log{}},
		{"evm/tx.proto", &evmtypes.MsgEVMTransaction{}},
		{"evm/types.proto", &evmtypes.Whitelist{}},
	}
	covered := make(map[string]bool, len(files))
	for _, file := range files {
		covered[file.name] = true
	}

	entries, err := os.ReadDir(filepath.Join("..", "api", "evm"))
	if err != nil {
		t.Fatalf("read api/evm proto sources: %v", err)
	}
	sources := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
			continue
		}
		name := "evm/" + entry.Name()
		sources[name] = true
		if !covered[name] {
			t.Errorf("%s: proto source has no representative generated type in parity table", name)
		}
	}

	for _, file := range files {
		t.Run(file.name, func(t *testing.T) {
			if !sources[file.name] {
				t.Fatalf("%s: parity table entry has no proto source in api/evm", file.name)
			}
			source := bufFile(t, file.name)
			generated := generatedFile(t, file.representative)
			if generated.GetName() != file.name {
				t.Errorf("%s: file name: source %q generated %q", file.name, source.GetName(), generated.GetName())
			}
			for _, difference := range compareShapes(file.name, shapeOf(source), shapeOf(generated)) {
				t.Error(difference)
			}
		})
	}
}
