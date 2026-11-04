package pricefeed_test

import (
	"encoding/json"
	"strconv"
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
	"github.com/stretchr/testify/require"
)

func TestRepairM_480_ModuleEvents(t *testing.T) {
	TestAppModule(t)
}

func TestRepairM_480_UnequalPowerQuorum(t *testing.T) {
	for _, tc := range []struct {
		name       string
		reporters  []int
		power      int64
		minSources uint32
		aggregates bool
	}{
		{name: "exactly_two_thirds_refused", reporters: []int{0}, power: 6, minSources: 1},
		{name: "headcount_majority_below_two_thirds_refused", reporters: []int{1, 2}, power: 3, minSources: 2},
		{name: "above_two_thirds_accepted", reporters: []int{0, 2}, power: 7, minSources: 2, aggregates: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			am, k, key, ctx, operators := newQuorumModule(t)
			params := types.DefaultParams(types.DefaultAuthority())
			params.Markets = params.Markets[:1]
			params.MinSources = tc.minSources
			require.NoError(t, k.SetParams(ctx, params))
			var reporters []sdk.ValAddress
			var reportingPower int64
			for _, i := range tc.reporters {
				reporters = append(reporters, operators[i])
				reportingPower += []int64{6, 2, 1}[i]
			}
			require.Equal(t, tc.power, reportingPower)
			require.GreaterOrEqual(t, uint32(len(reporters)), params.MinSources)
			require.Equal(t, tc.aggregates, reportingPower*3 > 9*2)
			submitModulePrices(t, k, ctx, reporters)
			ctx = ctx.WithEventManager(sdk.NewEventManager())
			am.MidBlock(ctx, ctx.BlockHeight())
			if tc.aggregates {
				assertRepairM480Aggregate(t, k, key, ctx, uint32(len(reporters)))
			} else {
				assertRepairM480Skipped(t, k, ctx, len(reporters))
				require.Nil(t, ctx.KVStore(key).Get(types.AggregateKey(0)))
				price, err := keeper.NewQueryServerImpl(k).Price(sdk.WrapSDKContext(ctx), &types.QueryPriceRequest{MarketId: 0})
				require.NoError(t, err)
				require.Equal(t, &types.QueryPriceResponse{Index: "0", Mark: "0"}, price)
			}
		})
	}
}

func TestRepairM_480_GovernedMinSources(t *testing.T) {
	am, k, key, ctx, operators := newQuorumModule(t)
	params := types.DefaultParams(layerxgovtypes.GovernanceAuthority())
	params.Markets = params.Markets[:1]
	require.Equal(t, uint32(1), params.MinSources)
	require.NoError(t, k.SetParams(ctx, params))
	submitModulePrices(t, k, ctx, []sdk.ValAddress{operators[0], operators[2]})
	ctx = ctx.WithEventManager(sdk.NewEventManager())
	am.MidBlock(ctx, ctx.BlockHeight())
	assertRepairM480Aggregate(t, k, key, ctx, 2)
	before := append([]byte(nil), ctx.KVStore(key).Get(types.AggregateKey(0))...)

	registry := cdctypes.NewInterfaceRegistry()
	am.RegisterInterfaces(registry)
	layerxgovtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	am.RegisterServices(module.NewConfigurator(cdc, router, baseapp.NewGRPCQueryRouter()))

	want := params
	want.MinSources = 3
	proposal, err := NewLayerXProposal(want)
	require.NoError(t, err)
	require.NoError(t, proposal.ValidateBasic())
	encoded, err := cdc.Marshal(proposal)
	require.NoError(t, err)
	var decoded layerxgovtypes.LayerXProposal
	require.NoError(t, cdc.Unmarshal(encoded, &decoded))
	require.NoError(t, decoded.ValidateBasic())
	ctx = ctx.WithBlockHeight(11).WithEventManager(sdk.NewEventManager())
	require.NoError(t, layerxgov.NewProposalHandler(router)(ctx, &decoded))
	require.Equal(t, want, k.GetParams(ctx))
	paramsEvents := eventsOf(ctx, types.EventTypeParams)
	require.Len(t, paramsEvents, 1)
	require.Equal(t, map[string]string{types.AttributeAuthority: want.Authority}, attributes(paramsEvents[0]))

	ctx = ctx.WithEventManager(sdk.NewEventManager())
	am.MidBlock(ctx, ctx.BlockHeight())
	assertRepairM480Skipped(t, k, ctx, 2)
	require.Equal(t, before, ctx.KVStore(key).Get(types.AggregateKey(0)))
	price, err := keeper.NewQueryServerImpl(k).Price(sdk.WrapSDKContext(ctx), &types.QueryPriceRequest{MarketId: 0})
	require.NoError(t, err)
	require.Equal(t, &types.QueryPriceResponse{
		Index: "64250000000000000000000", Mark: "64254000000000000000000",
		TimestampMs: uint64(moduleNowMs - 1_000), Height: 10, Sources: 2, Fresh: true,
	}, price)

	ctx = ctx.WithEventManager(sdk.NewEventManager())
	submitModulePrices(t, k, ctx, []sdk.ValAddress{operators[1]})
	ctx = ctx.WithEventManager(sdk.NewEventManager())
	am.MidBlock(ctx, ctx.BlockHeight())
	assertRepairM480Aggregate(t, k, key, ctx, 3)
}

