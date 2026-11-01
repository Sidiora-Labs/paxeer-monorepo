package keeper_test

import (
	"math/big"
	"testing"
	"time"

	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeedkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	pricefeedtestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/testutil"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/stretchr/testify/require"
)

func TestListedMarketPricedAtEndBlock(t *testing.T) {
	testApp := app.Setup(t, false, false, false)
	ctx := testApp.GetContextForDeliverTx([]byte{}).WithBlockHeight(10).
		WithBlockTime(time.Unix(listingTime, 0).UTC()).WithEventManager(sdk.NewEventManager())
	ctx = ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx))
	feed, ctx := pricefeedtestutil.NewKeeper(testApp, ctx)
	perpsKey := sdk.NewKVStoreKey(types.StoreKey)
	ctx = pricefeedtestutil.WithPriceFeedStore(ctx, perpsKey)
	k := keeper.NewKeeper(perpsKey, &testApp.EvmKeeper, feed)
	params := pricefeedtypes.DefaultParams(pricefeedtypes.DefaultAuthority())
	require.NoError(t, feed.SetParams(ctx, params))
	require.Len(t, params.Markets, 18)
	_, err := feed.GetPrice(ctx, 18)
	require.ErrorIs(t, err, pricefeedtypes.ErrUnknownMarket)
	_, err = k.GetMarket(ctx, 18)
	require.ErrorIs(t, err, types.ErrUnknownMarket)

	params.Markets = append(params.Markets, pricefeedtypes.Market{
		ID: 18, Symbol: "NEW", ChainflowSymbol: "NEW", MinPrice: sdk.OneInt(),
		MaxPrice: sdk.NewIntWithDecimal(1, 30), Enabled: true,
	})
	response, err := pricefeedkeeper.NewMsgServerImpl(feed).SetParams(sdk.WrapSDKContext(ctx), &pricefeedtypes.MsgSetParams{
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
	require.NoError(t, err)
	require.NotNil(t, response)
	require.Equal(t, params, feed.GetParams(ctx))

	market := classEMarket(18, params.Markets[18].Symbol)
	market.Symbol = "NEW-PERP"
	market.OiCapUsd = sdk.ZeroInt()
	listed, err := keeper.NewMsgServerImpl(k).SetMarket(sdk.WrapSDKContext(ctx), &types.MsgSetMarket{
		Authority: k.Authority(), Market: market,
	})
	require.NoError(t, err)
	require.NotNil(t, listed)
	stored, err := k.GetMarket(ctx, market.ID)
	require.NoError(t, err)
	market.ListedAt = uint64(ctx.BlockTime().Unix())
	require.Equal(t, market, stored)
	require.Nil(t, ctx.KVStore(perpsKey).Get(types.MarketStateKey(market.ID)))

	pubkey := ed25519.GenPrivKeyFromSecret([]byte("listed market consensus")).PubKey()
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
	require.Len(t, testApp.StakingKeeper.GetBondedValidatorsByPower(ctx), 1)
	feeder, _, err := feed.RegisterFeedKey(ctx, operator,
		secp256k1.GenPrivKeyFromSecret([]byte("listed market feed")).PubKey().Bytes())
	require.NoError(t, err)

	ctx = ctx.WithBlockHeight(ctx.BlockHeight() + 1).WithBlockTime(ctx.BlockTime().Add(time.Second)).
		WithEventManager(sdk.NewEventManager())
	index := sdk.NewIntWithDecimal(1234, types.USDDecimals)
	mark := sdk.NewIntWithDecimal(1235, types.USDDecimals)
	timestamp := uint64(ctx.BlockTime().UnixMilli() - 500)
	submitted, err := pricefeedkeeper.NewMsgServerImpl(feed).SubmitPrices(sdk.WrapSDKContext(ctx), &pricefeedtypes.MsgSubmitPrices{
		Feeder: feeder.String(), Observations: []pricefeedtypes.Observation{{
			MarketId: market.ID, Sequence: 1, IndexPrice: index.String(), MarkPrice: mark.String(),
			SourceTimestampMs: timestamp,
		}},
	})
	require.NoError(t, err)
	require.NotNil(t, submitted)
	require.Equal(t, uint32(1), submitted.Accepted)
	require.Zero(t, submitted.Refused)
	feed.Aggregate(ctx)
	price, err := feed.GetPrice(ctx, market.ID)
	require.NoError(t, err)
	require.True(t, price.Fresh)
	require.Equal(t, uint32(1), price.Sources)
	require.Equal(t, ctx.BlockHeight(), price.Height)
	require.Equal(t, timestamp, price.TimestampMs)
	require.Equal(t, index, price.Index)
	require.Equal(t, mark, price.Mark)
	require.Nil(t, ctx.KVStore(perpsKey).Get(types.MarketStateKey(market.ID)))

	records := k.EndBlock(ctx)
	for _, event := range ctx.EventManager().Events() {
		require.NotEqual(t, keeper.EventTypeEndBlockError, event.Type, "%+v", event)
	}
	require.NotEmpty(t, ctx.KVStore(perpsKey).Get(types.MarketStateKey(market.ID)))
	state, err := k.GetMarketState(ctx, market.ID)
	require.NoError(t, err)
	require.Equal(t, market.ID, state.MarketID)
	require.Equal(t, price.Index.BigInt(), state.IndexPrice.ToBig())
	require.Equal(t, price.Mark.BigInt(), state.PricefeedMark.ToBig())
	require.Equal(t, price.TimestampMs, state.PriceTimestampMs)
	require.True(t, state.Fresh)
	require.Equal(t, index.BigInt(), state.MarkPrice.ToBig())
	require.True(t, state.PremiumEma.IsZero())
	require.Equal(t, []types.EventRecord{{
		Name: types.EventMarkPriceUpdated,
		Args: []any{new(big.Int).SetUint64(market.ID), index.BigInt(), index.BigInt(), index.BigInt()},
	}}, eventsNamed(records, types.EventMarkPriceUpdated))
}
