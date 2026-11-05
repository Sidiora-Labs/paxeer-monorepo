package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// Fixtures are committed through go-git; no Git executable or external service
// participates in the producer, on-disk publication, reopen, or setup consumer.
func writeM855(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func readM855(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func absentM855(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("expected %s absent, got %v", path, err)
	}
}

func commitM855(t *testing.T, repo *git.Repository, files map[string]string) plumbing.Hash {
	t.Helper()
	wt, err := repo.Worktree()
	if err != nil {
		t.Fatal(err)
	}
	for path, content := range files {
		writeM855(t, path, content)
		if _, err := wt.Add(path); err != nil {
			t.Fatal(err)
		}
	}
	hash, err := wt.Commit("recorded release fixture", &git.CommitOptions{Author: &object.Signature{Name: "Release fixture", Email: "fixture@example.invalid", When: time.Unix(1700000000, 0)}})
	if err != nil {
		t.Fatal(err)
	}
	return hash
}

func repoM855(t *testing.T) *git.Repository {
	t.Helper()
	t.Chdir(t.TempDir())
	repo, err := git.PlainInit(".", false)
	if err != nil {
		t.Fatal(err)
	}
	commitM855(t, repo, map[string]string{
		tagFile:                        "v6.3\n",
		commonDir + "/precompiles.go":  "package common\nconst CommonValue = 1\n",
		commonDir + "/evm_events.go":   "package common\nconst EventValue = 1\n",
		"precompiles/bank/bank.go":     "package bank\nimport common \"" + commonPkgPath + "\"\nconst Bank = common.CommonValue\n",
		"precompiles/oracle/oracle.go": "package oracle\nconst Oracle = 1\n",
		"precompiles/bank/abi.json":    `{"name":"bank"}`,
		"precompiles/oracle/abi.json":  `{"name":"oracle"}`,
		"precompiles/bank/versions":    "v6.4\n",
		"precompiles/oracle/versions":  "v6.4\n",
	})
	commitM855(t, repo, map[string]string{tagFile: "v6.3\nv6.4\n"})
	return repo
}

func docsM855(t *testing.T, phrases ...string) {
	t.Helper()
	text := readM855(t, "README.md")
	for _, phrase := range phrases {
		if !strings.Contains(text, phrase) {
			t.Errorf("recovery runbook missing %q", phrase)
		}
	}
}

func TestRepairM_855_InterruptedArchiveResumes(t *testing.T) {
	docsM855(t, ".complete.json", ".pending", "git status --short", "partial", "go run ./scripts/bump_version")
	for _, boundary := range []string{"common", "module_archives", "first_registration", "before_marker"} {
		t.Run(boundary, func(t *testing.T) {
			repo := repoM855(t)
			commitM855(t, repo, map[string]string{commonDir + "/precompiles.go": "package common\nconst CommonValue = 2\n"})
			writeM855(t, tagFile, "v6.3\nv6.4\nv6.5\n")
			modules, err := discoverModules()
			if err != nil {
				t.Fatal(err)
			}
			pending, manifest, err := stageArchive(modules, "v6.5", "v65", commonPkgPath+"/legacy/v65")
			if err != nil {
				t.Fatal(err)
			}
			n := 1
			if boundary == "module_archives" {
				n = len(manifest.Files) - 2
			}
			if boundary == "first_registration" {
				n = len(manifest.Files) - 1
			}
			if boundary == "before_marker" {
				n = len(manifest.Files)
			}
			// Stop between real publication operations, then discard the producer's
			// in-memory state. run must reopen its on-disk manifest to recover.
			for _, file := range manifest.Files[:n] {
				if err := publishArchiveFile(pending, file); err != nil {
					t.Fatal(err)
				}
			}
			manifest = nil
			repo = nil
			_, marker := archivePaths("v65")
			absentM855(t, marker)
			absentM855(t, "precompiles/bank/setup.go")
			absentM855(t, "precompiles/oracle/setup.go")
			// Recovery must use the recorded snapshot, rather than mixing in new code.
			writeM855(t, commonDir+"/precompiles.go", "package common\nconst CommonValue = 999\n")
			if err := run(); err != nil {
				t.Fatalf("reopen/resume: %v", err)
			}
			if got := readM855(t, commonDir+"/legacy/v65/precompiles.go"); !strings.Contains(got, "CommonValue = 2") || !strings.Contains(got, "package v65") {
				t.Fatalf("wrong recovered common: %s", got)
			}
			bank := readM855(t, "precompiles/bank/legacy/v65/bank.go")
			if !strings.Contains(bank, commonPkgPath+"/legacy/v65") || !strings.Contains(bank, "package v65") {
				t.Fatalf("wrong import/package rewrite: %s", bank)
			}
			for _, mod := range []string{"bank", "oracle"} {
				if got := readM855(t, "precompiles/"+mod+"/versions"); got != "v6.4\nv6.5\n" {
					t.Fatalf("%s registrations: %q", mod, got)
				}
				if got := readM855(t, "precompiles/"+mod+"/legacy/v65/abi.json"); got != `{"name":"`+mod+`"}` {
					t.Fatalf("%s ABI: %q", mod, got)
				}
				setup := readM855(t, "precompiles/"+mod+"/setup.go")
				if !strings.Contains(setup, `"v6.4":`) || !strings.Contains(setup, "latestUpgrade:") {
					t.Fatalf("setup consumer did not register versions: %s", setup)
				}
			}
			var completed archiveManifest
			if err := json.Unmarshal([]byte(readM855(t, marker)), &completed); err != nil {
				t.Fatal(err)
			}
			if err := validatePublishedArchive(&completed); err != nil {
				t.Fatal(err)
			}
			absentM855(t, pending)
			before := readM855(t, marker)
			if err := run(); err != nil {
				t.Fatalf("completed rerun: %v", err)
			}
			if got := readM855(t, marker); got != before {
				t.Fatal("completed rerun changed marker")
			}
		})
	}
}

func TestRepairM_855_ArchiveSetValidatedBeforePublication(t *testing.T) {
	for _, defect := range []string{"module_syntax", "duplicate_registration", "unreadable_abi"} {
		t.Run(defect, func(t *testing.T) {
			repo := repoM855(t)
			commitM855(t, repo, map[string]string{commonDir + "/precompiles.go": "package common\nconst CommonValue = 2\n"})
			writeM855(t, tagFile, "v6.3\nv6.4\nv6.5\n")
			switch defect {
			case "module_syntax":
				writeM855(t, "precompiles/oracle/oracle.go", "package oracle\nfunc broken(\n")
			case "duplicate_registration":
				writeM855(t, "precompiles/oracle/versions", "v6.4\nv6.5\n")
			case "unreadable_abi":
				if err := os.Remove("precompiles/oracle/abi.json"); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir("precompiles/oracle/abi.json", 0750); err != nil {
					t.Fatal(err)
				}
			}
			if err := run(); err == nil {
				t.Fatal("invalid complete archive set accepted")
			}
			absentM855(t, commonDir+"/legacy/v65")
			absentM855(t, "precompiles/bank/legacy/v65")
			absentM855(t, "precompiles/oracle/legacy/v65")
			absentM855(t, "precompiles/bank/setup.go")
			if got := readM855(t, "precompiles/bank/versions"); got != "v6.4\n" {
				t.Fatalf("earlier module modified: %q", got)
			}
			pending, _ := archivePaths("v65")
			absentM855(t, pending)
		})
	}
}

func TestRepairM_855_ExactTagLineSelectsBaseline(t *testing.T) {
	repo := repoM855(t)
	commitM855(t, repo, map[string]string{tagFile: "v6.40\nv6.4.0\n"})
	commitM855(t, repo, map[string]string{"precompiles/bank/bank.go": "package bank\nconst Bank = 2\n"})
	introduced := commitM855(t, repo, map[string]string{tagFile: "v6.40\nv6.4.0\n \tv6.4 \n\n"})
	commitM855(t, repo, map[string]string{"precompiles/oracle/oracle.go": "package oracle\nconst Oracle = 2\n"})
	reopened, err := git.PlainOpen(".")
	if err != nil {
		t.Fatal(err)
	}
	base, err := gitFindCommit(reopened, "v6.4")
	if err != nil {
		t.Fatal(err)
	}
	if base.Hash != introduced {
		t.Fatalf("substring baseline %s instead of exact introduction %s", base.Hash, introduced)
	}
	// The release's current tag list is separate from its historical content.
	writeM855(t, tagFile, "v6.4\nv6.5\n")
	if err := run(); err != nil {
		t.Fatal(err)
	}
	absentM855(t, "precompiles/bank/legacy/v65")
	if got := readM855(t, "precompiles/bank/versions"); got != "v6.4\n" {
		t.Fatalf("unchanged bank registered: %q", got)
	}
	if got := readM855(t, "precompiles/oracle/legacy/v65/oracle.go"); !strings.Contains(got, "Oracle = 2") {
		t.Fatalf("changed oracle omitted: %s", got)
	}
}

func removeObjectM855(t *testing.T, hash plumbing.Hash) {
	t.Helper()
	value := hash.String()
	if err := os.Remove(filepath.Join(".git", "objects", value[:2], value[2:])); err != nil {
		t.Fatal(err)
	}
}

func TestRepairM_855_HistoryErrorsRefusePublication(t *testing.T) {
	docsM855(t, "exact trimmed tag line", "missing history", "nonzero", "restore readable history", "before any archive")
	for _, defect := range []string{"content", "tree", "tag_subtree", "comparison_subtree", "iterator", "shallow", "missing_tag", "oversized_line"} {
		t.Run(defect, func(t *testing.T) {
			repo := repoM855(t)
			head, err := repo.Head()
			if err != nil {
				t.Fatal(err)
			}
			base, err := repo.CommitObject(head.Hash())
			if err != nil {
				t.Fatal(err)
			}
			blob, err := base.File(tagFile)
			if err != nil {
				t.Fatal(err)
			}
			// The HEAD candidate remains readable; errors deeper in history must
			// invalidate it rather than returning that already-matching candidate.
			commitM855(t, repo, map[string]string{tagFile: "v6.3\n v6.4 \n", "precompiles/bank/bank.go": "package bank\nconst Bank = 2\n"})
			expected := "walking tag history"
			switch defect {
			case "content":
				removeObjectM855(t, blob.Hash)
			case "tree":
				removeObjectM855(t, base.TreeHash)
			case "tag_subtree", "comparison_subtree":
				tree, err := base.Tree()
				if err != nil {
					t.Fatal(err)
				}
				path := "node"
				if defect == "comparison_subtree" {
					path = commonDir
					expected = "reading base subtree"
				}
				entry, err := tree.FindEntry(path)
				if err != nil {
					t.Fatal(err)
				}
				removeObjectM855(t, entry.Hash)
			case "iterator":
				removeObjectM855(t, base.Hash)
			case "shallow":
				if err := repo.Storer.SetShallow([]plumbing.Hash{base.Hash}); err != nil {
					t.Fatal(err)
				}
			case "missing_tag":
				commitM855(t, repo, map[string]string{tagFile: "v6.40\n"})
				expected = "exact tag v6.4"
			case "oversized_line":
				commitM855(t, repo, map[string]string{tagFile: "v6.4\n" + strings.Repeat("x", 70000) + "\n"})
			}
			writeM855(t, tagFile, "v6.3\nv6.4\nv6.5\n")
			err = run()
			if err == nil || !strings.Contains(err.Error(), expected) {
				t.Fatalf("%s: wanted %q refusal, got %v", defect, expected, err)
			}
			absentM855(t, commonDir+"/legacy/v65")
			absentM855(t, "precompiles/bank/legacy/v65")
			absentM855(t, "precompiles/bank/setup.go")
			if got := readM855(t, "precompiles/bank/versions"); got != "v6.4\n" {
				t.Fatalf("history error modified versions: %q", got)
			}
		})
	}
}

func TestRepairM_855_UnmarkedAndDamagedArchivesRefused(t *testing.T) {
	for _, defect := range []string{"unmarked_common", "unmarked_module", "staged_payload", "registration_conflict", "completed_payload", "completed_registration"} {
		t.Run(defect, func(t *testing.T) {
			repoM855(t)
			writeM855(t, tagFile, "v6.3\nv6.4\nv6.5\n")
			pending, marker := archivePaths("v65")
			if strings.HasPrefix(defect, "unmarked") {
				mod := "common"
				if defect == "unmarked_module" {
					mod = "bank"
				}
				writeM855(t, "precompiles/"+mod+"/legacy/v65/partial.go", "package v65\n")
			} else {
				var err error
				var manifest *archiveManifest
				pending, manifest, err = stageArchive([]string{"bank", "oracle"}, "v6.5", "v65", commonPkgPath+"/legacy/v65")
				if err != nil {
					t.Fatal(err)
				}
				switch defect {
				case "staged_payload":
					writeM855(t, filepath.Join(pending, "files", "precompiles/oracle/legacy/v65/oracle.go"), "package corrupt\n")
				case "registration_conflict":
					writeM855(t, "precompiles/oracle/versions", "v6.4\nv6.6\n")
				default:
					if err := publishArchive(pending, manifest); err != nil {
						t.Fatal(err)
					}
					if defect == "completed_payload" {
						writeM855(t, "precompiles/oracle/legacy/v65/oracle.go", "package corrupt\n")
					} else {
						writeM855(t, "precompiles/oracle/versions", "v6.4\n")
					}
				}
			}
			if err := run(); err == nil {
				t.Fatalf("accepted %s", defect)
			}
			absentM855(t, "precompiles/bank/setup.go")
			if !strings.HasPrefix(defect, "completed") {
				absentM855(t, marker)
			}
			if defect == "staged_payload" || defect == "registration_conflict" {
				absentM855(t, commonDir+"/legacy/v65")
			}
		})
	}
}

func TestRepairM_855_NormalizationAndNoChanges(t *testing.T) {
	for _, changed := range []bool{false, true} {
		t.Run(map[bool]string{false: "unchanged", true: "changed"}[changed], func(t *testing.T) {
			repo := repoM855(t)
			if changed {
				commitM855(t, repo, map[string]string{"precompiles/bank/bank.go": "package bank\nconst Bank = 2\n"})
			}
			writeM855(t, tagFile, "v6.3\nv6.4\nv6.5.2\n")
			if err := run(); err != nil {
				t.Fatal(err)
			}
			if got := readM855(t, tagFile); got != "v6.3\nv6.4\nv6.5\n" {
				t.Fatalf("normalized tags: %q", got)
			}
			_, marker := archivePaths("v65")
			_ = readM855(t, marker)
			if changed {
				_ = readM855(t, "precompiles/bank/legacy/v65/bank.go")
			} else {
				absentM855(t, "precompiles/bank/legacy/v65")
			}
			absentM855(t, "precompiles/oracle/legacy/v65")
			if err := run(); err != nil {
				t.Fatalf("idempotent normalized release: %v", err)
			}
		})
	}
}

func TestRepairM_855_GenuineMissingPathsRemainSupported(t *testing.T) {
	t.Chdir(t.TempDir())
	repo, err := git.PlainInit(".", false)
	if err != nil {
		t.Fatal(err)
	}
	root := commitM855(t, repo, map[string]string{"initial.txt": "before tags and precompiles\n"})
	introduced := commitM855(t, repo, map[string]string{tagFile: "v6.4\n", commonDir + "/precompiles.go": "package common\nconst Value = 1\n"})
	base, err := gitFindCommit(repo, "v6.4")
	if err != nil {
		t.Fatal(err)
	}
	if base.Hash != introduced {
		t.Fatalf("wrong introduction: %s, want %s", base.Hash, introduced)
	}
	before, err := repo.CommitObject(root)
	if err != nil {
		t.Fatal(err)
	}
	changed, err := gitPathHasChanges(repo, before, commonDir)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Fatal("genuinely new subtree was not selected")
	}
}
