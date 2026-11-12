package keeper_test

import (
	"encoding/json"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/launchpadtest"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestChainLaunchStoreRoundTrip(t *testing.T) {
	e := newEnv(t)
	pool := common.BytesToAddress([]byte{0x01})
	holder := common.BytesToAddress([]byte{0x04})
	chain := types.LaunchedChain{RegistryNumber: 7, ChainID: 91, Pool: pool.Hex(),
		Token: common.BytesToAddress([]byte{0x02}).Hex(), FeeRightID: 3,
		Creator: common.BytesToAddress([]byte{0x03}).Hex(), LaunchBlock: 10}
	_, found, err := e.k.GetLaunchedChain(e.ctx, chain.RegistryNumber)
	require.NoError(t, err)
	require.False(t, found)
	_, found = e.k.RegistryNumberByPool(e.ctx, pool)
	require.False(t, found)
	require.NoError(t, e.k.SetLaunchedChain(e.ctx, chain))
	got, found, err := e.k.GetLaunchedChain(e.ctx, chain.RegistryNumber)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, chain, got)
	number, found := e.k.RegistryNumberByPool(e.ctx, pool)
	require.True(t, found)
	require.Equal(t, chain.RegistryNumber, number)
	require.Equal(t, chain.ChainID, e.k.HomeChainByPool(e.ctx, pool))

	epoch := types.ChainRevenueEpoch{Chain: chain.RegistryNumber, Epoch: 1, Amount: sdk.NewInt(100),
		SnapshotBlock: 12, EligibleSupply: sdk.NewInt(1000), ClaimedTotal: sdk.NewInt(25), OpenedAt: 100}
	require.Zero(t, e.k.LastChainRevenueEpoch(e.ctx, chain.RegistryNumber))
	require.NoError(t, e.k.SetChainRevenueEpoch(e.ctx, epoch))
	gotEpoch, found, err := e.k.GetChainRevenueEpoch(e.ctx, chain.RegistryNumber, epoch.Epoch)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, epoch, gotEpoch)
	require.Equal(t, epoch.Epoch, e.k.LastChainRevenueEpoch(e.ctx, chain.RegistryNumber))
	require.False(t, e.k.HasChainRevenueClaim(e.ctx, chain.RegistryNumber, epoch.Epoch, holder))
	require.NoError(t, e.k.SetChainRevenueClaim(e.ctx, chain.RegistryNumber, epoch.Epoch, holder))
	require.True(t, e.k.HasChainRevenueClaim(e.ctx, chain.RegistryNumber, epoch.Epoch, holder))
	require.ErrorIs(t, e.k.SetChainRevenueClaim(e.ctx, chain.RegistryNumber, epoch.Epoch, holder), types.ErrAlreadyClaimed)
	second := epoch
	second.Epoch = 2
	require.NoError(t, e.k.SetChainRevenueEpoch(e.ctx, second))
	require.NoError(t, e.k.SetChainRevenueEpoch(e.ctx, epoch))
	require.Equal(t, uint64(2), e.k.LastChainRevenueEpoch(e.ctx, chain.RegistryNumber))
}

func TestChainLaunchStoreAbsentHomeChainIsRoot(t *testing.T) {
	e := newEnv(t)
	pool := common.BytesToAddress([]byte{0x11})
	require.Zero(t, e.k.HomeChainByPool(e.ctx, pool))
	e.k.SetHomeChainByPool(e.ctx, pool, 42)
	require.Equal(t, uint64(42), e.k.HomeChainByPool(e.ctx, pool))
	e.k.SetHomeChainByPool(e.ctx, pool, 0)
	require.Zero(t, e.k.HomeChainByPool(e.ctx, pool))
}

