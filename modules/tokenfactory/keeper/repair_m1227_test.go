package keeper_test

import "testing"

func TestRepairM_1227_ChangeAdminSignedTransaction(t *testing.T) {
	TestAdminSignedTransactionRenunciation(t)
}

func TestRepairM_1227_MintBlockedDestinationAtomic(t *testing.T) {
	TestProofIssuerMintBlockedDestinationAtomic(t)
}

func TestRepairM_1227_TokenfactoryGenesisStreamRecovery(t *testing.T) {
	TestProofGenesisTokenfactoryStreamDiskRecovery(t)
}
