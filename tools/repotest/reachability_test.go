package repotest_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func reachabilityFiles(t *testing.T, root string, suffixes ...string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		for _, suffix := range suffixes {
			if filepath.Ext(entry.Name()) == suffix || entry.Name() == suffix {
				paths = append(paths, filepath.ToSlash(path))
				break
			}
		}
		return nil
	})
	if err != nil {
		t.Fatalf("scan %s: %v", root, err)
	}
	return paths
}

func reachabilityRead(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return data
}

func TestEveryCTestMainHasBuildReference(t *testing.T) {
	buildPaths := []string{"Makefile", "chain.mk"}
	for _, root := range []string{"tools", "scripts", "tests"} {
		buildPaths = append(buildPaths, reachabilityFiles(t, root, ".sh", ".py", ".mk")...)
	}
	buildSources := make([][]byte, 0, len(buildPaths))
	for _, path := range buildPaths {
		buildSources = append(buildSources, reachabilityRead(t, path))
	}
	makefile := reachabilityRead(t, "Makefile")
	proofBuildSources := [][]byte{makefile}
	proofInclude := regexp.MustCompile(`(?m)^include[\t ]+(tools/paxeer-x/build/[^\s]+\.mk)[\t ]*$`)
	for _, include := range proofInclude.FindAllSubmatch(makefile, -1) {
		proofBuildSources = append(proofBuildSources, reachabilityRead(t, string(include[1])))
	}

	nonCode := regexp.MustCompile(`(?s)/\*.*?\*/|//[^\n]*|"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'`)
	mainDefinition := regexp.MustCompile(`\bint\s+main\s*\([^)]*\)\s*\{`)
	programs := 0
	for _, path := range reachabilityFiles(t, "tests", ".c") {
		if strings.HasPrefix(path, "tests/arith/") || strings.HasPrefix(path, "tests/codec/") ||
			strings.HasPrefix(path, "tests/crypto/") || strings.HasPrefix(path, "tests/fuzz/") {
			continue
		}
		source := nonCode.ReplaceAll(reachabilityRead(t, path), []byte(" "))
		if !mainDefinition.Match(source) {
			continue
		}
		programs++
		stem := strings.TrimSuffix(filepath.Base(path), ".c")
		reference := regexp.MustCompile(`\b(?:` + regexp.QuoteMeta(path) + `|` + regexp.QuoteMeta(stem) + `)\b`)
		sources := buildSources
		locations := "Makefile, chain.mk, tools/, scripts/ or tests/"
		if strings.HasPrefix(path, "tests/proof/") {
			sources = proofBuildSources
			locations = "Makefile or a tools/paxeer-x/build fragment included by Makefile"
		}
		referenced := false
		for _, source := range sources {
			if reference.Match(source) {
				referenced = true
				break
			}
		}
		if !referenced {
			t.Errorf("%s defines int main but has no whole-word path or stem build reference in %s", path, locations)
		}
	}
	if programs == 0 {
		t.Fatal("no C test programs found under tests/")
	}
}

func TestEveryRustTestFileIsCompiled(t *testing.T) {
	testPaths := reachabilityFiles(t, "tests", ".rs")
	if len(testPaths) == 0 {
		t.Fatal("no Rust test files found under tests/")
	}
	cargoRoots := make(map[string]bool)
	for _, path := range reachabilityFiles(t, "tests", "Cargo.toml") {
		cargoRoots[filepath.Dir(path)] = true
	}
	stringLiteral := regexp.MustCompile(`"(?:\\.|[^"\\])*"`)
	tomlLiteral := regexp.MustCompile(`"(?:\\.|[^"\\])*"|'[^'\r\n]*'`)
	referenced := make(map[string]bool)
	for _, path := range reachabilityFiles(t, ".", ".rs", "Cargo.toml") {
		if strings.HasPrefix(path, "tests/") {
			continue
		}
		literals := stringLiteral
		if filepath.Base(path) == "Cargo.toml" {
			literals = tomlLiteral
		}
		for _, literal := range literals.FindAll(reachabilityRead(t, path), -1) {
			value := string(literal[1 : len(literal)-1])
			referenced[filepath.Base(value)] = true
		}
	}
	for _, path := range testPaths {
		inPackage := false
		for dir := filepath.Dir(path); dir != "tests" && dir != "."; dir = filepath.Dir(dir) {
			if cargoRoots[dir] {
				inPackage = true
				break
			}
		}
		if inPackage {
			continue
		}
		if !referenced[filepath.Base(path)] {
			t.Errorf("%s has no string literal reference in a Rust source or Cargo.toml outside tests/", path)
		}
	}
}
