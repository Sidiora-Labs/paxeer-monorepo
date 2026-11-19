package config_test

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/ranking-algo/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
)

type platformDocumentValues struct {
	composer string
	values   string
	ranking  map[string]any
	schema   []byte
	fixed    map[string]string
	required []string
}

func platformReadJSON(t *testing.T, path string, target any) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, target); err != nil {
		t.Fatal(err)
	}
	return raw
}

func platformWrite(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func platformWriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	platformWrite(t, path, raw)
}

func platformValues(t *testing.T) platformDocumentValues {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	documents := filepath.Join(root, "deploy", "launchpad", "documents")
	var contract map[string]struct {
		Fixed    map[string]string `json:"fixed"`
		Required []string          `json:"required"`
	}
	platformReadJSON(t, filepath.Join(root, "deploy", "launchpad", "railway", "variables.json"), &contract)
	entry, ok := contract["ranking"]
	if !ok || len(entry.Fixed) == 0 || len(entry.Required) == 0 {
		t.Fatal("ranking variable contract is required")
	}
	f := platformDocumentValues{
		composer: filepath.Join(documents, "compose.py"), values: t.TempDir(),
		fixed: entry.Fixed, required: entry.Required,
	}
	common, err := os.ReadFile(filepath.Join(documents, "values", "common.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	platformWrite(t, filepath.Join(f.values, "common.example.json"), common)
	platformReadJSON(t, filepath.Join(documents, "values", "ranking.example.json"), &f.ranking)
	digest := strings.Repeat("0", 64)
	expected := production.ExpectedSchema{
		Service: "core/ranking-algo", RuntimeRole: "rankingd", RegistryDigest: digest, InterfaceDigest: digest,
		ReaderVersion: 1, Migrations: map[int64]string{1: digest}, Schemas: map[string]string{"ranking": digest},
		Epoch: 1, Region: "mainnet-beta",
		Privileges: []production.Grant{{Schema: "ranking", Object: "snapshots", Privileges: []string{"SELECT"}}},
	}
	f.schema, err = json.Marshal(expected)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(f.values, "expected-schema"), 0700); err != nil {
		t.Fatal(err)
	}
	platformWrite(t, filepath.Join(f.values, "expected-schema", "ranking.json"), f.schema)
	platformWriteJSON(t, filepath.Join(f.values, "ranking.example.json"), f.ranking)
	return f
}

func (f platformDocumentValues) compose(t *testing.T, mode string) map[string]string {
	t.Helper()
	out := t.TempDir()
	command := exec.Command("python3", f.composer, mode, f.values, "--out", out, "--service", "ranking")
	command.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("compose ranking: %v: %s", err, output)
	}
	path := filepath.Join(out, "ranking.json")
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("ranking environment file must have mode 0600: %v", err)
	}
	var composed map[string]string
	platformReadJSON(t, path, &composed)
	if len(composed) != 6 {
		t.Fatalf("ranking environment has %d variables, want three document/digest pairs", len(composed))
	}
	for name, encoded := range composed {
		if !strings.HasSuffix(name, "_DOCUMENT") {
			continue
		}
		raw, err := base64.StdEncoding.Strict().DecodeString(encoded)
		if err != nil {
			t.Fatalf("%s must be standard base64: %v", name, err)
		}
		sum := sha256.Sum256(raw)
		if composed[strings.TrimSuffix(name, "_DOCUMENT")+"_SHA256"] != hex.EncodeToString(sum[:]) {
			t.Fatalf("%s must bind the exact JSON bytes", name)
		}
		if bytes.Contains(output, []byte(encoded)) || bytes.Contains(output, raw) {
			t.Fatalf("%s must be written only to the environment file", name)
		}
	}
	return composed
}

