// Proves: layerxanchor.NewAppModule / AppModule.RegisterServices -> module.Configurator / baseapp.MsgServiceRouter -> node.App / layerxgov.NewProposalHandler
//
// Repairs node/app.go: New omitted the anchor service when removing the retired
// module from the module manager, so governance saw an unknown route instead of
// the live upgrade reader's result. Services remain registered independently of
// the deleted module's genesis, export and migration lifecycle. Registration
// has no state to reopen: consecutive calls use freshly decoded producer output
// and changing keeper state, including a merge applied after service creation.
package layerxanchor_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxanchor/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/stretchr/testify/require"
)

func TestProofRetirementAppConsumer(t *testing.T) {
	a := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil).WithBlockHeight(10).WithBlockTime(time.Unix(1_800_000_000, 0))
	k := a.LayerXAnchorKeeper
	upgrades := &a.UpgradeKeeper
	const mergeHeight = int64(100)

	require.Equal(t, app.MergeUpgrade, types.MergeUpgrade)
	require.Equal(t, app.AnchorStoreDeleteUpgrade, types.StoreDeleteUpgrade)
	require.NotEqual(t, types.MergeUpgrade, types.StoreDeleteUpgrade)
	// These are the routes installed by node.New, not a replacement router or
	// a direct keeper call that would bypass NewAppModule/RegisterServices.
	handler := a.MsgServiceRouter().Handler(&types.MsgUpdateParams{})
	require.NotNil(t, handler, "node/app.go: New must register the anchor retirement service")
	govern := a.GovKeeper.Router().GetRoute(govtypes.RouterKey)
	require.NotNil(t, govern)
	cdc := a.AppCodec()

	proposal := func(params types.Params) *govtypes.LayerXProposal {
		t.Helper()
		produced, err := govtypes.NewLayerXProposal("Update anchor quorum", "Exercise the real anchor retirement route", &types.MsgUpdateParams{
			Authority: params.Authority, Params: params,
		})
		require.NoError(t, err)
		wire, err := cdc.Marshal(produced)
		require.NoError(t, err)
		var decoded govtypes.LayerXProposal
		require.NoError(t, cdc.Unmarshal(wire, &decoded))
		require.NoError(t, decoded.ValidateBasic())
		return &decoded
	}
	update := func(at sdk.Context) {
		t.Helper()
		before := k.GetParams(at)
		params := before
		params.Threshold++
		require.NoError(t, govern(at, proposal(params)))
		require.Equal(t, params, k.GetParams(at))
		require.NotEqual(t, before, k.GetParams(at))
		require.Equal(t, types.EventParamsUpdated, at.EventManager().Events()[len(at.EventManager().Events())-1].Type)
	}
	refuse := func(at sdk.Context) {
		t.Helper()
		before := k.ExportGenesis(at)
		events := append(sdk.Events(nil), at.EventManager().Events()...)
		params := before.Params
		params.Threshold++
		p := proposal(params)
		// Both the application service consumer and its governance caller must
		// observe the refusal, with no state or event effects.
		result, err := handler(at, &types.MsgUpdateParams{Authority: params.Authority, Params: params})
		require.ErrorIs(t, err, types.ErrModuleRetired)
		require.Nil(t, result)
		require.ErrorIs(t, govern(at, p), types.ErrModuleRetired)
		require.Equal(t, before, k.ExportGenesis(at))
		require.Equal(t, events, at.EventManager().Events())
	}

	for i := 0; i < 2; i++ {
		update(ctx)
	}
	// A completed store-delete name or a merely scheduled merge cannot be
	// mistaken for the applied merge that controls this service.
	upgrades.SetDone(ctx, types.StoreDeleteUpgrade)
	require.NoError(t, upgrades.ScheduleUpgrade(ctx, upgradetypes.Plan{Name: types.MergeUpgrade, Height: mergeHeight}))
	for i := 0; i < 2; i++ {
		require.False(t, keeper.MergeRetired(ctx.WithBlockHeight(mergeHeight), upgrades))
		update(ctx.WithBlockHeight(mergeHeight))
	}

	t.Run("wrong_authority", func(t *testing.T) {
		before := k.ExportGenesis(ctx)
		params := before.Params
		params.Threshold++
		other := sdk.AccAddress(make([]byte, 20)).String()
		require.NotEqual(t, params.Authority, other)
		msg := &types.MsgUpdateParams{Authority: other, Params: params}
		require.NoError(t, msg.ValidateBasic())
		result, err := handler(ctx, msg)
		require.ErrorIs(t, err, types.ErrUnauthorized)
		require.Nil(t, result)
		require.Equal(t, before, k.ExportGenesis(ctx))
	})

	t.Run("malformed_proposal_output", func(t *testing.T) {
		before := k.ExportGenesis(ctx)
		produced := proposal(before.Params)
		wire, err := cdc.Marshal(produced)
		require.NoError(t, err)
		require.NotEmpty(t, wire)
		// Truncate actual producer bytes inside the last length-delimited
		// field; do not construct a substitute message or decoder.
		var decoded govtypes.LayerXProposal
		require.Error(t, cdc.Unmarshal(wire[:len(wire)-1], &decoded))
		require.Equal(t, before, k.ExportGenesis(ctx))
	})

	upgrades.SetDone(ctx.WithBlockHeight(mergeHeight), types.MergeUpgrade)
	require.Equal(t, mergeHeight, upgrades.GetDoneHeight(ctx, types.MergeUpgrade))
	for i := 0; i < 2; i++ {
		require.True(t, keeper.MergeRetired(ctx.WithBlockHeight(mergeHeight), upgrades))
		refuse(ctx.WithBlockHeight(mergeHeight))
		refuse(ctx.WithBlockHeight(mergeHeight + 1))
		// Reusing the very same registered service below the applied height
		// rules out caching either the active or retired result.
		update(ctx.WithBlockHeight(mergeHeight - 1))
	}

	// Once retired, the registered route needs only the upgrade store. It
	// must refuse before attempting to decode any deleted anchor state.
	t.Run("retired_before_anchor_state_decode", func(t *testing.T) {
		at, _ := ctx.WithBlockHeight(mergeHeight).CacheContext()
		at.KVStore(a.GetKey(types.StoreKey)).Set(types.ParamsKey, []byte("{"))
		params := k.GetParams(ctx)
		for i := 0; i < 2; i++ {
			result, err := handler(at, &types.MsgUpdateParams{Authority: params.Authority, Params: params})
			require.ErrorIs(t, err, types.ErrModuleRetired)
			require.Nil(t, result)
			require.ErrorIs(t, govern(at, proposal(params)), types.ErrModuleRetired)
		}
	})
}

