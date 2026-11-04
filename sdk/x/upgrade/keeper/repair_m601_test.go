package keeper_test

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	oracletypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/oracle/types"
	tokenfactorytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
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

func repairM601Keeper(home string) keeper.Keeper {
	return keeper.NewKeeper(nil, sdk.NewKVStoreKey(types.StoreKey), codec.NewProtoCodec(codectypes.NewInterfaceRegistry()), home, nil)
}

func repairM601Store(home string) (*rootmulti.Store, keeper.Keeper) {
	sc := config.DefaultStateCommitConfig()
	sc.MemIAVLConfig.AsyncCommitBuffer = 0
	ss := config.DefaultStateStoreConfig()
	ss.AsyncWriteBuffer = 0
	rs := rootmulti.NewStore(home, sc, ss, nil)
	key := sdk.NewKVStoreKey(types.StoreKey)
	rs.MountStoreWithDB(key, storetypes.StoreTypeIAVL, nil)
	return rs, keeper.NewKeeper(nil, key, codec.NewProtoCodec(codectypes.NewInterfaceRegistry()), home, nil)
}

func TestRepairM_601_AtomicHandoff(t *testing.T) {
	home := t.TempDir()
	k := repairM601Keeper(home)
	old := types.Plan{Name: "old-plan", Height: 20, Info: "old handoff"}
	newPlan := types.Plan{Name: "new-plan", Height: 30, Info: strings.Repeat("new handoff\n", 8192)}
	require.NoError(t, k.DumpUpgradeInfoWithInfoToDisk(old.Height, old.Name, old.Info))
	live, err := k.GetUpgradeInfoPath()
	require.NoError(t, err)
	oldFile, err := os.Open(live)
	require.NoError(t, err)
	defer oldFile.Close()
	oldStat, err := oldFile.Stat()
	require.NoError(t, err)
	oldBytes, err := os.ReadFile(live)
	require.NoError(t, err)

	// Readers must observe a complete tuple while the real producer publishes.
	ready := make(chan struct{})
	stop := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		close(ready)
		for {
			data, err := os.ReadFile(live)
			if err != nil {
				result <- err
				return
			}
			var got types.Plan
			if err := json.Unmarshal(data, &got); err != nil {
				result <- err
				return
			}
			if got != old && got != newPlan {
				result <- fmt.Errorf("reader observed mixed handoff: name=%q height=%d info length=%d", got.Name, got.Height, len(got.Info))
				return
			}
			select {
			case <-stop:
				result <- nil
				return
			default:
			}
		}
	}()
	<-ready
	var writeErr error
	for i := 0; i < 16; i++ {
		plan := old
		if i%2 == 1 {
			plan = newPlan
		}
		if writeErr = k.DumpUpgradeInfoWithInfoToDisk(plan.Height, plan.Name, plan.Info); writeErr != nil {
			break
		}
	}
	close(stop)
	require.NoError(t, <-result)
	require.NoError(t, writeErr)

	// A handle held by the old binary must retain the old inode and contents.
	retained, err := io.ReadAll(oldFile)
	require.NoError(t, err)
	require.Equal(t, oldBytes, retained)
	newStat, err := os.Stat(live)
	require.NoError(t, err)
	require.False(t, os.SameFile(oldStat, newStat), "publication must replace, never truncate, the live file")
	require.Equal(t, os.FileMode(0o600), newStat.Mode().Perm())
	siblings, err := filepath.Glob(live + ".tmp-*")
	require.NoError(t, err)
	require.Empty(t, siblings)

	k = repairM601Keeper(home)
	got, err := k.ReadUpgradeInfoFromDisk()
	require.NoError(t, err)
	require.Equal(t, storetypes.UpgradeInfo{Name: newPlan.Name, Height: newPlan.Height}, got)
	published, err := os.ReadFile(live)
	require.NoError(t, err)
	var fullPlan types.Plan
	require.NoError(t, json.Unmarshal(published, &fullPlan))
	require.Equal(t, newPlan, fullPlan)
}

