package brokerprovision

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nats-io/nats.go/jetstream"
)

func TestCommandRejectsUnavailableAndUnboundedSources(t *testing.T) {
	directory := t.TempDir()
	valid := writeDocument(t, smallDocument(t))
	invalid := filepath.Join(directory, "invalid.json")
	if err := os.WriteFile(invalid, []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BROKER_ADMISSION_ADDRESS", "nats://127.0.0.1:1")
	t.Setenv("BROKER_ADMISSION_TOKEN", "bounded-token")
	t.Setenv("BROKER_ADMISSION_EMPTY", "")
	t.Setenv("BROKER_ADMISSION_SPACES", " \n ")
	t.Setenv("BROKER_ADMISSION_LINES", "line\nline")
	t.Setenv("BROKER_ADMISSION_CREDENTIALS", "invalid credentials")
	base := []string{"check", "--document", valid, "--broker-env", "BROKER_ADMISSION_ADDRESS"}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		name string
		ctx  context.Context
		args []string
		want string
	}{
		{"context", nil, base, "context required"},
		{"document unavailable", context.Background(), []string{"check", "--document", filepath.Join(directory, "missing")}, "cannot open document"},
		{"document invalid", context.Background(), []string{"check", "--document", invalid}, "document"},
		{"authentication absent", context.Background(), base, "one credentials or token source required"},
		{"authentication ambiguous", context.Background(), append(append([]string{}, base...), "--token-env", "BROKER_ADMISSION_TOKEN", "--credentials-env", "BROKER_ADMISSION_CREDENTIALS"), "one credentials or token source required"},
		{"empty token", context.Background(), append(append([]string{}, base...), "--token-env", "BROKER_ADMISSION_EMPTY"), "token source invalid"},
		{"blank token", context.Background(), append(append([]string{}, base...), "--token-env", "BROKER_ADMISSION_SPACES"), "token source invalid"},
		{"multiline token", context.Background(), append(append([]string{}, base...), "--token-env", "BROKER_ADMISSION_LINES"), "token source invalid"},
		{"credentials", context.Background(), append(append([]string{}, base...), "--credentials-env", "BROKER_ADMISSION_CREDENTIALS"), "credentials source invalid"},
		{"cancellation", cancelled, append(append([]string{}, base...), "--token-env", "BROKER_ADMISSION_TOKEN"), "operation canceled"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			if status := Run(tc.ctx, tc.args, &out, &errOut); status != 2 || out.Len() != 0 || !strings.Contains(errOut.String(), tc.want) {
				t.Fatalf("source admission: status=%d out=%q err=%q", status, out.String(), errOut.String())
			}
		})
	}
	for _, s := range []source{{file: filepath.Join(directory, "missing")}, {file: directory}, {env: "BROKER_ADMISSION_EMPTY"}} {
		if raw, err := s.read(); err == nil || raw != nil {
			t.Fatalf("unavailable source: %q %v", raw, err)
		}
	}
	if _, err := Read(nil); err == nil {
		t.Fatal("nil document reader admitted")
	}
	if _, err := Check(context.Background(), nil, smallDocument(t)); err == nil {
		t.Fatal("absent broker admitted")
	}
	if _, err := Provision(context.Background(), nil, Document{}, Updates{}); err == nil {
		t.Fatal("invalid provision document admitted")
	}
	if equalJSON(make(chan int), "value") || equalJSON("value", make(chan int)) {
		t.Fatal("unencodable configuration compared equal")
	}
	config := jetstream.ConsumerConfig{Replicas: 3}
	if !equalConsumer(config, jetstream.ConsumerConfig{}, 3) {
		t.Fatal("inherited replica count differs")
	}
	metadata := applicationMetadata(map[string]string{"_nats.internal": "broker", "application": "value"})
	if len(metadata) != 1 || metadata["application"] != "value" {
		t.Fatalf("application metadata: %v", metadata)
	}
}

func TestCommandReportsUnwritableProvisioningOutput(t *testing.T) {
	nc, _, secret := realBroker(t)
	t.Setenv("BROKER_REPORT_ADDRESS", nc.ConnectedUrl())
	t.Setenv("BROKER_REPORT_TOKEN", secret)
	path := writeDocument(t, smallDocument(t))
	full, err := os.OpenFile("/dev/full", os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer full.Close()
	for _, action := range []string{"check", "provision"} {
		var errOut bytes.Buffer
		status := Run(context.Background(), []string{action, "--document", path, "--broker-env", "BROKER_REPORT_ADDRESS", "--token-env", "BROKER_REPORT_TOKEN", "--timeout", "2m"}, full, &errOut)
		if status != 2 || errOut.String() != "brokerprovision: cannot write report\n" {
			t.Fatalf("report failure: status=%d err=%s", status, &errOut)
		}
	}
	var errOut bytes.Buffer
	if status := Run(context.Background(), []string{"check", "--document", path, "--broker-env", "BROKER_REPORT_ADDRESS", "--token-env", "BROKER_REPORT_TOKEN", "--timeout", "2m"}, io.Discard, &errOut); status != 0 || errOut.Len() != 0 {
		t.Fatalf("provisioned broker after report failure: %d %s", status, &errOut)
	}
}
