package memiavl

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/wal"
)

func repair125Open(t *testing.T, opts Options) *DB {
	t.Helper()
	db, err := OpenDB(0, opts)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db
}

func repair125Set(name, key, value string) *proto.NamedChangeSet {
	return &proto.NamedChangeSet{
		Name: name,
		Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
			{Key: []byte(key), Value: []byte(value)},
		}},
	}
}

func repair125Pending(t *testing.T, db *DB) []byte {
	t.Helper()
	bz, err := db.pendingLogEntry.Marshal()
	require.NoError(t, err)
	return bz
}

func repair125AssertCommit(t *testing.T, db *DB, expected proto.CommitInfo) {
	t.Helper()
	require.Equal(t, expected, *db.LastCommitInfo())
	require.Equal(t, expected.Version, db.Version())
	require.Len(t, db.Trees(), len(expected.StoreInfos))
	for i, info := range expected.StoreInfos {
		tree := db.TreeByName(info.Name)
		require.NotNil(t, tree, info.Name)
		require.Same(t, tree, db.Trees()[i].Tree)
		require.Equal(t, info.Name, db.Trees()[i].Name)
		require.Equal(t, expected.Version, tree.Version(), info.Name)
		require.Equal(t, info.CommitId.Hash, tree.RootHash(), info.Name)
	}
}

func TestRepair_125_UpgradeIdempotence(t *testing.T) {
	for _, initial := range []uint32{0, 100} {
		t.Run(fmt.Sprint(initial), func(t *testing.T) {
			cs := NewCommitStore(t.TempDir(), Config{AsyncCommitBuffer: 0})
			t.Cleanup(func() { require.NoError(t, cs.Close()) })
			cs.opts.InitialVersion = initial
			require.NoError(t, cs.Initialize([]string{"bank", "layerxanchor", "legacy"}))
			committer, err := cs.LoadVersion(0, false)
			require.NoError(t, err)
			for _, name := range []string{"bank", "layerxanchor", "legacy"} {
				require.NoError(t, committer.ApplyChangeSets([]*proto.NamedChangeSet{
					repair125Set(name, "retained", name),
				}))
			}
			_, err = committer.Commit()
			require.NoError(t, err)
			bankHash := append([]byte(nil), cs.db.TreeByName("bank").RootHash()...)
			legacyHash := append([]byte(nil), cs.db.TreeByName("legacy").RootHash()...)

			mergeNames := []string{
				keys.LayerXProgramsStoreKey, keys.LayerXPayStoreKey,
				keys.LayerXLedgerStoreKey, keys.LayerXAuthStoreKey, keys.LayerXAgentStoreKey,
			}
			plan := []*proto.TreeNameUpgrade{{Name: "renamed", RenameFrom: "legacy"}}
			for _, name := range mergeNames {
				plan = append(plan, &proto.TreeNameUpgrade{Name: name})
			}
			require.NoError(t, committer.ApplyUpgrades(plan))
			for _, name := range mergeNames {
				require.NoError(t, committer.ApplyChangeSets([]*proto.NamedChangeSet{
					repair125Set(name, "retained", name),
				}))
			}
			programs := cs.db.TreeByName(keys.LayerXProgramsStoreKey)
			require.NoError(t, committer.ApplyUpgrades(plan))
			require.Same(t, programs, cs.db.TreeByName(keys.LayerXProgramsStoreKey))
			require.Equal(t, []byte(keys.LayerXProgramsStoreKey), programs.Get([]byte("retained")))
			_, err = committer.Commit()
			require.NoError(t, err)

			retire := []*proto.TreeNameUpgrade{{Name: "layerxanchor", Delete: true}}
			require.NoError(t, committer.ApplyUpgrades(retire))
			require.NoError(t, committer.ApplyUpgrades(retire))
			_, err = committer.Commit()
			require.NoError(t, err)
			expected := cloneCommitInfo(committer.LastCommitInfo())
			require.Zero(t, cs.db.SnapshotVersion(), "reopen must consume the actual WAL")
			require.NoError(t, cs.Close())

			committer, err = cs.LoadVersion(0, false)
			require.NoError(t, err)
			repair125AssertCommit(t, cs.db, expected)
			require.NoError(t, committer.ApplyUpgrades(plan))
			require.NoError(t, committer.ApplyUpgrades(retire))
			_, err = committer.Commit()
			require.NoError(t, err)
			require.Nil(t, cs.db.TreeByName("legacy"))
			require.Nil(t, cs.db.TreeByName("layerxanchor"))
			require.Equal(t, bankHash, cs.db.TreeByName("bank").RootHash())
			require.Equal(t, legacyHash, cs.db.TreeByName("renamed").RootHash())
			for _, name := range mergeNames {
				require.Equal(t, []byte(name), cs.db.TreeByName(name).Get([]byte("retained")))
			}
			expected = cloneCommitInfo(committer.LastCommitInfo())
			require.NoError(t, cs.db.RewriteSnapshot(context.Background()))
			require.NoError(t, cs.Close())

			committer, err = cs.LoadVersion(0, false)
			require.NoError(t, err)
			require.Equal(t, expected.Version, cs.db.SnapshotVersion())
			repair125AssertCommit(t, cs.db, expected)
			require.NoError(t, committer.ApplyUpgrades(plan))
			require.NoError(t, committer.ApplyUpgrades(retire))
			_, err = committer.Commit()
			require.NoError(t, err)
			expected = cloneCommitInfo(committer.LastCommitInfo())
			require.NoError(t, cs.Close())
			_, err = cs.LoadVersion(0, false)
			require.NoError(t, err)
			repair125AssertCommit(t, cs.db, expected)
			require.Equal(t, bankHash, cs.db.TreeByName("bank").RootHash())
			require.Equal(t, legacyHash, cs.db.TreeByName("renamed").RootHash())
		})
	}
}

