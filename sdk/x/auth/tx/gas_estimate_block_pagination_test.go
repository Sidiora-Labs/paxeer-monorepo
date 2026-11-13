package tx

import "testing"

func TestGasEstimateStaleAuthInfo(t *testing.T) {
	testBuilderGasEstimateRoundTrip(t)
}

func TestBlockTxReversePagination(t *testing.T) {
	testGetBlockWithTxsReversePagination(t)
}
