package tx

import "testing"

func TestRepair_303_GasEstimateStaleAuthInfo(t *testing.T) {
	testBuilderGasEstimateRoundTrip(t)
}

func TestRepair_303_BlockTxReversePagination(t *testing.T) {
	testGetBlockWithTxsReversePagination(t)
}