func TestRepair_125_UpgradePlanAtomicity(t *testing.T) {
	db := repair125Open(t, Options{
		Dir: t.TempDir(), CreateIfMissing: true,
		InitialStores: []string{"bank", "layerxanchor", "legacy"},
	})
	for _, name := range []string{"bank", "layerxanchor", "legacy"} {
		require.NoError(t, db.ApplyChangeSets([]*proto.NamedChangeSet{repair125Set(name, "key", name)}))
	}
	_, err := db.Commit()
	require.NoError(t, err)
	require.NoError(t, db.ApplyUpgrades([]*proto.TreeNameUpgrade{{Name: "accepted"}}))

	for _, failure := range []*proto.TreeNameUpgrade{
		{Name: "never", RenameFrom: "missing"},
		{Name: "bank", RenameFrom: "renamed"},
	} {
		before := cloneCommitInfo(db.WorkingCommitInfo())
		pending := repair125Pending(t, db)
		bank, anchor, legacy := db.TreeByName("bank"), db.TreeByName("layerxanchor"), db.TreeByName("legacy")
		err := db.ApplyUpgrades([]*proto.TreeNameUpgrade{
			{Name: "layerxanchor", Delete: true},
			{Name: "renamed", RenameFrom: "legacy"},
			{Name: "transient"}, failure,
		})
		require.Error(t, err)
		require.Equal(t, pending, repair125Pending(t, db))
		require.Equal(t, before, *db.WorkingCommitInfo())
		require.Same(t, bank, db.TreeByName("bank"))
		require.Same(t, anchor, db.TreeByName("layerxanchor"))
		require.Same(t, legacy, db.TreeByName("legacy"))
		require.Nil(t, db.TreeByName("renamed"))
		require.Nil(t, db.TreeByName("transient"))
		require.Nil(t, db.TreeByName("never"))
	}
	require.NoError(t, db.ApplyChangeSets([]*proto.NamedChangeSet{repair125Set("bank", "valid", "after-error")}))
	_, err = db.Commit()
	require.NoError(t, err)
	expected := cloneCommitInfo(db.LastCommitInfo())
	dir := db.dir
	require.NoError(t, db.Close())
	db = repair125Open(t, Options{Dir: dir})
	repair125AssertCommit(t, db, expected)
	require.Equal(t, []byte("after-error"), db.TreeByName("bank").Get([]byte("valid")))
	require.NotNil(t, db.TreeByName("accepted"))
	require.NotNil(t, db.TreeByName("layerxanchor"))
	require.NotNil(t, db.TreeByName("legacy"))
}

