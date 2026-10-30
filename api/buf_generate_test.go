package api_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"golang.org/x/mod/modfile"
	"gopkg.in/yaml.v3"
)

func TestBufGenerateMatchesCheckedInGo(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	read := func(name string) []byte {
		t.Helper()
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return data
	}
	run := func(dir, name string, args ...string) []byte {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), name, args...)
		cmd.Dir = dir
		var stdout, stderr bytes.Buffer
		cmd.Stdout, cmd.Stderr = &stdout, &stderr
		if err := cmd.Run(); err != nil {
			t.Fatalf("%s %s (in %s): %v\n%s\n%s", name, strings.Join(args, " "), dir, err, stdout.Bytes(), stderr.Bytes())
		}
		return stdout.Bytes()
	}
	module, err := modfile.Parse("go.mod", read(filepath.Join(root, "go.mod")), nil)
	if err != nil {
		t.Fatal(err)
	}
	if module.Module == nil {
		t.Fatal("go.mod has no module path")
	}
	modulePath := module.Module.Mod.Path
	pinned := func(name string) string {
		t.Helper()
		for _, req := range module.Require {
			if req.Mod.Path == name {
				return req.Mod.Version
			}
		}
		t.Fatalf("%s has no version pinned in go.mod", name)
		return ""
	}
	const cosmosProto = "github.com/regen-network/cosmos-proto"
	var downloaded struct {
		Path    string
		Version string
		Dir     string
		Error   string
	}
	if err := json.Unmarshal(run(root, "go", "mod", "download", "-json", cosmosProto+"@"+pinned(cosmosProto)), &downloaded); err != nil {
		t.Fatalf("decode cosmos-proto download: %v", err)
	}
	if downloaded.Error != "" || downloaded.Dir == "" || downloaded.Path != cosmosProto || downloaded.Version != pinned(cosmosProto) {
		t.Fatalf("unexpected cosmos-proto download: %+v", downloaded)
	}
	tmp := t.TempDir()
	pluginPath := filepath.Join(tmp, "protoc-gen-gocosmos")
	run(downloaded.Dir, "go", "build", "-o", pluginPath, "./protoc-gen-gocosmos")

	type input struct {
		Directory string `yaml:"directory"`
	}
	type plugin struct {
		Local []string `yaml:"local"`
		Opt   []string `yaml:"opt"`
		Out   string   `yaml:"out"`
	}
	var template struct {
		Version string   `yaml:"version"`
		Inputs  []input  `yaml:"inputs"`
		Plugins []plugin `yaml:"plugins"`
	}
	if err := yaml.Unmarshal(read(filepath.Join(root, "buf.gen.yaml")), &template); err != nil {
		t.Fatalf("decode buf.gen.yaml: %v", err)
	}
	if template.Version != "v2" || len(template.Inputs) < 2 || template.Inputs[0].Directory != "api" || template.Inputs[1].Directory != "sdk/proto" || len(template.Plugins) < 2 {
		t.Fatal("buf.gen.yaml must declare v2 api and sdk/proto inputs and both gocosmos and grpc-gateway plugins")
	}
	if strings.Join(template.Plugins[0].Local, " ") != "./build/proto/gocosmos/protoc-gen-gocosmos" {
		t.Fatalf("unexpected gocosmos plugin: %v", template.Plugins[0].Local)
	}
	const gateway = "github.com/grpc-ecosystem/grpc-gateway"
	if pinned(gateway) != "v1.16.0" || strings.Join(template.Plugins[1].Local, " ") != "go run "+gateway+"/protoc-gen-grpc-gateway@v1.16.0" {
		t.Fatalf("grpc-gateway plugin and go.mod must both pin v1.16.0: %v, %s", template.Plugins[1].Local, pinned(gateway))
	}
	template.Inputs = template.Inputs[:2]
	template.Plugins = template.Plugins[:2]
	template.Plugins[0].Local = []string{pluginPath}
	output := filepath.Join(tmp, "generated")
	for i := range template.Plugins {
		template.Plugins[i].Out = output
	}
	data, err := yaml.Marshal(template)
	if err != nil {
		t.Fatalf("encode temporary template: %v", err)
	}
	templatePath := filepath.Join(tmp, "buf.gen.yaml")
	if err := os.WriteFile(templatePath, data, 0600); err != nil {
		t.Fatal(err)
	}
	script := read(filepath.Join(root, "scripts/protoc.sh"))
	bufCommand := regexp.MustCompile(`(?m)^go run (github\.com/bufbuild/buf/cmd/buf@\S+) generate\s*$`).FindSubmatch(script)
	if len(bufCommand) != 2 {
		t.Fatal("scripts/protoc.sh must pin the buf CLI for its default generation command")
	}
	run(root, "go", "run", string(bufCommand[1]), "generate", "--template", templatePath)

	packageDirs := make(map[string]bool)
	goPackage := regexp.MustCompile(`(?m)^\s*option\s+go_package\s*=\s*"([^"]+)"\s*;`)
	for _, in := range template.Inputs {
		err := filepath.WalkDir(filepath.Join(root, in.Directory), func(name string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || filepath.Ext(name) != ".proto" {
				return nil
			}
			matches := goPackage.FindAllSubmatch(read(name), -1)
			if len(matches) != 1 {
				return fmt.Errorf("%s: want exactly one go_package option, got %d", name, len(matches))
			}
			pkg := strings.SplitN(string(matches[0][1]), ";", 2)[0]
			if !strings.HasPrefix(pkg, modulePath+"/") {
				return fmt.Errorf("%s: go_package %q is outside %s", name, pkg, modulePath)
			}
			dir := strings.TrimPrefix(pkg, modulePath+"/")
			if path.Clean(dir) != dir || strings.HasPrefix(dir, "../") {
				return fmt.Errorf("%s: invalid go_package directory %q", name, dir)
			}
			packageDirs[dir] = true
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	tracked := make(map[string]bool)
	for _, name := range bytes.Split(run(root, "git", "ls-files", "-z"), []byte{0}) {
		tracked[string(name)] = true
	}
	isGeneratedGo := func(name string) bool {
		return strings.HasSuffix(name, ".pb.go") || strings.HasSuffix(name, ".pb.gw.go")
	}
	produced := make(map[string]bool)
	err = filepath.WalkDir(output, func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !isGeneratedGo(name) {
			return nil
		}
		relative, err := filepath.Rel(output, name)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if !strings.HasPrefix(relative, modulePath+"/") {
			t.Errorf("%s: generated output is outside the repository module", relative)
			return nil
		}
		repoPath := strings.TrimPrefix(relative, modulePath+"/")
		if !packageDirs[path.Dir(repoPath)] {
			t.Errorf("%s: generated output is outside the inputs' go_package directories", repoPath)
		}
		produced[repoPath] = true
		if !tracked[repoPath] {
			t.Errorf("%s: generated file is missing from checked-in Go", repoPath)
			return nil
		}
		checkedIn, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(repoPath)))
		if err != nil {
			t.Errorf("%s: read checked-in Go: %v", repoPath, err)
			return nil
		}
		if generated := read(name); !bytes.Equal(generated, checkedIn) {
			first := 0
			for first < len(generated) && first < len(checkedIn) && generated[first] == checkedIn[first] {
				first++
			}
			start := max(0, first-80)
			t.Errorf("%s: generated Go differs byte for byte (generated %d bytes, checked-in %d bytes); first difference at byte %d\n generated: %q\nchecked-in: %q",
				repoPath, len(generated), len(checkedIn), first,
				generated[start:min(len(generated), first+160)], checkedIn[start:min(len(checkedIn), first+160)])
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk generated Go: %v", err)
	}
	if len(produced) == 0 {
		t.Error("buf generate produced no Go files")
	}
	for name := range tracked {
		if isGeneratedGo(name) && packageDirs[path.Dir(name)] && !produced[name] {
			t.Errorf("%s: orphaned checked-in Go was not produced by buf generate", name)
		}
	}
}
