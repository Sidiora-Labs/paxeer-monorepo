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
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nkeys"
)

func platformDocumentsWrite(t *testing.T, path string, raw []byte) {
	t.Helper()
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func platformDocumentsJSON(t *testing.T, path string, out any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, out); err != nil {
		t.Fatal(err)
	}
}

func platformDocumentsValues(t *testing.T) (string, string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate document values")
	}
	root := filepath.Clean(filepath.Join(filepath.Dir(file), "../../../../.."))
	values := t.TempDir()
	for _, name := range []string{"common.example.json", "pnl.example.json"} {
		raw, err := os.ReadFile(filepath.Join(root, "deploy/launchpad/documents/values", name))
		if err != nil {
			t.Fatal(err)
		}
		platformDocumentsWrite(t, filepath.Join(values, name), raw)
	}
	if err := os.Mkdir(filepath.Join(values, "expected-schema"), 0700); err != nil {
		t.Fatal(err)
	}
	digest := fmt.Sprintf("%x", sha256.Sum256([]byte("pnl schema configuration")))
	schema := production.ExpectedSchema{
		Service: "core/pnl-tracker", RuntimeRole: "pnl_runtime", RegistryDigest: digest,
		InterfaceDigest: digest, ReaderVersion: 1, Migrations: map[int64]string{160001: digest},
		Schemas: map[string]string{"pnl": digest}, Epoch: 1, Region: "mainnet-beta",
		Privileges: []production.Grant{{Schema: "pnl", Object: "positions", Privileges: []string{"SELECT"}}},
	}
	raw, err := json.Marshal(schema)
	if err != nil {
		t.Fatal(err)
	}
	platformDocumentsWrite(t, filepath.Join(values, "expected-schema/pnl.json"), raw)
	return root, values
}

func platformDocumentsCompose(t *testing.T, root, values string) map[string]string {
	t.Helper()
	out := t.TempDir()
	cmd := exec.Command("python3", filepath.Join(root, "deploy/launchpad/documents/compose.py"), "--values", values, "--out", out, "--service", "pnl")
	cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compose pnl: %v\n%s", err, output)
	}
	path := filepath.Join(out, "pnl.json")
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0600 {
		t.Fatalf("private document file: %v", err)
	}
	var result map[string]string
	platformDocumentsJSON(t, path, &result)
	return result
}

func platformDocumentsFields(value reflect.Value, visit func(string, reflect.Value)) {
	for i := 0; i < value.NumField(); i++ {
		field := value.Type().Field(i)
		if field.Anonymous {
			platformDocumentsFields(value.Field(i), visit)
		} else if name := strings.Split(field.Tag.Get("env"), ",")[0]; name != "" {
			visit(name, value.Field(i))
		}
	}
}

func platformDocumentsCredentials(t *testing.T) string {
	t.Helper()
	account, err := nkeys.CreateAccount()
	if err != nil {
		t.Fatal(err)
	}
	user, err := nkeys.CreateUser()
	if err != nil {
		t.Fatal(err)
	}
	accountPub, err := account.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	userPub, err := user.PublicKey()
	if err != nil {
		t.Fatal(err)
	}
	enc := base64.RawURLEncoding.EncodeToString
	signing := enc([]byte(`{"typ":"JWT","alg":"ed25519-nkey"}`)) + "." + enc([]byte(`{"sub":"`+userPub+`","iss":"`+accountPub+`","nats":{"type":"user","version":2}}`))
	signature, err := account.Sign([]byte(signing))
	if err != nil {
		t.Fatal(err)
	}
	seed, err := user.Seed()
	if err != nil {
		t.Fatal(err)
	}
	creds := "-----BEGIN NATS USER JWT-----\n" + signing + "." + enc(signature) + "\n------END NATS USER JWT------\n\n-----BEGIN USER NKEY SEED-----\n" + string(seed) + "\n------END USER NKEY SEED------\n"
	path := filepath.Join(t.TempDir(), "pnl.creds")
	platformDocumentsWrite(t, path, []byte(creds))
	return path
}