func platformEnvironment(t *testing.T, f platformDocumentValues, composed, credentials map[string]string) {
	t.Helper()
	var clear func(reflect.Type)
	clear = func(typ reflect.Type) {
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.Anonymous {
				clear(field.Type)
			} else if tag := field.Tag.Get("env"); tag != "" {
				t.Setenv(strings.Split(tag, ",")[0], "")
			}
		}
	}
	clear(reflect.TypeOf(config.Config{}))
	for key, value := range f.fixed {
		t.Setenv(key, value)
	}
	values := map[string]string{
		"DATABASE_URL": "postgres://rankingd:test-only-password@localhost:5432/ranking",
		"REDIS_URL":    "redis://localhost:6379/0", "REDIS_BULL_URL": "redis://localhost:6379/1",
		"RPC_URL": "https://rpc.example.com", "ZONE": "mainnet-beta",
		"DB_POOL_ID": "rankingd_pool", "INSTANCE_ID": "ranking-1",
	}
	for key, value := range credentials {
		values[key] = value
	}
	for _, key := range f.required {
		value, exists := composed[key]
		if !exists {
			value, exists = values[key]
		}
		if !exists || value == "" {
			t.Fatalf("test must supply required ranking variable %s", key)
		}
		t.Setenv(key, value)
	}
	for key := range composed {
		found := false
		for _, required := range f.required {
			found = found || required == key
		}
		if !found {
			t.Fatalf("composer emitted variable outside the ranking contract: %s", key)
		}
	}
}

func platformCertificates(t *testing.T, values map[string]any) map[string]string {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	certificate := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "Ranking test certificate"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true,
		KeyUsage:    x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:    []string{values["visibilityDNS"].(string), values["gatewayDNS"].(string)},
	}
	der, err := x509.CreateCertificate(rand.Reader, certificate, certificate, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})
	if _, err := tls.X509KeyPair(certPEM, keyPEM); err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for _, binding := range []struct{ file, digest string }{
		{"caFile", "caSha256"}, {"clientCertFile", "clientCertSha256"}, {"serverCertFile", "serverCertSha256"},
	} {
		path := filepath.Join(dir, binding.file+".pem")
		platformWrite(t, path, certPEM)
		values[binding.file] = path
		values[binding.digest] = production.Checksum(certPEM)
	}
	values["visibilityCertSHA256"] = production.Checksum(der)
	values["gatewayCertSHA256"] = production.Checksum(der)
	return map[string]string{
		values["clientKeyReference"].(string): string(keyPEM),
		values["serverKeyReference"].(string): string(keyPEM),
	}
}

func platformCursorKey(t *testing.T, cfg config.Config, want []byte) {
	t.Helper()
	var binding config.RankingRuntimeBinding
	if err := config.ReadBoundDocument(cfg.RankingRuntimeFile, cfg.RankingRuntimeSHA256, &binding); err != nil {
		t.Fatal(err)
	}
	if binding.CursorKeyReference != "TEST_RANKING_CURSOR_KEY" {
		t.Fatal("loaded ranking binding must reference the deployment cursor key")
	}
	provider, err := secrets.NewChecked(secrets.Options{Provider: secrets.ProviderType(cfg.SecretsProvider)})
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := provider.GetRequired(context.Background(), binding.CursorKeyReference)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.RawURLEncoding.Strict().DecodeString(encoded)
	if err != nil || len(decoded) < 32 || !bytes.Equal(decoded, want) {
		t.Fatal("loaded ranking binding must resolve the generated cursor key")
	}
}