func TestProofRetirementGenesisConsumerDecoding(t *testing.T) {
	a := app.Setup(t, false, false, false)
	t.Cleanup(func() { require.NoError(t, a.Close()) })
	ctx := a.GetContextForDeliverTx(nil)
	producer := layerxanchor.NewAppModule(a.LayerXAnchorKeeper, &a.UpgradeKeeper)
	encoding := app.MakeEncodingConfig()
	consumer, ok := app.ModuleBasics[types.ModuleName]
	require.True(t, ok, "node/app.go: ModuleBasics must retain the anchor decoder")
	for i := 0; i < 2; i++ {
		params := a.LayerXAnchorKeeper.GetParams(ctx)
		params.Threshold++
		require.NoError(t, a.LayerXAnchorKeeper.SetParams(ctx, params))
		wire := producer.ExportGenesis(ctx, a.AppCodec())
		// Use the anchor decoder registered in the node's basic manager.
		require.NoError(t, consumer.ValidateGenesis(a.AppCodec(), encoding.TxConfig, wire))
		var observed types.GenesisState
		require.NoError(t, json.Unmarshal(wire, &observed))
		require.Equal(t, params, observed.Params)
		err := consumer.ValidateGenesis(a.AppCodec(), encoding.TxConfig, wire[:len(wire)-1])
		require.Error(t, err)
		require.Contains(t, err.Error(), "failed to unmarshal layerxanchor genesis state")
	}
	// A well-formed transport with invalid params must also be refused by
	// the registered application handler before any keeper state changes.
	before := a.LayerXAnchorKeeper.ExportGenesis(ctx)
	params := before.Params
	params.Threshold = 0
	result, err := a.MsgServiceRouter().Handler(&types.MsgUpdateParams{})(ctx, &types.MsgUpdateParams{Authority: params.Authority, Params: params})
	require.ErrorIs(t, err, types.ErrParams)
	require.Nil(t, result)
	require.Equal(t, before, a.LayerXAnchorKeeper.ExportGenesis(ctx))
}
