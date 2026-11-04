package proposals

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepairM_365_ManifestJSONTail(t *testing.T) {
	raw, err := os.ReadFile(testManifest)
	if err != nil {
		t.Fatal(err)
	}
	for _, tail := range []string{"}", "]", " } ", " ] ", "{}", "null", "true", "0", "garbage", "{", "["} {
		t.Run(fmt.Sprintf("tail_%q", tail), func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "attestors.json")
			if err := os.WriteFile(path, append(append([]byte(nil), raw...), tail...), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := LoadAttestorManifest(path)
			refusal := fieldError(t, err)
			if refusal.File != path || refusal.Field != fieldDocument {
				t.Fatalf("wrong refusal: %v", refusal)
			}
			var report bytes.Buffer
			out := filepath.Join(root, "bodies")
			if err := Run([]string{"-manifest", path, ethereumPath, out}, &report); err == nil {
				t.Fatal("Run accepted a manifest with trailing input")
			}
			if _, err := os.Lstat(out); !os.IsNotExist(err) {
				t.Fatalf("refused manifest published bodies: %v", err)
			}
			if report.Len() != 0 {
				t.Fatalf("refused manifest reported outputs: %s", &report)
			}
		})
	}
	for _, tail := range []string{"", " \r\n\t "} {
		t.Run(fmt.Sprintf("accepted_%q", tail), func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "attestors.json")
			if err := os.WriteFile(path, append(append([]byte(nil), raw...), tail...), 0o644); err != nil {
				t.Fatal(err)
			}
			manifest := loadManifest(t, path)
			want := loadManifest(t, testManifest)
			if manifest.Threshold != want.Threshold || fmt.Sprint(manifest.Signers()) != fmt.Sprint(want.Signers()) {
				t.Fatal("whitespace changed manifest authority")
			}
			out := filepath.Join(root, "bodies")
			var report bytes.Buffer
			if err := Run([]string{"-manifest", path, ethereumPath, out}, &report); err != nil {
				t.Fatal(err)
			}
			assertRepairM365Bodies(t, out, generate(t, ethereumPath))
		})
	}
	TestManifestRefusals(t)
}

func TestRepairM_365_BridgeJSONTail(t *testing.T) {
	raw, err := os.ReadFile(ethereumPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, tail := range []string{"}", "]", " } ", " ] ", "{}", "null", "true", "0", "garbage", "{", "["} {
		t.Run(fmt.Sprintf("tail_%q", tail), func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "chain.json")
			if err := os.WriteFile(path, append(append([]byte(nil), raw...), tail...), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := LoadChainConfig(path)
			refusal := fieldError(t, err)
			if refusal.File != path || refusal.Field != fieldDocument {
				t.Fatalf("wrong refusal: %v", refusal)
			}
			out := filepath.Join(root, "bodies")
			var report bytes.Buffer
			if err := Run([]string{"-manifest", testManifest, path, out}, &report); err == nil {
				t.Fatal("Run accepted a chain with trailing input")
			}
			if _, err := os.Lstat(out); !os.IsNotExist(err) {
				t.Fatalf("refused chain published bodies: %v", err)
			}
			if report.Len() != 0 {
				t.Fatalf("refused chain reported outputs: %s", &report)
			}
		})
	}
	for _, tail := range []string{"", " \r\n\t "} {
		t.Run(fmt.Sprintf("accepted_%q", tail), func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "chain.json")
			if err := os.WriteFile(path, append(append([]byte(nil), raw...), tail...), 0o644); err != nil {
				t.Fatal(err)
			}
			out := filepath.Join(root, "bodies")
			var report bytes.Buffer
			if err := Run([]string{"-manifest", testManifest, path, out}, &report); err != nil {
				t.Fatal(err)
			}
			assertRepairM365Bodies(t, out, generate(t, ethereumPath))
		})
	}
	TestChainConfigurationRefusals(t)
}

func TestRepairM_365_OutputStagingFailure(t *testing.T) {
	testPublicationStagingFailures(t)
}

func TestRepairM_365_OutputRestartRecovery(t *testing.T) {
	testPublicationRestartRecovery(t)
}

