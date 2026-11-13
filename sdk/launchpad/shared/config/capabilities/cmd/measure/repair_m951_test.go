package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
)

var repairM951Sources = []string{
	"sdk/launchpad/shared/config/capabilities/measurement.go",
	"sdk/launchpad/shared/config/capabilities/cmd/measure/main.go",
	"sdk/launchpad/shared/config/capabilities/cmd/measure/source.go",
	"sdk/launchpad/shared/config/capabilities/catalog.go",
	"sdk/launchpad/shared/config/capabilities/preflight.go",
	"sdk/launchpad/shared/config/deployment/attest.go",
	"sdk/launchpad/shared/config/deployment/manifest.go",
	"sdk/launchpad/shared/config/deployment/revision.go",
	"sdk/launchpad/shared/config/deployment/public.go",
	"sdk/launchpad/shared/config/deployment/current.json",
	"sdk/launchpad/shared/config/capabilities/frozen.json",
	"sdk/launchpad/shared/config/capabilities/contract-support.json",
	"sdk/launchpad/shared/chain/readpolicy/policy.go",
	"sdk/launchpad/shared/go.mod",
	"sdk/launchpad/shared/go.sum",
	"spec/layerx-merge/spec.kvx",
	"spec/layerx-merge/contracts/launchpad.md",
	"sdk/launchpad/protocol/addresses.go",
	"apps/launchpad/web/src/core/network/deployment.ts",
	"sdk/launchpad/shared/config/capabilities/cmd/measure/repair_m951_test.go",
}

func repairM951Root(t *testing.T) string {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("test source location unavailable")
	}
	for root := filepath.Dir(source); ; root = filepath.Dir(root) {
		if _, err := os.Stat(filepath.Join(root, "spec/layerx-merge/spec.kvx")); err == nil {
			return root
		}
		if filepath.Dir(root) == root {
			t.Fatal("repository programme authority unavailable")
		}
	}
}

func repairM951Digest(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

func repairM951Issue(t *testing.T, err error, code string) {
	t.Helper()
	var issue deployment.Issue
	if !errors.As(err, &issue) || issue.Code != code {
		t.Fatalf("got %v, want %s", err, code)
	}
}

func repairM951Index(t *testing.T, root string) (sourceIndex, string, string) {
	t.Helper()
	output := filepath.Join(t.TempDir(), "source-index.json")
	if err := writeSourceIndex(root, output, repairM951Sources[len(repairM951Sources)-1:]); err != nil {
		t.Fatalf("produce candidate index: %v", err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	digest := repairM951Digest(data)
	pinned, err := readPinned(output, digest, 8<<20)
	if err != nil {
		t.Fatal(err)
	}
	var index sourceIndex
	if err := decode(pinned, &index); err != nil {
		t.Fatal(err)
	}
	if err := validateSource(root, index); err != nil {
		t.Fatalf("consume generated candidate index: %v", err)
	}
	return index, output, digest
}

func repairM951Candidate(t *testing.T) string {
	t.Helper()
	repository := repairM951Root(t)
	root := t.TempDir()
	for _, path := range repairM951Sources {
		data, err := os.ReadFile(filepath.Join(repository, path))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, path)), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, path), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestRepairM_951_CurrentAuthorityLayout(t *testing.T) {
	root := repairM951Root(t)
	if _, err := os.Stat(filepath.Join(root, "spec/production-system/spec.kvx")); !os.IsNotExist(err) {
		t.Fatalf("obsolete authority should be absent from this checkout: %v", err)
	}
	index, output, _ := repairM951Index(t, root)
	if index.SchemaVersion != 1 || len(index.Files) != len(repairM951Sources) {
		t.Fatalf("candidate inputs omitted: %+v", index)
	}
	actual := map[string]string{}
	for _, file := range index.Files {
		if _, exists := actual[file.Path]; exists {
			t.Fatalf("duplicate generated source: %s", file.Path)
		}
		actual[file.Path] = file.SHA256
	}
	for _, path := range repairM951Sources {
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			t.Fatal(err)
		}
		if actual[path] != repairM951Digest(data) {
			t.Fatalf("producer did not hash the complete current input %s", path)
		}
	}
	if _, exists := actual["spec/production-system/spec.kvx"]; exists {
		t.Fatal("producer requires the retired authority")
	}
	info, err := os.Stat(output)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("source index is not private: %v, %v", info, err)
	}
	before, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	repairM951Issue(t, writeSourceIndex(root, output, nil), "candidate_source_index_output_unavailable")
	after, err := os.ReadFile(output)
	if err != nil || string(before) != string(after) {
		t.Fatalf("producer overwrote an existing pinned index: %v", err)
	}
}

func TestRepairM_951_CommandProducesPinnedIndex(t *testing.T) {
	root := repairM951Root(t)
	output := filepath.Join(t.TempDir(), "source-index.json")
	previousFlags, previousArgs := flag.CommandLine, os.Args
	t.Cleanup(func() {
		flag.CommandLine, os.Args = previousFlags, previousArgs
	})
	flag.CommandLine = flag.NewFlagSet("measure", flag.ContinueOnError)
	os.Args = []string{"measure", "-write-source-index", "-source-root", root, "-source-index", output, repairM951Sources[len(repairM951Sources)-1]}
	if err := run(); err != nil {
		t.Fatalf("source-index command failed: %v", err)
	}
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	pinned, err := readPinned(output, repairM951Digest(data), 8<<20)
	if err != nil {
		t.Fatal(err)
	}
	var index sourceIndex
	if err := decode(pinned, &index); err != nil {
		t.Fatal(err)
	}
	if len(index.Files) != len(repairM951Sources) {
		t.Fatalf("command omitted candidate inputs: %+v", index)
	}
	if err := validateSource(root, index); err != nil {
		t.Fatalf("measurement consumer rejected command output: %v", err)
	}
}

