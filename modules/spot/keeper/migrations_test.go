package keeper_test

import (
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	pricefeedtest "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestMigrate1to2BuildsActiveLength(t *testing.T) {
	newKeeper := func() (sdk.Context, sdk.StoreKey, *keeper.Keeper) {
		app := testkeeper.EVMTestApp
		ctx, _ := app.GetContextForDeliverTx(nil).CacheContext()
		feed, ctx := pricefeedtest.NewKeeper(app, ctx)
		key := sdk.NewKVStoreKey(types.StoreKey)
		ctx = pricefeedtest.WithPriceFeedStore(ctx, key)
		return ctx, key, keeper.NewKeeper(key, app.BankKeeper, &app.EvmKeeper, feed)
	}
	lists := []struct {
		name   string
		prefix []byte
		count  func(*keeper.Keeper, sdk.Context) (uint64, error)
		at     func(*keeper.Keeper, sdk.Context, uint64) (uint64, error)
	}{
		{"limit", types.ActiveLimitPrefix, (*keeper.Keeper).GetActiveLimitOrderCount, (*keeper.Keeper).ActiveLimitOrderID},
		{"stop-limit", types.ActiveStopLimitPrefix, (*keeper.Keeper).GetActiveStopLimitOrderCount, (*keeper.Keeper).ActiveStopLimitOrderIDs},
		{"stop-loss", types.ActiveStopLossPrefix, (*keeper.Keeper).GetActiveStopLossOrderCount, (*keeper.Keeper).ActiveStopLossOrderIDs},
	}
	for _, tc := range []struct {
		name string
		ids  [][]uint64
	}{
		{"version-1", [][]uint64{{90, 3, 41, 8, 22}, {}, {72, 6}}},
		{"stop-limit", [][]uint64{{}, {81, 5, 37}, {}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, key, k := newKeeper()
			store := ctx.KVStore(key)
			for i, list := range lists {
				require.False(t, store.Has(types.ActiveLengthKey(list.prefix)))
				for position, id := range tc.ids[i] {
					store.Set(binary.BigEndian.AppendUint64(append([]byte(nil), list.prefix...), id),
						binary.BigEndian.AppendUint64(nil, uint64(position)))
					require.False(t, store.Has(types.ActivePositionKey(list.prefix, uint64(position))))
				}
			}
			require.NoError(t, keeper.NewMigrator(k).Migrate1to2(ctx))
			for i, list := range lists {
				count, err := list.count(k, ctx)
				require.NoError(t, err)
				require.Equal(t, uint64(len(tc.ids[i])), count, list.name)
				require.Equal(t, binary.BigEndian.AppendUint64(nil, count), store.Get(types.ActiveLengthKey(list.prefix)))
				for position, want := range tc.ids[i] {
					id, err := list.at(k, ctx, uint64(position))
					require.NoError(t, err)
					require.Equal(t, want, id, list.name)
					require.Equal(t, binary.BigEndian.AppendUint64(nil, want), store.Get(types.ActivePositionKey(list.prefix, uint64(position))))
					require.Equal(t, binary.BigEndian.AppendUint64(nil, uint64(position)),
						store.Get(binary.BigEndian.AppendUint64(append([]byte(nil), list.prefix...), want)))
				}
				_, err = list.at(k, ctx, count)
				require.Error(t, err)
			}
			message, broken := keeper.ActiveIndexInvariant(k)(ctx)
			require.False(t, broken, message)
		})
	}
	for _, list := range lists {
		for _, tc := range []struct {
			name  string
			id    uint64
			value []byte
		}{
			{"duplicate", 19, binary.BigEndian.AppendUint64(nil, 1)},
			{"out-of-range", 19, binary.BigEndian.AppendUint64(nil, 2)},
			{"max-position", 19, binary.BigEndian.AppendUint64(nil, math.MaxUint64)},
			{"malformed-value", 19, []byte{1}},
			{"zero-id", 0, binary.BigEndian.AppendUint64(nil, 2)},
			{"malformed-key", 19, binary.BigEndian.AppendUint64(nil, 0)},
		} {
			t.Run(fmt.Sprintf("%s/%s", list.name, tc.name), func(t *testing.T) {
				ctx, key, k := newKeeper()
				store := ctx.KVStore(key)
				for _, valid := range lists {
					for position, id := range []uint64{19, 3} {
						store.Set(binary.BigEndian.AppendUint64(append([]byte(nil), valid.prefix...), id),
							binary.BigEndian.AppendUint64(nil, uint64(position)))
					}
				}
				badKey := binary.BigEndian.AppendUint64(append([]byte(nil), list.prefix...), tc.id)
				if tc.name == "malformed-key" {
					badKey = append(badKey, 0)
				}
				store.Set(badKey, tc.value)
				require.Error(t, keeper.NewMigrator(k).Migrate1to2(ctx))
				require.Equal(t, tc.value, store.Get(badKey))
				for _, untouched := range lists {
					require.False(t, store.Has(types.ActiveLengthKey(untouched.prefix)))
					it := sdk.KVStorePrefixIterator(store, append(append([]byte(nil), types.ActivePositionPrefix...), untouched.prefix...))
					valid := it.Valid()
					it.Close()
					require.False(t, valid)
				}
			})
		}
	}
}

