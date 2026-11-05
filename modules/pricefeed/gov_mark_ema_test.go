package pricefeed_test

import (
	"testing"
	"time"

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

const MarkEmaTauMs uint64 = 1000

func TestGovernedMarkEmaTauChangesMark(t *testing.T) {
	testApp, am, k, _, ctx := newModule(t, 10)
	before := types.DefaultParams(layerxgovtypes.GovernanceAuthority())
	require.Equal(t, uint64(150000), before.MarkEmaTauMs)
	require.NoError(t, k.SetParams(ctx, before))

	pubkey := ed25519.GenPrivKeyFromSecret([]byte("governed-mark-ema-validator")).PubKey()
	operator := sdk.ValAddress(pubkey.Address())
	bond := sdk.NewCoin(testApp.StakingKeeper.BondDenom(ctx), testApp.StakingKeeper.TokensFromConsensusPower(ctx, 1))
	require.NoError(t, testApp.BankKeeper.MintCoins(ctx, minttypes.ModuleName, sdk.NewCoins(bond)))
	require.NoError(t, testApp.BankKeeper.SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName,
		sdk.AccAddress(operator), sdk.NewCoins(bond)))
	commission := testApp.StakingKeeper.MinCommissionRate(ctx)
	createValidator, err := stakingtypes.NewMsgCreateValidator(operator, pubkey, bond, stakingtypes.Description{},
		stakingtypes.NewCommissionRates(commission, commission, sdk.ZeroDec()), sdk.OneInt())
	require.NoError(t, err)
	_, err = stakingkeeper.NewMsgServerImpl(testApp.StakingKeeper).CreateValidator(sdk.WrapSDKContext(ctx), createValidator)
	require.NoError(t, err)
	_, err = testApp.StakingKeeper.ApplyAndReturnValidatorSetUpdates(ctx)
	require.NoError(t, err)

	validators := testApp.StakingKeeper.GetBondedValidatorsByPower(ctx)
	require.NotEmpty(t, validators)
	feeders := make([]sdk.AccAddress, 0, len(validators))
	for _, validator := range validators {
		feedKey := secp256k1.GenPrivKeyFromSecret(append([]byte("governed-mark-ema-feed-"), validator.GetOperator()...))
		feeder, _, err := k.RegisterFeedKey(ctx, validator.GetOperator(), feedKey.PubKey().Bytes())
		require.NoError(t, err)
		feeders = append(feeders, feeder)
	}

	server := keeper.NewMsgServerImpl(k)
	submit := func(block sdk.Context, sequence uint64, index, mark sdk.Int) {
		t.Helper()
		for _, feeder := range feeders {
			response, err := server.SubmitPrices(sdk.WrapSDKContext(block), &types.MsgSubmitPrices{
				Feeder: feeder.String(),
				Observations: []types.Observation{{
					MarketId: 0, Sequence: sequence, IndexPrice: index.String(), MarkPrice: mark.String(),
					SourceTimestampMs: uint64(block.BlockTime().UnixMilli()),
				}},
			})
			require.NoError(t, err)
			require.Equal(t, uint32(1), response.Accepted)
			require.Zero(t, response.Refused)
		}
	}

	initialIndex := sdk.NewIntWithDecimal(1000, 18)
	initialMark := initialIndex
	submit(ctx, 1, initialIndex, initialMark)
	pricefeed.MidBlocker(ctx, k)
	initial, err := k.GetPrice(ctx, 0)
	require.NoError(t, err)
	require.True(t, initial.Fresh)
	require.True(t, initial.Index.Equal(initialIndex))
	require.True(t, initial.Mark.Equal(initialMark))
	require.Equal(t, uint64(moduleNowMs), initial.TimestampMs)
	require.Equal(t, uint32(len(feeders)), initial.Sources)

	next := ctx.WithBlockHeight(ctx.BlockHeight() + 1).WithBlockTime(ctx.BlockTime().Add(time.Second))
	baseline, _ := next.CacheContext()
	governed, _ := next.CacheContext()
	registry := cdctypes.NewInterfaceRegistry()
	am.RegisterInterfaces(registry)
	layerxgovtypes.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)
	router := baseapp.NewMsgServiceRouter()
	router.SetInterfaceRegistry(registry)
	am.RegisterServices(module.NewConfigurator(cdc, router, baseapp.NewGRPCQueryRouter()))

	want := before
	want.MarkEmaTauMs = MarkEmaTauMs
	require.Less(t, want.MarkEmaTauMs, before.MarkEmaTauMs)
	proposal, err := layerxgovtypes.NewLayerXProposal("Reduce mark EMA tau", "Move the next mark closer to the observed mark median", &types.MsgSetParams{
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
	require.NoError(t, layerxgov.NewProposalHandler(router)(governed, proposal))
	require.Equal(t, want, k.GetParams(governed))
	require.Equal(t, before, k.GetParams(baseline))
	unchanged, err := k.GetPrice(governed, 0)
	require.NoError(t, err)
	require.Equal(t, initial, unchanged)

	nextIndex := initialIndex.MulRaw(110).QuoRaw(100)
	nextMark := initialMark
	for _, block := range []sdk.Context{baseline, governed} {
		submit(block, 2, nextIndex, nextMark)
		pricefeed.MidBlocker(block, k)
		price, err := k.GetPrice(block, 0)
		require.NoError(t, err)
		require.True(t, price.Fresh)
		require.True(t, price.Index.Equal(nextIndex))
		require.Equal(t, uint64(moduleNowMs+1000), price.TimestampMs)
		require.Equal(t, next.BlockHeight(), price.Height)
		require.Equal(t, uint32(len(feeders)), price.Sources)
	}

	defaultPrice, err := k.GetPrice(baseline, 0)
	require.NoError(t, err)
	governedPrice, err := k.GetPrice(governed, 0)
	require.NoError(t, err)
	wantDefaultMark := nextIndex.Sub(sdk.NewIntWithDecimal(100, 18).MulRaw(1000).QuoRaw(151000))
	require.True(t, defaultPrice.Mark.Equal(wantDefaultMark), "default mark = %s, want %s", defaultPrice.Mark, wantDefaultMark)
	require.True(t, governedPrice.Mark.Equal(sdk.NewIntWithDecimal(1050, 18)), "governed mark = %s", governedPrice.Mark)
	require.True(t, governedPrice.Mark.LT(nextIndex))
	require.True(t, nextIndex.Sub(governedPrice.Mark).Abs().GT(nextIndex.Sub(defaultPrice.Mark).Abs()),
		"governed mark %s must be farther from index %s than default mark %s", governedPrice.Mark, nextIndex, defaultPrice.Mark)
	require.True(t, governedPrice.Mark.Sub(nextMark).Abs().LT(defaultPrice.Mark.Sub(nextMark).Abs()),
		"governed mark %s must be closer to observed mark median %s than default mark %s", governedPrice.Mark, nextMark, defaultPrice.Mark)
	require.True(t, governedPrice.Mark.LT(defaultPrice.Mark), "lower tau must move the mark further downward")
}
