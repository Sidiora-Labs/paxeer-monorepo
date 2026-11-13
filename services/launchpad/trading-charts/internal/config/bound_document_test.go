package config

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
)

func TestBoundDocumentReadsExactFileAndInlineJSON(t *testing.T) {
	raw := []byte(`{"schema":"candles"}`)
	digest := fmt.Sprintf("%x", sha256.Sum256(raw))
	path := filepath.Join(t.TempDir(), "runtime.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var value struct {
		Schema string `json:"schema"`
	}
	if err := ReadBoundDocument(path, digest, &value); err != nil || value.Schema != "candles" {
		t.Fatalf("file document: %+v, %v", value, err)
	}
	c := Config{DeliveryConfigDocument: base64.StdEncoding.EncodeToString(raw), DeliveryConfigSHA256: digest}
	if err := c.registerInlineDocuments(); err != nil {
		t.Fatal(err)
	}
	if err := ReadBoundDocument("", digest, &value); err != nil || value.Schema != "candles" {
		t.Fatalf("inline document: %+v, %v", value, err)
	}
	if _, err := boundBytes(path, c.DeliveryConfigDocument, digest); err == nil {
		t.Fatal("two document sources were accepted")
	}
	if _, err := boundBytes(path, "", strings.Repeat("0", 64)); err == nil {
		t.Fatal("incorrect digest was accepted")
	}
	if _, err := boundBytes("", "invalid base64", digest); err == nil {
		t.Fatal("invalid inline data was accepted")
	}
	if err := os.WriteFile(path, []byte(`{"schema":"candles"} {}`), 0600); err != nil {
		t.Fatal(err)
	}
	trailing := []byte(`{"schema":"candles"} {}`)
	trailingDigest := fmt.Sprintf("%x", sha256.Sum256(trailing))
	if err := ReadBoundDocument(path, trailingDigest, &value); err == nil || !strings.Contains(err.Error(), "trailing") {
		t.Fatalf("trailing document: %v", err)
	}
}

func TestRuntimeFeaturesValidateDeploymentAndOrigins(t *testing.T) {
	f := ChartRuntimeFeatures{
		SchemaVersion: 2, EndpointSetVersion: "main", DeploymentVersion: deployment.Current().DeploymentVersion,
		AnchorHeight: 1, AnchorHash: "0x" + strings.Repeat("a", 64), CanonicalAPIOrigin: "https://charts.example.org",
		ReplayGatewayOrigin: "https://replay.example.org", CanonicalSeriesModel: "exact-v1",
		DisplayPrecision: 6, MaxHistoryOrigins: 2, RollupBatch: 4, RollupIntervalMillis: 1000,
	}
	encode := func(t *testing.T, features ChartRuntimeFeatures) Config {
		t.Helper()
		raw, err := json.Marshal(features)
		if err != nil {
			t.Fatal(err)
		}
		return Config{FeaturesConfigDocument: base64.StdEncoding.EncodeToString(raw), FeaturesConfigSHA256: fmt.Sprintf("%x", sha256.Sum256(raw))}
	}
	c := encode(t, f)
	got, err := c.RuntimeFeatures()
	if err != nil || got.CanonicalSeriesModel != f.CanonicalSeriesModel {
		t.Fatalf("valid features: %+v, %v", got, err)
	}
	f.CanonicalAPIOrigin = "http://remote.example.org"
	c = encode(t, f)
	if _, err := c.RuntimeFeatures(); err == nil || !strings.Contains(err.Error(), "https") {
		t.Fatalf("remote HTTP origin: %v", err)
	}
	f.CanonicalAPIOrigin = "https://charts.example.org"
	f.DeploymentVersion = "unknown"
	c = encode(t, f)
	if _, err := c.RuntimeFeatures(); err == nil || !strings.Contains(err.Error(), "deployment_version_mismatch") {
		t.Fatalf("deployment version: %v", err)
	}
	f.DeploymentVersion = deployment.Current().DeploymentVersion
	f.SchemaVersion = 1
	c = encode(t, f)
	if _, err := c.RuntimeFeatures(); err == nil || !strings.Contains(err.Error(), "charts_features_invalid") {
		t.Fatalf("feature schema version: %v", err)
	}
	f.SchemaVersion = 2
	f.CanonicalAPIOrigin = "https://user@example.org"
	c = encode(t, f)
	if _, err := c.RuntimeFeatures(); err == nil || !strings.Contains(err.Error(), "charts_origin_invalid") {
		t.Fatalf("credentialed chart origin: %v", err)
	}
}

func TestRuntimeDocumentsRejectMissingAndUnboundedSources(t *testing.T) {
	digest := strings.Repeat("a", 64)
	for _, tc := range []struct {
		name, path, inline, expected string
	}{
		{"missing file", filepath.Join(t.TempDir(), "missing.json"), "", "runtime_document_invalid"},
		{"overlong inline", "", strings.Repeat("a", 1398105), "runtime_document_invalid"},
		{"empty inline", "", base64.StdEncoding.EncodeToString(nil), "runtime_document_binding_missing"},
		{"empty bytes", "", "==", "runtime_document_invalid"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := boundBytes(tc.path, tc.inline, digest); err == nil || !strings.Contains(err.Error(), tc.expected) {
				t.Fatalf("bounded document %s: %v", tc.name, err)
			}
		})
	}
	cfg := Config{RuntimeBudgetFile: "/tmp/runtime.json", RuntimeBudgetDocument: "e30="}
	if err := cfg.registerInlineDocuments(); err == nil || !strings.Contains(err.Error(), "DB_BUDGET_FILE and DB_BUDGET_DOCUMENT") {
		t.Fatalf("dual runtime document: %v", err)
	}
	raw := []byte(`{"schema":"candles"}`)
	path := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	var value struct {
		Schema string `json:"schema"`
	}
	if err := ReadBoundDocument(path, fmt.Sprintf("%x", sha256.Sum256(raw)), &value); err != nil || value.Schema != "candles" {
		t.Fatalf("read single document: %+v, %v", value, err)
	}
	malformed := []byte(`{"schema":`)
	if err := os.WriteFile(path, malformed, 0600); err != nil {
		t.Fatal(err)
	}
	if err := ReadBoundDocument(path, fmt.Sprintf("%x", sha256.Sum256(malformed)), &value); err == nil {
		t.Fatal("malformed bound JSON was accepted")
	}
}

func TestRuntimeCredentialDocumentsRequireBoundIdentity(t *testing.T) {
	tooLarge := strings.Repeat("x", 1048577)
	for _, tc := range []struct{ name, credentials string }{
		{"oversized credentials", tooLarge},
		{"invalid JWT", "invalid"},
		{"JWT without seed", "-----BEGIN NATS USER JWT-----\na.b.c\n------END NATS USER JWT------"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if option, err := inlineNATSCredentials(tc.credentials); option != nil || err == nil || !strings.Contains(err.Error(), "broker_credentials_invalid") {
				t.Fatalf("inline broker identity: %v", err)
			}
		})
	}
	for _, tc := range []struct{ name, realm, profile, user, password, want string }{
		{"missing local credentials", realmPrivateFixture, profileOwnedLocal, "", "", "fixture_broker_credentials_missing"},
		{"missing local evidence", realmPrivateFixture, profileOwnedLocal, "user", "password", "fixture"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Config{RuntimeProfile: tc.profile, NATSUsername: tc.user, NATSPassword: tc.password}
			cfg.DeploymentRealm = tc.realm
			options, err := cfg.RuntimeNATSOptions()
			if options != nil || err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("local broker identity: %v", err)
			}
		})
	}
}
