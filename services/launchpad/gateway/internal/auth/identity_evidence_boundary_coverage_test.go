package auth

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestIdentityMeasurementSourceAndEvidenceBindings(t *testing.T) {
	root := t.TempDir()
	files, digest, err := identityMeasurementSources(root)
	if err == nil || files != nil || digest != "" || !strings.Contains(err.Error(), "actual source binding unavailable") {
		t.Fatalf("absent sources: files=%v digest=%q err=%v", files, digest, err)
	}
	source := filepath.Join(root, "services", "launchpad", "gateway", "internal", "auth")
	if err := os.MkdirAll(source, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "owned.go"), []byte("package auth\n"), 0600); err != nil {
		t.Fatal(err)
	}
	files, digest, err = identityMeasurementSources(root)
	if err != nil || len(files) != 1 || digest == "" || files["services/launchpad/gateway/internal/auth/owned.go"] != measurementHash([]byte("package auth\n")) {
		t.Fatalf("owned source binding: files=%v digest=%q err=%v", files, digest, err)
	}
	if err := os.Symlink(filepath.Join(source, "owned.go"), filepath.Join(source, "linked.go")); err != nil {
		t.Fatal(err)
	}
	files, digest, err = identityMeasurementSources(root)
	if err == nil || files != nil || digest != "" || !strings.Contains(err.Error(), "actual source binding unavailable") {
		t.Fatalf("linked source binding: files=%v digest=%q err=%v", files, digest, err)
	}
}

func TestIdentityMeasurementRejectsAmbiguousRuntimeAndExceededCapture(t *testing.T) {
	_, cfg := measurementFixtureConfig(t)
	cfg.RuntimeArchive = &IdentityRuntimeArchive{Directory: t.TempDir()}
	if err := validateIdentityMeasurement(cfg); err == nil || !strings.Contains(err.Error(), "ambiguous runtime provider") {
		t.Fatalf("runtime ownership: %v", err)
	}
	cfg.RuntimeArchive = nil
	cfg.Phases = append([]IdentityMeasurementPhase(nil), cfg.Phases...)
	cfg.Phases[0].Duration = 2 * time.Second
	cfg.MaxAcknowledgements = 1
	if err := validateIdentityMeasurement(cfg); err == nil || !strings.Contains(err.Error(), "admitted capture bound below offered workload") {
		t.Fatalf("capture capacity: %v", err)
	}
	cfg.Phases[0].Duration = time.Second
	cfg.MaxAcknowledgements = 8
	obstruction := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(obstruction, []byte("owned"), 0600); err != nil {
		t.Fatal(err)
	}
	cfg.EvidenceDirectory = filepath.Join(obstruction, "evidence")
	summary, err := RunIdentityMeasurement(context.Background(), cfg)
	if err == nil || !strings.Contains(err.Error(), "evidence directory unavailable") || summary.Requests != 0 {
		t.Fatalf("evidence directory: summary=%+v err=%v", summary, err)
	}
}

func TestIdentityDurabilityRejectsRepeatedReceiptsAndChangedCheckpoints(t *testing.T) {
	_, cfg := measurementFixtureConfig(t)
	receipt := IdentityAcknowledgement{SessionID: uuid.NewString(), ChallengeID: uuid.NewString()}
	checkpoint, err := CaptureIdentityDurability(context.Background(), cfg.Pool, cfg.Schema, cfg.Binding, []IdentityAcknowledgement{receipt, receipt})
	if err == nil || !strings.Contains(err.Error(), "invalid or repeated receipt") || len(checkpoint.Acknowledged) != 2 || len(checkpoint.RecordDigests) != 0 {
		t.Fatalf("repeated receipt: checkpoint=%+v err=%v", checkpoint, err)
	}
	before := IdentityDurabilityCheckpoint{Database: "owned", Schema: cfg.Schema, RecordDigests: map[string]string{"session": measurementHash([]byte("owned"))}, CapturedAt: time.Now().Add(-time.Second)}
	after := before
	after.CapturedAt = time.Now()
	if err := CompareIdentityRecovery(before, after); err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("unsigned checkpoint: %v", err)
	}
	after.Database = "another"
	if err := CompareIdentityRecovery(before, after); err == nil || !strings.Contains(err.Error(), "environment mismatch") {
		t.Fatalf("changed database binding: %v", err)
	}
	if identityEventAction("unknown") || !identityEventAction("session_created") {
		t.Fatal("revision action classification changed")
	}
}
