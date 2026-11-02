package api_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
)

func TestBufBuildFromApiDirMatchesWorkspaceImage(t *testing.T) {
	for _, name := range []string{"buf.yaml", "buf.gen.yaml", "buf.lock"} {
		if _, err := os.Stat(name); !os.IsNotExist(err) {
			t.Fatalf("api/%s must be absent so Buf uses the root workspace; stat error: %v", name, err)
		}
	}
	workspace := bufImage(t)
	apiDir, err := filepath.Abs(".")
	if err != nil {
		t.Fatalf("locate api directory: %v", err)
	}
	output := filepath.Join(t.TempDir(), "api-image.binpb")
	// Select the whole workspace; implicit input "." limits Buf to api/ and its imports.
	cmd := exec.CommandContext(t.Context(), "go", "run", "github.com/bufbuild/buf/cmd/buf@v1.58.0", "build", "--exclude-source-info", "..", "-o", output)
	cmd.Dir = apiDir
	if log, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build buf image from api directory: %v\n%s", err, log)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("read api directory buf image: %v", err)
	}
	image := new(descriptorpb.FileDescriptorSet)
	if err := proto.Unmarshal(data, image); err != nil {
		t.Fatalf("decode api directory buf image: %v", err)
	}

	if len(workspace.GetFile()) == 0 {
		t.Fatal("root workspace buf image contains no file descriptors")
	}
	if len(image.GetFile()) != len(workspace.GetFile()) {
		t.Errorf("api directory buf image has %d files, root workspace has %d", len(image.GetFile()), len(workspace.GetFile()))
	}
	files := make(map[string]*descriptorpb.FileDescriptorProto, len(image.GetFile()))
	for _, file := range image.GetFile() {
		name := file.GetName()
		if _, exists := files[name]; exists {
			t.Fatalf("%s: duplicate file in api directory buf image", name)
		}
		files[name] = file
	}
	seen := make(map[string]bool, len(workspace.GetFile()))
	for _, want := range workspace.GetFile() {
		name := want.GetName()
		if seen[name] {
			t.Fatalf("%s: duplicate file in root workspace buf image", name)
		}
		seen[name] = true
		got, exists := files[name]
		if !exists {
			t.Errorf("%s: file missing from api directory buf image", name)
			continue
		}
		if !proto.Equal(got, want) {
			t.Errorf("%s: api directory file descriptor differs from root workspace", name)
		}
		delete(files, name)
	}
	for name := range files {
		t.Errorf("%s: unexpected file in api directory buf image", name)
	}
}
