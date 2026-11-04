package keeper_test

import (
	"math"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	paxapp "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	rootmulti "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/storev2/rootmulti"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repair286Store(t *testing.T, home string) (*rootmulti.Store, keeper.Keeper) {
	t.Helper()
	sc := config.DefaultStateCommitConfig()
	sc.MemIAVLConfig.AsyncCommitBuffer = 0
	ss := config.DefaultStateStoreConfig()
	ss.AsyncWriteBuffer = 0
	rs := rootmulti.NewStore(home, sc, ss, nil)
	key := sdk.NewKVStoreKey(types.StoreKey)
	rs.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	require.NoError(t, rs.LoadLatestVersion())
	k := keeper.NewKeeper(nil, key, codec.NewProtoCodec(codectypes.NewInterfaceRegistry()), home, nil)
	return rs, k
}

func TestRepair_286_UpgradeNameOrder(t *testing.T) {
	type completed struct {
		name   string
		height int64
	}
	for _, tc := range []struct {
		name       string
		upgrades   []completed
		target     int64
		wantName   string
		wantHeight int64
	}{
		{"empty", nil, 18, "", 0},
		{"reverse_name_order", []completed{{"z-old", 10}, {"a-new", 20}}, 18, "a-new", 20},
		{"forward_name_order", []completed{{"a-old", 10}, {"z-new", 20}}, 18, "z-new", 20},
		{"minimum_qualifying_height", []completed{{"z-old", 10}, {"m-far", 30}, {"a-near", 20}}, 18, "a-near", 20},
		{"all_below", []completed{{"z-old", 10}, {"a-new", 20}}, 21, "", 0},
		{"only_below", []completed{{"only", 10}}, 11, "", 0},
		{"exact_height", []completed{{"z-old", 10}, {"a-new", 20}}, 20, "a-new", 20},
		{"before_first", []completed{{"z-old", 10}, {"a-new", 20}}, 1, "z-old", 10},
		{"equal_heights", []completed{{"a-tie", 20}, {"z-tie", 20}, {"zz-old", 10}}, 18, "z-tie", 20},
		{"equal_heights_reverse_insertion", []completed{{"zz-old", 10}, {"z-tie", 20}, {"a-tie", 20}}, 18, "z-tie", 20},
		{"maximum_height", []completed{{"z-old", 10}, {"a-max", math.MaxInt64}}, math.MaxInt64, "a-max", math.MaxInt64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rs, k := repair286Store(t, t.TempDir())
			defer func() { require.NoError(t, rs.Close()) }()
			cache := rs.CacheMultiStore()
			ctx := sdk.NewContext(cache, tmproto.Header{}, false)
			for _, u := range tc.upgrades {
				k.SetDone(ctx.WithBlockHeight(u.height), u.name)
			}
			name, height := k.GetClosestUpgrade(ctx, tc.target)
			require.Equal(t, tc.wantName, name)
			require.Equal(t, tc.wantHeight, height)
			for _, u := range tc.upgrades {
				require.Equal(t, u.height, k.GetDoneHeight(ctx, u.name), "lookup must preserve completed upgrades")
			}
			cache.Write()
			require.Equal(t, int64(1), rs.Commit(true).Version)
			committedCtx := sdk.NewContext(rs, tmproto.Header{Height: 1}, false)
			name, height = k.GetClosestUpgrade(committedCtx, tc.target)
			require.Equal(t, tc.wantName, name)
			require.Equal(t, tc.wantHeight, height)
			for _, u := range tc.upgrades {
				require.Equal(t, u.height, k.GetDoneHeight(committedCtx, u.name), "commit must preserve completed upgrades")
			}
		})
	}
}

