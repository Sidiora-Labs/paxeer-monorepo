// Proves: Keeper.PlaceLimit/PlaceStopLimit/PlaceStopLoss and Migrator.Migrate1to2 -> SDK CacheMultiStore and LevelDB spot keys -> Keeper active-order readers and ActiveIndexInvariant.
//
// The market fixture supplies real bank/EVM keepers; orders are produced by
// the exported keeper methods directly into the disk-backed spot store. Only
// derived v2 keys are removed to exercise the persisted v1 representation.
// The proof repairs migrations.go:Migrate1to2 retaining stale reverse positions
// and invariants.go:ActiveIndexInvariant accepting a zero order ID. The former
// must rebuild derived positions; the latter must reject the invalid ID even
// when the forward and reverse entries agree.
package keeper_test

import (
	"encoding/binary"
	"math/big"
	"testing"

	feedkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/spot/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/cachemulti"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func proofSpotOpenStore(t *testing.T, directory string) (*rootmulti.Store, sdk.StoreKey, func()) {
	t.Helper()
	db, err := dbm.NewDB("spot-proof", dbm.GoLevelDBBackend, directory)
	require.NoError(t, err)
	root := rootmulti.NewStore(db)
	closed := false
	closeStore := func() {
		t.Helper()
		if !closed {
			require.NoError(t, root.Close())
			closed = true
		}
	}
	t.Cleanup(closeStore)
	key := sdk.NewKVStoreKey(types.StoreKey)
	root.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	require.NoError(t, root.LoadLatestVersion())
	return root, key, closeStore
}

func proofSpotSnapshot(t *testing.T, store sdk.KVStore) map[string][]byte {
	t.Helper()
	out := make(map[string][]byte)
	iterator := store.Iterator(nil, nil)
	for ; iterator.Valid(); iterator.Next() {
		out[string(iterator.Key())] = append([]byte(nil), iterator.Value()...)
	}
	require.NoError(t, iterator.Close())
	return out
}

func proofSpotVersion1(t *testing.T, store sdk.KVStore) {
	t.Helper()
	for key := range proofSpotSnapshot(t, store) {
		if key[0] == types.ActivePositionPrefix[0] || key[0] == types.ActiveLengthPrefix[0] {
			store.Delete([]byte(key))
		}
	}
}

