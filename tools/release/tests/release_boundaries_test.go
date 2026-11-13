package release_test

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// Release boundary cases exercise distribution, rehearsal and recovery paths.

func runReleaseBoundaryCase(t *testing.T, name string) {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the release regression tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	path := filepath.Join(filepath.Dir(file), "test_release_boundaries.py")
	command := exec.CommandContext(ctx, "python3", "-m", "pytest", "-q", path+"::"+name)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("%s: %v\n%s", name, err, output)
	}
	t.Logf("%s", output)
}

func TestReleaseBuildDistribution(t *testing.T) {
	runReleaseBoundaryCase(t, "test_build_distribution")
}

func TestReleaseReproducibleDistribution(t *testing.T) {
	runReleaseBoundaryCase(t, "test_reproducible_distribution")
}

func TestReleaseVerificationDistribution(t *testing.T) {
	runReleaseBoundaryCase(t, "test_verification_distribution")
}

func TestReleaseRehearsalEvidenceBinding(t *testing.T) {
	runReleaseBoundaryCase(t, "test_rehearsal_evidence_binding")
}

func TestReleaseRehearsalRequiresFreshDriver(t *testing.T) {
	runReleaseBoundaryCase(t, "test_rehearsal_requires_fresh_driver")
}

func TestReleaseRollbackForwardRecovery(t *testing.T) {
	runReleaseBoundaryCase(t, "test_rollback_forward_recovery")
}

func TestReleaseFreshHandoffReachesMarkets(t *testing.T) {
	runReleaseBoundaryCase(t, "test_fresh_handoff_reaches_markets")
}

func TestReleaseProgrammeDriverEvidence(t *testing.T) {
	runReleaseBoundaryCase(t, "test_programme_driver_consumes_rpc_evidence")
}

func TestReleaseProgrammeMergedScenarios(t *testing.T) {
	runReleaseBoundaryCase(t, "test_programme_merged_scenarios")
}
