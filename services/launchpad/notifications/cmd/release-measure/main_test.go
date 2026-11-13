package main

import (
	"encoding/json"
	"errors"
	"flag"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/notifications"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
)

func TestExecuteRequiresBoundedReleaseConfiguration(t *testing.T) {
	originalFlags, originalArgs := flag.CommandLine, os.Args
	t.Cleanup(func() { flag.CommandLine, os.Args = originalFlags, originalArgs })
	run := func(args ...string) error {
		t.Helper()
		flag.CommandLine = flag.NewFlagSet("release-measure", flag.ContinueOnError)
		os.Args = append([]string{"release-measure"}, args...)
		return execute()
	}
	if err := run(); !errors.Is(err, notifications.ErrInvalid) {
		t.Fatalf("missing configuration accepted: %v", err)
	}
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"unknown":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run("-config", path); err == nil {
		t.Fatal("unknown configuration field accepted")
	}
	if err := os.WriteFile(path, []byte(`{} {}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run("-config", path); !errors.Is(err, notifications.ErrInvalid) {
		t.Fatalf("trailing JSON accepted: %v", err)
	}
	if err := os.WriteFile(path, []byte(`{"database_env":"DATABASE_URL","account_origin":"https://example.test","account_token_ref":"token","secret_provider":"env"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run("-config", path); !errors.Is(err, notifications.ErrInvalid) {
		t.Fatalf("missing measurement accepted: %v", err)
	}
}

func TestExecuteChecksBoundSourceAfterSecureRuntimeSetup(t *testing.T) {
	dsn := os.Getenv("NOTIFICATIONS_TEST_DATABASE_URL")
	if dsn == "" {
		t.Fatal("NOTIFICATIONS_TEST_DATABASE_URL must select an isolated PostgreSQL fixture")
	}
	originalFlags, originalArgs := flag.CommandLine, os.Args
	t.Cleanup(func() { flag.CommandLine, os.Args = originalFlags, originalArgs })
	root := t.TempDir()
	pki := testpki.New(t)
	leaf := pki.Leaf("release-client", "account.example")
	zeros := strings.Repeat("0", 64)
	steps := []notifications.ReleaseStep{}
	for _, name := range []string{"pool_launch", "callout_publish", "follow_author", "squad_join", "threshold_alert", "inbox", "feed", "leaderboard"} {
		step := notifications.ReleaseStep{Name: name, Method: http.MethodGet, URL: "https://account.example/" + name, Status: http.StatusOK, Assertions: []notifications.ReleaseAssertion{{Pointer: "/id", Equals: json.RawMessage(`"pool"`)}}}
		switch name {
		case "callout_publish", "threshold_alert":
			step.Assertions = []notifications.ReleaseAssertion{{Pointer: "/id", FromStep: "pool_launch", FromPointer: "/id"}}
		case "inbox":
			step.Assertions = []notifications.ReleaseAssertion{{Pointer: "/id", FromStep: "threshold_alert", FromPointer: "/id"}}
		case "feed":
			step.Assertions = []notifications.ReleaseAssertion{{Pointer: "/id", FromStep: "callout_publish", FromPointer: "/id"}}
		}
		steps = append(steps, step)
	}
	measurement := notifications.ReleaseConfig{Binding: notifications.ReleaseBinding{CandidateDigest: zeros, InterfaceDigest: zeros, TopologyDigest: zeros, DatasetDigest: zeros, SchemaVersion: "1", PolicyVersion: "1"}, SourceRoot: root, TopologyFile: filepath.Join(root, "topology"), InterfaceFile: filepath.Join(root, "interface"), SourceFiles: map[string]string{"candidate.txt": zeros}, OutputDir: filepath.Join(root, "capture"), TriggerFile: filepath.Join(root, "triggers"), AllowedAccounts: []string{"account"}, AllowedOrigins: []string{"https://account.example"}, IsolatedSandbox: true, Concurrency: 1, QueueCapacity: 1, MaxCaptureBytes: 1024, Steps: steps, DeviceReceiptFile: filepath.Join(root, "devices"), DeviceCaptureKeys: map[string]string{"device": "capture"}, AppBuildDigests: map[string]string{"apns": zeros, "fcm": zeros}}
	if err := measurement.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "candidate.txt"), []byte("changed candidate"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("RELEASE_ACCOUNT_TOKEN", "owner-token")
	c := configuration{Measurement: measurement, DatabaseEnv: "NOTIFICATIONS_TEST_DATABASE_URL", AccountOrigin: "https://account.example", AccountTokenRef: "RELEASE_ACCOUNT_TOKEN", TLSCAFile: pki.CAFile, TLSCertificateFile: leaf.CertFile, TLSKeyFile: leaf.KeyFile, SecretProvider: secrets.ProviderEnv}
	path := filepath.Join(root, "configuration.json")
	run := func(config configuration) error {
		t.Helper()
		raw, err := json.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, raw, 0600); err != nil {
			t.Fatal(err)
		}
		flag.CommandLine = flag.NewFlagSet("release-measure", flag.ContinueOnError)
		os.Args = []string{"release-measure", "-config", path}
		return execute()
	}
	if err := run(c); err == nil || !strings.Contains(err.Error(), "candidate file drift") {
		t.Fatalf("changed candidate accepted: %v", err)
	}
	missingDatabase := c
	missingDatabase.DatabaseEnv = "RELEASE_MISSING_DATABASE_URL"
	t.Setenv(missingDatabase.DatabaseEnv, "")
	if err := run(missingDatabase); !errors.Is(err, notifications.ErrUnavailable) {
		t.Fatalf("missing database selected: %v", err)
	}
	missingCertificate := c
	missingCertificate.TLSCertificateFile = filepath.Join(root, "missing.pem")
	if err := run(missingCertificate); err == nil {
		t.Fatal("missing client certificate accepted")
	}
	missingCA := c
	missingCA.TLSCAFile = filepath.Join(root, "missing-ca.pem")
	if err := run(missingCA); err == nil {
		t.Fatal("missing CA accepted")
	}
	badCA := c
	badCA.TLSCAFile = filepath.Join(root, "bad-ca.pem")
	if err := os.WriteFile(badCA.TLSCAFile, []byte("not-a-certificate"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := run(badCA); !errors.Is(err, notifications.ErrInvalid) {
		t.Fatalf("invalid CA accepted: %v", err)
	}
	missingToken := c
	missingToken.AccountTokenRef = "RELEASE_MISSING_ACCOUNT_TOKEN"
	t.Setenv(missingToken.AccountTokenRef, "")
	if err := run(missingToken); !errors.Is(err, notifications.ErrUnavailable) {
		t.Fatalf("missing account token accepted: %v", err)
	}
	badOrigin := c
	badOrigin.AccountOrigin = "http://account.example"
	if err := run(badOrigin); !errors.Is(err, notifications.ErrInvalid) {
		t.Fatalf("unverified account origin accepted: %v", err)
	}
}
