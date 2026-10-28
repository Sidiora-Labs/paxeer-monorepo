package perps_test

import (
	"encoding/json"
	"math/big"
	"os"
	"testing"

	perpstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func marketGoldenView(t *testing.T, e *ordersEnv, name string, id *big.Int) []byte {
	t.Helper()
	input, err := e.perps.Pack(name, id)
	require.NoError(t, err, name)
	result, _, err := e.evm.StaticCall(ordersTraderA, ordersPerps, input, ordersCallGas)
	require.NoError(t, err, "%s reverted with %x", name, result)
	return result
}

func viewGetOrder(t *testing.T, e *ordersEnv, orderID *big.Int) []byte {
	t.Helper()
	return marketGoldenView(t, e, "getOrder", orderID)
}

func viewGetMarket(t *testing.T, e *ordersEnv, marketID *big.Int) []byte {
	t.Helper()
	return marketGoldenView(t, e, "getMarket", marketID)
}

func TestViewGoldenOrderAndMarket(t *testing.T) {
	fixture, err := os.ReadFile("testdata/views_market_golden.json")
	require.NoError(t, err)
	var golden map[string]string
	require.NoError(t, json.Unmarshal(fixture, &golden))
	require.Len(t, golden, 4)

	e := newOrdersEnv(t)
	e.fund(ordersTraderA, 1_000)
	params := limitOrder(true, 6_000, 20, 59_000)
	params.Cloid = ordersCloid
	params.Deadline = uint64(ordersNowMs/1000 + 3600)
	placed := e.mustCall(ordersTraderA, "placeOrder", params)
	require.Equal(t, []interface{}{big.NewInt(1)}, placed)
	orderID := placed[0].(*big.Int)
	order := e.order(orderID.Uint64())
	require.Equal(t, perpstypes.OrderStatusResting, order.Status)
	require.Equal(t, perpstypes.OrderTypeLimit, order.Type)
	require.Equal(t, perpstypes.TimeInForceGTC, order.TimeInForce)
	require.Equal(t, uint256.Int{}, order.FilledBase)
	require.Equal(t, uint64(1), e.account(ordersTraderA).OpenOrderCount)

	actual := map[string][]byte{
		"getOrder":        viewGetOrder(t, e, orderID),
		"getOrderDetails": marketGoldenView(t, e, "getOrderDetails", orderID),
		"getMarket":       viewGetMarket(t, e, params.MarketId),
		"getMarketConfig": marketGoldenView(t, e, "getMarketConfig", params.MarketId),
	}
	for _, name := range []string{"getOrder", "getOrderDetails", "getMarket", "getMarketConfig"} {
		t.Run(name, func(t *testing.T) {
			hex, ok := golden[name]
			require.True(t, ok, "missing golden for %s", name)
			want, err := hexutil.Decode(hex)
			require.NoError(t, err, name)
			require.NotEmpty(t, want, name)
			require.Equal(t, want, actual[name], "%s ABI output changed", name)
		})
	}
}
