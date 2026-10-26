package simulation_test

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"testing"

	launchpad "github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/simulation"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"
	"github.com/stretchr/testify/require"
)

func TestDecodeStore(t *testing.T) {
	marshal := func(value any) []byte {
		t.Helper()
		bz, err := json.Marshal(value)
		require.NoError(t, err)
		return bz
	}
	paramsA, paramsB := types.DefaultParams(), types.DefaultParams()
	paramsB.CreationFee = sdk.NewInt(2_000_000)
	paramsB.QuoteToken = "0x1111111111111111111111111111111111111111"
	marketA := types.Market{
		Denom: "factory/creator/token", Index: 1, Name: "Token", Symbol: "TOK",
		TotalSupply: sdk.NewInt(1_000_000), RealQuoteBalance: sdk.NewInt(123),
		Generation: types.Generation2, PoolAddress: "0x2222222222222222222222222222222222222222",
	}
	marketB := marketA
	marketB.Name = "Updated Token"
	marketB.RealQuoteBalance = sdk.NewInt(456)
	marketB.Paused = true
	marketBytesA, marketBytesB := marshal(marketA), marshal(marketB)
	// JSON storage normalizes nil sdk.Int fields to zero on decoding.
	require.NoError(t, json.Unmarshal(marketBytesA, &marketA))
	require.NoError(t, json.Unmarshal(marketBytesB, &marketB))
	surfaceA := keeper.Surface{Role: types.RolePool, Generation: types.Generation1, Market: marketA.Denom}
	surfaceB := keeper.Surface{Role: types.RolePool, Generation: types.Generation2, Market: marketB.Denom}

	tests := []struct {
		name string
		key  []byte
		a, b []byte
		want string
	}{
		{"params", types.ParamsKey, marshal(paramsA), marshal(paramsB), fmt.Sprintf("Params: %+v\nParams: %+v", paramsA, paramsB)},
		{"market count", types.MarketCountKey, binary.BigEndian.AppendUint64(nil, 0), binary.BigEndian.AppendUint64(nil, 1<<40+7), "MarketCount: 0\nMarketCount: 1099511627783"},
		{"market", types.MarketKey(marketA.Denom), marketBytesA, marketBytesB, fmt.Sprintf("Market: %+v\nMarket: %+v", marketA, marketB)},
		{"surface", types.SurfaceKey([20]byte{0xab}), marshal(surfaceA), marshal(surfaceB), "Surface: {Role:SidioraPool Generation:1 Market:factory/creator/token}\nSurface: {Role:SidioraPool Generation:2 Market:factory/creator/token}"},
		{"unknown", []byte{0xff, 0x10}, []byte{0xab}, []byte{0xcd}, "launchpad prefix 0xff: AB\nlaunchpad prefix 0xff: CD"},
		{"unhandled prefix", types.ProtocolFeesKey("quote"), []byte("12"), []byte("34"), "launchpad prefix 0x03: 3132\nlaunchpad prefix 0x03: 3334"},
		{"zero prefix", []byte{0x00}, []byte{0x01}, []byte{0x02}, "launchpad prefix 0x00: 01\nlaunchpad prefix 0x00: 02"},
	}
	registry := make(sdk.StoreDecoderRegistry)
	launchpad.NewAppModule(nil).RegisterStoreDecoder(registry)
	require.Contains(t, registry, types.StoreKey)
	require.NotNil(t, registry[types.StoreKey])
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a, b := kv.Pair{Key: tt.key, Value: tt.a}, kv.Pair{Key: tt.key, Value: tt.b}
			require.Equal(t, tt.want, simulation.NewDecodeStore()(a, b))
			require.Equal(t, tt.want, registry[types.StoreKey](a, b))
		})
	}
	for _, tt := range tests[:4] {
		t.Run(tt.name+" corrupt value", func(t *testing.T) {
			valid := kv.Pair{Key: tt.key, Value: tt.a}
			corrupt := kv.Pair{Key: tt.key, Value: []byte("invalid")}
			require.Panics(t, func() { simulation.NewDecodeStore()(corrupt, valid) })
			require.Panics(t, func() { simulation.NewDecodeStore()(valid, corrupt) })
		})
	}
	t.Run("empty key", func(t *testing.T) {
		require.PanicsWithValue(t, "launchpad: empty store key", func() {
			simulation.NewDecodeStore()(kv.Pair{}, kv.Pair{})
		})
	})
}
