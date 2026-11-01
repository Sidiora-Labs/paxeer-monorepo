package keeper_test

import (
	"testing"

	custodytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxcustody/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

type SetUpgradeKeeper interface {
	SetUpgradeKeeper(types.UpgradeKeeper)
}

func TestAppExchangeRetiredAfterV6120(t *testing.T) {
	app := testkeeper.EVMTestApp
	var _ SetUpgradeKeeper = app.LayerXExchangeKeeper
	ctx, _ := app.GetContextForDeliverTx(nil).WithBlockHeight(2).CacheContext()
	ctx = ctx.WithEventManager(sdk.NewEventManager())
	require.Equal(t, int64(1), app.UpgradeKeeper.GetDoneHeight(ctx, types.RetiredFromUpgrade))

	market := types.Market{
		MarketId:      custodytypes.Hash32([32]byte{1}),
		MarginAssetId: custodytypes.Hash32([32]byte{2}),
		Enabled:       true,
	}
	params := types.DefaultParams()
	params.Authority = govtypes.GovernanceAuthority()
	params.Markets = []types.Market{market}
	require.NoError(t, app.LayerXExchangeKeeper.SetParams(ctx, params))
	before := app.LayerXExchangeKeeper.GetParams(ctx)
	require.Equal(t, params, before)

	market.Enabled = false
	proposal, err := govtypes.NewLayerXProposal(
		"Disable exchange market", "Disable a listed market after v6.12.0.",
		&types.MsgSetMarket{Authority: params.Authority, Market: market},
	)
	require.NoError(t, err)
	encoded, err := app.AppCodec().Marshal(proposal)
	require.NoError(t, err)
	var decoded govtypes.LayerXProposal
	require.NoError(t, app.AppCodec().Unmarshal(encoded, &decoded))
	require.NoError(t, decoded.ValidateBasic())
	msgs, err := decoded.GetMessages()
	require.NoError(t, err)
	require.Equal(t, []sdk.Msg{&types.MsgSetMarket{Authority: params.Authority, Market: market}}, msgs)

	err = layerxgov.NewProposalHandler(app.MsgServiceRouter())(ctx, &decoded)
	require.ErrorIs(t, err, types.ErrRetired)
	require.Equal(t, "LXP_EXCHANGE_RETIRED", types.ErrorCode(err))
	require.Equal(t, before, app.LayerXExchangeKeeper.GetParams(ctx))
	require.Empty(t, ctx.EventManager().Events())
}
