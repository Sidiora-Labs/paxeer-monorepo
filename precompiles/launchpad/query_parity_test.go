package launchpad_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/launchpad"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

func TestQueryMarketMatchesPoolViews(t *testing.T) {
	e := newPoolEnv(t)
	router := baseapp.NewQueryServerTestHelper(e.ctx(), testkeeper.EVMTestApp.InterfaceRegistry())
	types.RegisterQueryServer(router, keeper.NewQuerier(e.k))
	client := types.NewQueryClient(router)

	assertParity := func(wantPaused bool) types.MarketInfo {
		t.Helper()
		info := *abi.ConvertType(e.view("getPoolInfo"), new(launchpad.PoolInfo)).(*launchpad.PoolInfo)
		paused := e.view("paused").(bool)
		router.Ctx = e.ctx()
		response, err := client.Market(context.Background(), &types.QueryMarketRequest{Denom: e.denom})
		require.NoError(t, err)
		require.NotNil(t, response)
		market := response.Market
		require.Equal(t, e.denom, market.Denom)
		require.Equal(t, e.pool.Hex(), market.PoolAddress)
		require.Equal(t, info.TokenAddress.Hex(), market.TokenAddress)
		require.Equal(t, info.OpticalAddress.Hex(), market.Optical)
		require.Equal(t, info.VirtualUsdlReserve.String(), market.VirtualQuoteReserve.String())
		require.Equal(t, info.RealUsdlBalance.String(), market.RealQuoteBalance.String())
		require.Equal(t, info.TokenReserve.String(), market.TokenReserve.String())
		require.Equal(t, info.CreationTimestamp.Int64(), market.CreationTime)
		require.Equal(t, info.CumulativeVolume.String(), market.CumulativeVolume.String())
		require.Equal(t, wantPaused, paused)
		require.Equal(t, paused, market.Paused)
		return market
	}

	initial := assertParity(false)
	require.Equal(t, n(poolVirtualQuote).String(), initial.VirtualQuoteReserve.String())
	require.True(t, initial.RealQuoteBalance.IsZero())
	require.Equal(t, n(poolTokenReserve).String(), initial.TokenReserve.String())
	require.True(t, initial.CumulativeVolume.IsZero())

	const paid int64 = 1_000_000_000
	deadline := n(genesisTime.Add(time.Hour).Unix())
	tokenBeforeBuy := e.balance(e.token, e.holder)
	e.transfer(e.usdl, e.holder, e.pool, paid)
	buy := e.write(e.holder, "swap", n(paid), n(1), true, e.holder, deadline)
	require.Len(t, buy, 1)
	bought := buy[0].(*big.Int)
	require.Positive(t, bought.Sign())
	require.Equal(t, new(big.Int).Add(tokenBeforeBuy, bought), e.balance(e.token, e.holder))
	afterBuy := assertParity(false)
	require.True(t, afterBuy.RealQuoteBalance.IsPositive())
	require.Equal(t, new(big.Int).Sub(n(poolTokenReserve), bought).String(), afterBuy.TokenReserve.String())
	require.Equal(t, n(paid).String(), afterBuy.CumulativeVolume.String())

	sellIn := new(big.Int).Div(bought, n(2))
	require.Positive(t, sellIn.Sign())
	require.True(t, sellIn.IsInt64())
	e.transfer(e.token, e.holder, e.pool, sellIn.Int64())
	quoteBeforeSell := e.balance(e.usdl, e.holder)
	sell := e.write(e.holder, "swap", sellIn, n(1), false, e.holder, deadline)
	require.Len(t, sell, 1)
	sold := sell[0].(*big.Int)
	require.Positive(t, sold.Sign())
	require.Equal(t, new(big.Int).Add(quoteBeforeSell, sold), e.balance(e.usdl, e.holder))
	afterSell := assertParity(false)
	require.Equal(t, new(big.Int).Sub(afterBuy.RealQuoteBalance.BigInt(), sold).String(), afterSell.RealQuoteBalance.String())
	require.Equal(t, new(big.Int).Add(afterBuy.TokenReserve.BigInt(), sellIn).String(), afterSell.TokenReserve.String())
	require.Equal(t, new(big.Int).Add(n(paid), sold).String(), afterSell.CumulativeVolume.String())

	require.Empty(t, e.write(e.guardian, "pause"))
	assertParity(true)
	require.Empty(t, e.write(e.guardian, "unpause"))
	assertParity(false)
}