func TestChainLaunchGenesisRoundTrip(t *testing.T) {
	e := newSurfacedEnv(t)
	creator := e.account(200_000_000)
	first := e.create(creator, types.FeeStrategyClaim)
	second := e.create(creator, types.FeeStrategyClaim)
	makeChain := func(number, chainID uint64, market types.Market) types.LaunchedChain {
		return types.LaunchedChain{RegistryNumber: number, ChainID: chainID, Pool: market.PoolAddress,
			Token: market.TokenAddress, FeeRightID: market.FeeRightID,
			Creator: e.evmAddress(creator).Hex(), LaunchBlock: e.ctx.BlockHeight()}
	}
	for _, record := range []types.LaunchedChain{makeChain(9, 109, second), makeChain(3, 103, first)} {
		require.NoError(t, e.k.SetLaunchedChain(e.ctx, record))
	}
	for _, chain := range []uint64{9, 3} {
		require.NoError(t, e.k.SetChainRevenueEpoch(e.ctx, types.ChainRevenueEpoch{Chain: chain,
			Epoch: 1, Amount: sdk.NewInt(50), SnapshotBlock: 2, EligibleSupply: sdk.NewInt(100),
			ClaimedTotal: sdk.NewInt(10), OpenedAt: genesisTime.Unix()}))
		require.NoError(t, e.k.SetChainRevenueClaim(e.ctx, chain, 1, common.BytesToAddress([]byte{byte(chain)})))
	}
	exported := e.k.ExportGenesis(e.ctx)
	require.NoError(t, exported.Validate())
	require.Equal(t, []uint64{3, 9}, []uint64{exported.LaunchedChains[0].RegistryNumber,
		exported.LaunchedChains[1].RegistryNumber})
	require.Equal(t, []uint64{3, 9}, []uint64{exported.ChainRevenueEpochs[0].Chain,
		exported.ChainRevenueEpochs[1].Chain})
	require.Len(t, exported.PoolHomeChains, 2)
	require.Len(t, exported.ChainRevenueClaims, 2)

	raw, err := json.Marshal(exported)
	require.NoError(t, err)
	var decoded types.GenesisState
	require.NoError(t, json.Unmarshal(raw, &decoded))
	require.NoError(t, decoded.Validate())
	wrongMarket := decoded
	wrongMarket.LaunchedChains = append([]types.LaunchedChain(nil), decoded.LaunchedChains...)
	wrongMarket.LaunchedChains[0].Token = common.BytesToAddress([]byte{0xee}).Hex()
	require.ErrorIs(t, wrongMarket.Validate(), types.ErrInvalidGenesis)
	duplicateClaim := decoded
	duplicateClaim.ChainRevenueClaims = append(append([]types.ChainRevenueClaim(nil), decoded.ChainRevenueClaims...), decoded.ChainRevenueClaims[0])
	require.ErrorIs(t, duplicateClaim.Validate(), types.ErrInvalidGenesis)
	fresh, ctx := launchpadtest.NewKeeper(testkeeper.EVMTestApp, e.ctx)
	fresh.InitGenesis(ctx, decoded)
	sameJSON(t, exported, fresh.ExportGenesis(ctx))
	for _, record := range exported.LaunchedChains {
		got, found, err := fresh.GetLaunchedChain(ctx, record.RegistryNumber)
		require.NoError(t, err)
		require.True(t, found)
		require.Equal(t, record, got)
		require.Equal(t, record.ChainID, fresh.HomeChainByPool(ctx, common.HexToAddress(record.Pool)))
		require.Equal(t, uint64(1), fresh.LastChainRevenueEpoch(ctx, record.RegistryNumber))
		require.True(t, fresh.HasChainRevenueClaim(ctx, record.RegistryNumber, 1,
			common.BytesToAddress([]byte{byte(record.RegistryNumber)})))
	}

	empty, emptyCtx := launchpadtest.NewKeeper(testkeeper.EVMTestApp, e.ctx)
	empty.InitGenesis(emptyCtx, *types.DefaultGenesis())
	emptyExport := empty.ExportGenesis(emptyCtx)
	require.Empty(t, emptyExport.LaunchedChains)
	require.Empty(t, emptyExport.PoolHomeChains)
	require.Empty(t, emptyExport.ChainRevenueEpochs)
	require.Empty(t, emptyExport.ChainRevenueClaims)
}
