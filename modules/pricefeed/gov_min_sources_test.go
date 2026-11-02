package pricefeed_test

import (
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

func TestGovernedMinSourcesSkipsAggregation(t *testing.T) {
	testApp, am, k, key, ctx := newModule(t, 10)
	params := types.DefaultParams(layerxgovtypes.GovernanceAuthority())
	params.Markets = params.Markets[:1]
	require.True(t, params.Markets[0].Enabled)
	SetParams(t, k, ctx, params)
	require.Equal(t, uint32(1), k.GetParams(ctx).MinSources)

	existing := len(testApp.StakingKeeper.GetBondedValidatorsByPower(ctx))
	require.LessOrEqual(t, existing, 2)
	for i := existing; i < 2; i++ {
		pubkey := ed25519.GenPrivKeyFromSecret([]byte{'g', 'o', 'v', byte(i)}).PubKey()
		operator := sdk.ValAddress(pubkey.Address())
		bond := sdk.NewCoin(testApp.StakingKeeper.BondDenom(ctx),
			testApp.StakingKeeper.TokensFromConsensusPower(ctx, 1))
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
	_, err := testApp.StakingKeeper.ApplyAndReturnValidatorSetUpdates(ctx)
	require.NoError(t, err)
	validators := testApp.StakingKeeper.GetBondedValidatorsByPower(ctx)
	require.Len(t, validators, 2)

	registry := cdctypes.NewInterfaceRegistry()
	am.RegisterInterfaces(registry)
	layerxgovtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	am.RegisterServices(module.NewConfigurator(cdc, router, baseapp.NewGRPCQueryRouter()))
	require.NotNil(t, router.HandlerByTypeURL(sdk.MsgTypeURL(&types.MsgSetParams{})))

	want := params
	want.MinSources = 3
	proposal, err := layerxgovtypes.NewLayerXProposal("Raise pricefeed minimum sources",
		"Require three sources for pricefeed aggregation", &types.MsgSetParams{
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
	require.NoError(t, layerxgov.NewProposalHandler(router)(ctx, proposal))
	require.Equal(t, want, k.GetParams(ctx))

	server := keeper.NewMsgServerImpl(k)
	const price = "65000000000000000000000"
	const sourceMs = uint64(moduleNowMs - 1_000)
	for i, validator := range validators {
		operator := validator.GetOperator()
		pubkey := secp256k1.GenPrivKeyFromSecret([]byte{'f', 'e', 'e', 'd', byte(i)}).PubKey()
		feeder, _, err := k.RegisterFeedKey(ctx, operator, pubkey.Bytes())
		require.NoError(t, err)
		encoded, err := sdk.Bech32ifyAddressBytes(types.AccountAddressPrefix, feeder)
		require.NoError(t, err)
		response, err := server.SubmitPrices(sdk.WrapSDKContext(ctx), &types.MsgSubmitPrices{
			Feeder: encoded,
			Observations: []types.Observation{{
				MarketId: 0, Sequence: 1, IndexPrice: price, MarkPrice: price, SourceTimestampMs: sourceMs,
			}},
		})
		require.NoError(t, err)
		require.Equal(t, &types.MsgSubmitPricesResponse{Accepted: 1, Refused: 0}, response)
		record, found := k.GetObservation(ctx, 0, operator)
		require.True(t, found)
		require.Equal(t, price, record.IndexPrice.String())
		require.Equal(t, price, record.MarkPrice.String())
		require.Equal(t, sourceMs, record.SourceTimestampMs)
		require.Equal(t, ctx.BlockHeight(), record.Height)
	}
	require.Nil(t, ctx.KVStore(key).Get(types.AggregateKey(0)))

	ctx = ctx.WithEventManager(sdk.NewEventManager())
	emitSkipped(t, k, ctx)
	require.Empty(t, eventsOf(ctx, types.EventTypeAggregate))
	require.Nil(t, ctx.KVStore(key).Get(types.AggregateKey(0)))
}

func emitSkipped(t *testing.T, k keeper.Keeper, ctx sdk.Context) {
	t.Helper()
	k.Aggregate(ctx)
	require.Equal(t, sdk.Events{sdk.NewEvent("pricefeed_aggregate_skipped",
		sdk.NewAttribute(types.AttributeMarketID, "0"),
		sdk.NewAttribute(types.AttributeSources, "2"),
		sdk.NewAttribute(types.AttributeRequired, "3"),
	)}, ctx.EventManager().Events())
}