func TestRepair_125_DuplicateStoreReplay(t *testing.T) {
	for _, initial := range []uint32{0, 100} {
		t.Run(fmt.Sprint(initial), func(t *testing.T) {
			db := repair125Open(t, Options{
				Dir: t.TempDir(), CreateIfMissing: true, InitialVersion: initial,
				InitialStores: []string{"bank", "other", "idle"},
			})
			first := &proto.NamedChangeSet{Name: "bank", Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
				{Key: []byte("shared"), Value: []byte("first")},
				{Key: []byte("gone"), Value: []byte("delete-me")},
			}}}
			second := &proto.NamedChangeSet{Name: "bank", Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
				{Key: []byte("shared"), Value: []byte("second")},
				{Key: []byte("gone"), Delete: true},
				{Key: []byte("kept"), Value: []byte("present")},
			}}}
			require.Empty(t, db.pendingLogEntry.Changesets)
			require.NoError(t, db.ApplyChangeSets([]*proto.NamedChangeSet{
				first, repair125Set("other", "other", "value"), second,
			}))
			require.Len(t, first.Changeset.Pairs, 2, "merging must not mutate the caller's changeset")
			require.NoError(t, db.ApplyChangeSet("bank", repair125Set("bank", "shared", "last").Changeset))
			version, err := db.Commit()
			require.NoError(t, err)
			require.Equal(t, utils.NextVersion(0, initial), version)
			entry, err := db.GetWAL().ReadAt(1)
			require.NoError(t, err)
			require.Len(t, entry.Changesets, 2)
			require.Equal(t, "bank", entry.Changesets[0].Name)
			require.Equal(t, "other", entry.Changesets[1].Name)
			require.Len(t, entry.Changesets[0].Changeset.Pairs, 6)
			require.Equal(t, "first", string(entry.Changesets[0].Changeset.Pairs[0].Value))
			require.Equal(t, "second", string(entry.Changesets[0].Changeset.Pairs[2].Value))
			require.True(t, entry.Changesets[0].Changeset.Pairs[3].Delete)
			require.Equal(t, "last", string(entry.Changesets[0].Changeset.Pairs[5].Value))
			expected := cloneCommitInfo(db.LastCommitInfo())
			dir := db.dir
			require.Zero(t, db.SnapshotVersion())
			require.NoError(t, db.Close())
			db = repair125Open(t, Options{Dir: dir})
			repair125AssertCommit(t, db, expected)
			require.Equal(t, []byte("last"), db.TreeByName("bank").Get([]byte("shared")))
			require.Equal(t, []byte("present"), db.TreeByName("bank").Get([]byte("kept")))
			require.Nil(t, db.TreeByName("bank").Get([]byte("gone")))
			require.Equal(t, []byte("value"), db.TreeByName("other").Get([]byte("other")))
		})
	}
}