func TestPlatformDocuments(t *testing.T) {
	root, values := platformDocumentsValues(t)
	composed := platformDocumentsCompose(t, root, values)
	var contract map[string]struct {
		Fixed    map[string]string `json:"fixed"`
		Required []string          `json:"required"`
	}
	platformDocumentsJSON(t, filepath.Join(root, "deploy/launchpad/railway/variables.json"), &contract)
	entry, ok := contract["pnl"]
	if !ok || len(entry.Fixed) == 0 || len(entry.Required) == 0 {
		t.Fatal("pnl variable contract is missing")
	}
	platformDocumentsFields(reflect.ValueOf(Config{}), func(name string, _ reflect.Value) {
		t.Setenv(name, "")
		if err := os.Unsetenv(name); err != nil {
			t.Fatal(err)
		}
	})
	testValues := map[string]string{
		"DATABASE_URL": "postgres://pnl_runtime:test-only-password@localhost:5432/pnl",
		"DB_POOL_ID":   "pnld_pool", "INSTANCE_ID": "pnl-1",
		"NATS_CREDENTIALS": platformDocumentsCredentials(t), "NATS_URL": "nats://localhost:4222",
		"REDIS_BULL_URL": "redis://localhost:6379/1", "REDIS_URL": "redis://localhost:6379/0",
		"RPC_URL": "https://api5.mainnet-beta.paxeer.network", "RPC_URL_FALLBACK": "https://api2.mainnet-beta.paxeer.network",
		"WEBHOOK_HMAC_SECRET": "test-only-pnl-webhook-secret-32-characters", "ZONE": "mainnet-beta",
	}
	for key, value := range entry.Fixed {
		t.Setenv(key, value)
	}
	documentNames := []string{}
	for _, key := range entry.Required {
		value, ok := composed[key]
		if !ok {
			value, ok = testValues[key]
		}
		if !ok || value == "" {
			t.Fatalf("no value for required variable %s", key)
		}
		t.Setenv(key, value)
		if strings.HasSuffix(key, "_DOCUMENT") {
			documentNames = append(documentNames, key)
			raw, err := base64.StdEncoding.Strict().DecodeString(value)
			if err != nil {
				t.Fatal(err)
			}
			digestKey := strings.TrimSuffix(key, "_DOCUMENT") + "_SHA256"
			if composed[digestKey] != fmt.Sprintf("%x", sha256.Sum256(raw)) {
				t.Fatalf("%s does not bind %s", digestKey, key)
			}
		}
	}
	if len(documentNames) != 3 || len(composed) != 6 {
		t.Fatal("pnl requires three digest-bound documents")
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("load composed documents: %v", err)
	}
	loaded := map[string]string{}
	platformDocumentsFields(reflect.ValueOf(cfg), func(name string, value reflect.Value) {
		loaded[name] = fmt.Sprint(value.Interface())
	})
	for name, expected := range entry.Fixed {
		if loaded[name] != expected {
			t.Errorf("fixed variable %s was not loaded", name)
		}
	}
	for _, name := range entry.Required {
		if loaded[name] != os.Getenv(name) {
			t.Errorf("required variable %s was not loaded", name)
		}
	}
	budgetRaw, err := boundBytes("", cfg.RuntimeBudgetDocument, cfg.RuntimeBudgetSHA256)
	if err != nil {
		t.Fatal(err)
	}
	budget, err := production.ParseConnectionBudget(budgetRaw)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = budget.BudgetPoolOptions("pnld", cfg.RuntimePoolID, cfg.RuntimeInstanceID); err != nil {
		t.Fatal(err)
	}
	want, err := cfg.runtimeExpectedSchema()
	if err != nil || want.Service != "core/pnl-tracker" || want.RuntimeRole != "pnl_runtime" {
		t.Fatalf("pnl expected schema: %v", err)
	}
	var delivery struct {
		Schema, SourceStream, ConsumerID, Generation, Owner, ProjectionVersion string
		Epoch                                                                  int64
		Region                                                                 string
		Consumer                                                               natsdriver.ConsumerConfig
	}
	if err = ReadBoundDocument("", cfg.DeliveryConfigSHA256, &delivery); err != nil {
		t.Fatal(err)
	}
	if delivery.Schema != "pnl" || delivery.SourceStream != "chain125-canonical" || delivery.ConsumerID != "pnl_inventory" || delivery.Generation != "g1" || delivery.Owner != cfg.RuntimeInstanceID || delivery.ProjectionVersion != "inventory-v1" || delivery.Consumer.Subject != accounting.InventorySubject {
		t.Fatal("delivery inventory bindings differ from pnl values")
	}
	if _, err = queue.NewEpochGuard("recovery", delivery.Epoch, delivery.Region); err != nil {
		t.Fatal(err)
	}
	if _, err = delivery.Consumer.BrokerConsumerConfig(); err != nil {
		t.Fatal(err)
	}
	if delivery.Consumer.FetchWait != 250*time.Millisecond || delivery.Consumer.AckWait != 30*time.Second {
		t.Fatal("consumer periods must be encoded as nanoseconds")
	}
	options, err := cfg.RuntimeNATSOptions()
	if err != nil {
		t.Fatal(err)
	}
	var broker nats.Options
	for _, option := range options {
		if err = option(&broker); err != nil {
			t.Fatal(err)
		}
	}
	if broker.UserJWT == nil || broker.SignatureCB == nil {
		t.Fatal("NATS credentials must bind a user and signing key")
	}
	if jwt, err := broker.UserJWT(); err != nil || jwt == "" {
		t.Fatalf("NATS user credential: %v", err)
	}
	if _, err = broker.SignatureCB([]byte("pnl")); err != nil {
		t.Fatal(err)
	}
	for _, key := range documentNames {
		t.Run("digest/"+key, func(t *testing.T) {
			raw, err := base64.StdEncoding.Strict().DecodeString(composed[key])
			if err != nil {
				t.Fatal(err)
			}
			raw[len(raw)-1] ^= 1
			t.Setenv(key, base64.StdEncoding.EncodeToString(raw))
			if _, err = Load(); err == nil || !strings.Contains(err.Error(), key) || !strings.Contains(err.Error(), "runtime_document_binding_mismatch") {
				t.Fatalf("%s with altered bytes: %v", key, err)
			}
		})
	}
}