func TestProofSpotStorage(t *testing.T) {
	f := newStopFixture(t)
	initial := f.k.ExportGenesis(f.ctx)
	directory := t.TempDir()
	root, key, closeStore := proofSpotOpenStore(t, directory)

	// Compose actual application stores with the persistent spot store using
	// the SDK's production multi-store, including its real cache/write path.
	parents := make(map[storetypes.StoreKey]storetypes.CacheWrapper)
	keys := make(map[string]storetypes.StoreKey)
	for _, appKey := range f.ctx.MultiStore().StoreKeys() {
		if appKey.Name() != types.StoreKey && appKey.Name() != f.feedKey.Name() {
			parents[appKey] = f.ctx.MultiStore().GetKVStore(appKey)
			keys[appKey.Name()] = appKey
		}
	}
	parents[f.feedKey] = f.ctx.KVStore(f.feedKey)
	keys[f.feedKey.Name()] = f.feedKey
	parents[key] = root.GetKVStore(key)
	keys[key.Name()] = key
	metadata := dbm.NewMemDB()
	t.Cleanup(func() { require.NoError(t, metadata.Close()) })
	multi := cachemulti.NewStore(metadata, parents, keys, nil, nil, nil, 0)
	f.ctx = f.ctx.WithMultiStore(multi)
	f.key = key
	app := testkeeper.EVMTestApp
	feed := feedkeeper.NewKeeper(f.feedKey, app.StakingKeeper, app.AccountKeeper, &app.EvmKeeper)
	f.k = keeper.NewKeeper(key, app.BankKeeper, &app.EvmKeeper, feed)
	f.k.InitGenesis(f.ctx, *initial)
	f.tokenCall("approve(address,uint256)", common.HexToAddress(types.PECOROrdersAddress), big.NewInt(1_000_000_000_000))

	var limitIDs, stopLimitIDs, stopLossIDs []uint64
	expiry := uint64(f.ctx.BlockTime().Unix()) + 3600
	for i := 0; i < 3; i++ {
		id, _, err := f.k.PlaceLimit(f.ctx, f.user, f.stable, f.base, sdk.NewInt(100_000_000), stopTestPrice(60), expiry, types.OrderTypeLimitSell)
		require.NoError(t, err)
		limitIDs = append(limitIDs, id)
		id, err = f.k.PlaceStopLimit(f.ctx, f.user, f.stable, f.base, sdk.NewInt(100_000_000), stopTestPrice(55), stopTestPrice(60), expiry, types.OrderTypeStopLimitBuy)
		require.NoError(t, err)
		stopLimitIDs = append(stopLimitIDs, id)
		id, err = f.k.PlaceStopLoss(f.ctx, f.user, f.base, f.stable, sdk.NewInt(100_000_000), stopTestPrice(40), expiry)
		require.NoError(t, err)
		stopLossIDs = append(stopLossIDs, id)
	}
	_, err := f.k.CancelLimit(f.ctx, f.user, limitIDs[0])
	require.NoError(t, err)
	require.NoError(t, f.k.CancelStopLimit(f.ctx, f.user, stopLimitIDs[0]))
	require.NoError(t, f.k.CancelStopLoss(f.ctx, f.user, stopLossIDs[0]))
	want := f.k.ExportGenesis(f.ctx)
	require.NoError(t, want.Validate())
	require.Equal(t, []uint64{limitIDs[2], limitIDs[1]}, want.ActiveLimitOrders)
	require.Equal(t, []uint64{stopLimitIDs[2], stopLimitIDs[1]}, want.ActiveStopLimitOrders)
	require.Equal(t, []uint64{stopLossIDs[2], stopLossIDs[1]}, want.ActiveStopLossOrders)
	wantBytes := proofSpotSnapshot(t, f.ctx.KVStore(key))
	proofSpotVersion1(t, f.ctx.KVStore(key))
	multi.Write()
	root.Commit(true)
	closeStore()
	// Drop the producer's component and caches before reopening its database.
	f.ctx = sdk.Context{}
	f.k = nil
	multi = cachemulti.Store{}
	parents = nil

	root, key, closeStore = proofSpotOpenStore(t, directory)
	ctx := sdk.NewContext(root, app.GetContextForDeliverTx(nil).BlockHeader(), false)
	k := keeper.NewKeeper(key, app.BankKeeper, &app.EvmKeeper, feed)
	message, broken := keeper.ActiveIndexInvariant(k)(ctx)
	require.True(t, broken, message)
	require.NoError(t, keeper.NewMigrator(k).Migrate1to2(ctx))
	migrated := proofSpotSnapshot(t, ctx.KVStore(key))
	require.Equal(t, wantBytes, migrated)
	root.Commit(true)
	closeStore()
	ctx = sdk.Context{}
	k = nil

	root, key, closeStore = proofSpotOpenStore(t, directory)
	ctx = sdk.NewContext(root, app.GetContextForDeliverTx(nil).BlockHeader(), false)
	k = keeper.NewKeeper(key, app.BankKeeper, &app.EvmKeeper, feed)
	store := ctx.KVStore(key)
	require.Equal(t, migrated, proofSpotSnapshot(t, store))
	require.Equal(t, want, k.ExportGenesis(ctx))
	message, broken = keeper.ActiveIndexInvariant(k)(ctx)
	require.False(t, broken, message)
	lists := []struct {
		name   string
		prefix []byte
		ids    []uint64
		count  func(*keeper.Keeper, sdk.Context) (uint64, error)
		at     func(*keeper.Keeper, sdk.Context, uint64) (uint64, error)
	}{
		{"limit", types.ActiveLimitPrefix, want.ActiveLimitOrders, (*keeper.Keeper).GetActiveLimitOrderCount, (*keeper.Keeper).ActiveLimitOrderID},
		{"stop-limit", types.ActiveStopLimitPrefix, want.ActiveStopLimitOrders, (*keeper.Keeper).GetActiveStopLimitOrderCount, (*keeper.Keeper).ActiveStopLimitOrderIDs},
		{"stop-loss", types.ActiveStopLossPrefix, want.ActiveStopLossOrders, (*keeper.Keeper).GetActiveStopLossOrderCount, (*keeper.Keeper).ActiveStopLossOrderIDs},
	}
	for _, list := range lists {
		t.Run(list.name, func(t *testing.T) {
			count, err := list.count(k, ctx)
			require.NoError(t, err)
			require.Equal(t, uint64(len(list.ids)), count)
			// Check the byte-level storage contract independently of key helpers.
			require.Equal(t, binary.BigEndian.AppendUint64(nil, count), store.Get([]byte{0x16, list.prefix[0]}))
			for position, wantID := range list.ids {
				id, err := list.at(k, ctx, uint64(position))
				require.NoError(t, err)
				require.Equal(t, wantID, id)
				reverse := binary.BigEndian.AppendUint64([]byte{0x15, list.prefix[0]}, uint64(position))
				require.Equal(t, binary.BigEndian.AppendUint64(nil, wantID), store.Get(reverse))
				forward := binary.BigEndian.AppendUint64(append([]byte(nil), list.prefix...), wantID)
				require.Equal(t, binary.BigEndian.AppendUint64(nil, uint64(position)), store.Get(forward))
			}
			_, err = list.at(k, ctx, count)
			require.ErrorContains(t, err, "out of bounds")

			// A malformed producer input is refused without publishing even the
			// preceding lists' derived writes from the migration's cache.
			bad, _ := ctx.CacheContext()
			badStore := bad.KVStore(key)
			proofSpotVersion1(t, badStore)
			badStore.Set(stopTestKey(list.prefix, list.ids[0]), []byte{1})
			before := proofSpotSnapshot(t, badStore)
			require.Error(t, keeper.NewMigrator(k).Migrate1to2(bad))
			require.Equal(t, before, proofSpotSnapshot(t, badStore))
			message, broken := keeper.ActiveIndexInvariant(k)(bad)
			require.True(t, broken, message)

			// A zero ID used to pass when both sides of the index agreed.
			zero, _ := ctx.CacheContext()
			zeroStore := zero.KVStore(key)
			zeroStore.Delete(stopTestKey(list.prefix, list.ids[0]))
			zeroStore.Set(stopTestKey(list.prefix, 0), binary.BigEndian.AppendUint64(nil, 0))
			zeroStore.Set(types.ActivePositionKey(list.prefix, 0), binary.BigEndian.AppendUint64(nil, 0))
			message, broken = keeper.ActiveIndexInvariant(k)(zero)
			require.True(t, broken, message)
			require.Contains(t, message, "zero order id")
			before = proofSpotSnapshot(t, zeroStore)
			require.ErrorContains(t, keeper.NewMigrator(k).Migrate1to2(zero), "invalid active index entry")
			require.Equal(t, before, proofSpotSnapshot(t, zeroStore))
		})
	}

	// Stale derived entries must not survive rebuilding from forward entries.
	for _, list := range lists {
		store.Set(types.ActivePositionKey(list.prefix, uint64(len(list.ids))), binary.BigEndian.AppendUint64(nil, 999))
		store.Set([]byte{0x15, list.prefix[0], 0xff}, []byte{1})
	}
	message, broken = keeper.ActiveIndexInvariant(k)(ctx)
	require.True(t, broken, message)
	require.NoError(t, keeper.NewMigrator(k).Migrate1to2(ctx))
	require.Equal(t, migrated, proofSpotSnapshot(t, store))
	message, broken = keeper.ActiveIndexInvariant(k)(ctx)
	require.False(t, broken, message)
	require.NoError(t, keeper.NewMigrator(k).Migrate1to2(ctx))
	require.Equal(t, migrated, proofSpotSnapshot(t, store), "repeated migration must be byte-for-byte idempotent")
	root.Commit(true)
	closeStore()
}
