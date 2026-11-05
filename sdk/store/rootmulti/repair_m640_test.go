package rootmulti_test

import (
	"fmt"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repairM640DBState(t *testing.T, db dbm.DB) map[string][]byte {
	t.Helper()
	it, err := db.Iterator(nil, nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, it.Close()) }()
	state := make(map[string][]byte)
	for ; it.Valid(); it.Next() {
		state[string(it.Key())] = append([]byte(nil), it.Value()...)
	}
	require.NoError(t, it.Error())
	return state
}

func TestRepairM_640_RollbackRestartRecovery(t *testing.T) {
	for _, typ := range []storetypes.StoreType{storetypes.StoreTypeIAVL, storetypes.StoreTypeDB} {
		for _, target := range []int64{1, 2, 3, 4, 0, -1} {
			t.Run(fmt.Sprintf("store=%d/target=%d", typ, target), func(t *testing.T) {
				dir := t.TempDir()
				bank := storetypes.NewKVStoreKey("bank")
				auth := storetypes.NewKVStoreKey("auth")
				transient := storetypes.NewTransientStoreKey("transient")
				var store *rootmulti.Store
				var db dbm.DB
				t.Cleanup(func() {
					if store != nil {
						require.NoError(t, store.Close())
					}
				})
				open := func() {
					var err error
					db, err = dbm.NewGoLevelDB("state", dir)
					require.NoError(t, err)
					store = rootmulti.NewStore(db)
					store.MountStoreWithDB(bank, typ, nil)
					store.MountStoreWithDB(auth, typ, nil)
					store.MountStoreWithDB(transient, storetypes.StoreTypeTransient, nil)
					require.NoError(t, store.LoadLatestVersion())
				}
				open()

				var commits []storetypes.CommitID
				for height := int64(1); height <= 3; height++ {
					cache := store.CacheMultiStore()
					cache.GetKVStore(bank).Set([]byte("balance"), []byte(fmt.Sprint(height)))
					cache.GetKVStore(auth).Set([]byte("sequence"), []byte(fmt.Sprint(10*height)))
					if height == 1 {
						cache.GetKVStore(bank).Set([]byte("removed"), []byte("at-height-one"))
					} else {
						cache.GetKVStore(bank).Delete([]byte("removed"))
						cache.GetKVStore(bank).Set([]byte("added"), []byte("after-height-one"))
					}
					cache.Write()
					commit := store.Commit(true)
					require.Equal(t, height, commit.Version)
					commits = append(commits, commit)
				}
				store.GetKVStore(transient).Set([]byte("pending"), []byte("keep-on-refusal"))
				before := repairM640DBState(t, db)
				commitInfo, err := store.LastCommitInfo().Marshal()
				require.NoError(t, err)

				var api storetypes.CommitMultiStore = store
				err = api.RollbackToVersion(target)
				require.Error(t, err)
				if target > 0 {
					var unsupported *rootmulti.RollbackUnsupportedError
					require.ErrorAs(t, err, &unsupported)
					require.Equal(t, target, unsupported.Target)
					require.ErrorIs(t, err, rootmulti.ErrVersionedStateUnsupported)
					require.Contains(t, err.Error(), "sdk/storev2/rootmulti")
				} else {
					require.EqualError(t, err, fmt.Sprintf("invalid rollback height target: %d", target))
				}
				require.Equal(t, before, repairM640DBState(t, db), "refusal must not rewrite any persisted values or metadata")
				require.Equal(t, []byte("keep-on-refusal"), store.GetKVStore(transient).Get([]byte("pending")))

				assertLatest := func() {
					t.Helper()
					require.Equal(t, int64(3), rootmulti.GetLatestVersion(db))
					require.Equal(t, commits[2], store.LastCommitID())
					actualInfo, err := store.LastCommitInfo().Marshal()
					require.NoError(t, err)
					require.Equal(t, commitInfo, actualInfo)
					require.Equal(t, []byte("3"), store.GetKVStore(bank).Get([]byte("balance")))
					require.Equal(t, []byte("30"), store.GetKVStore(auth).Get([]byte("sequence")))
					require.Nil(t, store.GetKVStore(bank).Get([]byte("removed")))
					require.Equal(t, []byte("after-height-one"), store.GetKVStore(bank).Get([]byte("added")))
				}
				assertLatest()
				require.NoError(t, store.Close())
				store = nil
				open()
				assertLatest()
				require.Equal(t, before, repairM640DBState(t, db), "reopening must retain the original data and every commit record")

				cache := store.CacheMultiStore()
				cache.GetKVStore(bank).Set([]byte("balance"), []byte("4"))
				cache.Write()
				next := store.Commit(true)
				require.Equal(t, int64(4), next.Version, "commits must continue from the original latest height")
				require.NoError(t, store.Close())
				store = nil
				open()
				require.Equal(t, next, store.LastCommitID())
				require.Equal(t, int64(4), rootmulti.GetLatestVersion(db))
				require.Equal(t, []byte("4"), store.GetKVStore(bank).Get([]byte("balance")))
				require.Equal(t, []byte("30"), store.GetKVStore(auth).Get([]byte("sequence")))
			})
		}
	}
}

func TestRepairM_640_RollbackRejectedBeforeLoad(t *testing.T) {
	var store rootmulti.Store
	err := store.RollbackToVersion(1)
	var unsupported *rootmulti.RollbackUnsupportedError
	require.ErrorAs(t, err, &unsupported)
	require.Equal(t, int64(1), unsupported.Target)
	require.ErrorIs(t, err, rootmulti.ErrVersionedStateUnsupported)
	require.Contains(t, err.Error(), "sdk/storev2/rootmulti")
	require.Nil(t, store.LastCommitInfo())
}
