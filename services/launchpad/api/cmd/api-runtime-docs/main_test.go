package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/app"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/capabilities"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
)

func repositoryRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("source path unavailable")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "..", "..", ".."))
}

func commandInputs(t *testing.T) ([]string, deployment.PublicEndpoints) {
	t.Helper()
	root := repositoryRoot(t)
	raw, err := os.ReadFile(filepath.Join(root, "contracts", "launchpad", "artifacts", "contracts", "base", "BeaconProxy.sol", "BeaconProxy.json"))
	if err != nil {
		t.Fatal(err)
	}
	var artifact struct {
		DeployedBytecode string `json:"deployedBytecode"`
	}
	if err := json.Unmarshal(raw, &artifact); err != nil {
		t.Fatal(err)
	}
	code, err := hexutil.Decode(artifact.DeployedBytecode)
	if err != nil || len(code) == 0 {
		t.Fatalf("pool runtime: %v", err)
	}
	endpoints := deployment.PublicEndpoints{
		RPC: "https://api1.mainnet-beta.paxeer.network", Explorer: "https://paxscan.paxeer.app",
		AllowedRPCs: []string{"https://api1.mainnet-beta.paxeer.network"}, AllowedExplorers: []string{"https://paxscan.paxeer.app"},
	}
	path := filepath.Join(t.TempDir(), "endpoints.json")
	writeJSON(t, path, endpoints)
	return []string{"-repo", root, "-source-schema", "indexer", "-endpoints", path, "-pool-runtime", crypto.Keccak256Hash(code).Hex(), "-out", filepath.Join(t.TempDir(), "documents")}, endpoints
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
}

