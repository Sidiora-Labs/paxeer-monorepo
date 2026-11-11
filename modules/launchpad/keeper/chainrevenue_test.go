package keeper_test

import (
	"strconv"
	"testing"
	"time"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	chainstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

type chainRevenueFixture struct {
	*custodyEnv
	chain uint64
	pool  common.Address
	token common.Address
	base  string
}

func newChainRevenueFixture(t *testing.T, holdings ...holding) *chainRevenueFixture {
	t.Helper()
	c := newCustodyEnv(t)
	pool := common.HexToAddress(generation2Market(c.k.Generation2().SidioraNFT, 1, nil).PoolAddress)
	h := c.ctx.BlockHeight() + 1
	c.height(h)
	token := c.poolToken("Revenue Token", holdings...)
	market := c.airdropMarket(token, 0, 0, 0)
	chain := uint64(testkeeper.EVMTestApp.LayerXChainsKeeper.NextID(c.ctx))
	require.NoError(t, c.k.SetLaunchedChain(c.ctx, types.LaunchedChain{
		RegistryNumber: chain, ChainID: 190, Pool: market.PoolAddress, Token: market.TokenAddress,
		FeeRightID: 1, Creator: recordedCreator.Hex(), LaunchBlock: h,
	}))
	c.height(h + 10)
	return &chainRevenueFixture{custodyEnv: c, chain: chain, pool: pool, token: token,
		base: testkeeper.EVMTestApp.EvmKeeper.GetBaseDenom(c.ctx)}
}

func (f *chainRevenueFixture) fund(t *testing.T, coins sdk.Coins) {
	t.Helper()
	bank := testkeeper.EVMTestApp.BankKeeper
	require.NoError(t, bank.MintCoins(f.ctx, evmtypes.ModuleName, coins))
	require.NoError(t, bank.SendCoinsFromModuleToAccount(f.ctx, evmtypes.ModuleName,
		types.ChainRevenueAccount(f.chain), coins))
}

func (f *chainRevenueFixture) open(t *testing.T) types.ChainRevenueEpoch {
	t.Helper()
	record, err := f.k.OpenChainRevenueEpoch(f.ctx, f.caller, f.chain)
	require.NoError(t, err)
	return record
}

func (f *chainRevenueFixture) bankBalance(holder common.Address, denom string) sdk.Int {
	account := testkeeper.EVMTestApp.EvmKeeper.GetPaxAddressOrDefault(f.ctx, holder)
	return testkeeper.EVMTestApp.BankKeeper.GetBalance(f.ctx, account, denom).Amount
}

func TestChainRevenueEpochOpensWithSnapshotAndEligibleSupply(t *testing.T) {
	pool := common.HexToAddress(generation2Market(common.Address{}, 1, nil).PoolAddress)
	f := newChainRevenueFixture(t, holding{pool, 6}, holding{common.HexToAddress(types.DeadAddress), 2},
		holding{address("revenue holder A"), 3}, holding{address("revenue holder B"), 4})
	f.fund(t, sdk.NewCoins(sdk.NewInt64Coin(f.base, 101)))
	record := f.open(t)
	require.Equal(t, uint64(1), record.Epoch)
	require.Equal(t, uint64(f.ctx.BlockHeight()-1), record.SnapshotBlock)
	require.Equal(t, sdk.NewInt(7), record.EligibleSupply)
	require.Equal(t, sdk.NewInt(101), record.Amount)
	require.Equal(t, f.ctx.BlockTime().Unix(), record.OpenedAt)
	require.Equal(t, sdk.ZeroInt(), f.bankBalance(common.BytesToAddress(types.ChainRevenueAccount(f.chain)), f.base))
	require.Equal(t, sdk.NewInt(101), testkeeper.EVMTestApp.BankKeeper.GetBalance(f.ctx, f.k.ModuleAddress(), f.base).Amount)
	require.Equal(t, uint64(1), f.k.LastChainRevenueEpoch(f.ctx, f.chain))
	stored, found, err := f.k.GetChainRevenueEpoch(f.ctx, f.chain, 1)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, record, stored)
	events := eventsOf(f.ctx, types.EventTypeChainRevenueEpochOpened)
	require.Len(t, events, 1)
	require.Equal(t, strconv.FormatUint(f.chain, 10), events[0]["chain"])
	require.Equal(t, "1", events[0][types.AttributeKeyEpoch])
	require.Equal(t, "101", events[0][types.AttributeKeyAmount])
	require.Equal(t, "11", events[0]["snapshot_block"])
	require.Equal(t, "7", events[0]["eligible_supply"])
}