func assertRepairM480Skipped(t *testing.T, k keeper.Keeper, ctx sdk.Context, sources int) {
	t.Helper()
	require.Empty(t, eventsOf(ctx, types.EventTypeAggregate))
	skipped := eventsOf(ctx, types.EventTypeAggregateSkipped)
	require.Len(t, skipped, 1)
	require.Equal(t, map[string]string{
		types.AttributeMarketID: "0",
		types.AttributeSources:  strconv.Itoa(sources),
		types.AttributeRequired: strconv.FormatUint(uint64(k.GetParams(ctx).MinSources), 10),
	}, attributes(skipped[0]))
}

func assertRepairM480Aggregate(t *testing.T, k keeper.Keeper, key sdk.StoreKey, ctx sdk.Context, sources uint32) {
	t.Helper()
	var aggregate types.Aggregate
	stored := ctx.KVStore(key).Get(types.AggregateKey(0))
	require.NotNil(t, stored)
	require.NoError(t, json.Unmarshal(stored, &aggregate))
	require.Equal(t, types.Aggregate{
		IndexPrice: sdk.NewIntWithDecimal(64250, 18), MarkPrice: sdk.NewIntWithDecimal(64254, 18),
		Basis: sdk.NewIntWithDecimal(4, 18), TimestampMs: uint64(moduleNowMs - 1_000),
		Height: ctx.BlockHeight(), Sources: sources,
	}, aggregate)
	require.Empty(t, eventsOf(ctx, types.EventTypeAggregateSkipped))
	written := eventsOf(ctx, types.EventTypeAggregate)
	require.Len(t, written, 1)
	require.Equal(t, map[string]string{
		types.AttributeMarketID:    "0",
		types.AttributeIndexPrice:  "64250000000000000000000",
		types.AttributeMarkPrice:   "64254000000000000000000",
		types.AttributeBasis:       "4000000000000000000",
		types.AttributeTimestampMs: strconv.FormatUint(uint64(moduleNowMs-1_000), 10),
		types.AttributeSources:     strconv.FormatUint(uint64(sources), 10),
		types.AttributeRequired:    strconv.FormatUint(uint64(k.GetParams(ctx).MinSources), 10),
		types.AttributeHeight:      strconv.FormatInt(ctx.BlockHeight(), 10),
	}, attributes(written[0]))
	price, err := keeper.NewQueryServerImpl(k).Price(sdk.WrapSDKContext(ctx), &types.QueryPriceRequest{MarketId: 0})
	require.NoError(t, err)
	require.Equal(t, &types.QueryPriceResponse{
		Index: "64250000000000000000000", Mark: "64254000000000000000000",
		TimestampMs: uint64(moduleNowMs - 1_000), Height: ctx.BlockHeight(), Sources: sources, Fresh: true,
	}, price)
}
