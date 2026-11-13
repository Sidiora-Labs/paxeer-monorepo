package keeper_test

import "testing"

func TestAuthoritySignedTransactionRenunciation(t *testing.T) {
	TestAdminSignedTransactionRenunciation(t)
}

func TestMintBlockedDestinationAtomic(t *testing.T) {
	TestProofIssuerMintBlockedDestinationAtomic(t)
}

func TestTokenfactoryGenesisStreamRecovery(t *testing.T) {
	TestProofGenesisTokenfactoryStreamDiskRecovery(t)
}
