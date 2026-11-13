package decode

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompilerBindingsRejectChangedArtifactsAndInputs(t *testing.T) {
	root := "../../../../../contracts/launchpad"
	reviewed, err := os.ReadFile("../../../../../sdk/launchpad/protocol/catalog/reviewed.json")
	if err != nil {
		t.Fatal(err)
	}
	var catalog struct{ Contracts []map[string]json.RawMessage }
	if err = json.Unmarshal(reviewed, &catalog); err != nil {
		t.Fatal(err)
	}
	var original map[string]json.RawMessage
	for _, entry := range catalog.Contracts {
		var name string
		if err = json.Unmarshal(entry["name"], &name); err != nil {
			t.Fatal(err)
		}
		if name == "EventEmitter" {
			original = entry
			break
		}
	}
	if original == nil {
		t.Fatal("emitter compiler binding absent")
	}
	for _, mode := range []string{"bound", "artifact path", "source path", "artifact missing", "artifact digest", "artifact JSON", "artifact identity", "source missing", "source digest", "debug missing", "debug JSON", "debug escape", "build missing", "build digest", "build JSON", "compiler version", "compiled source"} {
		t.Run(mode, func(t *testing.T) {
			entry := map[string]json.RawMessage{}
			for key, value := range original {
				entry[key] = append(json.RawMessage(nil), value...)
			}
			get := func(key string) string {
				var value string
				if err := json.Unmarshal(entry[key], &value); err != nil {
					t.Fatal(err)
				}
				return value
			}
			set := func(key, value string) {
				wire, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				entry[key] = wire
			}
			artifactPath, sourcePath := get("artifactPath"), get("sourceName")
			artifact, err := os.ReadFile(filepath.Join(root, artifactPath))
			if err != nil {
				t.Fatal(err)
			}
			debugPath := strings.TrimSuffix(artifactPath, ".json") + ".dbg.json"
			debug, err := os.ReadFile(filepath.Join(root, debugPath))
			if err != nil {
				t.Fatal(err)
			}
			var info struct {
				BuildInfo string `json:"buildInfo"`
			}
			if err = json.Unmarshal(debug, &info); err != nil {
				t.Fatal(err)
			}
			buildPath := filepath.Clean(filepath.Join(filepath.Dir(artifactPath), info.BuildInfo))
			build, err := os.ReadFile(filepath.Join(root, buildPath))
			if err != nil {
				t.Fatal(err)
			}
			var compiled struct {
				Input struct {
					Sources map[string]struct{ Content string }
				}
			}
			if err = json.Unmarshal(build, &compiled); err != nil {
				t.Fatal(err)
			}
			directory := t.TempDir()
			write := func(path string, body []byte) {
				if err := os.MkdirAll(filepath.Dir(filepath.Join(directory, path)), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(directory, path), body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			for path := range compiled.Input.Sources {
				if !strings.HasPrefix(path, "contracts/") {
					continue
				}
				body, err := os.ReadFile(filepath.Join(root, path))
				if err != nil {
					t.Fatal(err)
				}
				write(path, body)
			}
			write(artifactPath, artifact)
			write(debugPath, debug)
			write(buildPath, build)
			// The catalog binds the current compiler output and its complete input.
			set("artifactSha256", measurementDigestBytes(artifact))
			set("buildInfoSha256", measurementDigestBytes(build))
			switch mode {
			case "artifact path":
				set("artifactPath", "../contract.json")
			case "source path":
				set("sourceName", "/contract.sol")
			case "artifact missing":
				if err = os.Remove(filepath.Join(directory, artifactPath)); err != nil {
					t.Fatal(err)
				}
			case "artifact digest":
				set("artifactSha256", strings.Repeat("0", 64))
			case "artifact JSON":
				write(artifactPath, []byte("{"))
				set("artifactSha256", measurementDigestBytes([]byte("{")))
			case "artifact identity":
				set("name", "Router")
			case "source missing":
				if err = os.Remove(filepath.Join(directory, sourcePath)); err != nil {
					t.Fatal(err)
				}
			case "source digest":
				set("sourceSha256", strings.Repeat("0", 64))
			case "debug missing":
				if err = os.Remove(filepath.Join(directory, debugPath)); err != nil {
					t.Fatal(err)
				}
			case "debug JSON":
				write(debugPath, []byte("{"))
			case "debug escape":
				write(debugPath, []byte(`{"buildInfo":"../../../../../../outside.json"}`))
			case "build missing":
				if err = os.Remove(filepath.Join(directory, buildPath)); err != nil {
					t.Fatal(err)
				}
			case "build digest":
				set("buildInfoSha256", strings.Repeat("0", 64))
			case "build JSON":
				write(buildPath, []byte("{"))
				set("buildInfoSha256", measurementDigestBytes([]byte("{")))
			case "compiler version":
				set("compilerVersion", "0.0.0")
			case "compiled source":
				changed := false
				for path := range compiled.Input.Sources {
					if path != sourcePath && strings.HasPrefix(path, "contracts/") {
						write(path, []byte("pragma solidity ^0.8.0;"))
						changed = true
						break
					}
				}
				if !changed {
					t.Fatal("compiler dependency absent")
				}
			}
			wire, err := json.Marshal(map[string]any{"contracts": []map[string]json.RawMessage{entry}})
			if err != nil {
				t.Fatal(err)
			}
			err = checkedCatalogArtifacts(wire, directory)
			if mode == "bound" && err != nil || mode != "bound" && err == nil {
				t.Fatalf("compiler binding %s: %v", mode, err)
			}
		})
	}
}

func measurementDigestBytes(body []byte) string { return fmt.Sprintf("%x", sha256.Sum256(body)) }

func TestCompilerDocumentsCacheByExactContentWithinBound(t *testing.T) {
	for i := 0; i < 20; i++ {
		body := []byte(fmt.Sprintf(`{"solcVersion":"v%d","input":{"sources":{"contracts/Bound.sol":{"content":"source%d"}}}}`, i, i))
		first, err := parseDecoderCompilerInput(body)
		if err != nil || first.SolcVersion != fmt.Sprintf("v%d", i) || first.Input.Sources["contracts/Bound.sol"].Content != fmt.Sprintf("source%d", i) {
			t.Fatalf("compiler input identity %+v: %v", first, err)
		}
		second, err := parseDecoderCompilerInput(body)
		if err != nil || first != second {
			t.Fatalf("immutable compiler content was not reused: %v", err)
		}
	}
	if _, err := parseDecoderCompilerInput([]byte("{")); err == nil {
		t.Fatal("malformed compiler input admitted")
	}
	decoderCompilerInputs.Lock()
	count := len(decoderCompilerInputs.values)
	decoderCompilerInputs.Unlock()
	if count > 16 {
		t.Fatalf("compiler input cache grew to %d", count)
	}
}
