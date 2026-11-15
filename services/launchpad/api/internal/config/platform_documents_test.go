package config

import (
	"crypto/tls"
	"encoding/base64"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/broker"
	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/nats-io/nats.go"
)

type platformDocumentContract struct {
	Fixed    map[string]string `json:"fixed"`
	Required []string          `json:"required"`
}

func platformDocumentsRoot(t *testing.T) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", "..", "..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	return root
}

func platformDocumentsRead(t *testing.T, path string) []byte {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

func platformDocumentsWriteJSON(t *testing.T, path string, value any) {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
}

func platformDocumentsValues(t *testing.T, service string) (string, string) {
	t.Helper()
	root := platformDocumentsRoot(t)
	values := t.TempDir()
	for _, name := range []string{"common.example.json", service + ".example.json"} {
		raw := platformDocumentsRead(t, filepath.Join(root, "deploy", "launchpad", "documents", "values", name))
		if err := os.WriteFile(filepath.Join(values, name), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Mkdir(filepath.Join(values, "expected-schema"), 0700); err != nil {
		t.Fatal(err)
	}
	role := "apid_rest"
	if service == "api-sockets" {
		role = "apid_sockets"
	}
	digest := production.Checksum([]byte("test-schema"))
	want := production.ExpectedSchema{
		Service: service, RuntimeRole: role, RegistryDigest: digest, InterfaceDigest: digest,
		ReaderVersion: 1, Migrations: map[int64]string{1: digest},
		Schemas: map[string]string{"api_stream": digest}, Epoch: 1, Region: "mainnet-beta",
		Privileges: []production.Grant{{Schema: "api_stream", Object: "frames", Privileges: []string{"SELECT"}}},
	}
	platformDocumentsWriteJSON(t, filepath.Join(values, "expected-schema", service+".json"), want)
	return root, values
}

func platformDocumentsCompose(root, values, out, service, source string) ([]byte, error) {
	command := exec.Command("python3", filepath.Join(root, "deploy", "launchpad", "documents", "compose.py"), source, values, "--out", out, "--service", service)
	command.Env = append(os.Environ(), "PYTHONDONTWRITEBYTECODE=1")
	return command.CombinedOutput()
}

func platformDocumentsEnvironment(t *testing.T, root, service string, documents map[string]string) {
	t.Helper()
	configType := reflect.TypeOf(Config{})
	for i := 0; i < configType.NumField(); i++ {
		key := strings.Split(configType.Field(i).Tag.Get("env"), ",")[0]
		if key != "" {
			unsetEnv(t, key)
		}
	}
	var contract map[string]platformDocumentContract
	if err := json.Unmarshal(platformDocumentsRead(t, filepath.Join(root, "deploy", "launchpad", "railway", "variables.json")), &contract); err != nil {
		t.Fatal(err)
	}
	entry, ok := contract[service]
	if !ok {
		t.Fatalf("missing variable contract for %s", service)
	}
	pki := testpki.New(t)
	client := pki.Leaf("api", "api.example.invalid")
	peer := pki.Leaf("identity", "identity.example.invalid")
	_, creds := inlineNATSCreds(t)
	credsPath := filepath.Join(t.TempDir(), "nats.creds")
	if err := os.WriteFile(credsPath, []byte(creds), 0600); err != nil {
		t.Fatal(err)
	}
	poolID := "apid_rest_pool"
	if service == "api-sockets" {
		poolID = "apid_sockets_pool"
	}
	variables := map[string]string{
		"DATABASE_URL": "postgres://test:test@localhost:5432/launchpad_test",
		"REDIS_URL":    "redis://localhost:6379/0",
		"DB_POOL_ID":   poolID, "INSTANCE_ID": "test-api-1", "ZONE": "mainnet-beta",
		"IDENTITY_ASSERTION_ISSUER": "test-api", "IDENTITY_CA_FILE": pki.CAFile,
		"IDENTITY_CLIENT_CERT_FILE": client.CertFile, "IDENTITY_CLIENT_KEY_FILE": client.KeyFile,
		"IDENTITY_PEER_CERT_SHA256": peer.SHA256, "IDENTITY_SERVICE_URL": "https://identity.example.invalid",
		"IDENTITY_SIGNING_KEY": testpki.Ed25519(t).StdBase64,
		"STATS_SERVICE_URL":    "https://stats.example.invalid",
		"STREAM_SIGNING_KEY":   "test-stream-signing-key-for-platform-documents",
		"NATS_URL":             "nats://localhost:4222", "NATS_CREDENTIALS": credsPath,
	}
	for key, value := range entry.Fixed {
		t.Setenv(key, value)
	}
	for _, key := range entry.Required {
		value, composed := documents[key]
		if strings.HasSuffix(key, "_DOCUMENT") || strings.HasSuffix(key, "_SHA256") && key != "IDENTITY_PEER_CERT_SHA256" {
			if !composed {
				t.Fatalf("composer omitted %s", key)
			}
		} else {
			value = variables[key]
		}
		if value == "" {
			t.Fatalf("missing required test value %s", key)
		}
		t.Setenv(key, value)
	}
	required := map[string]bool{}
	for _, key := range entry.Required {
		required[key] = true
	}
	for key := range documents {
		if !required[key] {
			t.Fatalf("composer emitted undeclared variable %s", key)
		}
	}
}

func TestPlatformDocuments(t *testing.T) {
	for _, service := range []string{"api", "api-sockets"} {
		t.Run(service, func(t *testing.T) {
			root, values := platformDocumentsValues(t, service)
			out := t.TempDir()
			output, err := platformDocumentsCompose(root, values, out, service, "--examples")
			if err != nil {
				t.Fatalf("compose %s: %v: %s", service, err, output)
			}
			path := filepath.Join(out, service+".json")
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != 0600 {
				t.Fatalf("composed file must be private: %v", err)
			}
			var documents map[string]string
			if err := json.Unmarshal(platformDocumentsRead(t, path), &documents); err != nil {
				t.Fatal(err)
			}
			if len(documents) != 6 {
				t.Fatalf("got %d variables, want three document/digest pairs", len(documents))
			}
			for key, encoded := range documents {
				if !strings.HasSuffix(key, "_DOCUMENT") {
					continue
				}
				raw, err := base64.StdEncoding.Strict().DecodeString(encoded)
				if err != nil || len(raw) == 0 {
					t.Fatalf("%s: invalid base64 document: %v", key, err)
				}
				digest := documents[strings.TrimSuffix(key, "_DOCUMENT")+"_SHA256"]
				if digest != production.Checksum(raw) {
					t.Fatalf("%s: digest does not identify document bytes", key)
				}
				if strings.Contains(string(output), encoded) || strings.Contains(string(output), string(raw)) {
					t.Fatalf("%s: document printed to command output", key)
				}
				switch key {
				case "DB_BUDGET_DOCUMENT":
					if _, err := production.LoadConnectionBudget(raw, digest); err != nil {
						t.Fatalf("budget: %v", err)
					}
				case "EXPECTED_SCHEMA_DOCUMENT":
					want := platformDocumentsRead(t, filepath.Join(values, "expected-schema", service+".json"))
					if string(raw) != string(want) {
						t.Fatal("expected schema bytes changed")
					}
					if schema, err := production.LoadExpectedSchema(raw, digest); err != nil || schema.Service != service {
						t.Fatalf("expected schema for %s: %v", service, err)
					}
				}
			}
			platformDocumentsEnvironment(t, root, service, documents)
			cfg, err := Load()
			if err != nil {
				t.Fatalf("Load %s: %v", service, err)
			}
			if role, err := cfg.RuntimeRole(); err != nil || role != "apid:"+cfg.APIRole {
				t.Fatalf("runtime role = %q: %v", role, err)
			}
			ca, cert, key, err := cfg.IdentityTLSMaterial()
			if err != nil || len(ca) == 0 {
				t.Fatalf("identity material: %v", err)
			}
			if _, err := tls.X509KeyPair(cert, key); err != nil {
				t.Fatalf("identity certificate and key: %v", err)
			}
			if service == "api" {
				var readConfig, want store.AccountingReadConfig
				if err := ReadBoundDocument(cfg.AccountingReadFile, cfg.AccountingReadSHA256, &readConfig); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(platformDocumentsRead(t, filepath.Join(values, "api.example.json")), &want); err != nil {
					t.Fatal(err)
				}
				if readConfig != want {
					t.Fatal("accounting document differs from deployment values")
				}
				poolConfig, err := pgxpool.ParseConfig(cfg.DatabaseURL)
				if err != nil {
					t.Fatal(err)
				}
				pool, err := pgxpool.NewWithConfig(t.Context(), poolConfig)
				if err != nil {
					t.Fatal(err)
				}
				defer pool.Close()
				if _, err := store.NewAccountingReadModel(pool, readConfig); err != nil {
					t.Fatalf("accounting read configuration: %v", err)
				}
			} else {
				var consumers []natsdriver.ConsumerConfig
				if err := ReadBoundDocument(cfg.StreamConsumerFile, cfg.StreamConsumerSHA256, &consumers); err != nil {
					t.Fatal(err)
				}
				if len(consumers) != 1 {
					t.Fatalf("got %d topic assignments, want 1", len(consumers))
				}
				consumer := consumers[0]
				if consumer.FetchWait != 250*time.Millisecond || consumer.AckWait != 30*time.Second || consumer.Batch != 8 || consumer.MaxAckPending != 32 || consumer.MaxWaiting != 1 || consumer.MaxDeliver != 10 || consumer.Stream != "MAINNET_BETA_125_LIVE" || consumer.Durable != "api_sockets_125_stats" || consumer.Subject != "market.stats.0xce20da50f92d21d8bee497910c782e9aa25b6263" {
					t.Fatal("consumer document differs from deployment values")
				}
				if _, err := consumer.BrokerConsumerConfig(); err != nil {
					t.Fatal(err)
				}
				if _, err := broker.CanonicalTopics([]string{consumer.Subject}); err != nil {
					t.Fatal(err)
				}
				option, err := cfg.NATSCredentialsOption()
				if err != nil {
					t.Fatal(err)
				}
				var options nats.Options
				if err := option(&options); err != nil || options.UserJWT == nil {
					t.Fatalf("broker credential file: %v", err)
				}
				if _, err := options.UserJWT(); err != nil {
					t.Fatal(err)
				}
			}
			for name, encoded := range documents {
				if !strings.HasSuffix(name, "_DOCUMENT") {
					continue
				}
				t.Run("changed_"+name, func(t *testing.T) {
					raw, err := base64.StdEncoding.DecodeString(encoded)
					if err != nil {
						t.Fatal(err)
					}
					raw[len(raw)-1] ^= 1
					t.Setenv(name, base64.StdEncoding.EncodeToString(raw))
					if _, err := Load(); err == nil || !strings.Contains(err.Error(), name) || !strings.Contains(err.Error(), "runtime_document_binding_mismatch") {
						t.Fatalf("Load accepted changed %s or returned an unrelated error: %v", name, err)
					}
				})
			}
		})
	}
}

func TestPlatformDocumentsRejectInvalidValues(t *testing.T) {
	for _, service := range []string{"api", "api-sockets"} {
		t.Run(service, func(t *testing.T) {
			root, values := platformDocumentsValues(t, service)
			example := platformDocumentsRead(t, filepath.Join(values, service+".example.json"))
			var fields map[string]any
			if service == "api" {
				if err := json.Unmarshal(example, &fields); err != nil {
					t.Fatal(err)
				}
			} else {
				var assignments []map[string]any
				if err := json.Unmarshal(example, &assignments); err != nil {
					t.Fatal(err)
				}
				fields = assignments[0]
			}
			check := func(t *testing.T, value any, key string) {
				t.Helper()
				platformDocumentsWriteJSON(t, filepath.Join(values, service+".json"), value)
				output, err := platformDocumentsCompose(root, values, t.TempDir(), service, "--values")
				if err == nil || !strings.Contains(string(output), key) {
					t.Fatalf("composer must refuse %s and name its key: %v: %s", key, err, output)
				}
			}
			for key := range fields {
				for _, missing := range []bool{true, false} {
					name := "malformed_" + key
					if missing {
						name = "missing_" + key
					}
					t.Run(name, func(t *testing.T) {
						candidate := map[string]any{}
						for key, value := range fields {
							candidate[key] = value
						}
						if missing {
							delete(candidate, key)
						} else {
							candidate[key] = false
						}
						var value any = candidate
						if service == "api-sockets" {
							value = []any{candidate}
						}
						check(t, value, key)
					})
				}
			}
			if service == "api" {
				for _, tc := range []struct{ key, value string }{
					{"ChartsSchema", "Invalid-Schema"},
					{"ProjectionGeneration", "contains a space"},
					{"ProjectionVersion", strings.Repeat("v", 65)},
					{"SourceStream", ""},
					{"StatsConsumerID", fields["ChartsConsumerID"].(string)},
				} {
					t.Run(tc.key, func(t *testing.T) {
						candidate := map[string]any{}
						for key, value := range fields {
							candidate[key] = value
						}
						candidate[tc.key] = tc.value
						check(t, candidate, tc.key)
					})
				}
			} else {
				for _, tc := range []struct {
					key   string
					value any
				}{
					{"Stream", "invalid.stream"}, {"Durable", ""}, {"Subject", "market.stats.*"},
					{"Subject", strings.Repeat("a", 201)}, {"Batch", 0}, {"Batch", 65}, {"Batch", 1.5},
					{"MaxAckPending", 7}, {"MaxAckPending", 10001}, {"MaxWaiting", 0}, {"MaxWaiting", 33},
					{"MaxDeliver", 0}, {"MaxDeliver", 101}, {"FetchWaitMS", 9}, {"FetchWaitMS", 1001},
					{"AckWaitMS", 999}, {"AckWaitMS", 3600001},
				} {
					t.Run(tc.key, func(t *testing.T) {
						candidate := map[string]any{}
						for key, value := range fields {
							candidate[key] = value
						}
						candidate[tc.key] = tc.value
						check(t, []any{candidate}, tc.key)
					})
				}
				check(t, []any{fields, fields}, "Subject")
				check(t, []any{}, "STREAM_CONSUMER_CONFIG_DOCUMENT")
				check(t, make([]any, 129), "STREAM_CONSUMER_CONFIG_DOCUMENT")
			}
		})
	}
}

func TestPlatformDocumentsDeploymentValues(t *testing.T) {
	for _, service := range []string{"api", "api-sockets"} {
		t.Run(service, func(t *testing.T) {
			root, values := platformDocumentsValues(t, service)
			raw := platformDocumentsRead(t, filepath.Join(values, service+".example.json"))
			if service == "api" {
				var input map[string]any
				if err := json.Unmarshal(raw, &input); err != nil {
					t.Fatal(err)
				}
				input["ProjectionGeneration"] = "mainnet_beta_125_g2"
				platformDocumentsWriteJSON(t, filepath.Join(values, service+".json"), input)
			} else {
				var input []map[string]any
				if err := json.Unmarshal(raw, &input); err != nil {
					t.Fatal(err)
				}
				input[0]["FetchWaitMS"] = 1000
				input[0]["AckWaitMS"] = 3600000
				platformDocumentsWriteJSON(t, filepath.Join(values, service+".json"), input)
			}
			out := t.TempDir()
			if output, err := platformDocumentsCompose(root, values, out, service, "--values"); err != nil {
				t.Fatalf("compose deployment values: %v: %s", err, output)
			}
			var documents map[string]string
			if err := json.Unmarshal(platformDocumentsRead(t, filepath.Join(out, service+".json")), &documents); err != nil {
				t.Fatal(err)
			}
			platformDocumentsEnvironment(t, root, service, documents)
			cfg, err := Load()
			if err != nil {
				t.Fatal(err)
			}
			if service == "api" {
				var readConfig store.AccountingReadConfig
				if err := ReadBoundDocument("", cfg.AccountingReadSHA256, &readConfig); err != nil || readConfig.ProjectionGeneration != "mainnet_beta_125_g2" {
					t.Fatalf("deployment generation: %v", err)
				}
			} else {
				var consumers []natsdriver.ConsumerConfig
				if err := ReadBoundDocument("", cfg.StreamConsumerSHA256, &consumers); err != nil || len(consumers) != 1 {
					t.Fatalf("deployment consumers: %v", err)
				}
				if consumers[0].FetchWait != time.Second || consumers[0].AckWait != time.Hour {
					t.Fatal("deployment periods were not converted from milliseconds")
				}
				if _, err := consumers[0].BrokerConsumerConfig(); err != nil {
					t.Fatal(err)
				}
			}
		})
	}
}
