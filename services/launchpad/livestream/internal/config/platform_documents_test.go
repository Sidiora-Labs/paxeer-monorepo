package config

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
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

	if len(composed) != 48 {
		t.Fatalf("livestream composer wrote %d variables, want 48 runtime settings and document bindings", len(composed))
	}
	raw, err := os.ReadFile(filepath.Join(valuesDir, "livestream.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	var values struct {
		Environment map[string]string `json:"environment"`
		Network     struct {
			RPC string `json:"rpc_url"`
		} `json:"network"`
		Addresses struct {
			Bind string `json:"bind"`
			Port int    `json:"port"`
		} `json:"addresses"`
		Names struct {
			Zone string `json:"zone"`
		} `json:"names"`
		Limits struct {
			Wallet int `json:"max_streams_per_wallet"`
			Global int `json:"max_global_streams"`
		} `json:"limits"`
		Periods struct {
			Refresh   int    `json:"secrets_refresh_seconds"`
			Timestamp string `json:"webhook_timestamp_unit"`
		} `json:"periods"`
		Endpoints struct {
			API string `json:"livepeer_api_url"`
		} `json:"endpoints"`
		Credentials struct {
			Key string `json:"livepeer_api_key"`
		} `json:"credentials"`
		BudgetPolicy json.RawMessage `json:"budget_policy"`
	}
	if err := json.Unmarshal(raw, &values); err != nil {
		t.Fatal(err)
	}
	expected := map[string]string{}
	for name, value := range contract.Fixed {
		expected[name] = value
	}
	for name, value := range values.Environment {
		expected[name] = value
	}
	for name, value := range map[string]string{
		"BIND_ADDRESS": values.Addresses.Bind, "PORT": strconv.Itoa(values.Addresses.Port),
		"RPC_URL": values.Network.RPC, "ZONE": values.Names.Zone,
		"MAX_STREAMS_PER_WALLET": strconv.Itoa(values.Limits.Wallet), "MAX_GLOBAL_STREAMS": strconv.Itoa(values.Limits.Global),
		"SECRETS_REFRESH_SECONDS": strconv.Itoa(values.Periods.Refresh), "LIVEPEER_WEBHOOK_TIMESTAMP_UNIT": values.Periods.Timestamp,
		"LIVEPEER_API_URL": values.Endpoints.API, "LIVEPEER_API_KEY": values.Credentials.Key,
	} {
		expected[name] = value
	}
	budget, err := os.ReadFile(filepath.Join(valuesDir, "common.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	schema, err := os.ReadFile(filepath.Join(valuesDir, "expected-schema", "livestream.json"))
	if err != nil {
		t.Fatal(err)
	}
	for name, document := range map[string][]byte{"DB_BUDGET": budget, "EXPECTED_SCHEMA": schema, "LIVESTREAM_BUDGET_POLICY": values.BudgetPolicy} {
		if name != "EXPECTED_SCHEMA" {
			var value any
			if err := json.Unmarshal(document, &value); err != nil {
				t.Fatal(err)
			}
			document, err = json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
		}
		expected[name+"_DOCUMENT"] = base64.StdEncoding.EncodeToString(document)
		expected[name+"_SHA256"] = fmt.Sprintf("%x", sha256.Sum256(document))
	}
	if !reflect.DeepEqual(composed, expected) {
		for name, value := range expected {
			if actual, ok := composed[name]; !ok || actual != value {
				t.Errorf("composed variable %s differs from its deployment value", name)
			}
		}
		t.Fatal("composed environment differs from deployment values")
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
	schema := []byte(`{"service":"media/livestream","runtime_role":"livestream_runtime","reader_version":1}`)
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
	configuration := reflect.TypeOf(Config{})
	for i := 0; i < configuration.NumField(); i++ {
		name, _, _ := strings.Cut(configuration.Field(i).Tag.Get("env"), ",")
		t.Setenv(name, "")
	}
	for name, value := range contract.Fixed {
		t.Setenv(name, value)
	}
	for name, value := range composed {
		t.Setenv(name, value)
	}
	testValues := map[string]string{
		"DATABASE_URL":           os.Getenv("L40_TEST_POSTGRES_DSN"),
		"DB_POOL_ID":             "livestreamd_pool",
		"IDENTITY_SERVICE_TOKEN": "test-identity-token",
		"INSTANCE_ID":            "livestream-test-1",
		"REDIS_URL":              os.Getenv("L40_TEST_REDIS_URL"),
	}
	for _, name := range contract.Required {
		if value, ok := composed[name]; ok {
			t.Setenv(name, value)
			continue
		}
		value, ok := testValues[name]
		if !ok || value == "" {
			t.Fatalf("no test value for required variable %s", name)
		}
		t.Setenv(name, value)
	}
	t.Setenv("LOG_LEVEL", "info")
	if composed["DB_BUDGET_DOCUMENT"] == "" || composed["EXPECTED_SCHEMA_DOCUMENT"] == "" {
		t.Fatal(fmt.Sprintf("composed startup documents are incomplete: %v", composed))
	}
}
