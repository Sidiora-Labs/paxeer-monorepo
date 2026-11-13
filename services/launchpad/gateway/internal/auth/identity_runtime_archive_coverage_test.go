package auth

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIdentityRuntimeArchiveUsesBoundOwningProcessEvidence(t *testing.T) {
	f, cfg := measurementFixtureConfig(t)
	directory := t.TempDir()
	if err := f.s.WriteRuntimeMeasurementEvidence(directory, cfg.SourceRoot, cfg.Binding); err != nil {
		t.Fatal(err)
	}
	archive := IdentityRuntimeArchive{Directory: directory, Binding: cfg.Binding}
	snapshot, err := archive.CaptureRuntimeMeasurement()
	if err != nil || snapshot.ProcessID < 1 || snapshot.CapturedAt.IsZero() {
		t.Fatalf("owned runtime evidence: snapshot=%+v err=%v", snapshot, err)
	}
	cfg.Runtime = nil
	cfg.RuntimeArchive = &archive
	ctx, cancel := context.WithCancel(f.ctx)
	cancel()
	summary, err := RunIdentityMeasurement(ctx, cfg)
	if err == nil || summary.FinalAcceptance || len(summary.RuntimeSnapshots) != 2 || summary.RuntimeSnapshots[0].ProcessID != snapshot.ProcessID || !strings.Contains(strings.Join(summary.Failures, ","), "workload_canceled") {
		t.Fatalf("archived runtime workload: snapshots=%d failures=%v err=%v", len(summary.RuntimeSnapshots), summary.Failures, err)
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 {
		t.Fatalf("runtime archive entries: %d err=%v", len(entries), err)
	}
	if err := os.WriteFile(filepath.Join(directory, "runtime-99999999999999999999.json"), []byte(`{"binding":{},"snapshot":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := archive.CaptureRuntimeMeasurement(); err == nil {
		t.Fatal("unbound later runtime evidence displaced the owned snapshot")
	}
}