func TestRepairM_951_CompleteCandidateHashVerification(t *testing.T) {
	root := repairM951Candidate(t)
	index, _, _ := repairM951Index(t, root)
	for _, file := range index.Files {
		t.Run(file.Path, func(t *testing.T) {
			path := filepath.Join(root, file.Path)
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, append(append([]byte(nil), original...), '\n'), 0600); err != nil {
				t.Fatal(err)
			}
			repairM951Issue(t, validateSource(root, index), "candidate_source_changed")
			if err := os.WriteFile(path, original, 0600); err != nil {
				t.Fatal(err)
			}
			if err := validateSource(root, index); err != nil {
				t.Fatalf("restored candidate rejected: %v", err)
			}
		})
	}
}

func TestRepairM_951_EveryAuthorityRequired(t *testing.T) {
	root := repairM951Candidate(t)
	index, _, _ := repairM951Index(t, root)
	for i, file := range index.Files[:len(index.Files)-1] {
		t.Run(file.Path, func(t *testing.T) {
			missing := sourceIndex{SchemaVersion: 1, Files: append([]sourceFile(nil), index.Files[:i]...)}
			missing.Files = append(missing.Files, index.Files[i+1:]...)
			repairM951Issue(t, validateSource(root, missing), "candidate_source_authority_missing")
		})
	}
	for _, path := range []string{"spec/layerx-merge/spec.kvx", "spec/layerx-merge/contracts/launchpad.md"} {
		t.Run("unavailable/"+path, func(t *testing.T) {
			full := filepath.Join(root, path)
			original, err := os.ReadFile(full)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(full); err != nil {
				t.Fatal(err)
			}
			repairM951Issue(t, validateSource(root, index), "candidate_source_changed")
			output := filepath.Join(t.TempDir(), "source-index.json")
			repairM951Issue(t, writeSourceIndex(root, output, nil), "candidate_source_changed")
			if _, err := os.Stat(output); !os.IsNotExist(err) {
				t.Fatalf("missing authority produced evidence: %v", err)
			}
			if err := os.WriteFile(full, original, 0600); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRepairM_951_ObsoleteAuthorityCannotSubstitute(t *testing.T) {
	root := repairM951Candidate(t)
	index, _, _ := repairM951Index(t, root)
	programme, err := os.ReadFile(filepath.Join(root, "spec/layerx-merge/spec.kvx"))
	if err != nil {
		t.Fatal(err)
	}
	obsolete := "spec/production-system/spec.kvx"
	if err := os.MkdirAll(filepath.Dir(filepath.Join(root, obsolete)), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, obsolete), programme, 0600); err != nil {
		t.Fatal(err)
	}
	for _, current := range []string{"spec/layerx-merge/spec.kvx", "spec/layerx-merge/contracts/launchpad.md"} {
		legacy := sourceIndex{SchemaVersion: 1}
		for _, file := range index.Files {
			if file.Path != current {
				legacy.Files = append(legacy.Files, file)
			}
		}
		legacy.Files = append(legacy.Files, sourceFile{Path: obsolete, SHA256: repairM951Digest(programme)})
		repairM951Issue(t, validateSource(root, legacy), "candidate_source_authority_missing")
	}
}

func TestRepairM_951_PinnedIndexAndPathGuards(t *testing.T) {
	root := repairM951Candidate(t)
	index, output, digest := repairM951Index(t, root)
	data, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(output, append(data, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	_, err = readPinned(output, digest, 8<<20)
	repairM951Issue(t, err, "pinned_file_digest_mismatch")
	for _, path := range []string{"", "../outside", "..", "/outside", "sdk/../outside", index.Files[0].Path} {
		t.Run("invalid/"+path, func(t *testing.T) {
			invalid := sourceIndex{SchemaVersion: 1, Files: append([]sourceFile(nil), index.Files...)}
			invalid.Files = append(invalid.Files, sourceFile{Path: path, SHA256: strings.Repeat("a", 64)})
			repairM951Issue(t, validateSource(root, invalid), "candidate_source_path_invalid")
			destination := filepath.Join(t.TempDir(), "index.json")
			repairM951Issue(t, writeSourceIndex(root, destination, []string{path}), "candidate_source_path_invalid")
			if _, err := os.Stat(destination); !os.IsNotExist(err) {
				t.Fatalf("invalid path produced evidence: %v", err)
			}
		})
	}
	for _, invalid := range []sourceIndex{{}, {SchemaVersion: 2, Files: index.Files}, {SchemaVersion: 1}, {SchemaVersion: 1, Files: make([]sourceFile, 10001)}} {
		repairM951Issue(t, validateSource(root, invalid), "candidate_source_index_invalid")
	}
	badDigest := sourceIndex{SchemaVersion: 1, Files: append([]sourceFile(nil), index.Files...)}
	badDigest.Files[len(badDigest.Files)-1].SHA256 = strings.Repeat("0", 64)
	repairM951Issue(t, validateSource(root, badDigest), "candidate_source_changed")
}
