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

func TestApplyUpgradesDeletesAnchorTreeAndKeepsMergeTrees(t *testing.T) {
	const anchorName = "layerxanchor"
	mergeStores := []string{
		keys.LayerXProgramsStoreKey,
		keys.LayerXPayStoreKey,
		keys.LayerXLedgerStoreKey,
		keys.LayerXAuthStoreKey,
		keys.LayerXAgentStoreKey,
	}
	initialStores, err := keys.AllModulesExcept(mergeStores...)
	require.NoError(t, err)
	initialStores = append(initialStores, anchorName)
	expectedNames := append([]string(nil), keys.MemIAVLStoreKeys...)
	sort.Strings(expectedNames)

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

			populate := func(db *DB, name string) {
				t.Helper()
				require.NoError(t, db.ApplyChangeSet(name, proto.ChangeSet{
					Pairs: []*proto.KVPair{
						{Key: []byte("first"), Value: []byte(name + "-first")},
						{Key: []byte("second"), Value: []byte(name + "-second")},
					},
				}))
			}
			for _, name := range initialStores {
				populate(initialDB, name)
			}
			initialCommit, err := initialDB.Commit()
			require.NoError(t, err)
			require.Equal(t, utils.NextVersion(0, initialVersion), initialCommit)
			require.NoError(t, initialDB.Close())

			db, err := OpenDB(0, Options{Dir: dir})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, db.Close()) })
			var upgrades []*proto.TreeNameUpgrade
			for _, name := range mergeStores {
				require.Nil(t, db.TreeByName(name), name)
				upgrades = append(upgrades, &proto.TreeNameUpgrade{Name: name})
			}
			require.NoError(t, db.ApplyUpgrades(upgrades))
			for _, name := range mergeStores {
				populate(db, name)
			}
			mergeCommit, err := db.Commit()
			require.NoError(t, err)
			require.Equal(t, initialCommit+1, mergeCommit)
			require.Len(t, db.Trees(), len(expectedNames)+1)
			anchor := db.TreeByName(anchorName)
			require.NotNil(t, anchor)
			require.Equal(t, []byte(anchorName+"-first"), anchor.Get([]byte("first")))

			hashes := make(map[string][]byte, len(expectedNames))
			for _, name := range expectedNames {
				tree := db.TreeByName(name)
				require.NotNil(t, tree, name)
				require.False(t, tree.IsEmpty(), name)
				hashes[name] = append([]byte(nil), tree.RootHash()...)
			}

			require.NoError(t, db.ApplyUpgrades([]*proto.TreeNameUpgrade{
				{Name: anchorName, Delete: true},
			}))
			require.Nil(t, db.TreeByName(anchorName))
			retirementVersion, err := db.Commit()
			require.NoError(t, err)
			require.Equal(t, mergeCommit+1, retirementVersion)

			assertRetired := func(db *DB) {
				t.Helper()
				require.Equal(t, retirementVersion, db.Version())
				require.Nil(t, db.TreeByName(anchorName))
				require.Len(t, db.Trees(), len(expectedNames))
				require.Len(t, db.LastCommitInfo().StoreInfos, len(expectedNames))
				for i, name := range expectedNames {
					require.Equal(t, name, db.Trees()[i].Name)
					tree := db.TreeByName(name)
					require.NotNil(t, tree, name)
					require.Same(t, db.Trees()[i].Tree, tree, name)
					require.Equal(t, retirementVersion, tree.Version(), name)
					require.Equal(t, hashes[name], tree.RootHash(), name)
					require.Equal(t, []byte(name+"-first"), tree.Get([]byte("first")), name)
					require.Equal(t, []byte(name+"-second"), tree.Get([]byte("second")), name)
					info := db.LastCommitInfo().StoreInfos[i]
					require.Equal(t, name, info.Name)
					require.Equal(t, retirementVersion, info.CommitId.Version, name)
					require.Equal(t, hashes[name], info.CommitId.Hash, name)
				}
			}
			assertRetired(db)
			require.NoError(t, db.Close())

			reopened, err := OpenDB(0, Options{Dir: dir})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, reopened.Close()) })
			assertRetired(reopened)
			require.EqualError(t, reopened.ApplyUpgrades([]*proto.TreeNameUpgrade{
				{Name: anchorName, Delete: true},
			}), "unknown tree name layerxanchor")
		})
	}
}
