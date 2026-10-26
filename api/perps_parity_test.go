package api_test

import (
	"os"
	"path/filepath"
	"testing"

	_ "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	"google.golang.org/protobuf/types/descriptorpb"
)

type perpsDescriptorParity struct {
	t *testing.T
}

func (p perpsDescriptorParity) registeredFile(name string) *descriptorpb.FileDescriptorProto {
	p.t.Helper()
	return registeredFile(p.t, name)
}

func (p perpsDescriptorParity) bufFile(name string) *descriptorpb.FileDescriptorProto {
	p.t.Helper()
	return bufFile(p.t, name)
}

func (p perpsDescriptorParity) shapeOf(file *descriptorpb.FileDescriptorProto) descriptorShape {
	return shapeOf(file)
}

func (p perpsDescriptorParity) compareShapes(name string) {
	p.t.Helper()
	generated := p.shapeOf(p.registeredFile(name))
	source := p.shapeOf(p.bufFile(name))
	for _, difference := range compareShapes(name, source, generated) {
		p.t.Error(difference)
	}
}

func TestPerpsDescriptorParity(t *testing.T) {
	const root = "../api/perps"
	count := 0
	var checkDirectory func(string)
	checkDirectory = func(relative string) {
		t.Helper()
		directory := filepath.Join(root, relative)
		entries, err := os.ReadDir(directory)
		if err != nil {
			t.Fatalf("read proto directory %s: %v", directory, err)
		}
		for _, entry := range entries {
			path := filepath.Join(relative, entry.Name())
			if entry.IsDir() {
				checkDirectory(path)
				continue
			}
			if filepath.Ext(entry.Name()) != ".proto" {
				continue
			}
			count++
			name := filepath.ToSlash(filepath.Join("perps", path))
			t.Run(name, func(t *testing.T) {
				perpsDescriptorParity{t: t}.compareShapes(name)
			})
		}
	}
	checkDirectory("")
	if count == 0 {
		t.Fatalf("%s: no .proto files found", root)
	}
}
