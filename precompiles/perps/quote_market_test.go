package perps_test

import (
	"math/big"
	"testing"

	perpskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	perpstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestQuoteMarketHandlerMatchesKeeperQuote(t *testing.T) {
	env := newQuotesEnv(t, 60_000)
	env.rest(true, usdWord(59_900), "0.01")
	env.rest(false, usdWord(60_100), "0.02")
	bid, found, err := env.k.BestBid(env.ctx, 0)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, usdWord(59_900), bid)
	ask, found, err := env.k.BestAsk(env.ctx, 0)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, usdWord(60_100), ask)

	state, err := env.k.GetMarketState(env.ctx, 0)
	require.NoError(t, err)
	state.MarkPrice = usdWord(60_010)
	state.OracleSum = usdWord(119_980)
	state.PremiumCount = 2
	state.LongOpenInterest = fixedWord(t, "0.01")
	state.ShortOpenInterest = fixedWord(t, "0.02")
	rate := fixedWord(t, "0.000036")
	state.HourlyRate.Neg(&rate)
	env.k.SetMarketState(env.ctx, state)

	before := env.storeHash()
	want, err := env.k.QuoteMarket(env.ctx, 0)
	require.NoError(t, err)
	require.Negative(t, want.FundingRatePerSecond.Sign())
	require.Negative(t, want.FundingRate24h.Sign())
	require.True(t, want.Enabled)
	require.False(t, want.PriceStale)

	method, ok := env.abi.Methods["quoteMarket"]
	require.True(t, ok)
	packed, err := method.Inputs.Pack(big.NewInt(0))
	require.NoError(t, err)
	args, err := method.Inputs.Unpack(packed)
	require.NoError(t, err)
	ret, err := env.executor.Execute(env.ctx, &method, quotesHolder, quotesHolder, args, big.NewInt(0), true, nil, nil)
	require.NoError(t, err)
	out, err := method.Outputs.Unpack(ret)
	require.NoError(t, err)
	require.Len(t, out, 1)
	got := abi.ConvertType(out[0], new(perpskeeper.MarketQuote)).(*perpskeeper.MarketQuote)
	require.Equal(t, want, *got)
	expected, err := method.Outputs.Pack(want)
	require.NoError(t, err)
	require.Equal(t, expected, ret)
	require.Equal(t, before, env.storeHash())

	_, err = env.k.QuoteMarket(env.ctx, 30)
	require.ErrorIs(t, err, perpstypes.ErrUnknownMarket)
	packed, err = method.Inputs.Pack(big.NewInt(30))
	require.NoError(t, err)
	args, err = method.Inputs.Unpack(packed)
	require.NoError(t, err)
	ret, err = env.executor.Execute(env.ctx, &method, quotesHolder, quotesHolder, args, big.NewInt(0), true, nil, nil)
	require.ErrorIs(t, err, perpstypes.ErrUnknownMarket)
	require.Len(t, ret, 4)
	require.Equal(t, crypto.Keccak256([]byte("PerpsUnknownMarket()"))[:4], ret)
	require.Equal(t, before, env.storeHash())
}
