package types_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/stretchr/testify/require"
)

func TestModuleRetiredErrorIsCode29OfTheCustodyCodespace(t *testing.T) {
	require.Equal(t, types.ModuleName, types.ErrModuleRetired.Codespace())
	require.Equal(t, uint32(29), types.ErrModuleRetired.ABCICode())

	otherCodes := map[string]uint32{
		"ErrInvalidHex":          types.ErrInvalidHex.ABCICode(),
		"ErrInvalidParams":       types.ErrInvalidParams.ABCICode(),
		"ErrInvalidAsset":        types.ErrInvalidAsset.ABCICode(),
		"ErrUnknownAsset":        types.ErrUnknownAsset.ABCICode(),
		"ErrAssetDisabled":       types.ErrAssetDisabled.ABCICode(),
		"ErrInvalidDeposit":      types.ErrInvalidDeposit.ABCICode(),
		"ErrInvalidClaim":        types.ErrInvalidClaim.ABCICode(),
		"ErrNullifierUsed":       types.ErrNullifierUsed.ABCICode(),
		"ErrClaimNotReady":       types.ErrClaimNotReady.ABCICode(),
		"ErrClaimNotPending":     types.ErrClaimNotPending.ABCICode(),
		"ErrNotFinalized":        types.ErrNotFinalized.ABCICode(),
		"ErrNotAuthorized":       types.ErrNotAuthorized.ABCICode(),
		"ErrExitNotEligible":     types.ErrExitNotEligible.ABCICode(),
		"ErrExitConsumed":        types.ErrExitConsumed.ABCICode(),
		"ErrInvalidExit":         types.ErrInvalidExit.ABCICode(),
		"ErrInvalidRelease":      types.ErrInvalidRelease.ABCICode(),
		"ErrInvalidCheckpoint":   types.ErrInvalidCheckpoint.ABCICode(),
		"ErrInvalidGenesis":      types.ErrInvalidGenesis.ABCICode(),
		"ErrWrongNetwork":        types.ErrWrongNetwork.ABCICode(),
		"ErrInvalidProof":        types.ErrInvalidProof.ABCICode(),
		"ErrInvalidDepositRoot":  types.ErrInvalidDepositRoot.ABCICode(),
		"ErrDepositRootProposer": types.ErrDepositRootProposer.ABCICode(),
		"ErrDepositRootExists":   types.ErrDepositRootExists.ABCICode(),
		"ErrGovernanceNotActive": types.ErrGovernanceNotActive.ABCICode(),
		"ErrExitCheckpoint":      types.ErrExitCheckpoint.ABCICode(),
		"ErrExitFrozen":          types.ErrExitFrozen.ABCICode(),
		"ErrDepositFrozen":       types.ErrDepositFrozen.ABCICode(),
	}
	for name, code := range otherCodes {
		require.NotEqual(t, uint32(29), code, "%s also uses the retired-module code", name)
	}
}

func TestMergePlanUpgradeIsTheCommonMergePlan(t *testing.T) {
	require.Equal(t, pcommon.MergeUpgrade, types.MergePlanUpgrade)
}
