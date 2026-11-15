package config

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	dbproduction "github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/nats-io/nats.go"
)

func platformWrite(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func platformRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func platformValues(t *testing.T) (string, string, string) {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	documents := filepath.Join(root, "deploy", "launchpad", "documents")
	values := t.TempDir()
	for _, name := range []string{"common.example.json", "stats.example.json"} {
		platformWrite(t, filepath.Join(values, name), platformRead(t, filepath.Join(documents, "values", name)))
	}
	if err := os.Mkdir(filepath.Join(values, "expected-schema"), 0700); err != nil {
		t.Fatal(err)
	}
	digest := strings.Repeat("a", 64)
	schema := dbproduction.ExpectedSchema{
		Service: "stats", RuntimeRole: "statsd", RegistryDigest: digest, InterfaceDigest: digest,
		ReaderVersion: 1, Migrations: map[int64]string{1: digest}, Schemas: map[string]string{"stats": digest},
		Epoch: 1, Region: "mainnet-beta",
		Privileges: []dbproduction.Grant{{Schema: "stats", Object: "pool_stats", Privileges: []string{"SELECT"}}},
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	platformWrite(t, filepath.Join(values, "expected-schema", "stats.json"), raw)
	return root, documents, values
}

func platformCompose(t *testing.T, documents, values, out string) ([]byte, error) {
	t.Helper()
	return exec.Command("python3", filepath.Join(documents, "compose.py"), "--examples", values, "--out", out, "--service", "stats").CombinedOutput()
}

func platformIdentity(t *testing.T, directory string) map[string]string {
	t.Helper()
	pub, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	ca := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "stats test CA"},
		NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour),
		IsCA: true, BasicConstraintsValid: true, KeyUsage: x509.KeyUsageCertSign,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, ca, ca, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	client := &x509.Certificate{
		SerialNumber: big.NewInt(2), Subject: pkix.Name{CommonName: "stats"},
		NotBefore: ca.NotBefore, NotAfter: ca.NotAfter, KeyUsage: x509.KeyUsageDigitalSignature,
		ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, client, ca, pub, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	caPath, clientPath := filepath.Join(directory, "ca.pem"), filepath.Join(directory, "client.pem")
	platformWrite(t, caPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER}))
	platformWrite(t, clientPath, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER}))
	pin := sha256.Sum256(caDER)
	return map[string]string{
		"IDENTITY_SERVICE_URL":  "https://identity.example:8443",
		"IDENTITY_MTLS_CA_FILE": caPath, "IDENTITY_MTLS_CLIENT_CERT_FILE": clientPath,
		"IDENTITY_MTLS_CLIENT_KEY_PEM":   string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER})),
		"IDENTITY_MTLS_PEER_CERT_SHA256": hex.EncodeToString(pin[:]),
		"IDENTITY_ASSERTION_SIGNING_KEY": base64.StdEncoding.EncodeToString(key),
	}
}

