package release_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// The corrected gate for 22.33.889 is the Python file. These compatibility
// entry points execute the same real release-boundary cases, without a second
// implementation of the producer, consumer or recovery logic.
func repairM889Case(t *testing.T, name string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the release regression tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	path := filepath.Join(filepath.Dir(file), "test_repair_22_33_889.py")
	command := exec.CommandContext(ctx, "python3", "-m", "pytest", "-q", path+"::"+name)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", name, err, output)
	}
	t.Logf("%s", output)
}

func TestRepairM_889_BuildDistribution(t *testing.T) {
	repairM889Case(t, "test_build_distribution")
}

func TestRepairM_889_ReproducibleDistribution(t *testing.T) {
	repairM889Case(t, "test_reproducible_distribution")
}

func TestRepairM_889_VerificationDistribution(t *testing.T) {
	repairM889Case(t, "test_verification_distribution")
}

func TestRepairM_889_RehearsalEvidenceBinding(t *testing.T) {
	repairM889Case(t, "test_rehearsal_evidence_binding")
}

func TestRepairM_889_RehearsalRequiresFreshDriver(t *testing.T) {
	repairM889Case(t, "test_rehearsal_requires_fresh_driver")
}

func TestRepairM_889_RollbackForwardRecovery(t *testing.T) {
	repairM889Case(t, "test_rollback_forward_recovery")
}

func TestRepairM_889_FreshHandoffReachesMarkets(t *testing.T) {
	repairM889Case(t, "test_fresh_handoff_reaches_markets")
}

func TestRepairM_889_ProgrammeDriverEvidence(t *testing.T) {
	repairM889Case(t, "test_programme_driver_consumes_rpc_evidence")
}

func TestRepairM_889_ProgrammeMergedScenarios(t *testing.T) {
	repairM889Case(t, "test_programme_merged_scenarios")
}
