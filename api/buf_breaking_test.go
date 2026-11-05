package api_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"gopkg.in/yaml.v3"
)

func TestBufBreakingAgainstBaseline(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatalf("locate repository root: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, "buf.baseline.binpb"))
	if err != nil {
		t.Fatalf("read buf baseline: %v", err)
	}
	baseline := new(descriptorpb.FileDescriptorSet)
	if err := proto.Unmarshal(data, baseline); err != nil {
		t.Fatalf("decode buf baseline: %v", err)
	}
	files := make(map[string]int)
	for _, file := range baseline.GetFile() {
		files[file.GetName()]++
	}
	for _, name := range []string{"evm/params.proto", "oracle/oracle.proto", "cosmos/bank/v1beta1/query.proto"} {
		if count := files[name]; count != 1 {
			t.Fatalf("buf baseline has %d occurrences of %s, want exactly 1", count, name)
		}
	}

	configData, err := os.ReadFile(filepath.Join(root, "buf.yaml"))
	if err != nil {
		t.Fatalf("read buf.yaml: %v", err)
	}
	var config struct {
		Breaking struct {
			Use []string `yaml:"use"`
		} `yaml:"breaking"`
	}
	if err := yaml.Unmarshal(configData, &config); err != nil {
		t.Fatalf("parse buf.yaml: %v", err)
	}
	for _, required := range []string{"WIRE", "WIRE_JSON"} {
		found := false
		for _, rule := range config.Breaking.Use {
			if rule == required {
				found = true
			}
		}
		if !found {
			t.Fatalf("buf.yaml breaking.use must include %s", required)
		}
	}

	binDir := t.TempDir()
	install := exec.CommandContext(t.Context(), "go", "install", "github.com/bufbuild/buf/cmd/buf@v1.58.0")
	install.Dir = root
	install.Env = append(os.Environ(), "GOBIN="+binDir)
	if output, err := install.CombinedOutput(); err != nil {
		t.Fatalf("install pinned buf CLI: %v\n%s", err, output)
	} else if len(output) != 0 {
		t.Logf("install pinned buf CLI:\n%s", output)
	}
	cmd := exec.CommandContext(t.Context(), filepath.Join(binDir, "buf"), "breaking", "--against", "buf.baseline.binpb")
	cmd.Dir = root
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("buf breaking against baseline: %v\n%s", err, output)
	}
	if len(output) != 0 {
		t.Errorf("buf breaking against baseline: expected empty output, got:\n%s", output)
	}
}
