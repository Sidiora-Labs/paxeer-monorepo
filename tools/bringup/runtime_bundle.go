package bringup

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

//go:embed runtime-bundle.json
var runtimeBundleDeclaration []byte

//go:embed runtime-dependencies.json
var runtimeDependencyDeclaration []byte

type runtimeSourcePackage struct {
	Path         string            `json:"path"`
	ImportPath   string            `json:"import_path"`
	Imports      []string          `json:"imports"`
	Dependencies []string          `json:"dependencies"`
	Files        map[string]string `json:"files"`
	Inputs       []string          `json:"inputs"`
}
type runtimeSourceGraph struct {
	Version   int                    `json:"version"`
	Producers []string               `json:"producers"`
	Manifests map[string]string      `json:"manifests"`
	Packages  []runtimeSourcePackage `json:"packages"`
}

type RuntimePlan struct {
	Version   int      `json:"version"`
	Bundle    string   `json:"bundle"`
	Names     []string `json:"names"`
	GoPaths   []string `json:"go_paths"`
	Libraries []string `json:"libraries"`
}

func ReadRuntimePlan(r io.Reader) (RuntimePlan, error) {
	var plan RuntimePlan
	decoder := json.NewDecoder(r)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&plan); err != nil {
		return plan, err
	}
	if plan.Version != 2 || plan.Bundle != "merged-runtime" || len(plan.Names) != 3 || plan.Names[0] != "paxd" || plan.Names[1] != "attestor" || plan.Names[2] != "bringup-inspect" || len(plan.Libraries) != 3 {
		return plan, errors.New("retired or incomplete runtime bundle")
	}
	expectedLibraries := []string{"wasm-runtime/internal/api/libwasmvm.x86_64.so", "wasm/x/wasm/artifacts/v152/api/libwasmvm152.x86_64.so", "wasm/x/wasm/artifacts/v155/api/libwasmvm155.x86_64.so"}
	for i, path := range expectedLibraries {
		if plan.Libraries[i] != path {
			return plan, errors.New("runtime library inventory differs")
		}
	}
	if len(plan.GoPaths) == 0 {
		return plan, errors.New("missing runtime source closure")
	}
	for _, path := range plan.GoPaths {
		if path == "custodyproof" || strings.Contains(path, "checkpoint-settlement") {
			return plan, errors.New("retired runtime source")
		}
	}
	return plan, nil
}

// ValidateRuntimeSources consumes the real go list -deps graph recorded by
// tests/runtime-dependencies.py. Digests bind its import resolution to the
// producer sources and module declarations; verification needs no toolchain.
func ValidateRuntimeSources(plan RuntimePlan, root string) error {
	return validateRuntimeSourceGraph(plan, root, runtimeDependencyDeclaration)
}

func validateRuntimeSourceGraph(plan RuntimePlan, root string, declaration []byte) error {
	covered := func(path string) bool {
		for _, p := range plan.GoPaths {
			if path == p || strings.HasPrefix(path, p+"/") {
				return true
			}
		}
		return false
	}
	for _, p := range plan.GoPaths {
		if filepath.IsAbs(p) || filepath.Clean(p) != p || p == "." || strings.HasPrefix(p, "../") {
			return errors.New("invalid runtime source path")
		}
		if _, e := os.Stat(filepath.Join(root, p)); e != nil {
			return e
		}
	}
	for _, p := range []string{"chain.mk", "Makefile", "rust-toolchain.toml", "tools/build/sanitizers.mk", "tools/paxeer-x/build", "services/platform/Makefile.inc", "spec/layerx-merge/contracts/address-table.md"} {
		if !covered(p) {
			return errors.New("native build dependency missing from source closure: " + p)
		}
	}
	var graph runtimeSourceGraph
	decoder := json.NewDecoder(strings.NewReader(string(declaration)))
	decoder.DisallowUnknownFields()
	if e := decoder.Decode(&graph); e != nil {
		return e
	}
	if graph.Version != 1 || len(graph.Producers) != 3 || len(graph.Packages) == 0 || len(graph.Manifests) == 0 {
		return errors.New("recorded producer import graph incomplete")
	}
	checkDigest := func(path, expected string) error {
		if !covered(path) {
			return errors.New("Go dependency missing from source closure: " + path)
		}
		if filepath.IsAbs(path) || filepath.Clean(path) != path || strings.HasPrefix(path, "../") {
			return errors.New("invalid recorded dependency path")
		}
		raw, e := os.ReadFile(filepath.Join(root, path))
		if e != nil {
			return e
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != expected {
			return errors.New("producer import graph stale; regenerate with go list -deps: " + path)
		}
		return nil
	}
	for path, digest := range graph.Manifests {
		if e := checkDigest(path, digest); e != nil {
			return e
		}
	}
	packages := map[string]bool{}
	for _, pkg := range graph.Packages {
		if !covered(pkg.Path) {
			return errors.New("Go dependency missing from source closure: " + pkg.Path)
		}
		if packages[pkg.Path] || pkg.ImportPath == "" || len(pkg.Files) == 0 {
			return errors.New("invalid recorded dependency package")
		}
		packages[pkg.Path] = true
		for name, digest := range pkg.Files {
			if e := checkDigest(filepath.Join(pkg.Path, name), digest); e != nil {
				return e
			}
		}
		for _, name := range pkg.Inputs {
			path := filepath.Join(pkg.Path, name)
			if !covered(path) {
				return errors.New("package input missing from source closure: " + path)
			}
			if _, e := os.Stat(filepath.Join(root, path)); e != nil {
				return e
			}
		}
	}
	for _, pkg := range graph.Packages {
		for _, dependency := range pkg.Dependencies {
			if !packages[dependency] {
				return errors.New("recorded Go dependency absent: " + dependency)
			}
		}
	}
	for i, producer := range []string{"daemon/paxd", "services/wallet/attestor/cmd/attestor", "tools/bringup/cmd/bringup-inspect"} {
		if graph.Producers[i] != producer || !packages[producer] {
			return errors.New("recorded producer root differs")
		}
	}
	return nil
}

func RuntimeBundlePlan() (RuntimePlan, error) {
	return ReadRuntimePlan(strings.NewReader(string(runtimeBundleDeclaration)))
}
