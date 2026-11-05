package layerx

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
	"unicode"
)

type portableCatalogueOperation struct {
	Plane       string `json:"plane"`
	Name        string `json:"name"`
	Method      string `json:"method"`
	Path        string `json:"path"`
	Request     string `json:"request"`
	Response    string `json:"response"`
	Idempotency bool   `json:"idempotency"`
	Bodyless    bool   `json:"bodyless"`
	Transport   string `json:"transport"`
	To          string `json:"to"`
	Selector    string `json:"selector"`
	PerLeg      bool   `json:"per_leg"`
}

func portableCatalogueFiles(t *testing.T) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	for _, path := range []string{
		"../dotnet/Generated/OperationCatalog.cs",
		"../swift/Sources/LayerXSDK/Generated/OperationCatalog.swift",
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		files[path] = data
	}
	return files
}

func TestPortableCataloguesMatchOperationsJson(t *testing.T) {
	data, err := os.ReadFile("../conformance/operations.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest struct {
		Schema     int                          `json:"schema"`
		Operations []portableCatalogueOperation `json:"operations"`
	}
	var grouped struct {
		Schema     int                                     `json:"schema"`
		Operations map[string][]portableCatalogueOperation `json:"operations"`
	}
	if err := json.Unmarshal(data, &grouped); err != nil {
		t.Fatal(err)
	}
	manifest.Schema = grouped.Schema
	identifiers := make([]string, 0, len(grouped.Operations))
	for identifier := range grouped.Operations {
		identifiers = append(identifiers, identifier)
	}
	sort.Strings(identifiers)
	for _, identifier := range identifiers {
		manifest.Operations = append(manifest.Operations, grouped.Operations[identifier]...)
	}
	if manifest.Schema != 1 || len(manifest.Operations) == 0 {
		t.Fatal("expected a nonempty version 1 operation manifest")
	}
	csharpDescriptor := regexp.MustCompile(`(?m)^\s*PlatformOperation\.(\w+) => new\(PlatformPlane\.(\w+), "([^"]+)", SdkHttpMethod\.(\w+), "([^"]*)", "([^"]*)", "([^"]*)", (true|false), (true|false)\),$`)
	swiftDescriptor := regexp.MustCompile(`(?m)^\s*case \.(\w+): return OperationDescriptor\(plane: \.(\w+), name: "([^"]+)", method: \.(\w+), path: "([^"]*)", requestType: "([^"]*)", responseType: "([^"]*)", requiresIdempotency: (true|false), bodyless: (true|false)\)$`)
	csharpTransport := regexp.MustCompile(`(?m)^\s*PlatformOperation\.(\w+) => new\("([^"]+)", "([^"]+)", (null|"[^"]*"), (null|"[^"]*"), (true|false)\),$`)
	swiftTransport := regexp.MustCompile(`(?m)^\s*case \.(\w+): return OperationTransport\(kind: "([^"]+)", method: "([^"]+)", to: (nil|"[^"]*"), selector: (nil|"[^"]*"), perLeg: (true|false)\)$`)
	for path, source := range portableCatalogueFiles(t) {
		t.Run(filepath.Ext(path), func(t *testing.T) {
			descriptorPattern, transportPattern := csharpDescriptor, csharpTransport
			enumPattern := regexp.MustCompile(`(?m)^    (\w+),?$`)
			isSwift := strings.HasSuffix(path, ".swift")
			if isSwift {
				descriptorPattern, transportPattern = swiftDescriptor, swiftTransport
				enumPattern = regexp.MustCompile(`(?m)^    case (\w+) = "([^"]+)"$`)
			}
			index := func(pattern *regexp.Regexp) map[string][]string {
				result := make(map[string][]string)
				for _, match := range pattern.FindAllSubmatch(source, -1) {
					fields := make([]string, len(match)-1)
					for i := range fields {
						fields[i] = string(match[i+1])
					}
					if _, exists := result[fields[0]]; exists {
						t.Fatalf("duplicate catalogue entry %s", fields[0])
					}
					result[fields[0]] = fields
				}
				if len(result) != len(manifest.Operations) {
					t.Fatalf("catalogue has %d entries, manifest has %d", len(result), len(manifest.Operations))
				}
				return result
			}
			descriptors, transports, cases := index(descriptorPattern), index(transportPattern), index(enumPattern)
			seen := make(map[string]bool)
			for _, operation := range manifest.Operations {
				key := operation.Plane + ":" + operation.Name
				if seen[key] {
					t.Fatalf("duplicate manifest operation %s", key)
				}
				seen[key] = true
				parts := strings.FieldsFunc(operation.Plane+"."+operation.Name, func(r rune) bool {
					return r == '.' || r == '-' || r == '_'
				})
				for i, part := range parts {
					if !isSwift || i != 0 {
						parts[i] = string(unicode.ToUpper(rune(part[0]))) + part[1:]
					}
				}
				symbol := strings.Join(parts, "")
				descriptor, descriptorOK := descriptors[symbol]
				transport, transportOK := transports[symbol]
				entry, caseOK := cases[symbol]
				if !descriptorOK || !transportOK || !caseOK {
					t.Fatalf("missing %s descriptor, transport or enum case (%s)", key, symbol)
				}
				if isSwift && entry[1] != key {
					t.Errorf("%s raw enum value = %q", key, entry[1])
				}
				kind := operation.Transport
				if kind == "" {
					kind = "http"
				}
				optional := func(value string) string {
					if value == "null" || value == "nil" {
						return ""
					}
					var decoded string
					if err := json.Unmarshal([]byte(value), &decoded); err != nil {
						t.Fatal(err)
					}
					return decoded
				}
				if transport[1] != kind || transport[2] != operation.Method || optional(transport[3]) != operation.To || optional(transport[4]) != operation.Selector || (transport[5] == "true") != operation.PerLeg {
					t.Errorf("%s transport = %v, want %+v", key, transport[1:], operation)
				}
				envelopeMethod := operation.Method
				if kind == "evm" {
					envelopeMethod = "POST"
				}
				if strings.ToLower(descriptor[1]) != operation.Plane || descriptor[2] != operation.Name || strings.ToUpper(descriptor[3]) != envelopeMethod || descriptor[4] != operation.Path || descriptor[5] != operation.Request || descriptor[6] != operation.Response || (descriptor[7] == "true") != operation.Idempotency || (descriptor[8] == "true") != operation.Bodyless {
					t.Errorf("%s descriptor = %v, want %+v", key, descriptor[1:], operation)
				}
			}
		})
	}
	repo, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("python3", filepath.Join(repo, "sdk/platform/generators/generate_portable.py"), "--check", repo)
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("portable generator drift: %v\n%s", err, output)
	}
}

func TestPortableCataloguesHaveNoProgramsRoute(t *testing.T) {
	for path, source := range portableCatalogueFiles(t) {
		if bytes.Contains(source, []byte("/v1/programs")) {
			t.Errorf("%s contains the retired programs route", path)
		}
	}
}