func TestRepair_286_HistoricalContextsAcrossRestart(t *testing.T) {
	home := t.TempDir()
	rs, k := repair286Store(t, home)
	for h := int64(1); h <= 21; h++ {
		ctx := sdk.NewContext(rs, tmproto.Header{Height: h}, false)
		if h == 10 {
			k.SetDone(ctx, "z-old")
		}
		if h == 20 {
			k.SetDone(ctx, "a-new")
			k.SetDone(ctx, "m-new")
		}
		require.Equal(t, h, rs.Commit(true).Version)
	}
	require.Equal(t, int64(21), rs.GetStateStore().GetLatestVersion())

	for _, phase := range []string{"before_restart", "after_restart"} {
		t.Run(phase, func(t *testing.T) {
			// Load the real BaseApp check state from the persisted store. Only the
			// upgrade store is needed by these node context providers.
			bapp := baseapp.NewBaseApp("repair-286", dbm.NewMemDB(), nil, nil,
				paxapp.TestAppOpts{})
			bapp.SetCMS(rs)
			require.NoError(t, bapp.LoadLatestVersion())
			app := &paxapp.App{BaseApp: bapp, UpgradeKeeper: k}
			defer func() { require.NoError(t, bapp.Close()) }()
			require.Equal(t, int64(10), k.GetDoneHeight(app.GetCheckCtx(), "z-old"))
			require.Equal(t, int64(20), k.GetDoneHeight(app.GetCheckCtx(), "a-new"))
			require.Equal(t, int64(20), k.GetDoneHeight(app.GetCheckCtx(), "m-new"))

			traceStore := evmkeeper.NewTraceSnapshotStore(32)
			app.EvmKeeper.SetTraceSnapshotStore(traceStore)
			latest := rs.SnapshotSCStore()
			require.NotNil(t, latest)
			traceStore.Put(latest.Version(), latest)
			// Recover historical snapshots from actual SC versions, then retain
			// their COW copies exactly as the trace snapshot store does in-node.
			for _, h := range []int64{10, 18, 20} {
				historical, err := latest.LoadVersion(h, true)
				require.NoError(t, err)
				defer func() { require.NoError(t, historical.Close()) }()
				snapshot := historical.Copy()
				require.NotNil(t, snapshot)
				traceStore.Put(h, snapshot)
			}
			defer traceStore.Close()

			for _, tc := range []struct {
				height     int64
				name       string
				doneHeight int64
			}{
				{10, "z-old", 10},
				{18, "m-new", 20},
				{20, "m-new", 20},
				{21, paxapp.LatestUpgrade, 0},
			} {
				name, doneHeight := k.GetClosestUpgrade(app.GetCheckCtx(), tc.height)
				if tc.doneHeight == 0 {
					require.Empty(t, name)
				} else {
					require.Equal(t, tc.name, name)
				}
				require.Equal(t, tc.doneHeight, doneHeight)
				rpcCtx := app.RPCContextProvider(tc.height)
				require.Equal(t, tc.name, rpcCtx.ClosestUpgradeName())
				require.NotNil(t, traceStore.Get(tc.height), "snapshot hit must be available")
				leased, releaseLease := traceStore.Lease(tc.height)
				require.NotNil(t, leased)
				_, err := rs.CacheMultiStoreFromCommitter(leased)
				require.NoError(t, err, "snapshot conversion must succeed without falling back")
				releaseLease()
				snapCtx, release := app.SnapshotAwareRPCContextProvider()(tc.height)
				require.Equal(t, tc.name, snapCtx.ClosestUpgradeName())
				require.Equal(t, rpcCtx.BlockHeight(), snapCtx.BlockHeight())
				require.True(t, rpcCtx.IsEVM())
				require.True(t, snapCtx.IsEVM())
				require.True(t, rpcCtx.IsTracing())
				require.True(t, snapCtx.IsTracing())
				require.False(t, rpcCtx.IsCheckTx())
				require.False(t, snapCtx.IsCheckTx())
				// At height 18 the historical store has no new upgrade yet;
				// both consumers must select it from the latest check state.
				if tc.height == 18 {
					require.Zero(t, k.GetDoneHeight(rpcCtx, "m-new"))
					require.Zero(t, k.GetDoneHeight(snapCtx, "m-new"))
				}
				release()
			}
			// A snapshot miss and an unsupported snapshot backend take the
			// real node fallback path and must preserve the same selection.
			missCtx, release := app.SnapshotAwareRPCContextProvider()(19)
			require.Equal(t, "m-new", missCtx.ClosestUpgradeName())
			release()
			app.EvmKeeper.SetTraceSnapshotStore(nil)
			fallbackCtx, release := app.SnapshotAwareRPCContextProvider()(18)
			require.Equal(t, "m-new", fallbackCtx.ClosestUpgradeName())
			release()
		})
		if phase == "before_restart" {
			// The preceding subtest closed the BaseApp and both disk stores.
			// Reopen the same directory with fresh keeper, codec and store keys.
			rs, k = repair286Store(t, home)
			require.Equal(t, int64(21), rs.LastCommitID().Version)
		}
	}
}
