package keeper_test

import (
	"testing"
	"time"

	minttypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/mint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeedkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/ed25519"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keys/secp256k1"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	stakingkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/keeper"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func TestPausedVenueStillLiquidates(t *testing.T) {
	e := newScenarioEnv(t, 1_029_000_000, 512)
	prices := pricefeedkeeper.NewKeeper(e.priceKey, e.app.StakingKeeper, e.app.AccountKeeper, &e.app.EvmKeeper)
	require.NoError(t, prices.SetParams(e.ctx, pricefeedtypes.DefaultParams(pricefeedtypes.DefaultAuthority())))
	consensusKey := ed25519.GenPrivKeyFromSecret([]byte("paused liquidation validator")).PubKey()
	operator := sdk.ValAddress(consensusKey.Address())
	bond := sdk.NewCoin(e.app.StakingKeeper.BondDenom(e.ctx), e.app.StakingKeeper.TokensFromConsensusPower(e.ctx, 1))
	require.NoError(t, e.app.BankKeeper.MintCoins(e.ctx, minttypes.ModuleName, sdk.NewCoins(bond)))
	require.NoError(t, e.app.BankKeeper.SendCoinsFromModuleToAccount(e.ctx, minttypes.ModuleName,
		sdk.AccAddress(operator), sdk.NewCoins(bond)))
	commission := e.app.StakingKeeper.MinCommissionRate(e.ctx)
	createValidator, err := stakingtypes.NewMsgCreateValidator(operator, consensusKey, bond, stakingtypes.Description{},
		stakingtypes.NewCommissionRates(commission, commission, sdk.ZeroDec()), sdk.OneInt())
	require.NoError(t, err)
	_, err = stakingkeeper.NewMsgServerImpl(e.app.StakingKeeper).CreateValidator(sdk.WrapSDKContext(e.ctx), createValidator)
	require.NoError(t, err)
	_, err = e.app.StakingKeeper.ApplyAndReturnValidatorSetUpdates(e.ctx)
	require.NoError(t, err)
	validators := e.app.StakingKeeper.GetBondedValidatorsByPower(e.ctx)
	require.NotEmpty(t, validators)
	feeders := make([]sdk.AccAddress, 0, len(validators))
	for _, validator := range validators {
		operator := validator.GetOperator()
		key := secp256k1.GenPrivKeyFromSecret([]byte("paused liquidation feed " + operator.String()))
		feeder, _, err := prices.RegisterFeedKey(e.ctx, operator, key.PubKey().Bytes())
		require.NoError(t, err)
		feeders = append(feeders, feeder)
	}
	observe := func(usd int64, sequence uint64) {
		t.Helper()
		price := sdk.NewIntWithDecimal(usd, types.USDDecimals)
		for _, feeder := range feeders {
			accepted, refused, err := prices.SubmitPrices(e.ctx, pricefeedtypes.MsgSubmitPrices{
				Feeder: feeder.String(),
				Observations: []pricefeedtypes.Observation{{
					MarketId: 0, IndexPrice: price.String(), MarkPrice: price.String(),
					SourceTimestampMs: uint64(e.ctx.BlockTime().UnixMilli()), Sequence: sequence,
				}},
			})
			require.NoError(t, err)
			require.Equal(t, uint32(1), accepted)
			require.Zero(t, refused)
		}
		prices.Aggregate(e.ctx)
		aggregate, err := prices.GetPrice(e.ctx, 0)
		require.NoError(t, err)
		require.True(t, aggregate.Fresh)
		require.Equal(t, uint32(len(feeders)), aggregate.Sources)
		require.Equal(t, uint64(e.ctx.BlockTime().UnixMilli()), aggregate.TimestampMs)
		require.Equal(t, e.ctx.BlockHeight(), aggregate.Height)
		require.Equal(t, price, aggregate.Index)
		require.Equal(t, price, aggregate.Mark)
	}

	observe(60_000, 1)
	e.pair(invNum(1, 18))
	e.bid(59_000)
	holder := scenarioAccounts[0]
	positionID := *uint256.NewInt(1)
	position := e.position(1)
	require.True(t, position.IsLong)
	require.Equal(t, invNum(1, 18), position.SizeBase)
	summary, err := e.k.AccountMargin(e.ctx, holder)
	require.NoError(t, err)
	require.True(t, summary.Equity.GTE(summary.MaintenanceMargin))
	liquidationPrice, err := e.k.LiquidationPrice(e.ctx, positionID)
	require.NoError(t, err)

	response, err := keeper.NewMsgServerImpl(e.k).SetPaused(sdk.WrapSDKContext(e.ctx), &types.MsgSetPaused{
		Authority: e.k.Authority(), Paused: true,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	params, err := e.k.GetParams(e.ctx)
	require.NoError(t, err)
	require.True(t, params.Paused)
	require.ErrorIs(t, e.k.CheckPause(e.ctx, "openPosition"), types.ErrPaused)

	e.ctx = e.ctx.WithBlockHeight(e.ctx.BlockHeight() + 1).WithBlockTime(e.ctx.BlockTime().Add(time.Second)).
		WithEventManager(sdk.NewEventManager())
	e.app.EvmKeeper.BeginBlock(e.ctx)
	observe(59_000, 2)
	mark := invNum(59_000, 18)
	require.True(t, mark.Lt(&liquidationPrice))

	cacheCtx, _ := e.ctx.CacheContext()
	_, err = e.k.UpdatePrices(cacheCtx)
	require.NoError(t, err)
	summary, err = e.k.AccountMargin(cacheCtx, holder)
	require.NoError(t, err)
	require.True(t, summary.Equity.LT(summary.MaintenanceMargin))
	before, err := e.k.GetMarketState(e.ctx, 0)
	require.NoError(t, err)
	require.Equal(t, invNum(60_000, 18), before.MarkPrice)
	require.Equal(t, position, e.position(1))

	records := e.k.EndBlock(e.ctx)
	for _, event := range e.ctx.EventManager().Events() {
		require.NotEqual(t, keeper.EventTypeEndBlockError, event.Type, "%+v", event)
	}
	state, err := e.k.GetMarketState(e.ctx, 0)
	require.NoError(t, err)
	require.True(t, state.Fresh)
	require.Equal(t, mark, state.MarkPrice)
	require.Equal(t, []types.EventRecord{liquidationEvent(positionID, holder, mark, common.Address{})},
		eventsNamed(records, types.EventLiquidation))
	require.Len(t, eventsNamed(records, types.EventFill), 1)
	require.Nil(t, e.ctx.KVStore(e.key).Get(types.PositionKey(positionID)))
	require.Nil(t, e.ctx.KVStore(e.key).Get(types.AccountMarketPositionKey(holder, 0)))
	account := e.account(holder)
	require.Zero(t, account.PositionCount)
	require.Equal(t, *uint256.NewInt(2_450_000), account.Collateral)
	params, err = e.k.GetParams(e.ctx)
	require.NoError(t, err)
	require.True(t, params.Paused)
}