func TestRepairM_601_InterruptedHandoffStartup(t *testing.T) {
	old := types.Plan{Name: "1.0.4beta", Height: 2, Info: "old oracle migration"}
	newPlan := types.Plan{Name: "1.1.1beta", Height: 2, Info: "new tokenfactory migration"}
	for _, tc := range []struct {
		name         string
		oldPublished bool
		stage        string
		publishNew   bool
		want         types.Plan
		added        string
	}{
		{"first_write_interrupted", false, "partial", false, types.Plan{}, ""},
		{"empty_temporary", true, "empty", false, old, oracletypes.StoreKey},
		{"partial_temporary", true, "partial", false, old, oracletypes.StoreKey},
		{"synced_before_rename", true, "complete", false, old, oracletypes.StoreKey},
		{"published_new", true, "partial", true, newPlan, tokenfactorytypes.StoreKey},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			rs, k := repairM601Store(home)
			require.NoError(t, rs.LoadLatestVersion())
			k.SetDone(sdk.NewContext(rs, tmproto.Header{Height: 1}, false), "persisted-marker")
			require.Equal(t, int64(1), rs.Commit(true).Version)
			require.NoError(t, rs.Close())
			if tc.oldPublished {
				require.NoError(t, k.DumpUpgradeInfoWithInfoToDisk(old.Height, old.Name, old.Info))
			}

			// Fault fixture: retain a prefix (or all) of an actual producer's
			// output as the temporary sibling left by an interrupted write.
			producer := repairM601Keeper(t.TempDir())
			require.NoError(t, producer.DumpUpgradeInfoWithInfoToDisk(newPlan.Height, newPlan.Name, newPlan.Info))
			source, err := producer.GetUpgradeInfoPath()
			require.NoError(t, err)
			data, err := os.ReadFile(source)
			require.NoError(t, err)
			switch tc.stage {
			case "empty":
				data = data[:0]
			case "partial":
				data = data[:len(data)/2]
			}
			live, err := k.GetUpgradeInfoPath()
			require.NoError(t, err)
			staged, err := os.CreateTemp(filepath.Dir(live), keeper.UpgradeInfoFileName+".tmp-*")
			require.NoError(t, err)
			_, err = staged.Write(data)
			require.NoError(t, err)
			require.NoError(t, staged.Sync())
			require.NoError(t, staged.Close())
			if tc.publishNew {
				require.NoError(t, k.DumpUpgradeInfoWithInfoToDisk(newPlan.Height, newPlan.Name, newPlan.Info))
			}

			// Reopen both the disk stores and keeper, as a new binary does.
			rs, k = repairM601Store(home)
			bapp := baseapp.NewBaseApp(t.Name(), dbm.NewMemDB(), nil, nil, paxapp.TestAppOpts{})
			bapp.SetCMS(rs)
			defer func() { require.NoError(t, bapp.Close()) }()
			got, err := k.ReadUpgradeInfoFromDisk()
			require.NoError(t, err)
			require.Equal(t, storetypes.UpgradeInfo{Name: tc.want.Name, Height: tc.want.Height}, got)
			if tc.want.Name != "" {
				published, err := os.ReadFile(live)
				require.NoError(t, err)
				var fullPlan types.Plan
				require.NoError(t, json.Unmarshal(published, &fullPlan))
				require.Equal(t, tc.want, fullPlan)
			} else {
				_, err := os.Stat(live)
				require.True(t, os.IsNotExist(err))
			}
			var addedKey *sdk.KVStoreKey
			if tc.added != "" {
				addedKey = sdk.NewKVStoreKey(tc.added)
				rs.MountStoreWithDB(addedKey, storetypes.StoreTypeIAVL, nil)
			}
			app := &paxapp.App{BaseApp: bapp, UpgradeKeeper: k}
			app.SetStoreUpgradeHandlers()
			require.NoError(t, app.LoadLatestVersion(), "real node startup must apply the recovered plan's store additions")
			require.Equal(t, int64(1), k.GetDoneHeight(app.GetCheckCtx(), "persisted-marker"))
			if addedKey != nil {
				cache := rs.CacheMultiStore()
				cache.GetKVStore(addedKey).Set([]byte("migration"), []byte(tc.want.Name))
				cache.Write()
				require.Equal(t, int64(2), rs.Commit(true).Version)
				require.Equal(t, []byte(tc.want.Name), rs.GetKVStore(addedKey).Get([]byte("migration")))
			}
			orphan, err := os.ReadFile(staged.Name())
			require.NoError(t, err)
			require.Equal(t, data, orphan, "startup must not promote an uncommitted temporary sibling")
		})
	}
}

