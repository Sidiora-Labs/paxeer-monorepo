package decode

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCatalogArtifactsRejectChangedSourceAndCompilerDocuments(t *testing.T) {
	raw, _ := frozen(t)
	var catalog struct {
		Contracts []map[string]any `json:"contracts"`
	}
	if err := json.Unmarshal(raw, &catalog); err != nil {
		t.Fatal(err)
	}
	original := catalog.Contracts[0]
	sourceRoot := "../../../../../contracts/launchpad"
	artifactPath := original["artifactPath"].(string)
	sourcePath := original["sourceName"].(string)
	debugPath := strings.TrimSuffix(artifactPath, ".json") + ".dbg.json"
	read := func(path string) []byte {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(sourceRoot, path))
		if err != nil {
			t.Fatal(err)
		}
		return body
	}
	artifact, source, debug := read(artifactPath), read(sourcePath), read(debugPath)
	var reference struct {
		BuildInfo string `json:"buildInfo"`
	}
	if err := json.Unmarshal(debug, &reference); err != nil {
		t.Fatal(err)
	}
	buildPath := filepath.Join(filepath.Dir(artifactPath), reference.BuildInfo)
	build := read(buildPath)
	digest := func(body []byte) string { sum := sha256.Sum256(body); return hex.EncodeToString(sum[:]) }
	for _, name := range []string{"catalog syntax", "source path", "artifact missing", "artifact digest", "artifact syntax", "artifact ABI", "artifact identity", "source missing", "source digest", "debug missing", "debug syntax", "build path", "build missing", "build digest", "build syntax", "compiler version", "compiled source missing", "compiled source drift"} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			copy := map[string]any{}
			for k, v := range original {
				copy[k] = v
			}
			files := map[string][]byte{artifactPath: artifact, sourcePath: source, debugPath: debug, buildPath: build}
			switch name {
			case "source path":
				copy["sourceName"] = "../source.sol"
			case "artifact missing":
				delete(files, artifactPath)
			case "artifact digest":
				copy["artifactSha256"] = strings.Repeat("0", 64)
			case "artifact syntax":
				files[artifactPath] = []byte(`{`)
				copy["artifactSha256"] = digest(files[artifactPath])
			case "artifact ABI":
				var value map[string]any
				if err := json.Unmarshal(artifact, &value); err != nil {
					t.Fatal(err)
				}
				value["abi"] = nil
				body, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				files[artifactPath] = body
				copy["artifactSha256"] = digest(body)
			case "artifact identity":
				copy["name"] = "DifferentContract"
			case "source missing":
				delete(files, sourcePath)
			case "source digest":
				copy["sourceSha256"] = strings.Repeat("0", 64)
			case "debug missing":
				delete(files, debugPath)
			case "debug syntax":
				files[debugPath] = []byte(`{`)
			case "build path":
				files[debugPath] = []byte(`{"buildInfo":"../../../../../../outside.json"}`)
			case "build missing":
				delete(files, buildPath)
			case "build digest":
				copy["buildInfoSha256"] = strings.Repeat("0", 64)
			case "build syntax":
				files[buildPath] = []byte(`{`)
				copy["buildInfoSha256"] = digest(files[buildPath])
			case "compiler version":
				copy["compilerVersion"] = "unrecognized"
			case "compiled source drift":
				var compiled decoderCompilerInput
				if err := json.Unmarshal(build, &compiled); err != nil {
					t.Fatal(err)
				}
				for path := range compiled.Input.Sources {
					if strings.HasPrefix(path, "contracts/") {
						files[path] = read(path)
					}
				}
				files[sourcePath] = append(append([]byte(nil), source...), []byte("\n")...)
				copy["sourceSha256"] = digest(files[sourcePath])
			}
			for path, body := range files {
				target := filepath.Join(root, path)
				if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(target, body, 0600); err != nil {
					t.Fatal(err)
				}
			}
			body, err := json.Marshal(map[string]any{"contracts": []map[string]any{copy}})
			if err != nil {
				t.Fatal(err)
			}
			if name == "catalog syntax" {
				body = []byte(`{`)
			}
			if err := checkedCatalogArtifacts(body, root); err == nil {
				t.Fatal("changed catalog artifact admitted")
			}
		})
	}
}

func TestMeasurementJSONRequiresOneBoundDocument(t *testing.T) {
	for _, body := range []string{`{`, `{"unexpected":true}`, `{} {}`} {
		path := filepath.Join(t.TempDir(), "input.json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256([]byte(body))
		digest := hex.EncodeToString(sum[:])
		if _, err := LoadDecoderMeasurementInput(path, digest); err == nil {
			t.Fatal("invalid measurement document admitted")
		}
	}
	if _, err := measurementJSON(t.TempDir(), strings.Repeat("0", 64), new(DecoderMeasurementInput)); err == nil {
		t.Fatal("directory accepted as measurement input")
	}
	for i := 0; i < 18; i++ {
		body, err := json.Marshal(decoderCompilerInput{SolcVersion: strings.Repeat("v", i+1)})
		if err != nil {
			t.Fatal(err)
		}
		parsed, err := parseDecoderCompilerInput(body)
		if err != nil || parsed.SolcVersion != strings.Repeat("v", i+1) {
			t.Fatalf("compiler document cache: %+v, %v", parsed, err)
		}
	}
	decoderCompilerInputs.Lock()
	count := len(decoderCompilerInputs.values)
	decoderCompilerInputs.Unlock()
	if count != 16 {
		t.Fatalf("compiler document capacity: %d", count)
	}
}
