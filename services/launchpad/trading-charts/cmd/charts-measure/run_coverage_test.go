package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
)

func TestRunPreparesBoundedArtifactsBeforeProviderDial(t *testing.T) {
	digest := strings.Repeat("a", 64)
	previousDigest, previousArgs, previousFlags := candidateDigest, os.Args, flag.CommandLine
	candidateDigest = digest
	t.Cleanup(func() {
		candidateDigest, os.Args, flag.CommandLine = previousDigest, previousArgs, previousFlags
	})
	for _, name := range []string{"RPC_URL", "NATS_URL", "S3_ENDPOINT", "S3_BUCKET", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY"} {
		t.Setenv(name, "configured")
	}
	t.Setenv("DATABASE_URL", "not a database URL")
	poolA, poolB := "0x"+strings.Repeat("a", 40), "0x"+strings.Repeat("b", 40)
	profile := config{
		CandidateDigest: digest, InterfaceDigest: digest, Schema: "candles", SourceStream: "canonical",
		NATSStream: "CHARTS", NATSDurable: "charts", Applications: []string{"charts"},
		Pools: []string{poolA, poolB}, HotPool: poolA, From: 1704067200, To: 1704672000,
		SampleMillis: 1000, MaximumBars: 1000, RawArtifactBudgetBytes: 1048576,
		StartAt: time.Unix(1704067200, 0), BurstOffsetSeconds: 60, RecoverySeconds: 60,
	}
	raw, err := json.Marshal(profile)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "profile.json")
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	output := filepath.Join(t.TempDir(), "measurement")
	flag.CommandLine = flag.NewFlagSet("charts-measure", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{"charts-measure", "-config", path, "-config-sha256", hex.EncodeToString(sum[:]), "-output", output}
	if err := run(); err == nil || !strings.Contains(err.Error(), "actual database unavailable") {
		t.Fatalf("provider URL refusal: %v", err)
	}
	for _, name := range []string{"samples.ndjson", "latencies.ndjson", "locks.ndjson"} {
		info, err := os.Stat(filepath.Join(output, name))
		if err != nil || info.Size() != 0 || info.Mode().Perm() != 0600 {
			t.Fatalf("bounded artifact %s: %+v, %v", name, info, err)
		}
	}
	t.Setenv("DATABASE_URL", internaltest.NewDatabase(t))
	output = filepath.Join(t.TempDir(), "database-measurement")
	flag.CommandLine = flag.NewFlagSet("charts-measure", flag.ContinueOnError)
	flag.CommandLine.SetOutput(io.Discard)
	os.Args = []string{"charts-measure", "-config", path, "-config-sha256", hex.EncodeToString(sum[:]), "-output", output}
	if err := run(); err == nil || !strings.Contains(err.Error(), "commit timestamp authority") {
		t.Fatalf("database timestamp authority: %v", err)
	}
	for _, tc := range []struct {
		name, output, occupied, want string
	}{
		{"relative output", "measurement", "", "absolute private output"},
		{"existing output file", filepath.Join(t.TempDir(), "occupied"), "directory", "file exists"},
		{"existing samples", filepath.Join(t.TempDir(), "samples"), "samples.ndjson", "file exists"},
		{"existing latencies", filepath.Join(t.TempDir(), "latencies"), "latencies.ndjson", "file exists"},
		{"existing locks", filepath.Join(t.TempDir(), "locks"), "locks.ndjson", "file exists"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.occupied == "directory" {
				if err := os.WriteFile(tc.output, nil, 0600); err != nil {
					t.Fatal(err)
				}
			} else if tc.occupied != "" {
				if err := os.Mkdir(tc.output, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(tc.output, tc.occupied), nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			flag.CommandLine = flag.NewFlagSet("charts-measure", flag.ContinueOnError)
			flag.CommandLine.SetOutput(io.Discard)
			os.Args = []string{"charts-measure", "-config", path, "-config-sha256", hex.EncodeToString(sum[:]), "-output", tc.output}
			if err := run(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("private artifact admission: %v", err)
			}
		})
	}
	for _, tc := range []struct {
		name, configuration, checksum, want string
	}{
		{"missing configuration document", filepath.Join(t.TempDir(), "absent.json"), hex.EncodeToString(sum[:]), "bounded regular input required"},
		{"relative configuration document", "relative.json", hex.EncodeToString(sum[:]), "absolute"},
		{"changed configuration checksum", path, strings.Repeat("0", 64), "input digest mismatch"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet("charts-measure", flag.ContinueOnError)
			flag.CommandLine.SetOutput(io.Discard)
			os.Args = []string{"charts-measure", "-config", tc.configuration, "-config-sha256", tc.checksum, "-output", filepath.Join(t.TempDir(), "measurement")}
			if err := run(); err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(tc.want)) {
				t.Fatalf("measurement configuration admission: %v", err)
			}
		})
	}
}