func TestMigrate1to2ExportRoundTrip(t *testing.T) {
	f := newStopFixture(t)
	expiry := uint64(f.ctx.BlockTime().Unix()) + 3600
	var limitIDs, stopLimitIDs, stopLossIDs []uint64
	for i := 0; i < 2; i++ {
		id, _, err := f.k.PlaceLimit(f.ctx, f.user, f.stable, f.base, sdk.NewInt(1_000_000_000), stopTestPrice(60), expiry, types.OrderTypeLimitSell)
		require.NoError(t, err)
		limitIDs = append(limitIDs, id)
		id, err = f.k.PlaceStopLimit(f.ctx, f.user, f.stable, f.base, sdk.NewInt(100_000_000), stopTestPrice(55), stopTestPrice(60), expiry, types.OrderTypeStopLimitBuy)
		require.NoError(t, err)
		stopLimitIDs = append(stopLimitIDs, id)
		id, err = f.k.PlaceStopLoss(f.ctx, f.user, f.base, f.stable, sdk.NewInt(1_000_000_000), stopTestPrice(40), expiry)
		require.NoError(t, err)
		stopLossIDs = append(stopLossIDs, id)
	}
	want := f.k.ExportGenesis(f.ctx)
	require.Len(t, want.LimitOrders, 2)
	require.Len(t, want.StopLimitOrders, 2)
	require.Len(t, want.StopLossOrders, 2)
	require.Equal(t, limitIDs, want.ActiveLimitOrders)
	require.Equal(t, stopLimitIDs, want.ActiveStopLimitOrders)
	require.Equal(t, stopLossIDs, want.ActiveStopLossOrders)

	store := f.ctx.KVStore(f.key)
	for _, index := range []struct {
		prefix []byte
		count  int
	}{
		{types.ActivePositionPrefix, 6},
		{types.ActiveLengthPrefix, 3},
	} {
		var keys [][]byte
		iterator := sdk.KVStorePrefixIterator(store, index.prefix)
		for ; iterator.Valid(); iterator.Next() {
			keys = append(keys, append([]byte(nil), iterator.Key()...))
		}
		iterator.Close()
		require.Len(t, keys, index.count)
		for _, key := range keys {
			store.Delete(key)
			require.False(t, store.Has(key))
		}
	}
	message, broken := keeper.ActiveIndexInvariant(f.k)(f.ctx)
	require.True(t, broken, message)
	require.NoError(t, keeper.NewMigrator(f.k).Migrate1to2(f.ctx))
	exported := f.k.ExportGenesis(f.ctx)
	require.Equal(t, want, exported)
	require.NoError(t, exported.Validate())
	message, broken = keeper.ActiveIndexInvariant(f.k)(f.ctx)
	require.False(t, broken, message)

	app := testkeeper.EVMTestApp
	feed, ctx := pricefeedtest.NewKeeper(app, f.ctx)
	key := sdk.NewKVStoreKey(types.StoreKey)
	ctx = pricefeedtest.WithPriceFeedStore(ctx, key)
	k := keeper.NewKeeper(key, app.BankKeeper, &app.EvmKeeper, feed)
	iterator := sdk.KVStorePrefixIterator(ctx.KVStore(key), nil)
	empty := !iterator.Valid()
	iterator.Close()
	require.True(t, empty)
	k.InitGenesis(ctx, *exported)
	require.Equal(t, exported, k.ExportGenesis(ctx))
	message, broken = keeper.ActiveIndexInvariant(k)(ctx)
	require.False(t, broken, message)
}
