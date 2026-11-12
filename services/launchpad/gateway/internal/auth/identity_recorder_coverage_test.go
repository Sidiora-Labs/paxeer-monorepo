package auth

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIdentityMeasurementRecorderClassifiesActualSamples(t *testing.T) {
	path := filepath.Join(t.TempDir(), "requests.jsonl")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	start := time.Now().UTC()
	r := &IdentityMeasurementRecorder{
		file: file, digest: sha256.New(),
		summary:       IdentityMeasurementSummary{PhaseWindows: map[string]*IdentityPhaseWindow{"measure": {StartedAt: start, WindowEndedAt: start.Add(time.Second)}}},
		phaseRequests: map[string]int64{}, phaseSuccessful: map[string]int64{}, phaseFailures: map[string]int64{},
	}
	samples := []IdentityRequestMeasurement{
		{Phase: "measure", Operation: loginOperation, Status: 200, ExpectedStatus: 200, CompletedAt: start.Add(100 * time.Millisecond), LatencyNS: int64(time.Millisecond), CorrectedLatencyNS: int64(2 * time.Millisecond)},
		{Phase: "measure", Operation: "refresh", Status: 200, ExpectedStatus: 200, CompletedAt: start.Add(2 * time.Second), LatencyNS: int64(time.Millisecond), CorrectedLatencyNS: int64(2 * time.Millisecond)},
		{Phase: "measure", Operation: "challenge", Status: 201, ExpectedStatus: 201, CompletedAt: start.Add(200 * time.Millisecond)},
		{Phase: "measure", Operation: "challenge", Status: 503, ExpectedStatus: 201},
		{Phase: "measure", Operation: "challenge", Status: 429, ExpectedStatus: 201},
		{Phase: "measure", Operation: "challenge", Failure: "generator_queue_full"},
		{Phase: "measure", Operation: "challenge", Status: 500, ExpectedStatus: 201},
		{Phase: "warmup", Operation: "challenge", Status: 500, ExpectedStatus: 201, Warmup: true},
	}
	for _, sample := range samples {
		r.append(sample)
	}
	window := r.summary.PhaseWindows["measure"]
	if r.failure != nil || r.summary.Requests != 7 || r.summary.SuccessfulRequests != 3 || r.summary.SuccessfulSignInRefresh != 2 || r.summary.Dependency503 != 1 || r.summary.Admission429 != 1 || r.summary.GeneratorRejected != 1 || r.summary.UnexpectedFailures != 2 || r.phaseFailures["measure"] != 4 || r.phaseSuccessful["measure"] != 2 || window.CompletedSignInRefreshInWindow != 1 || window.CompletedSignInRefreshInDrain != 1 {
		t.Fatalf("sample accounting: summary=%+v failures=%d successes=%d window=%+v err=%v", r.summary, r.phaseFailures["measure"], r.phaseSuccessful["measure"], window, r.failure)
	}
	if err := file.Sync(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil || strings.Count(string(raw), "\n") != len(samples) || measurementHash(raw) != hex.EncodeToString(r.digest.Sum(nil)) {
		t.Fatalf("raw sample capture: bytes=%d err=%v", len(raw), err)
	}
}