func TestPlatformDocuments(t *testing.T) {
	root, documents, values := platformValues(t)
	out := t.TempDir()
	output, err := platformCompose(t, documents, values, out)
	if err != nil {
		t.Fatalf("compose stats: %v: %s", err, output)
	}
	var composed map[string]string
	if err := json.Unmarshal(platformRead(t, filepath.Join(out, "stats.json")), &composed); err != nil {
		t.Fatal(err)
	}
	var contract map[string]struct {
		Fixed    map[string]string `json:"fixed"`
		Required []string          `json:"required"`
	}
	if err := json.Unmarshal(platformRead(t, filepath.Join(root, "deploy", "launchpad", "railway", "variables.json")), &contract); err != nil {
		t.Fatal(err)
	}
	entry := contract["stats"]
	var clearEnvironment func(reflect.Type)
	clearEnvironment = func(typ reflect.Type) {
		for i := 0; i < typ.NumField(); i++ {
			field := typ.Field(i)
			if field.Anonymous {
				clearEnvironment(field.Type)
			} else if name := strings.Split(field.Tag.Get("env"), ",")[0]; name != "" {
				t.Setenv(name, "")
			}
		}
	}
	clearEnvironment(reflect.TypeOf(Config{}))
	for name, value := range entry.Fixed {
		t.Setenv(name, value)
	}
	credentials := filepath.Join(out, "stats.creds")
	userJWT, creds := inlineNATSCreds(t)
	platformWrite(t, credentials, []byte(creds))
	other := map[string]string{
		"DATABASE_URL": "postgres://stats:test-password@localhost:5432/launchpad",
		"DB_POOL_ID":   "statsd_pool", "INSTANCE_ID": "stats-1", "ZONE": "mainnet-beta",
		"NATS_CREDENTIALS": credentials, "NATS_URL": "nats://localhost:4222",
		"REDIS_URL": "redis://localhost:6379/0", "REDIS_BULL_URL": "redis://localhost:6379/1",
		"RPC_URL": "https://rpc.example", "STATS_AUDIT_KEY": strings.Repeat("a", 64),
		"STATS_CURSOR_KEY": strings.Repeat("b", 64), "WEBHOOK_HMAC_SECRET": strings.Repeat("test", 8),
	}
	for name, value := range platformIdentity(t, out) {
		other[name] = value
	}
	for _, name := range entry.Required {
		value, ok := composed[name]
		if !ok {
			value, ok = other[name]
		}
		if !ok || value == "" {
			t.Fatalf("required contract value %s is missing", name)
		}
		t.Setenv(name, value)
	}
	if len(composed) != 6 {
		t.Fatalf("composed %d variables, want three document/digest pairs", len(composed))
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load composed stats documents: %v", err)
	}
	if cfg.ChainID != 125 || cfg.NodeEnv != "production" || cfg.RuntimeProfile != "container" || cfg.RuntimePoolID != "statsd_pool" {
		t.Fatal("stats platform configuration differs from the contract")
	}
	if err := cfg.ValidateIdentityClient(); err != nil {
		t.Fatal(err)
	}
	if _, _, err := cfg.Phase2Keys(); err != nil {
		t.Fatal(err)
	}
	if err := cfg.ValidateProductionWorkers(); err != nil {
		t.Fatal(err)
	}
	options, err := cfg.RuntimeNATSOptions()
	if err != nil {
		t.Fatal(err)
	}
	var natsOptions nats.Options
	for _, option := range options {
		if err := option(&natsOptions); err != nil {
			t.Fatal(err)
		}
	}
	if natsOptions.UserJWT == nil || natsOptions.SignatureCB == nil {
		t.Fatal("NATS file credentials are not bound")
	}
	if got, err := natsOptions.UserJWT(); err != nil || got != userJWT {
		t.Fatalf("NATS file JWT: %v", err)
	}
	if _, err := natsOptions.SignatureCB([]byte("stats startup")); err != nil {
		t.Fatal(err)
	}
	for _, prefix := range []string{"DB_BUDGET", "EXPECTED_SCHEMA", "DELIVERY_CONFIG"} {
		name, digest := prefix+"_DOCUMENT", composed[prefix+"_SHA256"]
		raw, err := base64.StdEncoding.Strict().DecodeString(composed[name])
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(raw)
		if hex.EncodeToString(sum[:]) != digest || bytes.Contains(output, []byte(composed[name])) {
			t.Fatalf("%s digest or output differs", name)
		}
		var decoded json.RawMessage
		if err := ReadBoundDocument("", digest, &decoded); err != nil || !bytes.Equal(raw, decoded) {
			t.Fatalf("read composed %s: %v", name, err)
		}
		switch prefix {
		case "DB_BUDGET":
			budget, err := dbproduction.LoadConnectionBudget(raw, digest)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := budget.BudgetPoolOptions("statsd", cfg.RuntimePoolID, cfg.RuntimeInstanceID); err != nil {
				t.Fatal(err)
			}
		case "EXPECTED_SCHEMA":
			schema, err := dbproduction.LoadExpectedSchema(raw, digest)
			if err != nil || schema.Service != "stats" {
				t.Fatalf("stats expected schema: %v", err)
			}
		}
		t.Run("RejectChanged"+prefix, func(t *testing.T) {
			changed := append([]byte(nil), raw...)
			changed[len(changed)-1] ^= 1
			t.Setenv(name, base64.StdEncoding.EncodeToString(changed))
			if _, err := Load(); err == nil || !strings.Contains(err.Error(), prefix+": runtime_document_binding_mismatch") {
				t.Fatalf("Load changed %s: %v", name, err)
			}
		})
	}
}

func TestPlatformDocumentsRejectMalformedValues(t *testing.T) {
	_, documents, values := platformValues(t)
	example := platformRead(t, filepath.Join(values, "stats.example.json"))
	cases := []struct {
		name, key string
		edit      func(map[string]any)
	}{
		{"missing owner", "owner", func(d map[string]any) { delete(d, "owner") }},
		{"invalid replay limit", "maxReplayBlocks", func(d map[string]any) { d["maxReplayBlocks"] = 4097 }},
		{"fractional epoch", "epoch", func(d map[string]any) { d["epoch"] = 1.5 }},
		{"missing phase", "phase2", func(d map[string]any) { delete(d, "phase2") }},
		{"invalid period", "phase2.cursorTtlSeconds", func(d map[string]any) { d["phase2"].(map[string]any)["cursorTtlSeconds"] = 59 }},
		{"duplicate windows", "phase2.windows.1", func(d map[string]any) { d["phase2"].(map[string]any)["windows"] = []int{60, 60} }},
		{"missing source map", "phase2.searchSources", func(d map[string]any) { delete(d["phase2"].(map[string]any), "searchSources") }},
		{"invalid consumer batch", "consumers.0.Batch", func(d map[string]any) { d["consumers"].([]any)[0].(map[string]any)["Batch"] = 0 }},
		{"invalid duration", "consumers.0.FetchWait", func(d map[string]any) { d["consumers"].([]any)[0].(map[string]any)["FetchWait"] = "250ms" }},
		{"disabled policy", "policy.Subjects.accounting.market.v1", func(d map[string]any) {
			d["policy"].(map[string]any)["Subjects"].(map[string]any)["accounting.market.v1"] = false
		}},
		{"missing subscriptions", "phase2.searchConsumers", func(d map[string]any) { d["phase2"].(map[string]any)["searchConsumers"] = []any{} }},
		{"unknown key", "chainId", func(d map[string]any) { d["chainId"] = 1 }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var d map[string]any
			if err := json.Unmarshal(example, &d); err != nil {
				t.Fatal(err)
			}
			tc.edit(d)
			raw, err := json.Marshal(d)
			if err != nil {
				t.Fatal(err)
			}
			platformWrite(t, filepath.Join(values, "stats.json"), raw)
			out := t.TempDir()
			output, err := platformCompose(t, documents, values, out)
			if err == nil || !strings.Contains(string(output), "missing or malformed "+tc.key) {
				t.Fatalf("compose malformed %s: %v: %s", tc.key, err, output)
			}
			if _, err := os.Stat(filepath.Join(out, "stats.json")); !os.IsNotExist(err) {
				t.Fatal("malformed stats values produced a document")
			}
		})
	}
}
