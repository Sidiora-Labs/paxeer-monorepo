package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestIdentityRecoveryEvidenceBindsFaultAndDurableRecords(t *testing.T) {
	now := time.Now().UTC()
	binding := IdentityMeasurementBinding{CandidateSHA256: measurementHash([]byte("gateway identity"))}
	before := IdentityDurabilityCheckpoint{
		Binding: binding, CapturedAt: now.Add(-4 * time.Second), Database: "gateway", Schema: "users",
		RecordDigests: map[string]string{"session": measurementHash([]byte("session"))}, Counts: map[string]int{"session": 1},
	}
	before.SHA256 = identityDurabilityDigest(before)
	after := before
	after.CapturedAt = now.Add(-time.Second)
	start, end := now.Add(-3*time.Second), now.Add(-2*time.Second)
	receipt, err := json.Marshal(IdentityFaultReceipt{Binding: binding, Kind: "az_loss", StartedAt: start, EndedAt: end, Recovered: true})
	if err != nil {
		t.Fatal(err)
	}
	root := t.TempDir()
	receiptPath := filepath.Join(root, "fault-receipt.json")
	if err := os.WriteFile(receiptPath, receipt, 0600); err != nil {
		t.Fatal(err)
	}
	digest := measurementHash(receipt)
	path := filepath.Join(root, "recovery.json")
	if err := WriteIdentityRecoveryEvidence(path, before, after, receiptPath, digest, start, end); err != nil {
		t.Fatalf("bound recovery evidence: %v", err)
	}
	var observed IdentityRecoveryEvidence
	if raw, err := os.ReadFile(path); err != nil || json.Unmarshal(raw, &observed) != nil || !observed.DurabilityPassed || !observed.AZLossAcceptance || observed.FaultReceiptSHA256 != digest || observed.Failure != "" {
		t.Fatalf("durable recovery projection: %+v err=%v", observed, err)
	}
	for _, tc := range []struct {
		name, receipt, hash, want string
	}{
		{"absent receipt", filepath.Join(root, "absent.json"), digest, "actual approved fault receipt unavailable"},
		{"changed receipt", receiptPath, strings.Repeat("0", 64), "fault receipt digest mismatch"},
	} {
		if err := WriteIdentityRecoveryEvidence(filepath.Join(root, tc.name+".json"), before, after, tc.receipt, tc.hash, start, end); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Fatalf("%s: error=%v, want %q", tc.name, err, tc.want)
		}
	}
	changed := after
	changed.Counts = map[string]int{"session": 2}
	changed.SHA256 = identityDurabilityDigest(changed)
	failedPath := filepath.Join(root, "changed-recovery.json")
	if err := WriteIdentityRecoveryEvidence(failedPath, before, changed, receiptPath, digest, start, end); err == nil || !strings.Contains(err.Error(), "acknowledged record checksum changed") {
		t.Fatalf("changed durable record: %v", err)
	}
	if raw, err := os.ReadFile(failedPath); err != nil || json.Unmarshal(raw, &observed) != nil || observed.DurabilityPassed || observed.AZLossAcceptance || observed.Failure != "durability_or_environment_mismatch" {
		t.Fatalf("changed recovery projection: %+v err=%v", observed, err)
	}
}
