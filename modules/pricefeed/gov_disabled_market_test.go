package pricefeed_test

import (
	"strconv"
	"testing"

	layerxgov "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov"
	layerxgovtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	cdctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func GetPrice(t *testing.T, k keeper.Keeper, ctx sdk.Context, marketID uint64) keeper.Price {
	t.Helper()
	price, err := k.GetPrice(ctx, marketID)
	require.NoError(t, err)
	return price
}

func TestGovernedDisabledMarketStalesPrice(t *testing.T) {
	testApp, am, k, key, ctx := newModule(t, 10)
	params := types.DefaultParams(layerxgovtypes.GovernanceAuthority())
	const solID, btcID = uint64(2), uint64(0)
	require.Equal(t, "SOL", params.Markets[solID].Symbol)
	require.Equal(t, "BTC", params.Markets[btcID].Symbol)
	require.True(t, params.Markets[solID].Enabled)
	require.NoError(t, k.SetParams(ctx, params))

	pubkey := ed25519.GenPrivKeyFromSecret([]byte("governed-disabled-market-validator")).PubKey()
	operator := sdk.ValAddress(pubkey.Address())
	bond := sdk.NewCoin(testApp.StakingKeeper.BondDenom(ctx), testApp.StakingKeeper.TokensFromConsensusPower(ctx, 1))
	require.NoError(t, testApp.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(bond)))
	require.NoError(t, testApp.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName,
		sdk.AccAddress(operator), sdk.NewCoins(bond)))
	commission := testApp.StakingKeeper.MinCommissionRate(ctx)
	create, err := stakingtypes.NewMsgCreateValidator(operator, pubkey, bond, stakingtypes.Description{},
		stakingtypes.NewCommissionRates(commission, commission, sdk.ZeroDec()), sdk.OneInt())
	require.NoError(t, err)
	_, err = stakingkeeper.NewMsgServerImpl(testApp.StakingKeeper).CreateValidator(sdk.WrapSDKContext(ctx), create)
	require.NoError(t, err)
	_, err = testApp.StakingKeeper.ApplyAndReturnValidatorSetUpdates(ctx)
	require.NoError(t, err)

	validators := testApp.StakingKeeper.GetBondedValidatorsByPower(ctx)
	require.NotEmpty(t, validators)
	feeders := make([]sdk.AccAddress, len(validators))
	observations := make([]types.ObservationRecord, len(validators))
	for i, validator := range validators {
		feedkey := secp256k1.GenPrivKeyFromSecret([]byte("governed-disabled-market-feed-" + validator.GetOperator().String()))
		feeders[i], _, err = k.RegisterFeedKey(ctx, validator.GetOperator(), feedkey.PubKey().Bytes())
		require.NoError(t, err)
	}

	server := keeper.NewMsgServerImpl(k)
	submit := func(feeder sdk.AccAddress, sequence uint64) *types.MsgSubmitPricesResponse {
		t.Helper()
		price := sdk.NewIntWithDecimal(150+int64(sequence), 18).String()
		response, err := server.SubmitPrices(sdk.WrapSDKContext(ctx), &types.MsgSubmitPrices{
			Feeder: feeder.String(),
			Observations: []types.Observation{
				{MarketId: solID, Sequence: sequence, IndexPrice: price, MarkPrice: price, SourceTimestampMs: uint64(moduleNowMs)},
				{MarketId: btcID, Sequence: sequence, IndexPrice: price, MarkPrice: price, SourceTimestampMs: uint64(moduleNowMs)},
			},
		})
		require.NoError(t, err)
		return response
	}
	for i, feeder := range feeders {
		response := submit(feeder, 1)
		require.Equal(t, uint32(2), response.Accepted)
		require.Zero(t, response.Refused)
		record, found := k.GetObservation(ctx, solID, validators[i].GetOperator())
		require.True(t, found)
		require.Equal(t, uint64(1), record.Sequence)
		observations[i] = record
	}
	MidBlocker(ctx, k)
	before := GetPrice(t, k, ctx, solID)
	require.True(t, before.Fresh)
	require.Equal(t, sdk.NewIntWithDecimal(151, 18), before.Index)
	require.Equal(t, before.Index, before.Mark)
	require.Equal(t, uint64(moduleNowMs), before.TimestampMs)
	require.Equal(t, ctx.BlockHeight(), before.Height)
	require.Equal(t, uint32(len(validators)), before.Sources)
	stored := append([]byte(nil), ctx.KVStore(key).Get(types.AggregateKey(solID))...)
	require.NotEmpty(t, stored)

	registry := cdctypes.NewInterfaceRegistry()
	am.RegisterInterfaces(registry)
	layerxgovtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	am.RegisterServices(module.NewConfigurator(cdc, router, baseapp.NewGRPCQueryRouter()))
	require.NotNil(t, router.HandlerByTypeURL(sdk.MsgTypeURL(&types.MsgSetParams{})))

	want := params
	want.Markets = append([]types.Market(nil), params.Markets...)
	want.Markets[solID].Enabled = false
	proposal, err := layerxgovtypes.NewLayerXProposal("Disable SOL pricefeed", "Stop SOL observations and aggregation", &types.MsgSetParams{
		Authority:                    want.Authority,
		Markets:                      want.Markets,
		StalenessWindowMs:            want.StalenessWindowMs,
		MaxClockSkewMs:               want.MaxClockSkewMs,
		MinSources:                   want.MinSources,
		MaxDivergenceBps:             want.MaxDivergenceBps,
		MaxDeviationBps:              want.MaxDeviationBps,
		MaxObservationsPerSubmission: want.MaxObservationsPerSubmission,
		MaxSubmissionBytes:           want.MaxSubmissionBytes,
		MarkEmaTauMs:                 want.MarkEmaTauMs,
		MaxMarkBasisBps:              want.MaxMarkBasisBps,
		MaxPriceAgeMs:                want.MaxPriceAgeMs,
	})
	require.NoError(t, err)
	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1).WithEventManager(sdk.NewEventManager())
	require.NoError(t, layerxgov.NewProposalHandler(router)(ctx, proposal))
	require.Equal(t, want, k.GetParams(ctx))
	require.False(t, k.GetParams(ctx).Markets[solID].Enabled)
	require.Equal(t, moduleNowMs, ctx.BlockTime().UnixMilli())

	for i, feeder := range feeders {
		response := submit(feeder, 2)
		require.Equal(t, uint32(1), response.Accepted)
		require.Equal(t, uint32(1), response.Refused)
		record, found := k.GetObservation(ctx, solID, validators[i].GetOperator())
		require.True(t, found)
		require.Equal(t, observations[i], record)
		control, found := k.GetObservation(ctx, btcID, validators[i].GetOperator())
		require.True(t, found)
		require.Equal(t, uint64(2), control.Sequence)
		require.Equal(t, ctx.BlockHeight(), control.Height)
	}
	refused := eventsOf(ctx, types.EventTypeObservationRefused)
	require.Len(t, refused, len(validators))
	for i, event := range refused {
		require.Equal(t, map[string]string{
			types.AttributeValidator: validators[i].GetOperator().String(),
			types.AttributeMarketID:  strconv.FormatUint(solID, 10),
			types.AttributeSequence:  "2",
			types.AttributeReason:    types.RefusalMarketDisabled,
		}, attributes(event))
	}

	MidBlocker(ctx, k)
	require.Equal(t, stored, ctx.KVStore(key).Get(types.AggregateKey(solID)))
	after := GetPrice(t, k, ctx, solID)
	require.False(t, after.Fresh)
	expected := before
	expected.Fresh = false
	require.Equal(t, expected, after)
	control := GetPrice(t, k, ctx, btcID)
	require.True(t, control.Fresh)
	require.Equal(t, sdk.NewIntWithDecimal(152, 18), control.Index)
	require.Equal(t, ctx.BlockHeight(), control.Height)
	require.Equal(t, uint32(len(validators)), control.Sources)
	written := eventsOf(ctx, types.EventTypeAggregate)
	require.Len(t, written, 1)
	require.Equal(t, strconv.FormatUint(btcID, 10), attributes(written[0])[types.AttributeMarketID])
	for _, event := range eventsOf(ctx, types.EventTypeAggregateSkipped) {
		require.NotEqual(t, strconv.FormatUint(solID, 10), attributes(event)[types.AttributeMarketID])
	}
}