func TestPlatformDocuments(t *testing.T) {
	f := platformValues(t)
	credentials := platformCertificates(t, f.ranking)
	cursor := make([]byte, 32)
	if _, err := rand.Read(cursor); err != nil {
		t.Fatal(err)
	}
	credentials["TEST_RANKING_CURSOR_KEY"] = base64.RawURLEncoding.EncodeToString(cursor)
	platformWriteJSON(t, filepath.Join(f.values, "ranking.example.json"), f.ranking)
	for _, mode := range []string{"--values", "--examples"} {
		t.Run(mode, func(t *testing.T) {
			composed := f.compose(t, mode)
			platformEnvironment(t, f, composed, credentials)
			cfg, err := config.Load()
			if err != nil {
				t.Fatalf("Load composed ranking documents: %v", err)
			}
			platformCursorKey(t, cfg, cursor)
			if cfg.NodeEnv != "production" || cfg.ChainID != 125 || cfg.Port != 8080 || cfg.RuntimeProfile != "container" || cfg.BindAddress != "0.0.0.0" || cfg.SecretsProvider != "env" || cfg.RuntimeZone != "mainnet-beta" || cfg.RuntimePoolID != "rankingd_pool" || cfg.RuntimeInstanceID != "ranking-1" {
				t.Fatal("ranking must use the platform contract's production configuration")
			}
			if cfg.TickInterval() != 30*time.Second || cfg.NewTickInterval() != 10*time.Second || cfg.RankingMaxEntries != 200 {
				t.Fatal("ranking must retain its production cadence and entry limit")
			}
			if cfg.RuntimeBudgetDocument != composed["DB_BUDGET_DOCUMENT"] || cfg.ExpectedSchemaDocument != composed["EXPECTED_SCHEMA_DOCUMENT"] || cfg.RankingRuntimeDocument != composed["RANKING_RUNTIME_DOCUMENT"] || cfg.RuntimeBudgetSHA256 != composed["DB_BUDGET_SHA256"] || cfg.ExpectedSchemaSHA256 != composed["EXPECTED_SCHEMA_SHA256"] || cfg.RankingRuntimeSHA256 != composed["RANKING_RUNTIME_SHA256"] {
				t.Fatal("Load must retain every composed document and digest")
			}
			var binding config.RankingRuntimeBinding
			if err := config.ReadBoundDocument("", cfg.RankingRuntimeSHA256, &binding); err != nil {
				t.Fatalf("ranking runtime binding: %v", err)
			}
			want := make(map[string]any)
			for key, value := range f.ranking {
				if key != "_example" {
					want[key] = value
				}
			}
			want["version"] = 2
			wantRaw, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			var expectedBinding config.RankingRuntimeBinding
			if err := json.Unmarshal(wantRaw, &expectedBinding); err != nil {
				t.Fatal(err)
			}
			if binding != expectedBinding {
				t.Fatal("runtime document must preserve every deployment value")
			}
			budgetRaw, err := base64.StdEncoding.DecodeString(cfg.RuntimeBudgetDocument)
			if err != nil {
				t.Fatal(err)
			}
			budget, err := production.LoadConnectionBudget(budgetRaw, cfg.RuntimeBudgetSHA256)
			if err != nil {
				t.Fatalf("composed budget: %v", err)
			}
			if _, err := budget.BudgetPoolOptions("rankingd", cfg.RuntimePoolID, cfg.RuntimeInstanceID); err != nil {
				t.Fatalf("ranking pool selection: %v", err)
			}
			schemaRaw, err := base64.StdEncoding.DecodeString(cfg.ExpectedSchemaDocument)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(schemaRaw, f.schema) {
				t.Fatal("expected schema bytes must be retained exactly")
			}
			if _, err := production.LoadExpectedSchema(schemaRaw, cfg.ExpectedSchemaSHA256); err != nil {
				t.Fatalf("composed expected schema: %v", err)
			}
			for _, name := range []string{"DB_BUDGET", "EXPECTED_SCHEMA", "RANKING_RUNTIME"} {
				t.Run(name, func(t *testing.T) {
					raw, err := base64.StdEncoding.DecodeString(composed[name+"_DOCUMENT"])
					if err != nil {
						t.Fatal(err)
					}
					for i, b := range raw {
						if b >= 'a' && b <= 'y' {
							raw[i]++
							break
						}
					}
					if !json.Valid(raw) {
						t.Fatal("changed document must remain valid JSON")
					}
					t.Setenv(name+"_DOCUMENT", base64.StdEncoding.EncodeToString(raw))
					if _, err := config.Load(); err == nil || !strings.Contains(err.Error(), name+": runtime_document_binding_mismatch") {
						t.Fatalf("Load must reject changed %s bytes by digest: %v", name, err)
					}
				})
			}
		})
	}
}

