package migration

import (
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/stretchr/testify/require"
)

func TestEVMMigrationRoutersRouteMergeStoresToMemIAVL(t *testing.T) {
	mergeStores := []string{
		keys.LayerXLedgerStoreKey,
		keys.LayerXAuthStoreKey,
		keys.LayerXPayStoreKey,
		keys.LayerXProgramsStoreKey,
		keys.SequencerStoreKey,
	}
	nonEVMModules, err := keys.AllModulesExcept(keys.EVMStoreKey)
	require.NoError(t, err)
	for _, store := range mergeStores {
		require.Contains(t, nonEVMModules, store)
	}

	for _, mode := range []config.WriteMode{config.MigrateEVM, config.EVMMigrated} {
		t.Run(string(mode), func(t *testing.T) {
			memIAVL := NewTestMemIAVLCommitStore(t, t.TempDir(), keys.MemIAVLStoreKeys)
			flatKV := NewTestFlatKVCommitStore(t, t.TempDir())
			if mode == config.MigrateEVM {
				require.NoError(t, memIAVL.ApplyChangeSets([]*proto.NamedChangeSet{{
					Name: keys.EVMStoreKey,
					Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
						{Key: []byte("a"), Value: []byte("migrate-first")},
						{Key: []byte("z"), Value: []byte("migrate-later")},
					}},
				}}))
				_, err := memIAVL.Commit()
				require.NoError(t, err)
			}

			router, err := BuildRouter(t.Context(), mode, memIAVL, flatKV, 1)
			require.NoError(t, err)
			key := []byte("shared-key")
			changesets := make([]*proto.NamedChangeSet, 0, len(mergeStores)+1)
			for _, store := range mergeStores {
				changesets = append(changesets, &proto.NamedChangeSet{
					Name: store,
					Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
						{Key: key, Value: []byte("value/" + store)},
					}},
				})
			}
			evmValue := []byte("evm-value")
			changesets = append(changesets, &proto.NamedChangeSet{
				Name: keys.EVMStoreKey,
				Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
					{Key: key, Value: evmValue},
				}},
			})
			require.NoError(t, router.ApplyChangeSets(changesets, true))

			for _, store := range mergeStores {
				t.Run(store, func(t *testing.T) {
					want := []byte("value/" + store)
					child := memIAVL.GetChildStoreByName(store)
					require.NotNil(t, child)
					require.Equal(t, want, child.Get(key), "routed write must reach memiavl")
					value, found := flatKV.Get(store, key)
					require.False(t, found, "merge store write must not reach flatkv")
					require.Nil(t, value)

					value, found, err := router.Read(store, key)
					require.NoError(t, err)
					require.True(t, found)
					require.Equal(t, want, value)

					readKey := []byte("memiavl-only")
					readValue := []byte("read/" + store)
					require.NoError(t, memIAVL.ApplyChangeSets([]*proto.NamedChangeSet{{
						Name: store,
						Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
							{Key: readKey, Value: readValue},
							{Key: key, Value: readValue},
						}},
					}}))
					for _, readKey := range [][]byte{readKey, key} {
						value, found, err = router.Read(store, readKey)
						require.NoError(t, err)
						require.True(t, found)
						require.Equal(t, readValue, value, "routed read must use current memiavl data")
						_, found = flatKV.Get(store, readKey)
						require.False(t, found)
					}
				})
			}

			value, found := flatKV.Get(keys.EVMStoreKey, key)
			require.True(t, found, "new EVM writes must reach flatkv")
			require.Equal(t, evmValue, value)
			require.Nil(t, memIAVL.GetChildStoreByName(keys.EVMStoreKey).Get(key))
			value, found, err = router.Read(keys.EVMStoreKey, key)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, evmValue, value)

			if mode == config.MigrateEVM {
				value, found = flatKV.Get(keys.EVMStoreKey, []byte("a"))
				require.True(t, found, "EVM migration must advance alongside merge store writes")
				require.Equal(t, []byte("migrate-first"), value)
				require.Nil(t, memIAVL.GetChildStoreByName(keys.EVMStoreKey).Get([]byte("a")))
				require.Equal(t, []byte("migrate-later"), memIAVL.GetChildStoreByName(keys.EVMStoreKey).Get([]byte("z")))
				_, found = flatKV.Get(keys.EVMStoreKey, []byte("z"))
				require.False(t, found, "EVM migration must still be in progress")
			}
		})
	}
}