func TestRepairM_601_WriteFailureAndCorruptPublishedFile(t *testing.T) {
	home := t.TempDir()
	k := repairM601Keeper(home)
	live, err := k.GetUpgradeInfoPath()
	require.NoError(t, err)
	// A directory at the destination forces a real rename failure after staging.
	require.NoError(t, os.Mkdir(live, 0o700))
	marker := filepath.Join(live, "preserved")
	require.NoError(t, os.WriteFile(marker, []byte("keep"), 0o600))
	require.Error(t, k.DumpUpgradeInfoToDisk(2, "new"))
	data, err := os.ReadFile(marker)
	require.NoError(t, err)
	require.Equal(t, []byte("keep"), data)
	siblings, err := filepath.Glob(live + ".tmp-*")
	require.NoError(t, err)
	require.Empty(t, siblings, "failed publication must clean its own temporary file")
	require.NoError(t, os.Remove(marker))
	require.NoError(t, os.Remove(live))
	require.NoError(t, k.DumpUpgradeInfoToDisk(2, "valid"))
	require.NoError(t, os.Truncate(live, 1))
	_, err = repairM601Keeper(home).ReadUpgradeInfoFromDisk()
	require.Error(t, err, "corruption of an already published file must remain visible")
}

type repairM601LookupCase struct {
	name   string
	plans  []types.Plan
	middle string
}

func repairM601LookupCases() []repairM601LookupCase {
	return []repairM601LookupCase{
		{"original_order", []types.Plan{{Name: "upgrade1", Height: 15}, {Name: "upgrade2", Height: 20}, {Name: "upgrade3", Height: 25}}, "upgrade2"},
		{"z_old_a_new", []types.Plan{{Name: "z-old", Height: 15}, {Name: "a-new", Height: 20}, {Name: "m-last", Height: 25}}, "a-new"},
		{"reverse_insertion", []types.Plan{{Name: "m-last", Height: 25}, {Name: "a-new", Height: 20}, {Name: "z-old", Height: 15}}, "a-new"},
		{"mixed_insertion", []types.Plan{{Name: "a-new", Height: 20}, {Name: "z-old", Height: 15}, {Name: "m-last", Height: 25}}, "a-new"},
		{"equal_height", []types.Plan{{Name: "z-old", Height: 15}, {Name: "a-new", Height: 20}, {Name: "c-tie", Height: 20}, {Name: "m-last", Height: 25}}, "c-tie"},
		{"equal_height_reverse", []types.Plan{{Name: "m-last", Height: 25}, {Name: "c-tie", Height: 20}, {Name: "a-new", Height: 20}, {Name: "z-old", Height: 15}}, "c-tie"},
		{"equal_height_mixed", []types.Plan{{Name: "a-new", Height: 20}, {Name: "z-old", Height: 15}, {Name: "m-last", Height: 25}, {Name: "c-tie", Height: 20}}, "c-tie"},
	}
}

