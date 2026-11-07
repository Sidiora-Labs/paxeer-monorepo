package perps_test

import (
	"math/big"
	"testing"

	perpskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/keeper"
	perpstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func (env *positionsEnv) viewGetOpenInterest(want *big.Int) {
	env.t.Helper()
	result := env.mustCall(positionsTraderA, "getOpenInterest", big.NewInt(0))
	outputs, err := env.perpsABI.Methods["getOpenInterest"].Outputs.Unpack(result)
	require.NoError(env.t, err)
	require.Len(env.t, outputs, 2)
	for i, output := range outputs {
		require.Zero(env.t, want.Cmp(output.(*big.Int)), "getOpenInterest side %d", i)
	}
}

func TestGovernedOpenInterestCapView(t *testing.T) {
	env := newPositionsEnv(t, 100_000_000)
	market, err := env.k.GetMarket(env.ctx(), 0)
	require.NoError(t, err)
	require.Equal(t, "BTC", market.Symbol)
	market.OiCapUsd = sdk.NewIntWithDecimal(500, perpstypes.USDDecimals)
	require.NoError(t, env.k.SetMarket(env.ctx(), env.k.Authority(), market))
	oldCap, err := env.k.OpenInterestCap(env.ctx(), market.ID)
	require.NoError(t, err)
	require.Equal(t, market.OiCapUsd, oldCap)

	env.viewGetOpenInterest(big.NewInt(0))
	env.requireRevert("PerpsOpenInterestCap", positionsTraderA, "openPosition", big.NewInt(0), env.usdl,
		big.NewInt(100_000_000), positionsE18(10), true)
	env.viewGetOpenInterest(big.NewInt(0))
	_, exists := env.position(positionsTraderA)
	require.False(t, exists)

	market.OiCapUsd = sdk.NewIntWithDecimal(2_000, perpstypes.USDDecimals)
	response, err := perpskeeper.NewMsgServerImpl(env.k).SetMarket(sdk.WrapSDKContext(env.ctx()), &perpstypes.MsgSetMarket{
		Authority: env.k.Authority(), Market: market,
	})
	require.NoError(t, err)
	require.NotNil(t, response)
	stored, err := env.k.GetMarket(env.ctx(), market.ID)
	require.NoError(t, err)
	require.Equal(t, market, stored)
	cap, err := env.k.OpenInterestCap(env.ctx(), market.ID)
	require.NoError(t, err)
	require.Equal(t, market.OiCapUsd, cap)
	require.Positive(t, cap.BigInt().Cmp(oldCap.BigInt()))
	env.viewGetOpenInterest(big.NewInt(0))

	id := env.open()
	position, exists := env.position(positionsTraderA)
	require.True(t, exists)
	require.Equal(t, id, position.ID.ToBig())
	require.True(t, position.IsLong)
	require.Equal(t, positionsWord(t, "16500000000000000"), position.SizeBase)
	require.Equal(t, positionsE18(60_000), position.EntryPrice.ToBig())
	maker, exists := env.position(positionsMakerB)
	require.True(t, exists)
	require.False(t, maker.IsLong)
	require.Equal(t, position.SizeBase, maker.SizeBase)
	interest, err := env.k.OpenInterest(env.ctx(), market.ID)
	require.NoError(t, err)
	require.Equal(t, position.SizeBase, interest.LongBase)
	require.Equal(t, maker.SizeBase, interest.ShortBase)
	want := positionsE18(990)
	require.Positive(t, want.Cmp(oldCap.BigInt()))
	require.Negative(t, want.Cmp(cap.BigInt()))
	env.viewGetOpenInterest(want)
	logs, changes := env.perpsLogs("OpenInterestChanged")
	require.Len(t, logs, 1)
	require.Len(t, changes, 1)
	require.Equal(t, want, changes[0]["longOI"])
	require.Equal(t, want, changes[0]["shortOI"])

	state, err := env.k.GetMarketState(env.ctx(), market.ID)
	require.NoError(t, err)
	state.MarkPrice = *uint256.MustFromBig(positionsE18(61_000))
	env.k.SetMarketState(env.ctx(), state)
	atMark := ordersFixed(t, "1006.5")
	require.Positive(t, atMark.Cmp(want))
	require.Positive(t, atMark.Cmp(oldCap.BigInt()))
	env.viewGetOpenInterest(atMark)
}
