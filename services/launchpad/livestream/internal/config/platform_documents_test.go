package config

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type launchpadServiceContract struct {
	Fixed    map[string]string `json:"fixed"`
	Required []string          `json:"required"`
}

func TestPlatformDocumentsComposeAndLoad(t *testing.T) {
	contract := readLivestreamContract(t)
	valuesDir, outputDir := writeLivestreamComposerInputs(t)
	composeLivestream(t, valuesDir, outputDir)
	composed := readComposedLivestream(t, outputDir)

	if len(composed) != 4 {
		t.Fatalf("livestream composer wrote %d variables, want budget and expected schema pairs", len(composed))
	}
	for _, name := range []string{"DB_BUDGET_DOCUMENT", "DB_BUDGET_SHA256", "EXPECTED_SCHEMA_DOCUMENT", "EXPECTED_SCHEMA_SHA256"} {
		if composed[name] == "" {
			t.Fatalf("composer omitted %s", name)
		}
	}

	setLivestreamEnvironment(t, contract, composed)
	if _, err := Load(); err != nil {
		t.Fatalf("composed livestream environment rejected: %v", err)
	}
}

func TestPlatformDocumentsRejectChangedDocument(t *testing.T) {
	contract := readLivestreamContract(t)
	valuesDir, outputDir := writeLivestreamComposerInputs(t)
	composeLivestream(t, valuesDir, outputDir)
	composed := readComposedLivestream(t, outputDir)

	for _, name := range []string{"DB_BUDGET_DOCUMENT", "EXPECTED_SCHEMA_DOCUMENT"} {
		t.Run(name, func(t *testing.T) {
			setLivestreamEnvironment(t, contract, composed)
			raw, err := base64.StdEncoding.Strict().DecodeString(composed[name])
			if err != nil {
				t.Fatalf("decode %s: %v", name, err)
			}
			if len(raw) == 0 {
				t.Fatalf("%s is empty", name)
			}
			raw[0] ^= 1
			t.Setenv(name, base64.StdEncoding.EncodeToString(raw))
			if _, err := Load(); err == nil {
				t.Fatalf("Load accepted a changed %s", name)
			}
		})
	}
}

func readLivestreamContract(t *testing.T) launchpadServiceContract {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locating platform document test")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(source), "../../../../.."))
	contractPath := filepath.Join(repo, "deploy/launchpad/railway/variables.json")
	raw, err := os.ReadFile(contractPath)
	if err != nil {
		t.Fatalf("read launchpad variable contract: %v", err)
	}
	var contract map[string]launchpadServiceContract
	if err := json.Unmarshal(raw, &contract); err != nil {
		t.Fatalf("decode launchpad variable contract: %v", err)
	}
	livestream, ok := contract["livestream"]
	if !ok {
		t.Fatal("livestream entry missing from variable contract")
	}
	return livestream
}

func writeLivestreamComposerInputs(t *testing.T) (string, string) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locating platform document test")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(source), "../../../../.."))
	valuesDir := t.TempDir()
	if err := os.Mkdir(filepath.Join(valuesDir, "expected-schema"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"common.example.json", "livestream.example.json"} {
		sourcePath := filepath.Join(repo, "deploy/launchpad/documents/values", name)
		raw, err := os.ReadFile(sourcePath)
		if err != nil {
			t.Fatalf("read example %s: %v", name, err)
		}
		if err := os.WriteFile(filepath.Join(valuesDir, name), raw, 0600); err != nil {
			t.Fatalf("write test value %s: %v", name, err)
		}
	}
	schema := []byte(`{"service":"livestream","runtime_role":"livestream_runtime","reader_version":1}`)
	if err := os.WriteFile(filepath.Join(valuesDir, "expected-schema", "livestream.json"), schema, 0600); err != nil {
		t.Fatal(err)
	}
	return valuesDir, t.TempDir()
}

func composeLivestream(t *testing.T, valuesDir, outputDir string) {
	t.Helper()
	_, source, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("locating platform document test")
	}
	repo := filepath.Clean(filepath.Join(filepath.Dir(source), "../../../../.."))
	composer := filepath.Join(repo, "deploy/launchpad/documents/compose.py")
	command := exec.Command("python3", composer, "--values", valuesDir, "--out", outputDir, "--service", "livestream")
	command.Dir = repo
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compose livestream documents: %v\n%s", err, output)
	}
}

func readComposedLivestream(t *testing.T, outputDir string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(outputDir, "livestream.json"))
	if err != nil {
		t.Fatalf("read composed livestream documents: %v", err)
	}
	var composed map[string]string
	if err := json.Unmarshal(raw, &composed); err != nil {
		t.Fatalf("decode composed livestream documents: %v", err)
	}
	return composed
}

func setLivestreamEnvironment(t *testing.T, contract launchpadServiceContract, composed map[string]string) {
	t.Helper()
	for name, value := range contract.Fixed {
		t.Setenv(name, value)
	}
	testValues := map[string]string{
		"DATABASE_URL":            "postgres://fixture:fixture@localhost:5432/livestream?sslmode=disable",
		"DB_POOL_ID":              "livestreamd_pool",
		"IDENTITY_SERVICE_TOKEN":  "test-identity-token",
		"INSTANCE_ID":             "livestream-test-1",
		"LIVEPEER_API_KEY":        "test-livepeer-api-key",
		"LIVEPEER_API_URL":        "https://livepeer.studio/api",
		"LIVEPEER_WEBHOOK_SECRET": strings.Repeat("w", 32),
		"REDIS_URL":               "redis://localhost:6379/0",
		"ZONE":                    "mainnet-beta-a",
	}
	for _, name := range contract.Required {
		if value, ok := composed[name]; ok {
			t.Setenv(name, value)
			continue
		}
		value, ok := testValues[name]
		if !ok {
			t.Fatalf("no test value for required variable %s", name)
		}
		t.Setenv(name, value)
	}
	t.Setenv("LIVEPEER_PROVIDER_MODE", "available")
	t.Setenv("LOG_LEVEL", "info")
	t.Setenv("MAX_STREAMS_PER_WALLET", "3")
	if composed["DB_BUDGET_DOCUMENT"] == "" || composed["EXPECTED_SCHEMA_DOCUMENT"] == "" {
		t.Fatal(fmt.Sprintf("composed startup documents are incomplete: %v", composed))
	}
}