func TestRepairM_365_InterruptedOutputsAreVerified(t *testing.T) {
	for _, corrupt := range []string{"published-body", "staged-proposal", "staged-readback", "unexpected-body", "symlink-body", "changed-request"} {
		t.Run(corrupt, func(t *testing.T) {
			fixture := newPublicationFixture(t, "solana")
			publication, err := stagePublication(fixture.outputs)
			if err != nil {
				t.Fatal(err)
			}
			if err := publication.Outputs[0].publish(); err != nil {
				t.Fatal(err)
			}
			body := filepath.Join(fixture.outputs[0].Path, fixture.outputs[0].Files[0].Name)
			original, err := os.ReadFile(body)
			if err != nil {
				t.Fatal(err)
			}
			var damaged string
			var restore []byte
			switch corrupt {
			case "published-body":
				damaged, restore = body, original
			case "staged-proposal":
				damaged = filepath.Join(publication.Outputs[1].Staged, fixture.outputs[1].Files[0].Name)
				restore = fixture.outputs[1].Files[0].Body
			case "staged-readback":
				damaged, restore = publication.Outputs[2].Staged, fixture.outputs[2].Files[0].Body
			case "unexpected-body":
				damaged = filepath.Join(fixture.outputs[0].Path, "unexpected.json")
			case "symlink-body":
				if err := os.Remove(body); err != nil {
					t.Fatal(err)
				}
				peer := filepath.Join(filepath.Dir(fixture.outputs[0].Path), "body-copy.json")
				if err := os.WriteFile(peer, original, 0o644); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(peer, body); err != nil {
					t.Fatal(err)
				}
			case "changed-request":
				originalConfig, err := os.ReadFile(fixture.config)
				if err != nil {
					t.Fatal(err)
				}
				damaged, restore = fixture.config, originalConfig
				changed := bytes.Replace(originalConfig,
					[]byte(fmt.Sprintf(`"finality_depth": %d`, fixture.readback.FinalityDepth)),
					[]byte(fmt.Sprintf(`"finality_depth": %d`, fixture.readback.FinalityDepth+1)), 1)
				if bytes.Equal(changed, originalConfig) {
					t.Fatal("configuration fixture has no finality value to change")
				}
				if err := os.WriteFile(damaged, changed, 0o644); err != nil {
					t.Fatal(err)
				}
			}
			if damaged != "" && corrupt != "changed-request" {
				if err := os.WriteFile(damaged, []byte("corrupt\n"), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			journal := publication.journal
			publication = nil
			var report bytes.Buffer
			if err := Run(fixture.args, &report); err == nil {
				t.Fatal("restart accepted changed output or request")
			}
			if report.Len() != 0 {
				t.Fatalf("failed recovery reported success: %s", &report)
			}
			if _, err := os.Lstat(journal); err != nil {
				t.Fatalf("failed recovery lost journal: %v", err)
			}
			for _, output := range fixture.outputs[1:] {
				if _, err := os.Lstat(output.Path); !os.IsNotExist(err) {
					t.Fatalf("failed recovery published %s: %v", output.Path, err)
				}
			}
			if corrupt == "symlink-body" {
				if err := os.Remove(body); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(body, original, 0o644); err != nil {
					t.Fatal(err)
				}
			} else if restore == nil {
				if err := os.Remove(damaged); err != nil {
					t.Fatal(err)
				}
			} else if err := os.WriteFile(damaged, restore, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := Run(fixture.args, &report); err != nil {
				t.Fatalf("same-argument recovery after repair: %v", err)
			}
			fixture.assertComplete(t, report.String())
		})
	}
}

func TestRepairM_365_ExistingRefusalsAndNestedOutputs(t *testing.T) {
	TestRunRefusesAnOutputDirectoryThatHoldsAnything(t)
	TestRunWritesNoProposalsOrBodiesWhenTheProposalsCannotBeWritten(t)
	TestRunWritesTheReadbackBesideTheBodies(t)
	for _, layout := range []string{"nested-readback", "nested-proposals", "bodies-under-proposals", "same-directory-alias", "readback-collision"} {
		t.Run(layout, func(t *testing.T) {
			fixture := newPublicationFixture(t, "ethereum")
			bodies, proposals, readback := fixture.outputs[0].Path, fixture.outputs[1].Path, fixture.outputs[2].Path
			switch layout {
			case "nested-readback":
				readback = filepath.Join(bodies, "readback.json")
			case "nested-proposals":
				proposals, readback = filepath.Join(bodies, "governance"), filepath.Join(bodies, "readback.json")
			case "bodies-under-proposals":
				bodies, readback = filepath.Join(proposals, "bodies"), filepath.Join(proposals, "readback.json")
			case "same-directory-alias":
				proposals = filepath.Join(bodies, ".")
			case "readback-collision":
				readback = filepath.Join(bodies, registerChainFile)
			}
			fixture.outputs[0].Path, fixture.outputs[1].Path, fixture.outputs[2].Path = bodies, proposals, readback
			fixture.args[7], fixture.args[9], fixture.args[11] = readback, proposals, bodies
			var report bytes.Buffer
			err := Run(fixture.args, &report)
			if strings.Contains(layout, "alias") || strings.Contains(layout, "collision") {
				if err == nil {
					t.Fatal("accepted conflicting output destinations")
				}
				if _, err := os.Lstat(bodies); !os.IsNotExist(err) {
					t.Fatalf("conflicting destinations published outputs: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			fixture.assertComplete(t, report.String())
		})
	}
}

func TestRepairM_365_ReadbackLinkRestart(t *testing.T) {
	fixture := newPublicationFixture(t, "solana")
	publication, err := stagePublication(fixture.outputs)
	if err != nil {
		t.Fatal(err)
	}
	for _, output := range publication.Outputs[:2] {
		if err := output.publish(); err != nil {
			t.Fatal(err)
		}
	}
	readback := publication.Outputs[2]
	// Stop at the link/unlink boundary of the real file publisher.
	if err := os.Link(readback.Staged, readback.Path); err != nil {
		t.Fatal(err)
	}
	publication = nil
	var report bytes.Buffer
	if err := Run(fixture.args, &report); err != nil {
		t.Fatalf("restart between readback link and unlink: %v", err)
	}
	fixture.assertComplete(t, report.String())
}

func TestRepairM_365_JournalRefusals(t *testing.T) {
	for _, corrupt := range []string{"closing-token", "extra-document", "unknown-field", "version", "staging-path", "output-body", "missing-output"} {
		t.Run(corrupt, func(t *testing.T) {
			fixture := newPublicationFixture(t, "ethereum")
			publication, err := stagePublication(fixture.outputs)
			if err != nil {
				t.Fatal(err)
			}
			journal := publication.journal
			original, err := os.ReadFile(journal)
			if err != nil {
				t.Fatal(err)
			}
			var document outputPublication
			if err := json.Unmarshal(original, &document); err != nil {
				t.Fatal(err)
			}
			var damaged []byte
			switch corrupt {
			case "closing-token":
				damaged = append(append([]byte(nil), original...), ']')
			case "extra-document":
				damaged = append(append([]byte(nil), original...), []byte("{}")...)
			case "unknown-field":
				damaged = append([]byte(`{"unexpected": true,`), bytes.TrimSpace(original)[1:]...)
			case "version":
				document.Version++
			case "staging-path":
				document.Outputs[0].Staged = fixture.outputs[0].Path
			case "output-body":
				document.Outputs[0].Files[0].Body = []byte("changed")
			case "missing-output":
				document.Outputs = document.Outputs[:2]
			}
			if damaged == nil {
				damaged, err = marshalJSON(document)
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := os.WriteFile(journal, damaged, 0o600); err != nil {
				t.Fatal(err)
			}
			publication = nil
			var report bytes.Buffer
			if err := Run(fixture.args, &report); err == nil {
				t.Fatal("restart accepted corrupt publication journal")
			}
			if report.Len() != 0 {
				t.Fatalf("corrupt journal reported success: %s", &report)
			}
			for _, output := range fixture.outputs {
				if _, err := os.Lstat(output.Path); !os.IsNotExist(err) {
					t.Fatalf("corrupt journal published %s: %v", output.Path, err)
				}
			}
			if err := os.WriteFile(journal, original, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := Run(fixture.args, &report); err != nil {
				t.Fatalf("recovery after restoring original journal: %v", err)
			}
			fixture.assertComplete(t, report.String())
		})
	}
}
