package types_test

import (
	"encoding/json"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func TestMarketInfoOfCopiesEveryField(t *testing.T) {
	var market types.Market
	source := reflect.ValueOf(&market).Elem()
	viewType := reflect.TypeOf(types.MarketInfo{})
	intType := reflect.TypeOf(sdk.Int{})
	for i := 0; i < source.NumField(); i++ {
		field := source.Type().Field(i)
		_, exists := viewType.FieldByName(field.Name)
		require.True(t, exists, "MarketInfo must represent Market.%s", field.Name)
		value := source.Field(i)
		switch value.Type() {
		case intType:
			amount := new(big.Int).Lsh(big.NewInt(1), 192)
			amount.Add(amount, big.NewInt(int64(i+1)))
			value.Set(reflect.ValueOf(sdk.NewIntFromBigInt(amount)))
		case reflect.TypeOf(types.FeeStrategy(0)):
			value.SetUint(uint64(types.FeeStrategyAirdrop))
		case reflect.TypeOf(types.Generation(0)):
			value.SetUint(uint64(types.Generation2))
		case reflect.TypeOf([]sdk.Int{}):
			value.Set(reflect.ValueOf([]sdk.Int{sdk.ZeroInt(), sdk.NewInt(123), sdk.NewInt(456)}))
		case reflect.TypeOf([]uint64{}):
			value.Set(reflect.ValueOf([]uint64{1 << 63, 123, 456}))
		default:
			switch value.Kind() {
			case reflect.String:
				value.SetString("market-" + field.Name)
			case reflect.Bool:
				value.SetBool(true)
			case reflect.Uint64:
				value.SetUint(1<<63 + uint64(i+1))
			case reflect.Int64:
				value.SetInt(1<<40 + int64(i+1))
			default:
				t.Fatalf("populate new Market field %s of type %s", field.Name, value.Type())
			}
		}
	}

	info := types.MarketInfoOf(market)
	view := reflect.ValueOf(info)
	encoded, err := json.Marshal(info)
	require.NoError(t, err)
	var wire map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(encoded, &wire))
	for i := 0; i < source.NumField(); i++ {
		field := source.Type().Field(i)
		original := source.Field(i).Interface()
		copied := view.FieldByName(field.Name)
		require.True(t, copied.IsValid(), "MarketInfo.%s is missing", field.Name)
		switch value := original.(type) {
		case sdk.Int:
			require.IsType(t, sdk.Int{}, copied.Interface(), field.Name)
			require.Equal(t, value.String(), copied.Interface().(sdk.Int).String(), field.Name)
			var decimal string
			require.NoError(t, json.Unmarshal(wire[strings.Split(field.Tag.Get("json"), ",")[0]], &decimal), field.Name)
			require.Equal(t, value.String(), decimal, field.Name)
		case []sdk.Int:
			decimals := make([]string, len(value))
			for j, amount := range value {
				decimals[j] = amount.String()
			}
			var serialized []string
			require.NoError(t, json.Unmarshal(wire["price_snapshots"], &serialized))
			require.Equal(t, decimals, serialized)
			require.Equal(t, value, copied.Interface(), field.Name)
		case types.FeeStrategy:
			require.Equal(t, uint32(value), copied.Interface(), field.Name)
		case types.Generation:
			require.Equal(t, uint32(value), copied.Interface(), field.Name)
		default:
			require.Equal(t, original, copied.Interface(), field.Name)
		}
	}

	market.PriceSnapshots[0] = sdk.NewInt(999)
	market.SnapshotHeights[0] = 999
	require.Equal(t, "0", info.PriceSnapshots[0].String())
	require.Equal(t, uint64(1<<63), info.SnapshotHeights[0])
	info.PriceSnapshots[1] = sdk.NewInt(888)
	info.SnapshotHeights[1] = 888
	require.Equal(t, "123", market.PriceSnapshots[1].String())
	require.Equal(t, uint64(123), market.SnapshotHeights[1])

	for strategy := types.FeeStrategyClaim; strategy <= types.FeeStrategyLpRewards; strategy++ {
		for generation := types.GenerationNative; generation <= types.Generation2; generation++ {
			market.FeeStrategy = strategy
			market.Generation = generation
			market.Paused = false
			info := types.MarketInfoOf(market)
			require.Equal(t, uint32(strategy), info.FeeStrategy)
			require.Equal(t, uint32(generation), info.Generation)
			require.False(t, info.Paused)
		}
	}

	for _, snapshots := range [][]sdk.Int{nil, {}} {
		market.PriceSnapshots = snapshots
		market.SnapshotHeights = nil
		info := types.MarketInfoOf(market)
		require.Empty(t, info.PriceSnapshots)
		require.Empty(t, info.SnapshotHeights)
	}
}
