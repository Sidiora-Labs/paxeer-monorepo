package config

import (
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPlatformDocuments(t *testing.T) {
	repoRoot := filepath.Join(filepath.Dir(filePath(t)), "../../../../..")
	values := filepath.Join(t.TempDir(), "values")
	if err := os.MkdirAll(filepath.Join(values, "expected-schema"), 0o700); err != nil {
		t.Fatal(err)
	}
	copyFile := func(source, target string) {
		raw, err := os.ReadFile(source)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, raw, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	copyFile(filepath.Join(repoRoot, "deploy/launchpad/documents/values/common.example.json"), filepath.Join(values, "common.json"))
	copyFile(filepath.Join(repoRoot, "deploy/launchpad/documents/values/charts.example.json"), filepath.Join(values, "charts.example.json"))
	expected := map[string]any{
		"service": "charts", "runtime_role": "chartsd", "registry_digest": strings.Repeat("0", 64),
		"interface_digest": strings.Repeat("0", 64), "reader_version": 1, "migrations": map[string]string{"1": strings.Repeat("0", 64)},
		"schemas": map[string]string{"candles": strings.Repeat("0", 64)}, "epoch": 1, "region": "mainnet-beta",
		"privileges": []map[string]any{{"schema": "candles", "object": "chart_stream", "privileges": []string{"SELECT"}}},
	}
	expectedBytes, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(values, "expected-schema", "charts.json"), expectedBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(t.TempDir(), "documents")
	command := exec.Command("python3", filepath.Join(repoRoot, "deploy/launchpad/documents/compose.py"), "--values", values, "--out", out, "--service", "charts")
	command.Dir = repoRoot
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compose: %v\n%s", err, output)
	}
	recordBytes, err := os.ReadFile(filepath.Join(out, "charts.json"))
	if err != nil {
		t.Fatal(err)
	}
	var documents map[string]string
	if err := json.Unmarshal(recordBytes, &documents); err != nil {
		t.Fatal(err)
	}

	env := map[string]string{
		"NODE_ENV": "production", "RUNTIME_PROFILE": "container", "BIND_ADDRESS": "0.0.0.0", "SECRETS_PROVIDER": "env", "CHAIN_ID": "125", "PORT": "8080",
		"EVENT_EMITTER_ADDRESS": "0x622E1bb90E0E2B6E6e5e9e2DbE1555389E8A3EF6", "POOL_REGISTRY_ADDRESS": "0x1dC357d2B7B850332a91F06A9E66634F46fC4Ba7", "ROUTER_ADDRESS": "0x78FB06f525cD964388f2305A489035263E31C6e4", "FACTORY_ADDRESS": "0x3993D004166738d1069B5C4e0a923f7B9D5D8830", "QUOTER_ADDRESS": "0x793006f4F321b8863F26DBD75b11bAa01DA181dc", "PROTOCOL_CONFIG_ADDRESS": "0xF3669eDc475Cb6E1F0d7635240ABD8e8FB7E20C6", "FEE_ACCUMULATOR_ADDRESS": "0x7b14808b72f61664cDb79D25E6d1587655571a66", "SIDIORA_NFT_ADDRESS": "0xe8DcC844a8006dbf64f5Bb851bCad90292a6409B", "FEES_ROUTER_ADDRESS": "0x05681dA6f611C2dfFD3E92611F6dE2d247647927", "POOL_BEACON_ADDRESS": "0x0c60Ea33B1DD602A17E1EF1875eE912F3fad71BE",
		"DATABASE_URL": "postgres://u:p@localhost:5432/charts", "REDIS_URL": "redis://localhost:6379", "REDIS_BULL_URL": "redis://localhost:6379/1", "RPC_URL": "https://api1.mainnet-beta.paxeer.network", "NATS_URL": "nats://nats.internal:4222", "NATS_CREDENTIALS": "test-credential", "WEBHOOK_HMAC_SECRET": strings.Repeat("t", 32), "DB_POOL_ID": "chartsd_pool", "INSTANCE_ID": "charts-test", "ZONE": "zone-a",
	}
	for key, value := range documents {
		env[key] = value
	}
	setEnvironment(t, env)
	if _, err := Load(); err != nil {
		t.Fatalf("Load composed documents: %v", err)
	}

	for name, encoded := range documents {
		if !strings.HasSuffix(name, "_DOCUMENT") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			setEnvironment(t, env)
			raw, err := base64.StdEncoding.Strict().DecodeString(encoded)
			if err != nil || len(raw) == 0 {
				t.Fatalf("decode %s: %v", name, err)
			}
			raw[0] ^= 1
			t.Setenv(name, base64.StdEncoding.EncodeToString(raw))
			if _, err := Load(); err == nil {
				t.Fatalf("Load accepted changed %s", name)
			}
		})
	}
}

func setEnvironment(t *testing.T, values map[string]string) {
	t.Helper()
	for key, value := range values {
		t.Setenv(key, value)
	}
}

func filePath(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime caller unavailable")
	}
	return file
}
