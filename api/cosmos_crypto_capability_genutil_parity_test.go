package api_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/multisig"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256r1"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/sr25519"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/capability/types"
	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/types"
	"google.golang.org/protobuf/types/descriptorpb"
)

type cosmosCryptoCapabilityGenutilParity struct {
	t *testing.T
}

func (p cosmosCryptoCapabilityGenutilParity) registeredFile(name string) *descriptorpb.FileDescriptorProto {
	p.t.Helper()
	return registeredFile(p.t, name)
}

func (p cosmosCryptoCapabilityGenutilParity) bufFile(name string) *descriptorpb.FileDescriptorProto {
	p.t.Helper()
	return bufFile(p.t, name)
}

func (p cosmosCryptoCapabilityGenutilParity) shapeOf(file *descriptorpb.FileDescriptorProto) descriptorShape {
	return shapeOf(file)
}

func (p cosmosCryptoCapabilityGenutilParity) compareShapes(name string, source, generated descriptorShape) []string {
	return compareShapes(name, source, generated)
}

func TestCosmosCryptoCapabilityGenutilDescriptorParity(t *testing.T) {
	const protoRoot = "../sdk/proto"
	files := 0
	for _, directory := range []string{
		"cosmos/crypto/ed25519",
		"cosmos/crypto/multisig",
		"cosmos/crypto/multisig/v1beta1",
		"cosmos/crypto/secp256k1",
		"cosmos/crypto/secp256r1",
		"cosmos/crypto/sr25519",
		"cosmos/capability/v1beta1",
		"cosmos/genutil/v1beta1",
	} {
		entries, err := os.ReadDir(filepath.Join(protoRoot, directory))
		if err != nil {
			t.Errorf("%s: read proto directory: %v", directory, err)
			continue
		}
		directoryFiles := 0
		for _, entry := range entries {
			if entry.IsDir() || filepath.Ext(entry.Name()) != ".proto" {
				continue
			}
			directoryFiles++
			files++
			name := filepath.ToSlash(filepath.Join(directory, entry.Name()))
			t.Run(name, func(t *testing.T) {
				parity := cosmosCryptoCapabilityGenutilParity{t: t}
				generated := parity.registeredFile(name)
				source := parity.bufFile(name)
				for _, difference := range parity.compareShapes(name, parity.shapeOf(source), parity.shapeOf(generated)) {
					t.Error(difference)
				}
			})
		}
		if directoryFiles == 0 {
			t.Errorf("%s: found no proto sources", directory)
		}
	}
	if files < 9 {
		t.Errorf("found %d proto sources, want at least the 9 baseline files plus additions", files)
	}
}
