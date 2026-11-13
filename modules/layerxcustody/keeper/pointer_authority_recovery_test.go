package keeper_test

import "testing"

func TestPointerAliasProtection(t *testing.T) {
	TestSetAssetRejectsPointerAliases(t)
}

func TestDefaultGovernanceAuthorityMerge(t *testing.T) {
	TestMergePlanReleasesCustodyToDefaultGovernance(t)
}

func TestExplicitAuthorityMerge(t *testing.T) {
	TestMergePlanReleasesCustodyToTheAuthority(t)
}

func TestRetirementGenesisRecovery(t *testing.T) {
	TestGenesisRoundTripAfterRetiredRelease(t)
}