func TestChainRevenueEpochRefusals(t *testing.T) {
	f := newChainRevenueFixture(t, holding{address("revenue holder"), 3})
	_, err := f.k.OpenChainRevenueEpoch(f.ctx, f.caller, f.chain+1)
	require.ErrorIs(t, err, types.ErrNotLaunchedChain)
	_, err = f.k.OpenChainRevenueEpoch(f.ctx, f.caller, f.chain)
	require.ErrorIs(t, err, types.ErrNothingToDistribute)
	f.fund(t, sdk.NewCoins(sdk.NewInt64Coin(f.base, 10)))
	f.open(t)
	f.fund(t, sdk.NewCoins(sdk.NewInt64Coin(f.base, 5)))
	_, err = f.k.OpenChainRevenueEpoch(f.ctx, f.caller, f.chain)
	require.ErrorIs(t, err, types.ErrEpochTooEarly)
	f.ctx = f.ctx.WithBlockTime(f.ctx.BlockTime().Add(time.Hour))
	second := f.open(t)
	require.Equal(t, uint64(2), second.Epoch)
	require.Equal(t, sdk.NewInt(5), second.Amount)

	zero := newChainRevenueFixture(t, holding{common.HexToAddress(generation2Market(common.Address{}, 1, nil).PoolAddress), 1})
	zero.fund(t, sdk.NewCoins(sdk.NewInt64Coin(zero.base, 10)))
	_, err = zero.k.OpenChainRevenueEpoch(zero.ctx, zero.caller, zero.chain)
	require.ErrorIs(t, err, types.ErrNothingToDistribute)
}

func TestChainRevenueClaimsSumToTheEpochAmount(t *testing.T) {
	holders := []common.Address{address("revenue A"), address("revenue B"), address("revenue C")}
	f := newChainRevenueFixture(t, holding{holders[0], 1}, holding{holders[1], 1}, holding{holders[2], 1})
	f.fund(t, sdk.NewCoins(sdk.NewInt64Coin(f.base, 100)))
	f.open(t)
	var total sdk.Int = sdk.ZeroInt()
	for n, holder := range holders {
		claimable, err := f.k.ChainRevenueClaimable(f.ctx, f.caller, f.chain, 1, holder)
		require.NoError(t, err)
		ctx, paid, err := f.k.ClaimChainRevenue(f.ctx, f.caller, f.chain, 1, holder)
		require.NoError(t, err)
		f.ctx = ctx
		require.Equal(t, claimable, paid)
		want := sdk.NewInt(33)
		if n == 2 {
			want = sdk.NewInt(34)
		}
		require.Equal(t, want, paid)
		require.Equal(t, want, f.bankBalance(holder, f.base))
		total = total.Add(paid)
		require.True(t, f.k.HasChainRevenueClaim(f.ctx, f.chain, 1, holder))
		updated, found, err := f.k.GetChainRevenueEpoch(f.ctx, f.chain, 1)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, total, updated.ClaimedTotal)
	}
	require.Equal(t, sdk.NewInt(100), total)
	require.Equal(t, sdk.ZeroInt(), testkeeper.EVMTestApp.BankKeeper.GetBalance(f.ctx, f.k.ModuleAddress(), f.base).Amount)
	record, found, err := f.k.GetChainRevenueEpoch(f.ctx, f.chain, 1)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, record.Amount, record.ClaimedTotal)
	require.Len(t, eventsOf(f.ctx, types.EventTypeChainRevenueClaimed), 3)
}

func TestChainRevenueSecondClaimIsRefused(t *testing.T) {
	holder := address("revenue claimant")
	f := newChainRevenueFixture(t, holding{holder, 1})
	f.fund(t, sdk.NewCoins(sdk.NewInt64Coin(f.base, 7)))
	f.open(t)
	_, amount, err := f.k.ClaimChainRevenue(f.ctx, f.caller, f.chain, 1, holder)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(7), amount)
	_, err = f.k.ChainRevenueClaimable(f.ctx, f.caller, f.chain, 1, holder)
	require.ErrorIs(t, err, types.ErrAlreadyClaimed)
	_, _, err = f.k.ClaimChainRevenue(f.ctx, f.caller, f.chain, 1, holder)
	require.ErrorIs(t, err, types.ErrAlreadyClaimed)
	require.Equal(t, sdk.NewInt(7), f.bankBalance(holder, f.base))
	f.ctx = f.ctx.WithBlockTime(f.ctx.BlockTime().Add(time.Hour))
	f.fund(t, sdk.NewCoins(sdk.NewInt64Coin(f.base, 5)))
	f.open(t)
	_, _, err = f.k.ClaimChainRevenueBatch(f.ctx, f.caller, f.chain, holder, []uint64{2, 1})
	require.ErrorIs(t, err, types.ErrAlreadyClaimed)
	require.False(t, f.k.HasChainRevenueClaim(f.ctx, f.chain, 2, holder))
	require.Equal(t, sdk.NewInt(7), f.bankBalance(holder, f.base))
	f.ctx = f.ctx.WithBlockTime(f.ctx.BlockTime().Add(time.Hour))
	f.fund(t, sdk.NewCoins(sdk.NewInt64Coin(f.base, 3)))
	f.open(t)
	ctx, amounts, err := f.k.ClaimChainRevenueBatch(f.ctx, f.caller, f.chain, holder, []uint64{2, 3})
	require.NoError(t, err)
	f.ctx = ctx
	require.Equal(t, []sdk.Int{sdk.NewInt(5), sdk.NewInt(3)}, amounts)
	require.Equal(t, sdk.NewInt(15), f.bankBalance(holder, f.base))
}

