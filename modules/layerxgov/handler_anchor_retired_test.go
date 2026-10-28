package layerxgov_test

import (
	"strconv"
	"testing"
	"time"

	anchortypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	sdkgovtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/stretchr/testify/require"
)

func TestGovRouteRefusesRetiredAnchorParams(t *testing.T) {
	testApp := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, testApp.Close()) })
	ctx := testApp.GetContextForDeliverTx(nil).WithBlockTime(time.Unix(1_800_000_000, 0))
	k := testApp.LayerXAnchorKeeper
	const appliedHeight = int64(100)
	router := sdkgovtypes.NewRouter().AddRoute(govtypes.RouterKey, layerxgov.NewProposalHandler(testApp.MsgServiceRouter()))

	proposal := func(t *testing.T, params anchortypes.Params) *govtypes.LayerXProposal {
		t.Helper()
		p, err := govtypes.NewLayerXProposal("Update anchor params", "Set the anchor quorum through governance", &anchortypes.MsgUpdateParams{
			Authority: govtypes.GovernanceAuthority(), Params: params,
		})
		require.NoError(t, err)
		require.NoError(t, p.ValidateBasic())
		return p
	}
	update := func(t *testing.T, at sdk.Context) {
		t.Helper()
		before := k.GetParams(at)
		require.Equal(t, govtypes.GovernanceAuthority(), before.Authority)
		params := before
		params.Threshold++
		p := proposal(t, params)
		require.NoError(t, router.GetRoute(p.ProposalRoute())(at, p))
		require.NotEqual(t, before, k.GetParams(at))
		require.Equal(t, params, k.GetParams(at))
	}

	require.Zero(t, testApp.UpgradeKeeper.GetDoneHeight(ctx, app.MergeUpgrade))
	t.Run("before_merge_plan", func(t *testing.T) {
		update(t, ctx.WithBlockHeight(appliedHeight))
	})

	testApp.UpgradeKeeper.SetDone(ctx.WithBlockHeight(appliedHeight), app.MergeUpgrade)
	require.Equal(t, appliedHeight, testApp.UpgradeKeeper.GetDoneHeight(ctx, app.MergeUpgrade))
	t.Run("before_applied_height", func(t *testing.T) {
		update(t, ctx.WithBlockHeight(appliedHeight-1))
	})

	for _, height := range []int64{appliedHeight, appliedHeight + 1} {
		t.Run("retired/"+strconv.FormatInt(height, 10), func(t *testing.T) {
			at := ctx.WithBlockHeight(height).WithEventManager(sdk.NewEventManager())
			before := k.GetParams(at)
			params := before
			params.Threshold++
			p := proposal(t, params)
			events := at.EventManager().Events()

			err := router.GetRoute(p.ProposalRoute())(at, p)
			require.ErrorIs(t, err, anchortypes.ErrModuleRetired)
			codespace, code, _ := sdkerrors.ABCIInfo(err, false)
			require.Equal(t, anchortypes.ModuleName, codespace)
			require.Equal(t, uint32(19), code)
			require.Equal(t, before, k.GetParams(at))
			require.Equal(t, events, at.EventManager().Events())
		})
	}
}
