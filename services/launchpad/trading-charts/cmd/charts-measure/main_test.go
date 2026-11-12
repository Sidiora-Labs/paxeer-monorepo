package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBoundedDocumentReadsDigestBoundJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "profile.json")
	raw := []byte(`{"schema":"candles"}`)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(raw)
	digest := hex.EncodeToString(sum[:])
	var value struct {
		Schema string `json:"schema"`
	}
	if err := boundedDocument(path, digest, 1024, &value); err != nil || value.Schema != "candles" {
		t.Fatalf("read profile: %+v, %v", value, err)
	}
	for _, tc := range []struct {
		name, content, digest string
		maximum               int64
	}{
		{"wrong digest", string(raw), strings.Repeat("0", 64), 1024},
		{"over budget", string(raw), digest, 2},
		{"unknown field", `{"schema":"candles","other":1}`, "", 1024},
		{"trailing data", `{"schema":"candles"} {}`, "", 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.content), 0600); err != nil {
				t.Fatal(err)
			}
			checkDigest := tc.digest
			if checkDigest == "" {
				sum := sha256.Sum256([]byte(tc.content))
				checkDigest = hex.EncodeToString(sum[:])
			}
			if err := boundedDocument(path, checkDigest, tc.maximum, &value); err == nil {
				t.Fatal("invalid profile was accepted")
			}
		})
	}
}

func TestMeasurementStageAndArtifactBudget(t *testing.T) {
	c := config{BurstOffsetSeconds: 60}
	for _, tc := range []struct {
		at   time.Duration
		want string
	}{{59 * time.Second, "steady"}, {60 * time.Second, "burst"}, {359 * time.Second, "burst"}, {360 * time.Second, "steady"}} {
		if got := stage(tc.at, c); got != tc.want {
			t.Fatalf("stage(%s) = %q, want %q", tc.at, got, tc.want)
		}
	}
	file, err := os.CreateTemp(t.TempDir(), "artifact")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	budget := &rawBudget{maximum: 5}
	w := boundedArtifact{file: file, budget: budget}
	if n, err := w.Write([]byte("abc")); err != nil || n != 3 || budget.used != 3 {
		t.Fatalf("first write: %d, %v, budget %+v", n, err, budget)
	}
	if n, err := w.Write([]byte("def")); err == nil || n != 0 || budget.used != 3 {
		t.Fatalf("over budget write: %d, %v, budget %+v", n, err, budget)
	}
	if n, err := w.Write([]byte("de")); err != nil || n != 2 || budget.used != 5 {
		t.Fatalf("final write: %d, %v, budget %+v", n, err, budget)
	}
	raw, err := os.ReadFile(file.Name())
	if err != nil || string(raw) != "abcde" {
		t.Fatalf("artifact: %q, %v", raw, err)
	}
}

func TestMeasurementConfigRequiresBoundedWeeklyWindow(t *testing.T) {
	digest := strings.Repeat("a", 64)
	previous := candidateDigest
	candidateDigest = digest
	t.Cleanup(func() { candidateDigest = previous })
	for _, name := range []string{"DATABASE_URL", "RPC_URL", "NATS_URL", "S3_ENDPOINT", "S3_BUCKET", "S3_ACCESS_KEY_ID", "S3_SECRET_ACCESS_KEY"} {
		t.Setenv(name, "configured")
	}
	poolA := "0x" + strings.Repeat("a", 40)
	poolB := "0x" + strings.Repeat("b", 40)
	c := config{
		CandidateDigest: digest, InterfaceDigest: digest, Schema: "candles", SourceStream: "canonical",
		NATSStream: "CHARTS", NATSDurable: "charts", Applications: []string{"charts"},
		Pools: []string{poolA, poolB}, HotPool: poolA, From: 1704067200, To: 1704672000,
		SampleMillis: 1000, MaximumBars: 1000, RawArtifactBudgetBytes: 1048576,
		StartAt: time.Unix(1704067200, 0), BurstOffsetSeconds: 60, RecoverySeconds: 60,
	}
	if err := c.validate(); err != nil {
		t.Fatalf("valid weekly profile: %v", err)
	}
	invalid := c
	invalid.Pools = []string{poolA, poolA}
	if err := invalid.validate(); err == nil || !strings.Contains(err.Error(), "unique actual pools") {
		t.Fatalf("duplicate pool: %v", err)
	}
	invalid = c
	invalid.To++
	if err := invalid.validate(); err == nil || !strings.Contains(err.Error(), "bounded complete UTC parent boundary") {
		t.Fatalf("partial week: %v", err)
	}
	invalid = c
	invalid.HotPool = "0x" + strings.Repeat("c", 40)
	if err := invalid.validate(); err == nil || !strings.Contains(err.Error(), "admitted hot pool") {
		t.Fatalf("unknown hot pool: %v", err)
	}
	for _, tc := range []struct {
		name, want string
		change     func(*config)
	}{
		{"unbound source stream", "candidate-bound measurement profile", func(p *config) { p.SourceStream = "" }},
		{"invalid application identity", "PostgreSQL application identity", func(p *config) { p.Applications = []string{"invalid name"} }},
		{"unaligned parent start", "complete UTC parent boundary", func(p *config) { p.From++ }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := c
			tc.change(&candidate)
			if err := candidate.validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("measurement profile %s: %v", tc.name, err)
			}
		})
	}
	t.Setenv("S3_BUCKET", "")
	if err := c.validate(); err == nil || !strings.Contains(err.Error(), "required provider input absent: S3_BUCKET") {
		t.Fatalf("archive provider input: %v", err)
	}
}
