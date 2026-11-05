package rootmulti_test

import (
	"context"
	"fmt"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/rootmulti"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repair292Store(t *testing.T, typ storetypes.StoreType) (storetypes.CommitMultiStore, storetypes.StoreKey, []storetypes.CommitID) {
	t.Helper()
	var store storetypes.CommitMultiStore = rootmulti.NewStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	key := storetypes.NewKVStoreKey("bank")
	store.MountStoreWithDB(key, typ, nil)
	require.NoError(t, store.LoadLatestVersion())

	var commits []storetypes.CommitID
	for i, value := range []string{"first", "second"} {
		cache := store.CacheMultiStore()
		cache.GetKVStore(key).Set([]byte("balance"), []byte(value))
		cache.Write()
		commit := store.Commit(true)
		require.Equal(t, int64(i+1), commit.Version)
		require.Equal(t, []byte(value), store.GetKVStore(key).Get([]byte("balance")))
		commits = append(commits, commit)
	}
	return store, key, commits
}

func TestRepair_292_VersionedQueryAndExport(t *testing.T) {
	for _, typ := range []storetypes.StoreType{storetypes.StoreTypeIAVL, storetypes.StoreTypeDB} {
		t.Run(fmt.Sprint(typ), func(t *testing.T) {
			store, key, commits := repair292Store(t, typ)
			var api storetypes.MultiStore = store
			for _, dirty := range []bool{false, true} {
				t.Run(fmt.Sprintf("uncommitted=%t", dirty), func(t *testing.T) {
					if dirty {
						store.GetKVStore(key).Set([]byte("balance"), []byte("uncommitted"))
					}
					for name, load := range map[string]func(int64) (storetypes.CacheMultiStore, error){
						"query":  api.CacheMultiStoreWithVersion,
						"export": api.CacheMultiStoreForExport,
					} {
						for _, height := range []int64{commits[0].Version, commits[1].Version, commits[1].Version + 1, 0, -1} {
							t.Run(fmt.Sprintf("%s/height=%d", name, height), func(t *testing.T) {
								cache, err := load(height)
								require.ErrorIs(t, err, rootmulti.ErrVersionedStateUnsupported)
								require.Contains(t, err.Error(), fmt.Sprintf("requested height %d", height))
								require.Nil(t, cache, "an unsupported height must not expose current values")
							})
						}
					}
				})
			}
			require.Equal(t, commits[1], store.LastCommitID())
			require.Equal(t, []byte("uncommitted"), store.GetKVStore(key).Get([]byte("balance")))
		})
	}
}

func TestRepair_292_ABCIHistoricalQuery(t *testing.T) {
	store, key, commits := repair292Store(t, storetypes.StoreTypeIAVL)
	queryable := store.(storetypes.Queryable)
	for _, prove := range []bool{false, true} {
		for _, height := range []int64{commits[0].Version, commits[1].Version, commits[1].Version + 1, -1} {
			t.Run(fmt.Sprintf("height=%d/prove=%t", height, prove), func(t *testing.T) {
				response := queryable.Query(context.Background(), abci.RequestQuery{
					Path: "/bank/key", Data: []byte("balance"), Height: height, Prove: prove,
				})
				require.Equal(t, sdkerrors.ErrInvalidRequest.ABCICode(), response.Code)
				require.Contains(t, response.Log, rootmulti.ErrVersionedStateUnsupported.Error())
				require.Contains(t, response.Log, fmt.Sprintf("requested height %d", height))
				require.Empty(t, response.Value)
				require.Nil(t, response.ProofOps)
			})
		}
	}

	cache := store.CacheMultiStore()
	cache.GetKVStore(key).Set([]byte("balance"), []byte("working"))
	require.Equal(t, []byte("second"), store.GetKVStore(key).Get([]byte("balance")))
	cache.Write()
	response := queryable.Query(context.Background(), abci.RequestQuery{Path: "/bank/key", Data: []byte("balance")})
	require.Zero(t, response.Code, response.Log)
	require.Equal(t, []byte("working"), response.Value)
	require.Equal(t, commits[1], store.LastCommitID())
}
