package chainconfig

import "testing"

func TestBridgeBoundaryTests(t *testing.T) {
	testCapDestinationBoundaries(t)
}

func TestSolanaMintIdentity(t *testing.T) {
	testAssetAddressIdentities(t, KindSolana)
}

func TestEVMAddressIdentity(t *testing.T) {
	testAssetAddressIdentities(t, KindEVM)
}
