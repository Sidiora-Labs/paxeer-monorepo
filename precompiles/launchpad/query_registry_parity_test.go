package launchpad_test

import (
	"context"
	"math/big"
	"testing"

	launchpadkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/launchpad"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestQueryMarketsMatchPoolRegistryViews(t *testing.T) {
	e := newPoolEnv(t)
	gs := e.k.ExportGenesis(e.ctx())
	require.Len(t, gs.Markets, 1)
	require.Len(t, gs.PoolMetadata, 1)
	first := gs.Markets[0]
	wantPools := []common.Address{e.pool}
	for i, denom := range []string{"factory/launchpad/zulu", "factory/launchpad/alpha"} {
		market := first
		market.Index = uint64(i + 2)
		market.Denom = denom
		market.PoolAddress = poolAddress(denom + " pool").Hex()
		market.TokenAddress = poolAddress(denom + " token").Hex()
		market.Pointer = market.TokenAddress
		market.Optical = poolAddress(denom + " optical").Hex()
		market.FeeRightID = uint64(100 + i)
		market.CreationTime += int64(60 * (i + 1))
		gs.Markets = append(gs.Markets, market)
		gs.Surfaces = append(gs.Surfaces, types.Surface{Address: market.PoolAddress, Role: types.RolePool})
		gs.PoolMetadata = append(gs.PoolMetadata, types.PoolMetadata{
			Pool: market.PoolAddress, Creator: e.guardian.Hex(), Token: market.TokenAddress,
			Optical: market.Optical, NFTID: market.FeeRightID, CreatedAt: market.CreationTime,
			CreatedBlock: int64(10 + i),
		})
		wantPools = append(wantPools, common.HexToAddress(market.PoolAddress))
	}
	gs.Markets[0], gs.Markets[2] = gs.Markets[2], gs.Markets[0]
	require.NoError(t, gs.Validate())
	e.k.InitGenesis(e.ctx(), *gs)
	require.NoError(t, e.k.InstallRoleSurfaces(e.ctx(), &testkeeper.EVMTestApp.EvmKeeper))

	registry := launchpadkeeper.Generation2Surfaces[types.RolePoolRegistry]
	surface, found, err := e.k.GetSurface(e.ctx(), registry)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, types.RolePoolRegistry, surface.Role)
	require.Equal(t, launchpadkeeper.SurfaceRuntime, e.stateDB.GetCode(registry))
	registryABI := readABI(t, "abi/PoolRegistry.json")
	registryView := func(t *testing.T, name string, args ...interface{}) interface{} {
		t.Helper()
		input, err := registryABI.Pack(name, args...)
		require.NoError(t, err)
		ret, _, err := e.evm.StaticCall(e.holder, registry, input, poolCallGas)
		require.NoError(t, err, "%s returned %x", name, ret)
		out, err := registryABI.Methods[name].Outputs.Unpack(ret)
		require.NoError(t, err, name)
		require.Len(t, out, 1, name)
		return out[0]
	}

	router := baseapp.NewQueryServerTestHelper(e.ctx(), testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, launchpadkeeper.NewQuerier(e.k))
	client := types.NewQueryClient(router)
	allPools := registryView(t, "getAllPools", big.NewInt(0), big.NewInt(100)).([]common.Address)
	require.Equal(t, wantPools, allPools)
	router.Ctx = e.ctx()
	allMarkets, err := client.Markets(context.Background(), &types.QueryMarketsRequest{Limit: 100})
	require.NoError(t, err)
	require.NotNil(t, allMarkets)
	require.Equal(t, uint64(len(wantPools)), allMarkets.Total)
	require.Len(t, allMarkets.Markets, len(allPools))
	for i, market := range allMarkets.Markets {
		require.Equal(t, uint64(i+1), market.Index)
		require.Equal(t, allPools[i].Hex(), market.PoolAddress)
		t.Run("metadata/"+market.Denom, func(t *testing.T) {
			view := abi.ConvertType(registryView(t, "getPoolMetadata", allPools[i]),
				new(launchpad.RegistryPoolMetadata)).(*launchpad.RegistryPoolMetadata)
			router.Ctx = e.ctx()
			response, err := client.PoolMetadata(context.Background(), &types.QueryPoolMetadataRequest{Pool: market.PoolAddress})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.True(t, response.Found)
			require.NotNil(t, response.Metadata)
			metadata := response.Metadata
			require.Equal(t, view.Creator.Hex(), metadata.Creator)
			require.Equal(t, view.Token.Hex(), metadata.Token)
			require.Equal(t, view.Optical.Hex(), metadata.Optical)
			require.Equal(t, view.NftId, new(big.Int).SetUint64(metadata.NFTID))
			require.Equal(t, view.CreatedAt, big.NewInt(metadata.CreatedAt))
			require.Equal(t, view.CreatedBlock, big.NewInt(metadata.CreatedBlock))
			want := gs.PoolMetadata[i]
			require.Equal(t, &types.PoolMetadataInfo{
				Creator: want.Creator, Token: want.Token, Optical: want.Optical, NFTID: want.NFTID,
				CreatedAt: want.CreatedAt, CreatedBlock: want.CreatedBlock,
			}, metadata)
		})
	}

	for _, page := range []struct {
		name          string
		offset, limit uint64
		start, end    int
	}{
		{name: "first", limit: 1, start: 0, end: 1},
		{name: "middle", offset: 1, limit: 1, start: 1, end: 2},
		{name: "second page", offset: 1, limit: 2, start: 1, end: 3},
		{name: "last partial page", offset: 2, limit: 2, start: 2, end: 3},
		{name: "at end", offset: 3, limit: 1, start: 3, end: 3},
		{name: "past end", offset: 9, limit: 1, start: 3, end: 3},
	} {
		t.Run("markets/"+page.name, func(t *testing.T) {
			pools := registryView(t, "getAllPools", new(big.Int).SetUint64(page.offset),
				new(big.Int).SetUint64(page.limit)).([]common.Address)
			require.Equal(t, wantPools[page.start:page.end], pools)
			router.Ctx = e.ctx()
			response, err := client.Markets(context.Background(), &types.QueryMarketsRequest{Offset: page.offset, Limit: page.limit})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, allMarkets.Total, response.Total)
			require.Len(t, response.Markets, len(pools))
			for i, market := range response.Markets {
				require.Equal(t, page.offset+uint64(i)+1, market.Index)
				require.Equal(t, pools[i].Hex(), market.PoolAddress)
				require.Equal(t, allMarkets.Markets[page.start+i].Denom, market.Denom)
			}
		})
	}
}
