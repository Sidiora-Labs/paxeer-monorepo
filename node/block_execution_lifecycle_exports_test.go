package app

import "testing"

// These bridges let the external proof suite exercise package-private fixtures.
func CheckBenchmarkOutputAndShutdown(t *testing.T) { TestInitBenchmark_Success(t) }
func CheckGigaActivationSets(t *testing.T) {
	TestGigaCustomPrecompilesSwitchAtHeight(t)
	TestGigaFailFastCoversMergeAddresses(t)
}
