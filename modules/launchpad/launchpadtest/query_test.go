package launchpadtest_test

import (
	"context"
	"math/big"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

func TestQueryThroughTestApp(t *testing.T) {
	d, creator := freshDomain(t)
	d.approve(d.usdl, creator, d.keeper.GetParams(d.ctx()).CreationFee.BigInt())
	out := d.do(creator, keeper.Generation2Surfaces[types.RoleRouter], types.RoleRouter,
		"createMarket", "Query Token", "QURY", uint8(types.FeeStrategyClaim), common.Address{})
	require.Len(t, out, 3)
	token, pool, feeRightID := out[0].(common.Address), out[1].(common.Address), out[2].(*big.Int)
	require.NotEqual(t, common.Address{}, token)
	require.NotEqual(t, common.Address{}, pool)
	require.NotEqual(t, token, pool)
	require.True(t, feeRightID.IsUint64())
	require.Positive(t, feeRightID.Sign())
	collection := keeper.Generation2Surfaces[types.RoleFeeRights]
	denom := types.ModuleName + "/" + pool.Hex()

	router := baseapp.NewQueryServerTestHelper(d.ctx(), testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, keeper.NewQuerier(d.keeper))
	client := types.NewQueryClient(router)
	ctx := context.Background()

	market, err := client.Market(ctx, &types.QueryMarketRequest{Denom: denom})
	require.NoError(t, err)
	require.NotNil(t, market)
	require.Equal(t, denom, market.Market.Denom)
	require.Equal(t, "Query Token", market.Market.Name)
	require.Equal(t, "QURY", market.Market.Symbol)
	require.Equal(t, uint32(types.Generation2), market.Market.Generation)
	require.Equal(t, pool.Hex(), market.Market.PoolAddress)
	require.Equal(t, token.Hex(), market.Market.TokenAddress)
	require.Equal(t, collection.Hex(), market.Market.FeeRightCollection)
	require.Equal(t, feeRightID.Uint64(), market.Market.FeeRightID)

	for _, address := range []common.Address{pool, token} {
		t.Run("market-by-address/"+address.Hex(), func(t *testing.T) {
			response, err := client.MarketByAddress(ctx, &types.QueryMarketByAddressRequest{Address: address.Hex()})
			require.NoError(t, err)
			require.NotNil(t, response)
			require.Equal(t, market.Market, response.Market)
		})
	}

	metadata, err := client.PoolMetadata(ctx, &types.QueryPoolMetadataRequest{Pool: pool.Hex()})
	require.NoError(t, err)
	require.NotNil(t, metadata)
	require.True(t, metadata.Found)
	require.Equal(t, &types.PoolMetadataInfo{
		Creator: creator.Hex(), Token: token.Hex(), Optical: (common.Address{}).Hex(),
		NFTID: feeRightID.Uint64(), CreatedAt: domainGenesisTime.Unix(), CreatedBlock: d.ctx().BlockHeight(),
	}, metadata.Metadata)

	right, err := client.FeeRight(ctx, &types.QueryFeeRightRequest{Collection: collection.Hex(), Id: feeRightID.Uint64()})
	require.NoError(t, err)
	require.NotNil(t, right)
	require.True(t, right.Found)
	require.Equal(t, &types.FeeRightInfo{
		ID: feeRightID.Uint64(), Owner: creator.Hex(), Approved: "",
		Strategy: uint32(types.FeeStrategyClaim), Pool: pool.Hex(),
	}, right.FeeRight)
}