func TestPlatformDocumentsValues(t *testing.T) {
	root, values := platformDocumentsValues(t)
	var original map[string]any
	platformDocumentsJSON(t, filepath.Join(values, "pnl.example.json"), &original)
	cases := []struct {
		key   string
		value any
	}{
		{"sourceStream", "invalid.stream"}, {"consumerId", ""}, {"generation", true},
		{"owner", strings.Repeat("a", 129)}, {"projectionVersion", ""},
		{"projectionVersion", strings.Repeat("é", 65)}, {"epoch", 0}, {"epoch", uint64(1) << 63},
		{"epoch", true}, {"region", "MAINNET"}, {"consumer", nil},
		{"consumer.stream", "invalid.stream"}, {"consumer.durable", ""},
		{"consumer.batch", 0}, {"consumer.batch", 65}, {"consumer.batch", true},
		{"consumer.maxAckPending", 15}, {"consumer.maxAckPending", 10001},
		{"consumer.maxWaiting", 0}, {"consumer.maxWaiting", 33},
		{"consumer.maxDeliver", 0}, {"consumer.maxDeliver", 101},
		{"consumer.fetchWaitMS", 9}, {"consumer.fetchWaitMS", 1001}, {"consumer.fetchWaitMS", 10.5},
		{"consumer.ackWaitMS", 999}, {"consumer.ackWaitMS", 3600001},
		{"unexpected", true}, {"consumer.unexpected", true},
	}
	for key, value := range original {
		cases = append(cases, struct {
			key   string
			value any
		}{key, nil})
		if consumer, ok := value.(map[string]any); ok {
			for name := range consumer {
				cases = append(cases, struct {
					key   string
					value any
				}{"consumer." + name, nil})
			}
		}
	}
	for _, tc := range cases {
		t.Run(tc.key+"/"+fmt.Sprint(tc.value), func(t *testing.T) {
			var source map[string]any
			platformDocumentsJSON(t, filepath.Join(values, "pnl.example.json"), &source)
			target, key := source, tc.key
			if strings.HasPrefix(key, "consumer.") {
				target = source["consumer"].(map[string]any)
				key = strings.TrimPrefix(key, "consumer.")
			}
			if tc.value == nil {
				delete(target, key)
			} else {
				target[key] = tc.value
			}
			raw, err := json.Marshal(source)
			if err != nil {
				t.Fatal(err)
			}
			platformDocumentsWrite(t, filepath.Join(values, "pnl.json"), raw)
			cmd := exec.Command("python3", filepath.Join(root, "deploy/launchpad/documents/compose.py"), "--values", values, "--out", t.TempDir(), "--service", "pnl")
			cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
			output, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(output), tc.key) {
				t.Fatalf("invalid %s must be named: %v\n%s", tc.key, err, output)
			}
		})
	}
}