func TestRepair_125_RejectedChangesetWAL(t *testing.T) {
	db := repair125Open(t, Options{
		Dir: t.TempDir(), CreateIfMissing: true, InitialStores: []string{"bank", "idle"},
	})
	require.NoError(t, db.ApplyChangeSets([]*proto.NamedChangeSet{repair125Set("bank", "seed", "original")}))
	_, err := db.Commit()
	require.NoError(t, err)
	require.NoError(t, db.RewriteSnapshot(context.Background()))
	require.NoError(t, db.ApplyChangeSets([]*proto.NamedChangeSet{repair125Set("bank", "accepted", "pending")}))
	before := cloneCommitInfo(db.WorkingCommitInfo())
	pending := repair125Pending(t, db)
	batch := []*proto.NamedChangeSet{
		repair125Set("bank", "seed", "rejected"),
		repair125Set("unknown", "poison", "rejected"),
	}
	require.EqualError(t, db.ApplyChangeSets(batch), "unknown tree name unknown")
	require.Equal(t, before, *db.WorkingCommitInfo())
	require.Equal(t, pending, repair125Pending(t, db))
	require.Equal(t, []byte("original"), db.TreeByName("bank").Get([]byte("seed")))
	require.EqualError(t, db.MultiTree.ApplyChangeSets(batch), "unknown tree name unknown")
	require.Equal(t, before, *db.WorkingCommitInfo())
	require.EqualError(t, db.ApplyChangeSet("unknown", batch[1].Changeset), "unknown tree name unknown")
	require.Equal(t, pending, repair125Pending(t, db))
	require.NoError(t, db.ApplyChangeSets([]*proto.NamedChangeSet{repair125Set("bank", "valid", "committed")}))
	_, err = db.Commit()
	require.NoError(t, err)
	last, err := db.GetWAL().LastOffset()
	require.NoError(t, err)
	entry, err := db.GetWAL().ReadAt(last)
	require.NoError(t, err)
	require.Len(t, entry.Changesets, 1)
	require.Equal(t, "bank", entry.Changesets[0].Name)
	expected := cloneCommitInfo(db.LastCommitInfo())
	dir := db.dir
	require.NoError(t, db.Close())
	db = repair125Open(t, Options{Dir: dir})
	require.Equal(t, int64(1), db.SnapshotVersion())
	repair125AssertCommit(t, db, expected)
	require.Equal(t, []byte("original"), db.TreeByName("bank").Get([]byte("seed")))
	require.Equal(t, []byte("pending"), db.TreeByName("bank").Get([]byte("accepted")))
	require.Equal(t, []byte("committed"), db.TreeByName("bank").Get([]byte("valid")))
	require.Nil(t, db.TreeByName("unknown"))
}

