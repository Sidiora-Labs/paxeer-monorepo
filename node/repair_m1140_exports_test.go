package app

import "testing"

// These bridges let the external proof suite exercise package-private fixtures.
func RepairM1140Benchmark(t *testing.T) { TestInitBenchmark_Success(t) }
func RepairM1140GigaSets(t *testing.T) {
	TestGigaCustomPrecompilesSwitchAtHeight(t)
	TestGigaFailFastCoversMergeAddresses(t)
}
