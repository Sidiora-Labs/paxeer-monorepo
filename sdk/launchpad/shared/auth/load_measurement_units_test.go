package auth

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestSignedLoadPlanAndVerificationSummary(t *testing.T) {
	digest := strings.Repeat("a", 64)
	plan := SignedLoadPlan{
		RunID: uuid.NewString(), CandidateSHA256: digest, SpecSHA256: digest,
		SourceSHA256: map[string]string{"shared/auth/contract.go": digest},
		Environment:  "qualification", ProofKind: valueEoaEip191, Region: "zone-a",
		DurationSeconds: 10, WarmupSeconds: 1, TargetRPS: 10,
		Concurrency: 2, QueueCapacity: 10, RequestTimeoutMS: 1000, SampleIntervalMS: 100,
	}
	plan.WorkloadSHA256 = SignedLoadWorkloadDigest(plan)
	if err := plan.Validate(); err != nil {
		t.Fatal(err)
	}
	changed := plan
	changed.TargetRPS++
	if SignedLoadWorkloadDigest(changed) == plan.WorkloadSHA256 {
		t.Fatal("workload digest did not bind the request rate")
	}
	changed = plan
	changed.TargetRPS = 20001
	if err := changed.Validate(); err == nil {
		t.Fatal("unbounded request rate was admitted")
	}
	changed = plan
	changed.SourceSHA256 = map[string]string{"shared/auth/contract.go": "invalid"}
	if err := changed.Validate(); err == nil {
		t.Fatal("invalid source digest was admitted")
	}

	var measurements AuthVerificationMeasurements
	if _, err := measurements.Snapshot(); err == nil {
		t.Fatal("empty verification measurements were accepted")
	}
	measurements.Observe(valueEip191, valueAccepted, 100*time.Nanosecond)
	measurements.Observe(valueEip191, "signer", 300*time.Nanosecond)
	measurements.Observe(valueEip1271, "contract_rejected", 200*time.Nanosecond)
	summary, err := measurements.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if summary[valueEip191].Samples != 2 || summary[valueEip191].P95NS != 300 || summary[valueEip191].P99NS != 300 || summary[valueEip191].Categories[valueAccepted] != 1 || summary[valueEip191].Categories["signer"] != 1 {
		t.Fatalf("EOA verification summary: %+v", summary[valueEip191])
	}
	if summary[valueEip1271].Samples != 1 || summary[valueEip1271].Categories["contract_rejected"] != 1 {
		t.Fatalf("contract verification summary: %+v", summary[valueEip1271])
	}
	summary[valueEip191].Categories[valueAccepted] = 99
	again, err := measurements.Snapshot()
	if err != nil || again[valueEip191].Categories[valueAccepted] != 1 {
		t.Fatalf("verification summary shared mutable counts: %+v, %v", again, err)
	}
	measurements.Observe("unknown", valueAccepted, time.Nanosecond)
	if _, err := measurements.Snapshot(); err == nil {
		t.Fatal("invalid verification method was accepted")
	}
}

func TestSignedLoadRecoveryAndResultEncoding(t *testing.T) {
	digest := strings.Repeat("b", 64)
	before := AuthStateSnapshot{ObservedAt: time.Now().Add(-time.Second), LedgerRecords: 2, LedgerSHA256: digest, AuditRecords: 2, AuditSHA256: digest}
	after := before
	after.ObservedAt = time.Now()
	if err := CompareAuthRecovery(before, after); err != nil {
		t.Fatal(err)
	}
	after.AuditRecords++
	if err := CompareAuthRecovery(before, after); err == nil {
		t.Fatal("changed durable audit count was accepted")
	}
	var output bytes.Buffer
	result := SignedLoadResult{ScheduledRequests: 2, MeasuredRequests: 2, SuccessfulRequests: 2}
	if err := result.WriteJSON(&output); err != nil {
		t.Fatal(err)
	}
	var decoded SignedLoadResult
	if err := json.Unmarshal(output.Bytes(), &decoded); err != nil || decoded.ScheduledRequests != 2 || decoded.SuccessfulRequests != 2 {
		t.Fatalf("encoded measurement result: %+v, %v", decoded, err)
	}
	if err := result.WriteJSON(nil); err == nil {
		t.Fatal("missing result writer was accepted")
	}
}