func TestRepair_125_ReplayGap(t *testing.T) {
	for _, initial := range []uint32{0, 100} {
		for _, missingFirst := range []bool{false, true} {
			t.Run(fmt.Sprintf("initial_%d/missing_first_%t", initial, missingFirst), func(t *testing.T) {
				db := repair125Open(t, Options{
					Dir: t.TempDir(), CreateIfMissing: true, InitialVersion: initial,
					InitialStores: []string{"bank", "idle"},
				})
				var entries []proto.ChangelogEntry
				for block := 1; block <= 4; block++ {
					if block == 4 {
						require.NoError(t, db.ApplyUpgrades([]*proto.TreeNameUpgrade{{Name: "future"}}))
					}
					require.NoError(t, db.ApplyChangeSets([]*proto.NamedChangeSet{
						repair125Set("bank", "height", fmt.Sprint(block)),
					}))
					_, err := db.Commit()
					require.NoError(t, err)
					entry, err := db.GetWAL().ReadAt(uint64(block))
					require.NoError(t, err)
					entries = append(entries, entry)
					if block == 1 && !missingFirst {
						require.NoError(t, db.RewriteSnapshot(context.Background()))
					}
				}
				dir := db.dir
				require.NoError(t, db.Close())
				stream, err := wal.NewChangelogWAL(utils.GetChangelogPath(dir), wal.Config{})
				require.NoError(t, err)
				if missingFirst {
					require.NoError(t, stream.TruncateBefore(2))
				} else {
					require.NoError(t, stream.TruncateAfter(2))
					require.NoError(t, stream.Write(entries[3]))
				}
				require.NoError(t, stream.Close())

				reopened, err := OpenDB(0, Options{Dir: dir, ReadOnly: true})
				if reopened != nil {
					require.NoError(t, reopened.Close())
				}
				require.ErrorContains(t, err, "changelog version gap")
				require.Nil(t, reopened, "startup must not expose incomplete recovered state")

				mtree, err := LoadMultiTree(context.Background(), filepath.Join(dir, "current"), Options{})
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, mtree.Close()) })
				before := cloneCommitInfo(mtree.LastCommitInfo())
				stream, err = wal.NewChangelogWAL(utils.GetChangelogPath(dir), wal.Config{})
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, stream.Close()) })
				delta, present, err := computeWALIndexDelta(stream)
				require.NoError(t, err)
				require.True(t, present)
				lastDelta, err := computeLastWALIndexDelta(stream)
				require.NoError(t, err)
				err = mtree.Catchup(context.Background(), stream, max(delta, lastDelta), 0)
				require.ErrorContains(t, err, "changelog version gap")
				require.Equal(t, before, cloneCommitInfo(mtree.LastCommitInfo()))
				require.Nil(t, mtree.TreeByName("future"))
				if missingFirst {
					require.Empty(t, mtree.Trees())
					require.Zero(t, mtree.Version())
				} else {
					require.Equal(t, []byte("1"), mtree.TreeByName("bank").Get([]byte("height")))
					for _, tree := range mtree.Trees() {
						require.Equal(t, before.Version, tree.Version())
						require.Nil(t, tree.pendingChanges, "gap validation must precede async mutation")
					}
				}
			})
		}
	}
}

func TestRepair_125_ReplayInitialHeightAndDuplicates(t *testing.T) {
	for _, initial := range []uint32{0, 1, 100} {
		t.Run(fmt.Sprint(initial), func(t *testing.T) {
			db := repair125Open(t, Options{
				Dir: t.TempDir(), CreateIfMissing: true, InitialVersion: initial,
				InitialStores: []string{"bank", "idle"},
			})
			require.NoError(t, db.ApplyChangeSets([]*proto.NamedChangeSet{repair125Set("bank", "height", "first")}))
			firstVersion, err := db.Commit()
			require.NoError(t, err)
			require.Equal(t, utils.NextVersion(0, initial), firstVersion)
			firstInfo := cloneCommitInfo(db.LastCommitInfo())
			require.NoError(t, db.ApplyChangeSets([]*proto.NamedChangeSet{repair125Set("bank", "height", "second")}))
			_, err = db.Commit()
			require.NoError(t, err)
			expected := cloneCommitInfo(db.LastCommitInfo())
			first, err := db.GetWAL().ReadAt(1)
			require.NoError(t, err)
			second, err := db.GetWAL().ReadAt(2)
			require.NoError(t, err)
			dir := db.dir
			require.NoError(t, db.Close())
			stream, err := wal.NewChangelogWAL(utils.GetChangelogPath(dir), wal.Config{})
			require.NoError(t, err)
			require.NoError(t, stream.TruncateAfter(1))
			require.NoError(t, stream.Write(first))
			require.NoError(t, stream.Write(second))
			require.NoError(t, stream.Close())

			historical, err := OpenDB(firstVersion, Options{Dir: dir, ReadOnly: true})
			require.NoError(t, err)
			repair125AssertCommit(t, historical, firstInfo)
			require.Equal(t, []byte("first"), historical.TreeByName("bank").Get([]byte("height")))
			require.NoError(t, historical.Close())
			db = repair125Open(t, Options{Dir: dir})
			repair125AssertCommit(t, db, expected)
			require.Equal(t, []byte("second"), db.TreeByName("bank").Get([]byte("height")))
		})
	}
}
