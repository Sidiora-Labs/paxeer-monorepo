package auth

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestIdentityMeasurementRecordsCanceledWorkload(t *testing.T) {
	_, cfg := measurementFixtureConfig(t)
	canceled, stop := context.WithCancel(context.Background())
	stop()
	summary, err := RunIdentityMeasurement(canceled, cfg)
	if err == nil || summary.FinalAcceptance || summary.Requests != 0 || summary.GeneratorRejected != 0 || !slices.Contains(summary.Failures, "workload_canceled") || summary.Assertions["generator_unsaturated"] {
		t.Fatalf("canceled workload: summary=%+v err=%v", summary, err)
	}
	raw, err := os.ReadFile(filepath.Join(cfg.EvidenceDirectory, "requests.jsonl"))
	if err != nil || summary.RawSHA256 != measurementHash(raw) {
		t.Fatalf("canceled workload capture: digest=%q err=%v", summary.RawSHA256, err)
	}
	if _, err := os.Stat(filepath.Join(cfg.EvidenceDirectory, "summary.json")); err != nil {
		t.Fatalf("canceled workload summary: %v", err)
	}
}
