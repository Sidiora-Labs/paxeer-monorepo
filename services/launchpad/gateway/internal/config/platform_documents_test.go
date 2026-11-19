package config

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	osexec "os/exec"
	"path/filepath"
	"testing"
)

func TestPlatformDocuments(t *testing.T) {
	root := filepath.Join("..", "..", "..", "..", "..")
	values := t.TempDir()
	if err := os.Mkdir(filepath.Join(values, "expected-schema"), 0o755); err != nil {
		t.Fatal(err)
	}
	common, err := os.ReadFile(filepath.Join(root, "deploy", "launchpad", "documents", "values", "common.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(values, "common.json"), common, 0o600); err != nil {
		t.Fatal(err)
	}
	livestream, err := os.ReadFile(filepath.Join(root, "deploy", "launchpad", "documents", "values", "livestream.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(values, "livestream.json"), livestream, 0o600); err != nil {
		t.Fatal(err)
	}
	example, err := os.ReadFile(filepath.Join(root, "deploy", "launchpad", "documents", "values", "gateway.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	var gatewayValues map[string]any
	if err := json.Unmarshal(example, &gatewayValues); err != nil {
		t.Fatal(err)
	}
	runtimeValues := gatewayValues["runtime"].(map[string]any)
	files := t.TempDir()
	for _, name := range []string{"ca.pem", "client-cert.pem", "server-cert.pem"} {
		path := filepath.Join(files, name)
		data := []byte(name + " test bytes")
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
		runtimeValues[map[string]string{"ca.pem": "caFile", "client-cert.pem": "clientCertFile", "server-cert.pem": "serverCertFile"}[name]] = path
		hash := sha256.Sum256(data)
		runtimeValues[map[string]string{"ca.pem": "caSha256", "client-cert.pem": "clientCertSha256", "server-cert.pem": "serverCertSha256"}[name]] = hex.EncodeToString(hash[:])
	}
	runtimeValues["identityListenAddress"] = "127.0.0.1:19443"
	gatewayRaw, err := json.Marshal(gatewayValues)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(values, "gateway.json"), gatewayRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	expected := map[string]any{
		"service": "media/gateway", "runtime_role": "mediagatewayd", "registry_digest": "0000000000000000000000000000000000000000000000000000000000000000",
		"interface_digest": "0000000000000000000000000000000000000000000000000000000000000000", "reader_version": 1,
		"migrations": map[string]string{"1": "0000000000000000000000000000000000000000000000000000000000000000"},
		"schemas":    map[string]string{"app": "0000000000000000000000000000000000000000000000000000000000000000"},
		"epoch":      1, "region": "mainnet-beta", "privileges": []map[string]any{{"schema": "app", "object": "items", "privileges": []string{"SELECT"}}},
	}
	expectedRaw, err := json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(values, "expected-schema", "gateway.json"), expectedRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	command := osexec.Command("python3", filepath.Join(root, "deploy", "launchpad", "documents", "compose.py"), "--values", values, "--out", out, "--service", "gateway")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("compose: %v: %s", err, output)
	}

	composedRaw, err := os.ReadFile(filepath.Join(out, "gateway.json"))
	if err != nil {
		t.Fatal(err)
	}
	var composed map[string]string
	if err := json.Unmarshal(composedRaw, &composed); err != nil {
		t.Fatal(err)
	}
	setPlatformDocumentEnv(t, composed)
	if _, err := Load(); err != nil {
		t.Fatalf("Load composed gateway documents: %v", err)
	}

	for _, document := range []struct {
		name   string
		sha    string
		prefix string
	}{
		{"DB_BUDGET_DOCUMENT", "DB_BUDGET_SHA256", "DB_BUDGET"},
		{"EXPECTED_SCHEMA_DOCUMENT", "EXPECTED_SCHEMA_SHA256", "EXPECTED_SCHEMA"},
		{"GATEWAY_RUNTIME_DOCUMENT", "GATEWAY_RUNTIME_SHA256", "GATEWAY_RUNTIME"},
		{"IDENTITY_CONFIG_DOCUMENT", "IDENTITY_CONFIG_SHA256", "IDENTITY_CONFIG"},
	} {
		raw, err := base64.StdEncoding.DecodeString(composed[document.name])
		if err != nil {
			t.Fatalf("%s: decode: %v", document.name, err)
		}
		if len(raw) == 0 {
			t.Fatalf("%s: empty document", document.name)
		}
		changed := append([]byte(nil), raw...)
		changed[0] ^= 1
		t.Setenv(document.name, base64.StdEncoding.EncodeToString(changed))
		if _, err := Load(); err == nil || !containsDocumentName(err, document.prefix) {
			t.Fatalf("%s: changed document accepted: %v", document.name, err)
		}
		t.Setenv(document.name, composed[document.name])
		if _, err := Load(); err != nil {
			t.Fatalf("%s: restoring document: %v", document.name, err)
		}
	}
}

func setPlatformDocumentEnv(t *testing.T, composed map[string]string) {
	t.Helper()
	for key, value := range map[string]string{
		"NODE_ENV": "production", "RUNTIME_PROFILE": "container", "BIND_ADDRESS": "0.0.0.0", "SECRETS_PROVIDER": "env", "PORT": "8080",
		"CORS_ALLOWED_ORIGINS": "https://app.kindlelaunch.example", "DATABASE_URL": "postgres://gateway:test@127.0.0.1:5432/gateway", "DB_POOL_ID": "mediagatewayd_pool", "INSTANCE_ID": "gateway-platform-test", "ZONE": "mainnet-beta-eu",
		"GATEWAY_JWT_SECRET": "gateway-platform-test-secret", "IDENTITY_LIMITER_KEY": "gateway-platform-limiter-key", "IDENTITY_SERVICE_TOKEN": "gateway-platform-service-token", "IDENTITY_SIGNING_SECRET": "gateway-platform-signing-secret",
		"METADATA_UPLOAD_URL": "https://metadata.kindlelaunch.example", "PUBLIC_URL": "https://app.kindlelaunch.example", "REDIS_URL": "redis://127.0.0.1:6379/0", "RPC_URL": "https://rpc.kindlelaunch.example",
		"S3_ACCESS_KEY_ID": "gateway-platform-access", "S3_ENDPOINT": "https://objects.kindlelaunch.example", "S3_SECRET_ACCESS_KEY": "gateway-platform-secret", "SOCIAL_HTTP_URL": "https://social.kindlelaunch.example", "SOCIAL_WS_URL": "wss://social.kindlelaunch.example",
		"METADATA_BUCKET": "media-approved", "GATEWAY_RUNTIME_FILE": "", "IDENTITY_CONFIG_FILE": "", "DB_BUDGET_FILE": "", "EXPECTED_SCHEMA_FILE": "",
	} {
		t.Setenv(key, value)
	}
	for key, value := range composed {
		t.Setenv(key, value)
	}
}

func containsDocumentName(err error, name string) bool {
	return err != nil && (len(name) > 0 && (fmt.Sprint(err) != "" && (contains(fmt.Sprint(err), name+"_DOCUMENT") || contains(fmt.Sprint(err), name))))
}

func contains(value, needle string) bool {
	for i := 0; i+len(needle) <= len(value); i++ {
		if value[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