func TestPlatformDocumentsValueValidation(t *testing.T) {
	f := platformValues(t)
	f.compose(t, "--examples")
	for key := range f.ranking {
		if key == "_example" {
			continue
		}
		t.Run(key, func(t *testing.T) {
			for _, condition := range []string{"missing", "null", "invalid"} {
				values := make(map[string]any, len(f.ranking))
				for name, value := range f.ranking {
					values[name] = value
				}
				switch condition {
				case "missing":
					delete(values, key)
				case "null":
					values[key] = nil
				case "invalid":
					values[key] = "!"
					switch key {
					case "recoveryEpoch", "maxAttempts", "maximumReads":
						values[key] = 0
					case "sourceStream":
						values[key] = strings.Repeat("s", 129)
					case "projectionVersion":
						values[key] = strings.Repeat("v", 65)
					}
				}
				platformWriteJSON(t, filepath.Join(f.values, "ranking.json"), values)
				command := exec.Command("python3", f.composer, "--values", f.values, "--out", t.TempDir(), "--service", "ranking")
				command.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
				output, err := command.CombinedOutput()
				if err == nil || !bytes.Contains(output, []byte("missing or malformed value "+key)) {
					t.Fatalf("composer must reject missing/malformed %s with its key: %v: %s", key, err, output)
				}
			}
		})
	}
}

func TestPrivateListenAddressContract(t *testing.T) {
	f := platformValues(t)
	credentials := platformCertificates(t, f.ranking)
	cursor := make([]byte, 32)
	if _, err := rand.Read(cursor); err != nil {
		t.Fatal(err)
	}
	credentials["TEST_RANKING_CURSOR_KEY"] = base64.RawURLEncoding.EncodeToString(cursor)
	platformWriteJSON(t, filepath.Join(f.values, "ranking.example.json"), f.ranking)
	platformEnvironment(t, f, f.compose(t, "--examples"), credentials)
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	contractRaw, err := os.ReadFile(filepath.Join(root, "deploy", "launchpad", "railway", "variables.json"))
	if err != nil {
		t.Fatal(err)
	}
	var contract map[string]struct {
		Fixed map[string]string `json:"fixed"`
	}
	if err := json.Unmarshal(contractRaw, &contract); err != nil {
		t.Fatal(err)
	}
	address := contract["ranking"].Fixed["RANKING_PRIVATE_LISTEN_ADDRESS"]
	if address != "[::]:8443" {
		t.Fatalf("private listener contract = %q, want [::]:8443", address)
	}
	t.Setenv("RANKING_PRIVATE_LISTEN_ADDRESS", address)
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("Load production contract: %v", err)
	}
	platformCursorKey(t, cfg, cursor)
	if cfg.NodeEnv != "production" || cfg.RuntimeProfile != "container" || cfg.Port != 8080 || cfg.PrivateListenAddress != address {
		t.Fatal("private listener must use the production contract")
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate production contract: %v", err)
	}
	for name, invalid := range map[string]string{
		"missing_ip":   ":8443",
		"hostname":     "localhost:8443",
		"missing_port": "[::]",
		"empty_port":   "[::]:",
		"health_port":  net.JoinHostPort("::", strconv.Itoa(cfg.Port)),
	} {
		t.Run(name, func(t *testing.T) {
			candidate := cfg
			candidate.PrivateListenAddress = invalid
			if err := candidate.Validate(); err == nil || !strings.Contains(err.Error(), "RANKING_PRIVATE_LISTEN_ADDRESS") {
				t.Fatalf("Validate accepted private listener %q: %v", invalid, err)
			}
		})
	}
}
