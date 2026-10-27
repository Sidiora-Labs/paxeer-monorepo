package memiavl

import (
	"sort"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
)

func TestApplyUpgradesAddsMergeTreesAtNextVersion(t *testing.T) {
	mergeStores := []string{
		keys.LayerXProgramsStoreKey,
		keys.LayerXPayStoreKey,
		keys.LayerXLedgerStoreKey,
		keys.LayerXAuthStoreKey,
		keys.LayerXAgentStoreKey,
	}
	initialStores, err := keys.AllModulesExcept(mergeStores...)
	require.NoError(t, err)

	for _, initialVersion := range []uint32{0, 100} {
		t.Run(strconv.FormatUint(uint64(initialVersion), 10), func(t *testing.T) {
			dir := t.TempDir()
			initialDB, err := OpenDB(0, Options{
				Dir:             dir,
				CreateIfMissing: true,
				InitialVersion:  initialVersion,
				InitialStores:   initialStores,
			})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, initialDB.Close()) })

			hashes := make(map[string][]byte, len(initialStores))
			for _, name := range initialStores {
				require.NoError(t, initialDB.ApplyChangeSet(name, proto.ChangeSet{
					Pairs: []*proto.KVPair{
						{Key: []byte("first"), Value: []byte(name + "-first")},
						{Key: []byte("second"), Value: []byte(name + "-second")},
					},
				}))
			}
			currentVersion, err := initialDB.Commit()
			require.NoError(t, err)
			require.Equal(t, utils.NextVersion(0, initialVersion), currentVersion)
			for _, name := range initialStores {
				hashes[name] = append([]byte(nil), initialDB.TreeByName(name).RootHash()...)
			}
			require.NoError(t, initialDB.Close())

			db, err := OpenDB(0, Options{Dir: dir})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			require.Equal(t, currentVersion, db.Version())
			require.Len(t, db.Trees(), len(initialStores))
			var upgrades []*proto.TreeNameUpgrade
			for _, name := range mergeStores {
				require.Nil(t, db.TreeByName(name), name)
				upgrades = append(upgrades, &proto.TreeNameUpgrade{Name: name})
			}
			nextVersion := utils.NextVersion(currentVersion, initialVersion)
			require.NoError(t, db.ApplyUpgrades(upgrades))
			require.Equal(t, currentVersion, db.Version())
			for _, name := range mergeStores {
				tree := db.TreeByName(name)
				require.NotNil(t, tree, name)
				require.Equal(t, uint32(nextVersion), tree.initialVersion, name)
				require.True(t, tree.IsEmpty(), name)
			}
			version, err := db.Commit()
			require.NoError(t, err)
			require.Equal(t, nextVersion, version)

			expectedNames := append(append([]string(nil), initialStores...), mergeStores...)
			sort.Strings(expectedNames)
			assertUpgraded := func(db *DB) {
				t.Helper()
				require.Equal(t, nextVersion, db.Version())
				require.Len(t, db.Trees(), len(expectedNames))
				require.Len(t, db.LastCommitInfo().StoreInfos, len(expectedNames))
				for i, name := range expectedNames {
					require.Equal(t, name, db.Trees()[i].Name)
					tree := db.TreeByName(name)
					require.NotNil(t, tree, name)
					require.Same(t, db.Trees()[i].Tree, tree, name)
					require.Equal(t, nextVersion, tree.Version(), name)
					info := db.LastCommitInfo().StoreInfos[i]
					require.Equal(t, name, info.Name)
					require.Equal(t, nextVersion, info.CommitId.Version, name)
					require.Equal(t, tree.RootHash(), info.CommitId.Hash, name)
				}
				for _, name := range mergeStores {
					require.True(t, db.TreeByName(name).IsEmpty(), name)
				}
				for _, name := range initialStores {
					tree := db.TreeByName(name)
					require.Equal(t, hashes[name], tree.RootHash(), name)
					require.Equal(t, []byte(name+"-first"), tree.Get([]byte("first")), name)
					require.Equal(t, []byte(name+"-second"), tree.Get([]byte("second")), name)
				}
			}
			assertUpgraded(db)
			require.NoError(t, db.Close())

			reopened, err := OpenDB(0, Options{Dir: dir})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, reopened.Close()) })
			assertUpgraded(reopened)
		})
	}
}
