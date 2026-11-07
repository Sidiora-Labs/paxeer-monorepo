package layerxcustody_test

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	"github.com/stretchr/testify/require"
)

func TestCustodyProposalIsRefusedFromTheMergePlanHeight(t *testing.T) {
	ctx := activate(inactiveCtx(), 7)
	k := custodyApp.LayerXCustodyKeeper
	handler := layerxcustody.NewProposalHandler(k)
	before := k.GetParams(ctx)
	params := before
	params.DepositRootAuthority = depositRootAuthority()
	require.NotEqual(t, before, params)
	proposal, err := types.NewCustodyProposal("Custody authority", "Update the deposit root authority",
		&types.MsgUpdateParams{Authority: types.GovernanceAuthority(), Params: params})
	require.NoError(t, err)
	require.NoError(t, proposal.ValidateBasic())

	const mergeHeight int64 = 20
	custodyApp.UpgradeKeeper.SetDone(ctx.WithBlockHeight(mergeHeight), types.MergePlanUpgrade)
	require.Equal(t, mergeHeight, custodyApp.UpgradeKeeper.GetDoneHeight(ctx, types.MergePlanUpgrade))

	t.Run("below merge height", func(t *testing.T) {
		early, _ := ctx.WithBlockHeight(mergeHeight - 1).CacheContext()
		require.NoError(t, handler(early, proposal))
		require.Equal(t, params, k.GetParams(early))
	})

	for _, tc := range []struct {
		name   string
		height int64
	}{
		{name: "at merge height", height: mergeHeight},
		{name: "above merge height", height: mergeHeight + 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			retired, _ := ctx.WithBlockHeight(tc.height).CacheContext()
			eventsBefore := len(retired.EventManager().Events())
			require.ErrorIs(t, handler(retired, proposal), types.ErrModuleRetired)
			require.Equal(t, before, k.GetParams(retired))
			require.Len(t, retired.EventManager().Events(), eventsBefore)
		})
	}

	t.Run("retirement precedes validation and unpacking", func(t *testing.T) {
		retired, _ := ctx.WithBlockHeight(mergeHeight).CacheContext()
		invalid := *proposal
		invalid.Title = ""
		require.Error(t, invalid.ValidateBasic())
		require.ErrorIs(t, handler(retired, &invalid), types.ErrModuleRetired)
		invalid.Title = proposal.Title
		invalid.Messages = nil
		require.Error(t, invalid.ValidateBasic())
		require.ErrorIs(t, handler(retired, &invalid), types.ErrModuleRetired)
		require.Equal(t, before, k.GetParams(retired))
	})

	t.Run("retirement precedes governance activation", func(t *testing.T) {
		retired, _ := ctx.CacheContext()
		retired = activate(retired, mergeHeight+1).WithBlockHeight(mergeHeight)
		require.ErrorIs(t, k.GovernanceExecutionActive(retired), types.ErrGovernanceNotActive)
		require.ErrorIs(t, handler(retired, proposal), types.ErrModuleRetired)
		require.Equal(t, before, k.GetParams(retired))
	})
}
