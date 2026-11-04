package chainconfig

import "testing"

func TestRepairM_336_BridgeBoundaryTests(t *testing.T) {
	testCapDestinationBoundaries(t)
}

func TestRepairM_336_SolanaMintCase(t *testing.T) {
	testAssetAddressIdentities(t, KindSolana)
}

func TestRepairM_336_EVMAddressIdentity(t *testing.T) {
	testAssetAddressIdentities(t, KindEVM)
}