func TestPlatformDocumentsDeploymentValues(t *testing.T) {
	root, values := platformDocumentsValues(t)
	for _, upper := range []bool{false, true} {
		t.Run(fmt.Sprint(upper), func(t *testing.T) {
			var source map[string]any
			platformDocumentsJSON(t, filepath.Join(values, "pnl.example.json"), &source)
			source["sourceStream"], source["consumerId"], source["generation"] = "chain125-replay", "pnl_replay", "g2"
			source["owner"], source["projectionVersion"], source["region"] = "pnl-2", "inventory-v2", "mainnet-beta-2"
			consumer := source["consumer"].(map[string]any)
			consumer["stream"], consumer["durable"] = "INVENTORY_V2", "pnl_replay"
			batch, pending, waiting, deliveries, fetch, ack := 1, 1, 1, 1, 10, 1000
			if upper {
				batch, pending, waiting, deliveries, fetch, ack = 64, 10000, 32, 100, 1000, 3600000
				source["epoch"] = int64(1<<63 - 1)
			}
			consumer["batch"], consumer["maxAckPending"], consumer["maxWaiting"] = batch, pending, waiting
			consumer["maxDeliver"], consumer["fetchWaitMS"], consumer["ackWaitMS"] = deliveries, fetch, ack
			raw, err := json.Marshal(source)
			if err != nil {
				t.Fatal(err)
			}
			platformDocumentsWrite(t, filepath.Join(values, "pnl.json"), raw)
			composed := platformDocumentsCompose(t, root, values)
			var document struct {
				Schema, SourceStream, ConsumerID, Generation, Owner, ProjectionVersion, Region string
				Epoch                                                                          int64
				Consumer                                                                       natsdriver.ConsumerConfig
			}
			if err = decodeBoundDocument("", composed["DELIVERY_CONFIG_DOCUMENT"], composed["DELIVERY_CONFIG_SHA256"], &document); err != nil {
				t.Fatal(err)
			}
			if document.Schema != "pnl" || document.SourceStream != "chain125-replay" || document.ConsumerID != "pnl_replay" || document.Generation != "g2" || document.Owner != "pnl-2" || document.ProjectionVersion != "inventory-v2" || document.Region != "mainnet-beta-2" {
				t.Fatal("deployment values must override example values")
			}
			c := document.Consumer
			if c.Stream != "INVENTORY_V2" || c.Durable != "pnl_replay" || c.Subject != accounting.InventorySubject || c.Batch != batch || c.MaxAckPending != pending || c.MaxWaiting != waiting || c.MaxDeliver != deliveries || c.FetchWait != time.Duration(fetch)*time.Millisecond || c.AckWait != time.Duration(ack)*time.Millisecond {
				t.Fatal("deployment consumer limits differ")
			}
			if _, err = c.BrokerConsumerConfig(); err != nil {
				t.Fatal(err)
			}
			if _, err = queue.NewEpochGuard("recovery", document.Epoch, document.Region); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPlatformDocumentsSchemaNames(t *testing.T) {
	_, values := platformDocumentsValues(t)
	var schema production.ExpectedSchema
	platformDocumentsJSON(t, filepath.Join(values, "expected-schema/pnl.json"), &schema)
	for _, name := range []string{"pnl", "core/pnl-tracker", "stats", "core/stats-workers"} {
		t.Run(name, func(t *testing.T) {
			schema.Service = name
			raw, err := json.Marshal(schema)
			if err != nil {
				t.Fatal(err)
			}
			cfg := Config{ExpectedSchemaDocument: base64.StdEncoding.EncodeToString(raw), ExpectedSchemaSHA256: fmt.Sprintf("%x", sha256.Sum256(raw))}
			got, err := cfg.runtimeExpectedSchema()
			if name == "pnl" || name == "core/pnl-tracker" {
				if err != nil || got.Service != "core/pnl-tracker" {
					t.Fatalf("pnl schema service %s: %v", name, err)
				}
			} else if err == nil || err.Error() != "runtime_schema_service_mismatch" {
				t.Fatalf("foreign schema service %s: %v", name, err)
			}
		})
	}
}

func TestPlatformDocumentsValuesFiles(t *testing.T) {
	root, values := platformDocumentsValues(t)
	if err := os.Remove(filepath.Join(values, "pnl.example.json")); err != nil {
		t.Fatal(err)
	}
	for _, raw := range []string{"", "{", "null", "[]", `{"projectionVersion":"\ud800"}`} {
		t.Run(fmt.Sprintf("values/%q", raw), func(t *testing.T) {
			path := filepath.Join(values, "pnl.json")
			if raw != "" {
				platformDocumentsWrite(t, path, []byte(raw))
			}
			cmd := exec.Command("python3", filepath.Join(root, "deploy/launchpad/documents/compose.py"), "--values", values, "--out", t.TempDir(), "--service", "pnl")
			cmd.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
			output, err := cmd.CombinedOutput()
			if err == nil || !strings.Contains(string(output), "pnl") {
				t.Fatalf("invalid pnl values file: %v\n%s", err, output)
			}
		})
	}
}
