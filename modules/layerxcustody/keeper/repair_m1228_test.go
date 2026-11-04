package keeper_test

import "testing"

func TestRepairM_1228_PointerAliases(t *testing.T) {
	TestSetAssetRejectsPointerAliases(t)
}

func TestRepairM_1228_DefaultAuthorityMerge(t *testing.T) {
	TestMergePlanReleasesCustodyToDefaultGovernance(t)
}

func TestRepairM_1228_ExplicitAuthorityMerge(t *testing.T) {
	TestMergePlanReleasesCustodyToTheAuthority(t)
}

func TestRepairM_1228_RetirementRecovery(t *testing.T) {
	TestGenesisRoundTripAfterRetiredRelease(t)
}
