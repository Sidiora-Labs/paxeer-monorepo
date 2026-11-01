package api_test

import (
	"os"
	"path"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/grpc/reflection"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/grpc/tmservice"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/grpc/reflection/v2alpha1"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/snapshots/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/query"
)

func TestCosmosBaseDescriptorParity(t *testing.T) {
	for _, directory := range []string{
		"cosmos/base/abci/v1beta1",
		"cosmos/base/kv/v1beta1",
		"cosmos/base/query/v1beta1",
		"cosmos/base/reflection/v1beta1",
		"cosmos/base/reflection/v2alpha1",
		"cosmos/base/snapshots/v1beta1",
		"cosmos/base/store/v1beta1",
		"cosmos/base/tendermint/v1beta1",
		"cosmos/base/v1beta1",
	} {
		entries, err := os.ReadDir(filepath.Join("..", "sdk", "proto", filepath.FromSlash(directory)))
		if err != nil {
			t.Errorf("%s: read proto directory: %v", directory, err)
			continue
		}
		files := 0
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
				continue
			}
			files++
			name := path.Join(directory, entry.Name())
			t.Run(name, func(t *testing.T) {
				source := shapeOf(bufFile(t, name))
				generated := shapeOf(registeredFile(t, name))
				for _, difference := range compareShapes(name, source, generated) {
					t.Error(difference)
				}
			})
		}
		if files == 0 {
			t.Errorf("%s: no proto sources found", directory)
		}
	}
}
