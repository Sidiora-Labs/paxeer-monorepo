package api_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"google.golang.org/protobuf/types/descriptorpb"
)

// Keep each descriptor lookup and comparison tied to its source file.
type cosmosUpgradeDescriptorFile struct {
	t    *testing.T
	name string
}

func (file cosmosUpgradeDescriptorFile) bufFile() *descriptorpb.FileDescriptorProto {
	file.t.Helper()
	return bufFile(file.t, file.name)
}

func (file cosmosUpgradeDescriptorFile) registeredFile() *descriptorpb.FileDescriptorProto {
	file.t.Helper()
	return registeredFile(file.t, file.name)
}

func (file cosmosUpgradeDescriptorFile) shapeOf(descriptor *descriptorpb.FileDescriptorProto) descriptorShape {
	file.t.Helper()
	return shapeOf(descriptor)
}

func (file cosmosUpgradeDescriptorFile) compareShapes(source, generated descriptorShape) []string {
	file.t.Helper()
	return compareShapes(file.name, source, generated)
}

func TestCosmosUpgradeDescriptorParity(t *testing.T) {
	const protoRoot = "../sdk/proto"
	const upgradeDirectory = "cosmos/upgrade/v1beta1"
	files := 0
	var checkDirectory func(string)
	checkDirectory = func(directory string) {
		t.Helper()
		entries, err := os.ReadDir(filepath.Join(protoRoot, directory))
		if err != nil {
			t.Fatalf("%s: read proto sources: %v", directory, err)
		}
		for _, entry := range entries {
			name := filepath.Join(directory, entry.Name())
			if entry.IsDir() {
				checkDirectory(name)
				continue
			}
			if filepath.Ext(entry.Name()) != ".proto" {
				continue
			}
			files++
			name = filepath.ToSlash(name)
			t.Run(name, func(t *testing.T) {
				file := cosmosUpgradeDescriptorFile{t: t, name: name}
				source := file.shapeOf(file.bufFile())
				generated := file.shapeOf(file.registeredFile())
				for _, difference := range file.compareShapes(source, generated) {
					t.Error(difference)
				}
			})
		}
	}
	checkDirectory(upgradeDirectory)
	if files == 0 {
		t.Fatalf("%s: no proto sources found", upgradeDirectory)
	}
}
