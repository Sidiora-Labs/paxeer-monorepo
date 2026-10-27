package pricefeed_test

import (
	"encoding/json"
	"fmt"
	"testing"

	layerxgov "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov"
	layerxgovtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed"
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

type Aggregate = types.Aggregate

func MidBlocker(ctx sdk.Context, k keeper.Keeper) {
	pricefeed.MidBlocker(ctx, k)
}

func TestGovernedAppendedMarketAggregates(t *testing.T) {
	testApp, am, k, key, ctx := newModule(t, 20)
	before := types.DefaultParams(layerxgovtypes.GovernanceAuthority())
	require.Len(t, before.Markets, 18)
	require.NoError(t, k.SetParams(ctx, before))
	_, err := k.GetPrice(ctx, 18)
	require.ErrorIs(t, err, types.ErrUnknownMarket)

	existing := len(testApp.StakingKeeper.GetBondedValidatorsByPower(ctx))
	require.LessOrEqual(t, existing, 3)
	for i := existing; i < 3; i++ {
		pubkey := ed25519.GenPrivKeyFromSecret([]byte(fmt.Sprintf("governed-market-validator-%d", i))).PubKey()
		operator := sdk.ValAddress(pubkey.Address())
		bond := sdk.NewCoin(testApp.StakingKeeper.BondDenom(ctx), testApp.StakingKeeper.TokensFromConsensusPower(ctx, 1))
		require.NoError(t, testApp.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(bond)))
		require.NoError(t, testApp.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName,
			sdk.AccAddress(operator), sdk.NewCoins(bond)))
		commission := testApp.StakingKeeper.MinCommissionRate(ctx)
		msg, err := stakingtypes.NewMsgCreateValidator(operator, pubkey, bond, stakingtypes.Description{},
			stakingtypes.NewCommissionRates(commission, commission, sdk.ZeroDec()), sdk.OneInt())
		require.NoError(t, err)
		_, err = stakingkeeper.NewMsgServerImpl(testApp.StakingKeeper).CreateValidator(sdk.WrapSDKContext(ctx), msg)
		require.NoError(t, err)
	}
	_, err = testApp.StakingKeeper.ApplyAndReturnValidatorSetUpdates(ctx)
	require.NoError(t, err)
	validators := testApp.StakingKeeper.GetBondedValidatorsByPower(ctx)
	require.Len(t, validators, 3)

	want := before
	want.Markets = append(append([]types.Market(nil), before.Markets...), types.Market{
		ID: 18, Symbol: "DOGE", ChainflowSymbol: "DOGE", Enabled: true,
		MinPrice: sdk.OneInt(), MaxPrice: sdk.NewIntWithDecimal(1, 30),
	})
	proposal, err := layerxgovtypes.NewLayerXProposal("Append pricefeed market 18", "List DOGE for validator price aggregation",
		&types.MsgSetParams{
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
	require.NoError(t, proposal.ValidateBasic())

	registry := cdctypes.NewInterfaceRegistry()
	am.RegisterInterfaces(registry)
	layerxgovtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	am.RegisterServices(module.NewConfigurator(cdc, router, baseapp.NewGRPCQueryRouter()))
	encoded, err := cdc.Marshal(proposal)
	require.NoError(t, err)
	var decoded layerxgovtypes.LayerXProposal
	require.NoError(t, cdc.Unmarshal(encoded, &decoded))
	require.Equal(t, before, k.GetParams(ctx))
	require.NoError(t, layerxgov.NewProposalHandler(router)(ctx, &decoded))
	require.Equal(t, want, k.GetParams(ctx))
	require.Len(t, types.Markets, 18)
	price, err := k.GetPrice(ctx, 18)
	require.NoError(t, err)
	require.False(t, price.Fresh)
	require.True(t, price.Index.IsZero())
	require.True(t, price.Mark.IsZero())

	server := keeper.NewMsgServerImpl(k)
	for i, validator := range validators {
		require.True(t, validator.IsBonded())
		feedKey := secp256k1.GenPrivKeyFromSecret([]byte(fmt.Sprintf("governed-market-feed-%d", i))).PubKey()
		feeder, _, err := k.RegisterFeedKey(ctx, validator.GetOperator(), feedKey.Bytes())
		require.NoError(t, err)
		index := sdk.NewIntWithDecimal(1000+int64(i), 17)
		mark := index.Add(sdk.NewIntWithDecimal(1, 18))
		sourceMs := uint64(moduleNowMs - 1000 - int64(i)*100)
		response, err := server.SubmitPrices(sdk.WrapSDKContext(ctx), &types.MsgSubmitPrices{
			Feeder: feeder.String(),
			Observations: []types.Observation{{
				MarketId: 18, Sequence: 1, IndexPrice: index.String(), MarkPrice: mark.String(),
				SourceTimestampMs: sourceMs,
			}},
		})
		require.NoError(t, err)
		require.Equal(t, &types.MsgSubmitPricesResponse{Accepted: 1, Refused: 0}, response)
		stored, found := k.GetObservation(ctx, 18, validator.GetOperator())
		require.True(t, found)
		require.Equal(t, types.ObservationRecord{
			Sequence: 1, IndexPrice: index, MarkPrice: mark, SourceTimestampMs: sourceMs, Height: ctx.BlockHeight(),
		}, stored)
	}
	require.Nil(t, ctx.KVStore(key).Get(types.AggregateKey(18)))
	ctx = ctx.WithEventManager(sdk.NewEventManager())

	MidBlocker(ctx, k)

	encoded = ctx.KVStore(key).Get(types.AggregateKey(18))
	require.NotNil(t, encoded)
	var aggregate Aggregate
	require.NoError(t, json.Unmarshal(encoded, &aggregate))
	wantIndex := sdk.NewIntWithDecimal(1001, 17)
	wantMark := sdk.NewIntWithDecimal(1011, 17)
	const wantTimestamp = uint64(moduleNowMs - 1200)
	require.Equal(t, Aggregate{
		IndexPrice: wantIndex, MarkPrice: wantMark, Basis: sdk.NewIntWithDecimal(1, 18),
		TimestampMs: wantTimestamp, Height: 20, Sources: 3,
	}, aggregate)
	price, err = k.GetPrice(ctx, 18)
	require.NoError(t, err)
	require.Equal(t, keeper.Price{
		Index: wantIndex, Mark: wantMark, TimestampMs: wantTimestamp, Height: 20, Sources: 3, Fresh: true,
	}, price)
	written := eventsOf(ctx, types.EventTypeAggregate)
	require.Len(t, written, 1)
	require.Equal(t, "18", attributes(written[0])[types.AttributeMarketID])
	for _, market := range before.Markets {
		require.Nil(t, ctx.KVStore(key).Get(types.AggregateKey(market.ID)))
	}
}
