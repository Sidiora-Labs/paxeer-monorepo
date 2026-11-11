package keeper_test

import (
	"strconv"
	"testing"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func launchChainEnv(t *testing.T) (*custodyEnv, sdk.Int) {
	t.Helper()
	c := newCustodyEnv(t)
	c.fund(recordedCreator, 2*creationFee)
	app := testkeeper.EVMTestApp
	params := app.LayerXChainsKeeper.GetParams(c.ctx)
	value := params.MinBond.Add(params.CreationFee)
	coins := sdk.NewCoins(sdk.NewCoin(app.EvmKeeper.GetBaseDenom(c.ctx), value.MulRaw(2)))
	require.NoError(t, app.BankKeeper.MintCoins(c.ctx, evmtypes.ModuleName, coins))
	payer := app.EvmKeeper.GetPaxAddressOrDefault(c.ctx, recordedCreator)
	require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(c.ctx, evmtypes.ModuleName, payer, coins))
	c.ctx = c.ctx.WithEventManager(sdk.NewEventManager())
	return c, value
}

func launchChain(t *testing.T, c *custodyEnv, chainName string, share uint16, value sdk.Int) (types.Market, types.LaunchedChain, error) {
	t.Helper()
	params := testkeeper.EVMTestApp.LayerXChainsKeeper.GetParams(c.ctx)
	ctx, market, record, err := c.k.LaunchChain(c.ctx, c.caller, nil, recordedCreator,
		recordedName, recordedSymbol, types.FeeStrategyClaim, common.Address{}, chainName,
		params.MinGasAllowance, params.MinByteAllowance, share, value)
	if err == nil {
		c.ctx = ctx
	}
	return market, record, err
}

func TestLaunchChainCreatesMarketChainAndBinding(t *testing.T) {
	c, value := launchChainEnv(t)
	app := testkeeper.EVMTestApp
	nextID := app.LayerXChainsKeeper.NextID(c.ctx)
	market, record, err := launchChain(t, c, "market-home", 100, value)
	require.NoError(t, err)
	require.Equal(t, uint64(nextID), record.RegistryNumber)
	require.Equal(t, chainstypes.ChainIDOf(nextID).Uint64(), record.ChainID)
	require.Equal(t, market.PoolAddress, record.Pool)
	require.Equal(t, market.TokenAddress, record.Token)
	require.Equal(t, market.FeeRightID, record.FeeRightID)
	require.Equal(t, recordedCreator.Hex(), record.Creator)
	require.Equal(t, c.ctx.BlockHeight(), record.LaunchBlock)
	stored, found, err := c.k.GetLaunchedChain(c.ctx, record.RegistryNumber)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, record, stored)
	pool := common.HexToAddress(market.PoolAddress)
	indexed, found := c.k.RegistryNumberByPool(c.ctx, pool)
	require.True(t, found)
	require.Equal(t, record.RegistryNumber, indexed)
	require.Equal(t, record.ChainID, c.k.HomeChainByPool(c.ctx, pool))
	_, found = c.k.GetMarket(c.ctx, market.Denom)
	require.True(t, found)
	domain, found := app.LayerXChainsKeeper.GetDomain(c.ctx, nextID)
	require.True(t, found)
	require.Equal(t, "market-home", domain.Name)
	require.Equal(t, common.BytesToAddress(types.ChainRevenueAccount(uint64(nextID))), domain.Operator)
	require.True(t, domain.Bond.Equal(app.LayerXChainsKeeper.GetParams(c.ctx).MinBond))
	var marketEvents, chainEvents int
	for _, event := range c.ctx.EventManager().Events() {
		switch event.Type {
		case types.EventTypeMarketCreated:
			marketEvents++
		case types.EventTypeChainLaunched:
			chainEvents++
			require.Contains(t, event.Attributes, sdk.NewAttribute("pool", record.Pool).ToKVPair())
			require.Contains(t, event.Attributes, sdk.NewAttribute("chain_id", strconv.FormatUint(record.ChainID, 10)).ToKVPair())
		}
	}
	require.Equal(t, 1, marketEvents)
	require.Equal(t, 1, chainEvents)
}

