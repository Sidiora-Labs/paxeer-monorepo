package config

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

type platformDocument struct {
	variable string
	data     []byte
	digest   string
}

func platformRepositoryRoot(t *testing.T) string {
	t.Helper()
	_, filename, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locating platform document test")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(filename), "../../../../.."))
	if _, err := os.Stat(filepath.Join(root, "deploy", "launchpad", "railway", "variables.json")); err != nil {
		t.Fatalf("repository root: %v", err)
	}
	return root
}

func platformValues(t *testing.T, root, values string) {
	t.Helper()
	common := filepath.Join(root, "deploy", "launchpad", "documents", "values", "common.example.json")
	indexer := filepath.Join(root, "deploy", "launchpad", "documents", "values", "indexer.example.json")
	if data, err := os.ReadFile(common); err != nil {
		t.Fatal(err)
	} else if err = os.WriteFile(filepath.Join(values, "common.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if data, err := os.ReadFile(indexer); err != nil {
		t.Fatal(err)
	} else if err = os.WriteFile(filepath.Join(values, "indexer.example.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	expected := filepath.Join(values, "expected-schema")
	if err := os.Mkdir(expected, 0o700); err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("0", 64)
	schema := map[string]any{
		"service":          "core/indexer",
		"runtime_role":     "indexerd",
		"registry_digest":  digest,
		"interface_digest": digest,
		"reader_version":   1,
		"migrations":       map[string]string{"1": digest},
		"schemas":          map[string]string{"indexer": digest},
		"epoch":            1,
		"region":           "mainnet-beta",
		"privileges":       []map[string]any{{"schema": "indexer", "object": "indexer_ingest_commits", "privileges": []string{"SELECT"}}},
	}
	data, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(expected, "indexer.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
}

func composePlatformEnvironment(t *testing.T, root, values, out string) map[string]string {
	t.Helper()
	command := exec.Command("python3", filepath.Join(root, "deploy", "launchpad", "documents", "compose.py"), "--values", values, "--out", out, "--service", "indexer")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compose: %v\n%s", err, output)
	}
	data, err := os.ReadFile(filepath.Join(out, "indexer.json"))
	if err != nil {
		t.Fatal(err)
	}
	var encoded map[string]string
	if err = json.Unmarshal(data, &encoded); err != nil {
		t.Fatal(err)
	}
	return encoded
}

func composePlatformDocuments(t *testing.T, root, values, out string) map[string]platformDocument {
	t.Helper()
	encoded := composePlatformEnvironment(t, root, values, out)
	documents := make(map[string]platformDocument)
	for _, variable := range []string{
		"DB_BUDGET_DOCUMENT",
		"EXPECTED_SCHEMA_DOCUMENT",
		"DELIVERY_CONFIG_DOCUMENT",
		"INDEXER_LIFECYCLE_CONFIG_DOCUMENT",
		"INDEXER_RECEIPT_CONFIG_DOCUMENT",
	} {
		digest := strings.TrimSuffix(variable, "_DOCUMENT") + "_SHA256"
		raw, decodeErr := base64.StdEncoding.DecodeString(encoded[variable])
		if decodeErr != nil || len(raw) == 0 || len(encoded[digest]) != 64 {
			t.Fatalf("composed %s is incomplete", variable)
		}
		documents[variable] = platformDocument{variable: variable, data: raw, digest: encoded[digest]}
	}
	return documents
}

func setPlatformEnvironment(t *testing.T, documents map[string]platformDocument, tamper string) {
	t.Helper()
	for key, value := range map[string]string{
		"NODE_ENV":                "production",
		"RUNTIME_PROFILE":         "container",
		"BIND_ADDRESS":            "0.0.0.0",
		"SECRETS_PROVIDER":        "env",
		"CHAIN_ID":                "125",
		"EVENT_EMITTER_ADDRESS":   "0x622E1bb90E0E2B6E6e5e9e2DbE1555389E8A3EF6",
		"POOL_REGISTRY_ADDRESS":   "0x1dC357d2B7B850332a91F06A9E66634F46fC4Ba7",
		"ROUTER_ADDRESS":          "0x78FB06f525cD964388f2305A489035263E31C6e4",
		"FACTORY_ADDRESS":         "0x3993D004166738d1069B5C4e0a923f7B9D5D8830",
		"QUOTER_ADDRESS":          "0x793006f4F321b8863F26DBD75b11bAa01DA181dc",
		"PROTOCOL_CONFIG_ADDRESS": "0xF3669eDc475Cb6E1F0d7635240ABD8e8FB7E20C6",
		"FEE_ACCUMULATOR_ADDRESS": "0x7b14808b72f61664cDb79D25E6d1587655571a66",
		"SIDIORA_NFT_ADDRESS":     "0xe8DcC844a8006dbf64f5Bb851bCad90292a6409B",
		"FEES_ROUTER_ADDRESS":     "0x05681dA6f611C2dfFD3E92611F6dE2d247647927",
		"POOL_BEACON_ADDRESS":     "0x0c60Ea33B1DD602A17E1EF1875eE912F3fad71BE",
		"PORT":                    "8080",
	} {
		t.Setenv(key, value)
	}
	for _, key := range []string{
		"DATABASE_URL", "REDIS_URL", "REDIS_BULL_URL", "RPC_URL", "NATS_URL", "NATS_CREDENTIALS",
		"DB_POOL_ID", "INSTANCE_ID", "ZONE",
	} {
		t.Setenv(key, map[string]string{
			"DATABASE_URL":     "postgres://indexer:password@database:5432/launchpad",
			"REDIS_URL":        "redis://redis:6379",
			"REDIS_BULL_URL":   "redis://redis:6379/1",
			"RPC_URL":          "https://rpc.mainnet-beta.paxeer.network",
			"NATS_URL":         "nats://nats:4222",
			"NATS_CREDENTIALS": "example-credential",
			"DB_POOL_ID":       "indexerd_pool",
			"INSTANCE_ID":      "indexerd-1",
			"ZONE":             "mainnet-beta-a",
		}[key])
	}
	for variable, document := range documents {
		data := append([]byte(nil), document.data...)
		if variable == tamper {
			data[0] ^= 1
		}
		t.Setenv(variable, base64.StdEncoding.EncodeToString(data))
		digest := strings.TrimSuffix(variable, "_DOCUMENT") + "_SHA256"
		t.Setenv(digest, document.digest)
	}
}

func TestPlatformDocumentsComposeAndLoad(t *testing.T) {
	root := platformRepositoryRoot(t)
	values := t.TempDir()
	platformValues(t, root, values)
	documents := composePlatformDocuments(t, root, values, t.TempDir())
	setPlatformEnvironment(t, documents, "")
	if _, err := Load(); err != nil {
		t.Fatalf("Load composed platform documents: %v", err)
	}
}

func TestPlatformDocumentsRejectChangedDocument(t *testing.T) {
	root := platformRepositoryRoot(t)
	values := t.TempDir()
	platformValues(t, root, values)
	documents := composePlatformDocuments(t, root, values, t.TempDir())
	for variable := range documents {
		t.Run(variable, func(t *testing.T) {
			setPlatformEnvironment(t, documents, variable)
			if _, err := Load(); err == nil {
				t.Fatalf("Load accepted changed %s", variable)
			}
		})
	}
}

func TestPlatformDocumentsFileFormComposeAndRead(t *testing.T) {
	root := platformRepositoryRoot(t)
	values := t.TempDir()
	platformValues(t, root, values)
	documents := composePlatformDocuments(t, root, values, t.TempDir())
	valuesPath := filepath.Join(values, "indexer.example.json")
	raw, err := os.ReadFile(valuesPath)
	if err != nil {
		t.Fatal(err)
	}
	var inputs map[string]any
	if err = json.Unmarshal(raw, &inputs); err != nil {
		t.Fatal(err)
	}
	runtimeDirectory := t.TempDir()
	fileBytes := map[string][]byte{}
	for _, name := range []string{"receipt", "lifecycle"} {
		prefix := "INDEXER_" + strings.ToUpper(name) + "_CONFIG"
		var rendered bytes.Buffer
		if err = json.Indent(&rendered, documents[prefix+"_DOCUMENT"].data, "", "  "); err != nil {
			t.Fatal(err)
		}
		rendered.WriteByte('\n')
		fileBytes[name] = append([]byte(nil), rendered.Bytes()...)
		if err = os.WriteFile(filepath.Join(values, name+".json"), fileBytes[name], 0o600); err != nil {
			t.Fatal(err)
		}
		inputs[name+"_rendered_file"] = name + ".json"
		inputs[name+"_runtime_path"] = filepath.Join(runtimeDirectory, name+".json")
	}
	raw, err = json.Marshal(inputs)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(valuesPath, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	composed := composePlatformEnvironment(t, root, values, t.TempDir())
	setPlatformEnvironment(t, documents, "")
	for _, name := range []string{"receipt", "lifecycle"} {
		prefix := "INDEXER_" + strings.ToUpper(name) + "_CONFIG"
		if _, exists := composed[prefix+"_DOCUMENT"]; exists {
			t.Fatalf("file form includes %s_DOCUMENT", prefix)
		}
		path := composed[prefix+"_FILE"]
		if path != inputs[name+"_runtime_path"] {
			t.Fatalf("%s_FILE differs from the runtime path", prefix)
		}
		if composed[prefix+"_SHA256"] != fmt.Sprintf("%x", sha256.Sum256(fileBytes[name])) {
			t.Fatalf("%s_SHA256 differs from rendered file bytes", prefix)
		}
		if err = os.WriteFile(path, fileBytes[name], 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv(prefix+"_DOCUMENT", "")
		t.Setenv(prefix+"_FILE", path)
		t.Setenv(prefix+"_SHA256", composed[prefix+"_SHA256"])
	}
	for load := 0; load < 2; load++ {
		cfg, loadErr := Load()
		if loadErr != nil {
			t.Fatalf("Load composed file documents: %v", loadErr)
		}
		if cfg.ReceiptConfigDocument != "" || cfg.LifecycleConfigDocument != "" {
			t.Fatal("file documents loaded as inline documents")
		}
		for _, binding := range []struct{ name, path, digest string }{
			{"receipt", cfg.ReceiptConfigFile, cfg.ReceiptConfigSHA256},
			{"lifecycle", cfg.LifecycleConfigFile, cfg.LifecycleConfigSHA256},
		} {
			var got, want map[string]any
			if err = json.Unmarshal(fileBytes[binding.name], &want); err != nil {
				t.Fatal(err)
			}
			if err = ReadBoundDocument(binding.path, binding.digest, &got); err != nil {
				t.Fatalf("ReadBoundDocument %s: %v", binding.name, err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("%s bound file differs from the rendered document", binding.name)
			}
		}
	}
	for _, name := range []string{"receipt", "lifecycle"} {
		prefix := "INDEXER_" + strings.ToUpper(name) + "_CONFIG"
		t.Run(name+"ChangedFile", func(t *testing.T) {
			path := composed[prefix+"_FILE"]
			changed := append(append([]byte(nil), fileBytes[name]...), '\n')
			if err := os.WriteFile(path, changed, 0o600); err != nil {
				t.Fatal(err)
			}
			var got map[string]any
			if err := ReadBoundDocument(path, composed[prefix+"_SHA256"], &got); err == nil || err.Error() != "runtime_document_binding_mismatch" {
				t.Fatalf("changed %s file: %v", name, err)
			}
			if err := os.WriteFile(path, fileBytes[name], 0o600); err != nil {
				t.Fatal(err)
			}
		})
		t.Run(name+"BothForms", func(t *testing.T) {
			t.Setenv(prefix+"_DOCUMENT", base64.StdEncoding.EncodeToString(fileBytes[name]))
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), prefix+"_FILE and "+prefix+"_DOCUMENT are both set") {
				t.Fatalf("both %s forms: %v", name, err)
			}
		})
	}
}
