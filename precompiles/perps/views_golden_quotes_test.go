package perps_test

import (
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"strings"
	"testing"

	perpstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/perps"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func goldenQuotesExecute(t *testing.T, env *ordersEnv, executor *perps.PrecompileExecutor,
	caller common.Address, name string, args ...interface{}) []byte {
	t.Helper()
	method, ok := env.perps.Methods[name]
	require.True(t, ok, name)
	packed, err := method.Inputs.Pack(args...)
	require.NoError(t, err, name)
	decoded, err := method.Inputs.Unpack(packed)
	require.NoError(t, err, name)
	ret, err := executor.Execute(env.ctx(), &method, caller, caller, decoded, new(big.Int),
		!perps.GasTable[name].Write, env.evm, nil)
	require.NoError(t, err, "%s reverted with %x", name, ret)
	_, err = method.Outputs.Unpack(ret)
	require.NoError(t, err, name)
	return ret
}

func quoteOpenPosition(t *testing.T, env *ordersEnv, executor *perps.PrecompileExecutor) []byte {
	t.Helper()
	return goldenQuotesExecute(t, env, executor,
		common.HexToAddress("0x00000000000000000000000000000000000c0004"), "quoteOpenPosition",
		big.NewInt(0), env.usdl, big.NewInt(100_000_000), ordersBig(10), true)
}

func quoteClosePosition(t *testing.T, env *ordersEnv, executor *perps.PrecompileExecutor, positionID *big.Int) []byte {
	t.Helper()
	return goldenQuotesExecute(t, env, executor, ordersTraderA, "quoteClosePosition", positionID)
}

func TestViewGoldenQuotes(t *testing.T) {
	fixture, err := os.ReadFile("testdata/views_quotes_golden.json")
	require.NoError(t, err)
	var golden map[string]string
	require.NoError(t, json.Unmarshal(fixture, &golden))
	require.Len(t, golden, 4)

	env := newOrdersEnv(t)
	precompile, err := perps.NewPrecompile(quotesKeepers{Keepers: &utils.EmptyKeepers{}, perps: env.k})
	require.NoError(t, err)
	executor, ok := precompile.GetExecutor().(*perps.PrecompileExecutor)
	require.True(t, ok)
	for _, account := range []common.Address{ordersTraderA, ordersTraderB, ordersKeeperC} {
		env.fund(account, 1_000)
	}

	ask := goldenQuotesExecute(t, env, executor, ordersTraderB, "placeOrder", limitOrder(false, 6_000, 20, 60_000))
	bid := goldenQuotesExecute(t, env, executor, ordersKeeperC, "placeOrder", limitOrder(true, 6_000, 20, 59_900))
	goldenQuotesExecute(t, env, executor, ordersTraderA, "placeOrder",
		newOrderParams(true, perpstypes.OrderTypeMarket, perpstypes.TimeInForceIOC, 1_200, 10))
	positionID := env.positionOf(ordersTraderA)
	position, err := env.k.GetPosition(env.ctx(), *uint256.MustFromBig(positionID))
	require.NoError(t, err)
	require.True(t, position.IsLong)
	require.Equal(t, fixedWord(t, "0.02"), position.SizeBase)
	for _, orderID := range [][]byte{ask, bid} {
		order, err := env.k.GetOrder(env.ctx(), *uint256.MustFromBig(new(big.Int).SetBytes(orderID)))
		require.NoError(t, err)
		require.True(t, order.FilledBase.Lt(&order.SizeBase))
	}

	before := (&quotesEnv{ctx: env.ctx(), perpsKey: env.key}).storeHash()
	cases := []struct {
		name string
		call func(*testing.T) []byte
	}{
		{"quoteMarket", func(t *testing.T) []byte {
			return goldenQuotesExecute(t, env, executor, ordersTraderA, "quoteMarket", big.NewInt(0))
		}},
		{"quoteOpenPosition", func(t *testing.T) []byte { return quoteOpenPosition(t, env, executor) }},
		{"quoteClosePosition", func(t *testing.T) []byte { return quoteClosePosition(t, env, executor, positionID) }},
		{"quotePartialClose", func(t *testing.T) []byte {
			return goldenQuotesExecute(t, env, executor, ordersTraderA, "quotePartialClose", positionID, ordersBig(600))
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			encoded, ok := golden[tc.name]
			require.True(t, ok, "missing golden for %s", tc.name)
			require.True(t, strings.HasPrefix(encoded, "0x"))
			want, err := hex.DecodeString(strings.TrimPrefix(encoded, "0x"))
			require.NoError(t, err)
			require.NotEmpty(t, want)
			require.Equal(t, want, tc.call(t), "%s ABI output", tc.name)
			require.Equal(t, before, (&quotesEnv{ctx: env.ctx(), perpsKey: env.key}).storeHash(), "quote mutated state")
		})
	}
}