func generatedDocument(t *testing.T) (app.NetworkAuthorityConfig, []byte, []string) {
	t.Helper()
	args, endpoints := commandInputs(t)
	var output bytes.Buffer
	if err := run(args, &output); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(args[len(args)-1], documentName)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	if want := fmt.Sprintf("%s %s %d\n", digest, documentName, len(raw)); output.String() != want {
		t.Fatalf("digest and size: got %q, want %q", output.String(), want)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o600 {
		t.Fatalf("document permissions: %v, %v", info, err)
	}
	info, err = os.Stat(filepath.Dir(path))
	if err != nil || info.Mode().Perm() != 0o750 {
		t.Fatalf("directory permissions: %v, %v", info, err)
	}
	var document app.NetworkAuthorityConfig
	if err := config.ReadBoundDocument(path, digest, &document); err != nil {
		t.Fatalf("API document decoding: %v", err)
	}
	if err := app.ValidateNetworkAuthorityConfig(document); err != nil {
		t.Fatalf("API document validation: %v", err)
	}
	manifest := deployment.Current()
	if document.SchemaVersion != 2 || document.SourceSchema != "indexer" || !reflect.DeepEqual(document.Manifest, manifest) ||
		!reflect.DeepEqual(document.Endpoints, endpoints) || !reflect.DeepEqual(document.GoAddresses, manifest.Addresses()) || !reflect.DeepEqual(document.FrontendAddresses, manifest.Addresses()) {
		t.Fatalf("document configuration differs from inputs: %+v", document)
	}
	var registry struct {
		Tasks map[string]struct {
			InterfaceBundle struct {
				Expected string `json:"expected_sha256"`
			} `json:"interface_bundle"`
		} `json:"tasks"`
	}
	registryRaw, err := os.ReadFile(registryPath(repositoryRoot(t)))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(registryRaw, &registry); err != nil {
		t.Fatal(err)
	}
	if document.SerialInterfaceSHA256 != registry.Tasks["1.99"].InterfaceBundle.Expected || document.ReviewedPoolRuntime != args[7] {
		t.Fatal("reviewed runtime or serial interface differs from repository inputs")
	}
	return document, raw, args
}

func TestNetworkAuthorityDocumentMatchesAPIAndEngine(t *testing.T) {
	document, _, _ := generatedDocument(t)
	client, err := rpc.DialOptions(context.Background(), document.Endpoints.RPC)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	reader, err := capabilities.NewPinnedRPCReader(client)
	if err != nil {
		t.Fatal(err)
	}
	engine, err := capabilities.NewPreflightEngine(reader, document.Manifest, document.ReviewedPoolRuntime, document.SerialInterfaceSHA256, time.Minute, 10*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if document.ExpectedRevisionSHA256 != engine.Revision().RevisionSHA256 {
		t.Fatal("document revision differs from API preflight engine")
	}
}

func TestNetworkAuthorityDocumentsAreDeterministic(t *testing.T) {
	_, first, args := generatedDocument(t)
	args[len(args)-1] = filepath.Join(t.TempDir(), "documents")
	var output bytes.Buffer
	if err := run(args, &output); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(filepath.Join(args[len(args)-1], documentName))
	if err != nil || !bytes.Equal(first, second) {
		t.Fatalf("document bytes differ: %v", err)
	}
	if output.String() != fmt.Sprintf("%x %s %d\n", sha256.Sum256(second), documentName, len(second)) {
		t.Fatal("second digest or size differs")
	}
}

func TestNetworkAuthorityDocumentRejectsChangedBindings(t *testing.T) {
	_, raw, _ := generatedDocument(t)
	for _, tc := range []struct {
		name, field string
		change      func(*app.NetworkAuthorityConfig)
	}{
		{"manifest address", "manifest", func(d *app.NetworkAuthorityConfig) {
			role := d.Manifest.Roles["Router"]
			role.Address = d.Manifest.Roles["Quoter"].Address
			d.Manifest.Roles["Router"] = role
		}},
		{"revision digest", "expectedRevisionSha256", func(d *app.NetworkAuthorityConfig) { d.ExpectedRevisionSHA256 = strings.Repeat("0", 64) }},
		{"schema version", "schemaVersion", func(d *app.NetworkAuthorityConfig) { d.SchemaVersion = 1 }},
		{"source schema", "sourceSchema", func(d *app.NetworkAuthorityConfig) { d.SourceSchema = "indexer;" }},
		{"go address", "goAddresses", func(d *app.NetworkAuthorityConfig) { d.GoAddresses["Router"] = d.Manifest.Roles["Quoter"].Address }},
		{"frontend address", "frontendAddresses", func(d *app.NetworkAuthorityConfig) {
			d.FrontendAddresses["Router"] = d.Manifest.Roles["Quoter"].Address
		}},
		{"pool runtime", "reviewedPoolRuntime", func(d *app.NetworkAuthorityConfig) { d.ReviewedPoolRuntime = "0x" + strings.Repeat("0", 64) }},
		{"serial digest", "serialInterfaceSha256", func(d *app.NetworkAuthorityConfig) { d.SerialInterfaceSHA256 = "invalid" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var document app.NetworkAuthorityConfig
			if err := json.Unmarshal(raw, &document); err != nil {
				t.Fatal(err)
			}
			tc.change(&document)
			path := filepath.Join(t.TempDir(), documentName)
			writeJSON(t, path, document)
			body, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var decoded app.NetworkAuthorityConfig
			if err := config.ReadBoundDocument(path, fmt.Sprintf("%x", sha256.Sum256(body)), &decoded); err != nil {
				t.Fatal(err)
			}
			if err := app.ValidateNetworkAuthorityConfig(decoded); err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("changed %s must be refused by name: %v", tc.name, err)
			}
		})
	}
}

func TestCommandRejectsInvalidInputs(t *testing.T) {
	args, _ := commandInputs(t)
	for _, tc := range []struct{ flag, value, field string }{
		{"-source-schema", "", "sourceSchema"}, {"-source-schema", "IndexER", "sourceSchema"},
		{"-source-schema", "indexer.schema", "sourceSchema"}, {"-source-schema", strings.Repeat("a", 64), "sourceSchema"},
		{"-pool-runtime", "", "-pool-runtime"}, {"-pool-runtime", "0x" + strings.Repeat("0", 64), "-pool-runtime"},
		{"-pool-runtime", "0x" + strings.Repeat("z", 64), "-pool-runtime"},
		{"-endpoints", "", "-endpoints"}, {"-endpoints", filepath.Join(t.TempDir(), "missing"), "-endpoints"},
		{"-repo", t.TempDir(), "-repo"},
	} {
		t.Run(tc.flag+"="+tc.value, func(t *testing.T) {
			next := append([]string(nil), args...)
			for i := 0; i < len(next); i += 2 {
				if next[i] == tc.flag {
					next[i+1] = tc.value
				}
			}
			var output bytes.Buffer
			if err := run(next, &output); err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("input must be refused by name: %v", err)
			}
			if output.Len() != 0 {
				t.Fatal("refused input printed a digest")
			}
			if _, err := os.Stat(args[len(args)-1]); !os.IsNotExist(err) {
				t.Fatalf("refused input wrote output: %v", err)
			}
		})
	}
	for _, schema := range []string{"a", "indexer_125", strings.Repeat("a", 63)} {
		next := append([]string(nil), args...)
		next[3], next[len(next)-1] = schema, t.TempDir()
		var output bytes.Buffer
		if err := run(next, &output); err != nil {
			t.Fatalf("admitted schema %q refused: %v", schema, err)
		}
	}
}

