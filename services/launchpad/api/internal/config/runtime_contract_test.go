package config

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	runtimedb "github.com/Sidiora-Technologies/KindleLaunch/shared/db"
)

func TestRuntimeDocumentsAndMaterial(t *testing.T) {
	raw := []byte(`{"service":"core/api"}`)
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	path := filepath.Join(t.TempDir(), "document.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, path, inline, digest string
		want                       string
	}{
		{"file", path, "", digest, ""},
		{"inline", "", base64.StdEncoding.EncodeToString(raw), digest, ""},
		{"missing source", "", "", digest, "runtime_document_binding_missing"},
		{"two sources", path, base64.StdEncoding.EncodeToString(raw), digest, "runtime_document_binding_missing"},
		{"bad digest", path, "", strings.Repeat("0", 64), "runtime_document_binding_mismatch"},
		{"invalid base64", "", "%%%", digest, "runtime_document_invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := boundBytes(tc.path, tc.inline, tc.digest)
			if tc.want == "" {
				if err != nil || string(got) != string(raw) {
					t.Fatalf("document = %q, %v", got, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("document error = %v, want %s", err, tc.want)
			}
		})
	}
	var doc struct {
		Service string `json:"service"`
	}
	if err := decodeBoundDocument(path, "", digest, &doc); err != nil || doc.Service != "core/api" {
		t.Fatalf("decoded document = %+v, %v", doc, err)
	}
	if err := decodeBoundDocument("", base64.StdEncoding.EncodeToString(append(raw, []byte(` {}`)...)), fmt.Sprintf("%x", sha256.Sum256(append(raw, []byte(` {}`)...))), &doc); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("trailing document = %v", err)
	}
	bad := []byte(`{"unknown":1}`)
	if err := decodeBoundDocument("", base64.StdEncoding.EncodeToString(bad), fmt.Sprintf("%x", sha256.Sum256(bad)), &doc); err == nil {
		t.Fatal("unknown document field admitted")
	}
	c := Config{RuntimeBudgetDocument: base64.StdEncoding.EncodeToString(raw), RuntimeBudgetSHA256: digest}
	if err := c.registerInlineDocuments(); err != nil {
		t.Fatal(err)
	}
	doc = struct {
		Service string `json:"service"`
	}{}
	if err := ReadBoundDocument("", digest, &doc); err != nil || doc.Service != "core/api" {
		t.Fatalf("registered document = %+v, %v", doc, err)
	}
	c.RuntimeBudgetFile = path
	if err := c.registerInlineDocuments(); err == nil {
		t.Fatal("ambiguous budget sources admitted")
	}
	c.RuntimeBudgetFile = ""
	c.RuntimeBudgetSHA256 = ""
	if err := c.registerInlineDocuments(); err == nil {
		t.Fatal("unbound inline document admitted")
	}
	for _, tc := range []struct{ path, inline, want string }{
		{path, "", string(raw)},
		{"", "inline-material", "inline-material"},
	} {
		got, err := materialBytes("FILE", tc.path, "INLINE", tc.inline)
		if err != nil || string(got) != tc.want {
			t.Fatalf("material = %q, %v", got, err)
		}
	}
	if _, err := materialBytes("FILE", "", "INLINE", ""); err == nil {
		t.Fatal("missing material admitted")
	}
	if _, err := materialBytes("FILE", path, "INLINE", "duplicate"); err == nil {
		t.Fatal("ambiguous material admitted")
	}
}

func TestRuntimeRoleAndPoolAdmission(t *testing.T) {
	for _, tc := range []struct {
		role, environment, want string
		valid                   bool
	}{
		{"rest", "production", "apid:rest", true},
		{"sockets", "production", "apid:sockets", true},
		{"", "development", "apid:development", true},
		{"development", "test", "apid:development", true},
		{"", "production", "", false},
		{"unknown", "production", "", false},
	} {
		c := Config{APIRole: tc.role, NodeEnv: tc.environment}
		got, err := c.RuntimeRole()
		if (err == nil) != tc.valid || got != tc.want {
			t.Fatalf("role %q environment %q = %q, %v", tc.role, tc.environment, got, err)
		}
	}
	for _, tc := range []struct {
		profile, bind string
		port          int
		want          string
	}{
		{"container", "127.0.0.1", 8080, "container_bind_mismatch"},
		{"owned_local", "0.0.0.0", 8080, "owned_loopback_required"},
		{"unknown", "127.0.0.1", 8080, "runtime_profile_required"},
		{"container", "0.0.0.0", 8080, "runtime_pool_configuration_missing"},
	} {
		c := Config{NodeEnv: "production", RuntimeProfile: tc.profile, BindAddress: tc.bind, Port: tc.port}
		pool, err := c.OpenRuntimePool(context.Background(), "apid:rest", runtimedb.PoolOptions{})
		if pool != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("pool admission = %v, %v; want %s", pool, err, tc.want)
		}
	}
	c := Config{NodeEnv: "production"}
	if err := c.CheckRuntimeSchema(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "runtime_schema_configuration_missing") {
		t.Fatalf("schema admission = %v", err)
	}
}