func TestChainRevenuePoolAndBurnAddressAreNotEligible(t *testing.T) {
	pool := common.HexToAddress(generation2Market(common.Address{}, 1, nil).PoolAddress)
	dead := common.HexToAddress(types.DeadAddress)
	holder := address("eligible revenue holder")
	f := newChainRevenueFixture(t, holding{pool, 5}, holding{dead, 3}, holding{holder, 2})
	f.fund(t, sdk.NewCoins(sdk.NewInt64Coin(f.base, 9)))
	record := f.open(t)
	require.Equal(t, sdk.NewInt(2), record.EligibleSupply)
	for _, excluded := range []common.Address{f.pool, dead} {
		_, err := f.k.ChainRevenueClaimable(f.ctx, f.caller, f.chain, 1, excluded)
		require.ErrorIs(t, err, types.ErrUnauthorized)
		_, _, err = f.k.ClaimChainRevenue(f.ctx, f.caller, f.chain, 1, excluded)
		require.ErrorIs(t, err, types.ErrUnauthorized)
	}
	_, amount, err := f.k.ClaimChainRevenue(f.ctx, f.caller, f.chain, 1, holder)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(9), amount)
}

func TestChainRevenueOtherDenominationsGoToTheTreasury(t *testing.T) {
	f := newChainRevenueFixture(t, holding{address("revenue holder"), 1})
	other := "uother"
	f.fund(t, sdk.NewCoins(sdk.NewInt64Coin(f.base, 20), sdk.NewInt64Coin(other, 11)))
	f.open(t)
	revenue := types.ChainRevenueAccount(f.chain)
	bank := testkeeper.EVMTestApp.BankKeeper
	require.Equal(t, sdk.ZeroInt(), bank.GetBalance(f.ctx, revenue, f.base).Amount)
	require.Equal(t, sdk.ZeroInt(), bank.GetBalance(f.ctx, revenue, other).Amount)
	treasury := testkeeper.EVMTestApp.EvmKeeper.GetPaxAddressOrDefault(f.ctx, f.k.TreasuryContract())
	require.Equal(t, sdk.NewInt(11), bank.GetBalance(f.ctx, treasury, other).Amount)
	require.Equal(t, sdk.NewInt(20), bank.GetBalance(f.ctx, f.k.ModuleAddress(), f.base).Amount)
}

func TestChainRevenueClaimableAfterRetirement(t *testing.T) {
	f := newChainRevenueFixture(t, holding{address("revenue retirement holder"), 1})
	f.fund(t, sdk.NewCoins(sdk.NewInt64Coin(f.base, 13)))
	f.open(t)
	// The revenue claim reads its fixed launch record and epoch after the
	// registry has marked the chain retired.
	app := testkeeper.EVMTestApp
	params := app.LayerXChainsKeeper.GetParams(f.ctx)
	value := params.MinBond.Add(params.CreationFee)
	payer := app.EvmKeeper.GetPaxAddressOrDefault(f.ctx, recordedCreator)
	coins := sdk.NewCoins(sdk.NewCoin(f.base, value))
	require.NoError(t, app.BankKeeper.MintCoins(f.ctx, evmtypes.ModuleName, coins))
	require.NoError(t, app.BankKeeper.SendCoinsFromModuleToAccount(f.ctx, evmtypes.ModuleName, payer, coins))
	operator := common.BytesToAddress(types.ChainRevenueAccount(f.chain))
	domain, err := app.LayerXChainsKeeper.CreateWithPayer(f.ctx, payer, operator, "revenue-retirement",
		params.MinGasAllowance, params.MinByteAllowance, 100, value, evmtypes.DefaultMinFeePerGas)
	require.NoError(t, err)
	require.Equal(t, f.chain, uint64(domain.Id))
	require.NoError(t, app.LayerXChainsKeeper.Retire(f.ctx, operator, domain.Id))
	retired, found := app.LayerXChainsKeeper.GetDomain(f.ctx, domain.Id)
	require.True(t, found)
	require.Equal(t, chainstypes.StatusRetired, retired.Status)
	holder := address("revenue retirement holder")
	claimable, err := f.k.ChainRevenueClaimable(f.ctx, f.caller, f.chain, 1, holder)
	require.NoError(t, err)
	require.Equal(t, sdk.NewInt(13), claimable)
	_, paid, err := f.k.ClaimChainRevenue(f.ctx, f.caller, f.chain, 1, holder)
	require.NoError(t, err)
	require.Equal(t, claimable, paid)
}