func TestLaunchChainRefusedChainLeavesNoMarket(t *testing.T) {
	for _, test := range []struct {
		name    string
		prepare func(*testing.T, *custodyEnv, sdk.Int)
		value   func(sdk.Int) sdk.Int
		share   func(chainstypes.Params) uint16
	}{
		{name: "taken name", prepare: func(t *testing.T, c *custodyEnv, value sdk.Int) {
			app := testkeeper.EVMTestApp
			params := app.LayerXChainsKeeper.GetParams(c.ctx)
			payer := app.EvmKeeper.GetPaxAddressOrDefault(c.ctx, recordedCreator)
			_, err := app.LayerXChainsKeeper.CreateWithPayer(c.ctx, payer, recordedCreator, "market-home",
				params.MinGasAllowance, params.MinByteAllowance, 100, value, evmtypes.DefaultMinFeePerGas)
			require.NoError(t, err)
		}},
		{name: "bond below minimum", value: func(value sdk.Int) sdk.Int { return value.SubRaw(1) }},
		{name: "fee share above maximum", share: func(params chainstypes.Params) uint16 { return params.MaxFeeShareBps + 1 }},
	} {
		t.Run(test.name, func(t *testing.T) {
			c, value := launchChainEnv(t)
			if test.prepare != nil {
				test.prepare(t, c, value)
			}
			app := testkeeper.EVMTestApp
			beforeID := app.LayerXChainsKeeper.NextID(c.ctx)
			beforeQuote := c.balanceOf(recordedCreator)
			beforeTreasury := c.ledger(keeper.TreasuryHeldLedger())
			share := uint16(100)
			if test.share != nil {
				share = test.share(app.LayerXChainsKeeper.GetParams(c.ctx))
			}
			if test.value != nil {
				value = test.value(value)
			}
			_, _, err := launchChain(t, c, "market-home", share, value)
			require.Error(t, err)
			require.Zero(t, c.k.GetMarketCount(c.ctx))
			require.Equal(t, beforeQuote, c.balanceOf(recordedCreator))
			require.Equal(t, beforeTreasury, c.ledger(keeper.TreasuryHeldLedger()))
			require.Equal(t, beforeID, app.LayerXChainsKeeper.NextID(c.ctx))
			for _, event := range c.ctx.EventManager().Events() {
				require.NotEqual(t, types.EventTypeMarketCreated, event.Type)
				require.NotEqual(t, types.EventTypeChainLaunched, event.Type)
			}
		})
	}
}

func TestLaunchChainRefusedMarketLeavesNoChain(t *testing.T) {
	c, value := launchChainEnv(t)
	app := testkeeper.EVMTestApp
	beforeID := app.LayerXChainsKeeper.NextID(c.ctx)
	beforeQuote := c.balanceOf(recordedCreator)
	beforeTreasury := c.ledger(keeper.TreasuryHeldLedger())
	ctx, _, _, err := c.k.LaunchChain(c.ctx, c.caller, nil, recordedCreator,
		recordedName, recordedSymbol, types.FeeStrategy(4), common.Address{}, "market-home",
		app.LayerXChainsKeeper.GetParams(c.ctx).MinGasAllowance,
		app.LayerXChainsKeeper.GetParams(c.ctx).MinByteAllowance, 100, value)
	require.Error(t, err)
	require.Zero(t, c.k.GetMarketCount(ctx))
	require.Equal(t, beforeID, app.LayerXChainsKeeper.NextID(ctx))
	_, found := app.LayerXChainsKeeper.GetDomain(ctx, beforeID)
	require.False(t, found)
	require.Equal(t, beforeQuote, c.balanceOf(recordedCreator))
	require.Equal(t, beforeTreasury, c.ledger(keeper.TreasuryHeldLedger()))
}

func TestLaunchChainChargesMatchCreateMarket(t *testing.T) {
	launched, value := launchChainEnv(t)
	created, _ := launchChainEnv(t)
	app := testkeeper.EVMTestApp
	denom := app.EvmKeeper.GetBaseDenom(launched.ctx)
	payer := app.EvmKeeper.GetPaxAddressOrDefault(launched.ctx, recordedCreator)
	beforeQuote := launched.balanceOf(recordedCreator)
	beforeBank := app.BankKeeper.GetBalance(launched.ctx, payer, denom).Amount
	market, record, err := launchChain(t, launched, "market-home", 100, value)
	require.NoError(t, err)
	launchQuoteCharge := beforeQuote - launched.balanceOf(recordedCreator)
	launchTreasury := launched.ledger(keeper.TreasuryHeldLedger())
	launchBankCharge := beforeBank.Sub(app.BankKeeper.GetBalance(launched.ctx, payer, denom).Amount)
	marketQuoteBefore := created.balanceOf(recordedCreator)
	ctx, directMarket, err := created.k.CreateMarket(created.ctx, created.caller, nil, recordedCreator,
		recordedName, recordedSymbol, types.FeeStrategyClaim, common.Address{})
	require.NoError(t, err)
	created.ctx = ctx
	requireSameMarket(t, directMarket, market)
	require.Equal(t, marketQuoteBefore-created.balanceOf(recordedCreator), launchQuoteCharge)
	require.Equal(t, created.ledger(keeper.TreasuryHeldLedger()), launchTreasury)
	require.Equal(t, int64(creationFee), launchQuoteCharge)
	require.True(t, launchBankCharge.Equal(value))
	require.Equal(t, record.ChainID, launched.k.HomeChainByPool(launched.ctx, common.HexToAddress(market.PoolAddress)))
}