func TestProductionSecretBindingAndRotation(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		environment string
		refresh     int
		want        string
	}{
		{"unknown", 30, "environment_invalid"},
		{"production", 0, "secret_refresh_bound_invalid"},
		{"production", 61, "secret_refresh_bound_invalid"},
	} {
		c := Config{NodeEnv: tc.environment, SecretsRefreshSeconds: tc.refresh}
		if _, _, err := c.ResolveProductionSecrets(ctx); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("secret admission %q %d: %v", tc.environment, tc.refresh, err)
		}
	}
	for _, env := range []string{"development", "test"} {
		c := Config{NodeEnv: env}
		check, stop, err := c.ResolveProductionSecrets(ctx)
		if err != nil || check != nil || stop == nil {
			t.Fatalf("%s secret path: check=%v stop=%v err=%v", env, check != nil, stop != nil, err)
		}
		stop()
	}
	for key, value := range map[string]string{
		"DATABASE_URL":         "postgres://localhost/api",
		"REDIS_URL":            "redis://localhost/0",
		"IDENTITY_SIGNING_KEY": "identity-signing-material",
		"STREAM_SIGNING_KEY":   "stream-signing-material",
		"ADMIN_API_KEY":        "admin-material",
	} {
		t.Setenv(key, value)
	}
	c := Config{NodeEnv: "production", SecretsProvider: "env", SecretsRefreshSeconds: 30, AdminAPIKey: "configured"}
	check, stop, err := c.ResolveProductionSecrets(ctx)
	if err != nil {
		t.Fatalf("bound secrets: %v", err)
	}
	defer stop()
	if c.DatabaseURL != "postgres://localhost/api" || c.RedisURL != "redis://localhost/0" || c.IdentitySigningKey != "identity-signing-material" || c.StreamSigningKey != "stream-signing-material" || c.AdminAPIKey != "admin-material" {
		t.Fatalf("resolved secret values: %+v", c)
	}
	if err := check(ctx); err != nil {
		t.Fatalf("unchanged secrets: %v", err)
	}
	t.Setenv("STREAM_SIGNING_KEY", "rotated-signing-material")
	if err := check(ctx); err == nil || !strings.Contains(err.Error(), "required_secret_rotated_restart_required") {
		t.Fatalf("rotation result: %v", err)
	}
	if err := check(ctx); err == nil || !strings.Contains(err.Error(), "required_secret_rotated_restart_required") {
		t.Fatalf("latched rotation: %v", err)
	}
}

func TestIdentityMaterialFilesAndBrokerCredentialAdmission(t *testing.T) {
	dir := t.TempDir()
	files := []struct{ name, body string }{{"ca.pem", "ca-data"}, {"cert.pem", "cert-data"}, {"key.pem", "key-data"}}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(dir, file.name), []byte(file.body), 0600); err != nil {
			t.Fatal(err)
		}
	}
	c := Config{IdentityCAFile: filepath.Join(dir, files[0].name), IdentityClientCertFile: filepath.Join(dir, files[1].name), IdentityClientKeyFile: filepath.Join(dir, files[2].name)}
	ca, cert, key, err := c.IdentityTLSMaterial()
	if err != nil || string(ca) != "ca-data" || string(cert) != "cert-data" || string(key) != "key-data" {
		t.Fatalf("file identity material: %q %q %q %v", ca, cert, key, err)
	}
	c.IdentityClientCertFile = filepath.Join(dir, "missing.pem")
	if _, _, _, err := c.IdentityTLSMaterial(); err == nil || !strings.Contains(err.Error(), "IDENTITY_CLIENT_CERT_FILE") {
		t.Fatalf("missing client certificate: %v", err)
	}
	for _, tc := range []struct{ file, inline, want string }{
		{"", "", "required"},
		{"file.creds", "inline", "both set"},
		{"", "malformed", "broker_credentials_invalid"},
	} {
		config := Config{NATSCredentials: tc.file, NATSCredentialsCreds: tc.inline}
		if _, err := config.NATSCredentialsOption(); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("credentials %q %q: %v", tc.file, tc.inline, err)
		}
	}
	option, err := (&Config{NATSCredentials: filepath.Join(dir, "user.creds")}).NATSCredentialsOption()
	if err != nil || option == nil {
		t.Fatalf("credential file option: %v", err)
	}
}