func repairM601AssertLookup(t *testing.T, k keeper.Keeper, ctx sdk.Context, tc repairM601LookupCase) {
	t.Helper()
	first, last := "z-old", "m-last"
	if tc.name == "original_order" {
		first, last = "upgrade1", "upgrade3"
	}
	for _, want := range []struct {
		target int64
		name   string
		height int64
	}{
		{0, first, 15}, {10, first, 15}, {15, first, 15},
		{16, tc.middle, 20}, {18, tc.middle, 20}, {20, tc.middle, 20},
		{21, last, 25}, {22, last, 25}, {25, last, 25}, {26, "", 0}, {math.MaxInt64, "", 0},
	} {
		name, height := k.GetClosestUpgrade(ctx, want.target)
		require.Equal(t, want.name, name, "target %d", want.target)
		require.Equal(t, want.height, height, "target %d", want.target)
	}
	for _, plan := range tc.plans {
		require.Equal(t, plan.Height, k.GetDoneHeight(ctx, plan.Name), "lookup must preserve completion records")
	}
}

func TestRepairM_601_LookupOrdering(t *testing.T) {
	for _, tc := range repairM601LookupCases() {
		t.Run(tc.name, func(t *testing.T) {
			home := t.TempDir()
			rs, k := repairM601Store(home)
			require.NoError(t, rs.LoadLatestVersion())
			cache := rs.CacheMultiStore()
			ctx := sdk.NewContext(cache, tmproto.Header{}, false)
			name, height := k.GetClosestUpgrade(ctx, 18)
			require.Empty(t, name)
			require.Zero(t, height)
			for _, plan := range tc.plans {
				k.SetDone(ctx.WithBlockHeight(plan.Height), plan.Name)
			}
			repairM601AssertLookup(t, k, ctx, tc)
			cache.Write()
			require.Equal(t, int64(1), rs.Commit(true).Version)
			require.NoError(t, rs.Close())
			rs, k = repairM601Store(home)
			defer func() { require.NoError(t, rs.Close()) }()
			require.NoError(t, rs.LoadLatestVersion())
			repairM601AssertLookup(t, k, sdk.NewContext(rs, tmproto.Header{Height: 1}, false), tc)
		})
	}
}

func TestRepairM_601_HistoricalConsumerRestart(t *testing.T) {
	home := t.TempDir()
	rs, k := repairM601Store(home)
	require.NoError(t, rs.LoadLatestVersion())
	for h := int64(1); h <= 26; h++ {
		ctx := sdk.NewContext(rs, tmproto.Header{Height: h}, false)
		switch h {
		case 15:
			k.SetDone(ctx, "z-old")
		case 20:
			k.SetDone(ctx, "a-new")
			k.SetDone(ctx, "c-tie")
		case 25:
			k.SetDone(ctx, "m-last")
		}
		require.Equal(t, h, rs.Commit(true).Version)
	}
	require.NoError(t, rs.Close())
	for restart := 0; restart < 2; restart++ {
		rs, k = repairM601Store(home)
		bapp := baseapp.NewBaseApp(t.Name(), dbm.NewMemDB(), nil, nil, paxapp.TestAppOpts{})
		bapp.SetCMS(rs)
		require.NoError(t, bapp.LoadLatestVersion())
		app := &paxapp.App{BaseApp: bapp, UpgradeKeeper: k}
		for _, tc := range []struct {
			height int64
			want   string
		}{
			{10, "z-old"}, {15, "z-old"}, {18, "c-tie"}, {20, "c-tie"},
			{22, "m-last"}, {25, "m-last"}, {26, paxapp.LatestUpgrade},
		} {
			ctx := app.RPCContextProvider(tc.height)
			require.Equal(t, tc.want, ctx.ClosestUpgradeName())
			require.Equal(t, tc.height, ctx.BlockHeight())
			require.True(t, ctx.IsEVM())
			require.True(t, ctx.IsTracing())
			require.False(t, ctx.IsCheckTx())
			fallback, release := app.SnapshotAwareRPCContextProvider()(tc.height)
			require.Equal(t, tc.want, fallback.ClosestUpgradeName())
			release()
			if tc.height == 18 {
				require.Zero(t, k.GetDoneHeight(ctx, "c-tie"), "consumer must choose from latest persisted state, not the historical store")
			}
		}
		require.NoError(t, bapp.Close())
	}
}
