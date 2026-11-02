package pricefeed_test

import (
	"testing"

	layerxgov "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov"
	layerxgovtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/gov/types"
	"github.com/stretchr/testify/require"
)

func SetParams(t *testing.T, k keeper.Keeper, ctx sdk.Context, params types.Params) {
	t.Helper()
	require.NoError(t, k.SetParams(ctx, params))
	require.Equal(t, params, k.GetParams(ctx))
}

func NewLayerXProposal(params types.Params) (*layerxgovtypes.LayerXProposal, error) {
	return layerxgovtypes.NewLayerXProposal("Update pricefeed sources", "Require two sources for pricefeed aggregation", &types.MsgSetParams{
		Authority:                    params.Authority,
		Markets:                      params.Markets,
		StalenessWindowMs:            params.StalenessWindowMs,
		MaxClockSkewMs:               params.MaxClockSkewMs,
		MinSources:                   params.MinSources,
		MaxDivergenceBps:             params.MaxDivergenceBps,
		MaxDeviationBps:              params.MaxDeviationBps,
		MaxObservationsPerSubmission: params.MaxObservationsPerSubmission,
		MaxSubmissionBytes:           params.MaxSubmissionBytes,
		MarkEmaTauMs:                 params.MarkEmaTauMs,
		MaxMarkBasisBps:              params.MaxMarkBasisBps,
		MaxPriceAgeMs:                params.MaxPriceAgeMs,
	})
}

func TestLayerXProposalRoutesPricefeedSetParams(t *testing.T) {
	_, am, k, _, ctx := newModule(t, 1)
	authority := layerxgovtypes.GovernanceAuthority()
	before := types.DefaultParams(authority)
	SetParams(t, k, ctx, before)
	require.Equal(t, uint32(1), before.MinSources)

	registry := cdctypes.NewInterfaceRegistry()
	am.RegisterInterfaces(registry)
	layerxgovtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	am.RegisterServices(module.NewConfigurator(cdc, router, baseapp.NewGRPCQueryRouter()))
	require.NotNil(t, router.HandlerByTypeURL(sdk.MsgTypeURL(&types.MsgSetParams{})))

	want := before
	want.MinSources = 2
	proposal, err := NewLayerXProposal(want)
	require.NoError(t, err)
	require.NoError(t, proposal.ValidateBasic())
	legacyJSON, err := govtypes.ModuleCdc.MarshalAsJSON(proposal)
	require.NoError(t, err)
	require.Contains(t, string(legacyJSON), `"pricefeed/MsgSetParams"`)

	encoded, err := cdc.Marshal(proposal)
	require.NoError(t, err)
	var decoded layerxgovtypes.LayerXProposal
	require.NoError(t, cdc.Unmarshal(encoded, &decoded))
	require.NoError(t, decoded.ValidateBasic())
	messages, err := decoded.GetMessages()
	require.NoError(t, err)
	require.Len(t, messages, 1)
	msg, ok := messages[0].(*types.MsgSetParams)
	require.True(t, ok)
	require.Equal(t, authority, msg.Authority)
	require.Equal(t, uint32(2), msg.MinSources)
	require.Equal(t, before, k.GetParams(ctx))

	require.NoError(t, layerxgov.NewProposalHandler(router)(ctx, &decoded))
	got := k.GetParams(ctx)
	require.Equal(t, uint32(2), got.MinSources)
	require.Equal(t, want, got)
	events := eventsOf(ctx, types.EventTypeParams)
	require.Len(t, events, 1)
	require.Equal(t, map[string]string{types.AttributeAuthority: authority}, attributes(events[0]))
}
