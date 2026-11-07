package api_test

import (
	"os"
	"path/filepath"
	"testing"

	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
)

func TestCosmosBankDescriptorParity(t *testing.T) {
	files := []struct {
		name           string
		representative interface{ Descriptor() ([]byte, []int) }
	}{
		{"cosmos/bank/v1beta1/authz.proto", &banktypes.SendAuthorization{}},
		{"cosmos/bank/v1beta1/bank.proto", &banktypes.Params{}},
		{"cosmos/bank/v1beta1/genesis.proto", &banktypes.GenesisState{}},
		{"cosmos/bank/v1beta1/query.proto", &banktypes.QueryBalanceRequest{}},
		{"cosmos/bank/v1beta1/tx.proto", &banktypes.MsgSend{}},
	}
	covered := make(map[string]bool, len(files))
	for _, file := range files {
		covered[file.name] = true
	}

	entries, err := os.ReadDir(filepath.Join("..", "sdk", "proto", "cosmos", "bank", "v1beta1"))
	if err != nil {
		t.Fatalf("read cosmos/bank/v1beta1 proto sources: %v", err)
	}
	sources := make(map[string]bool)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
			continue
		}
		name := "cosmos/bank/v1beta1/" + entry.Name()
		sources[name] = true
		if !covered[name] {
			t.Errorf("%s: proto source has no representative generated type in parity table", name)
		}
	}

	for _, file := range files {
		t.Run(file.name, func(t *testing.T) {
			if !sources[file.name] {
				t.Fatalf("%s: parity table entry has no proto source", file.name)
			}
			source := bufFile(t, file.name)
			// The engine bank package registers the same names, so use the SDK type's descriptor.
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