func TestCommandRejectsInvalidEndpoints(t *testing.T) {
	args, endpoints := commandInputs(t)
	for _, body := range []string{"{", "null", "{}", "[]", `{"RPC":42}`, `{"unknown":true}`, `{} {}`, `{} trailing`} {
		t.Run(body, func(t *testing.T) {
			if err := os.WriteFile(args[5], []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := run(args, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "-endpoints") {
				t.Fatalf("invalid endpoints accepted: %v", err)
			}
		})
	}
	for _, value := range []string{"http://api1.mainnet-beta.paxeer.network", endpoints.RPC + "?key=value", endpoints.RPC + "#fragment", endpoints.RPC + "?", " " + endpoints.RPC} {
		changed := endpoints
		changed.RPC, changed.AllowedRPCs = value, []string{value}
		writeJSON(t, args[5], changed)
		if err := run(args, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "RPC") {
			t.Fatalf("invalid public RPC accepted: %v", err)
		}
	}
	for _, changed := range []deployment.PublicEndpoints{
		{RPC: endpoints.RPC, Explorer: endpoints.Explorer, AllowedExplorers: endpoints.AllowedExplorers},
		{RPC: endpoints.RPC, Explorer: endpoints.Explorer, AllowedRPCs: endpoints.AllowedExplorers, AllowedExplorers: endpoints.AllowedExplorers},
		{RPC: endpoints.RPC, Explorer: endpoints.Explorer, AllowedRPCs: endpoints.AllowedRPCs},
	} {
		writeJSON(t, args[5], changed)
		if err := run(args, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "-endpoints") {
			t.Fatalf("unapproved endpoints accepted: %v", err)
		}
	}
}

func TestCommandRejectsInvalidRegistryAndOutput(t *testing.T) {
	args, _ := commandInputs(t)
	for _, body := range []string{"{", "null", "{}", `{"tasks":{"1.99":{"interface_bundle":{"expected_sha256":"invalid"}}}}`} {
		t.Run(body, func(t *testing.T) {
			next := append([]string(nil), args...)
			next[1] = t.TempDir()
			path := registryPath(next[1])
			if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := run(next, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "-repo") {
				t.Fatalf("invalid serial interface registry accepted: %v", err)
			}
		})
	}
	for _, directory := range []bool{false, true} {
		next := append([]string(nil), args...)
		next[len(next)-1] = t.TempDir()
		path := next[len(next)-1]
		if directory {
			path = filepath.Join(path, documentName)
			if err := os.Mkdir(path, 0o750); err != nil {
				t.Fatal(err)
			}
		} else {
			next[len(next)-1] = filepath.Join(path, "file")
			if err := os.WriteFile(next[len(next)-1], []byte("document"), 0o600); err != nil {
				t.Fatal(err)
			}
		}
		if err := run(next, &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "-out") {
			t.Fatalf("output collision accepted: %v", err)
		}
	}
	if err := run([]string{"-unknown"}, &bytes.Buffer{}); err == nil {
		t.Fatal("unknown flag accepted")
	}
	if err := run(append(args, "extra"), &bytes.Buffer{}); err == nil {
		t.Fatal("positional input accepted")
	}
}
