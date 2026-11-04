package memiavl

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/stretchr/testify/require"
)

// Regression: a Copy()'d tree must remain readable across a memiavl
// snapshot rewrite + reload. Before refcounting *Snapshot, the rewrite
// path called snapshot.Close() (munmap) while a held trace-baker copy
// was still pointing into it — crashing reads in cmpbody.
func TestTreeCopyOutlivesSnapshotRewrite(t *testing.T) {
	db, err := OpenDB(0, Options{
		Config:          Config{SnapshotKeepRecent: 0},
		Dir:             t.TempDir(),
		CreateIfMissing: true,
		InitialStores:   []string{"test"},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	cs := []*proto.NamedChangeSet{{
		Name: "test",
		Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
			{Key: []byte("hello"), Value: []byte("world")},
			{Key: []byte("hello1"), Value: []byte("world1")},
		}},
	}}
	require.NoError(t, db.ApplyChangeSets(cs))
	_, err = db.Commit()
	require.NoError(t, err)

	held := db.Copy()
	defer func() { _ = held.ReleaseSnapshotRefs() }()

	require.NoError(t, db.RewriteSnapshot(context.Background()))
	require.NoError(t, db.Reload())

	cs2 := []*proto.NamedChangeSet{{
		Name: "test",
		Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
			{Key: []byte("hello"), Value: []byte("OVERWRITTEN")},
		}},
	}}
	require.NoError(t, db.ApplyChangeSets(cs2))
	_, err = db.Commit()
	require.NoError(t, err)

	tree := held.TreeByName("test")
	require.NotNil(t, tree)
	require.Equal(t, "world", string(tree.Get([]byte("hello"))))
	require.Equal(t, "world1", string(tree.Get([]byte("hello1"))))
}

func TestMultipleCopiesIndependentLifecycle(t *testing.T) {
	db, err := OpenDB(0, Options{
		Config:          Config{SnapshotKeepRecent: 0},
		Dir:             t.TempDir(),
		CreateIfMissing: true,
		InitialStores:   []string{"test"},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	require.NoError(t, db.ApplyChangeSets([]*proto.NamedChangeSet{{
		Name: "test",
		Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
			{Key: []byte("k"), Value: []byte("v")},
		}},
	}}))
	_, err = db.Commit()
	require.NoError(t, err)

	copyA := db.Copy()
	copyB := db.Copy()

	require.NoError(t, db.RewriteSnapshot(context.Background()))
	require.NoError(t, db.Reload())

	require.NoError(t, copyA.ReleaseSnapshotRefs())

	tree := copyB.TreeByName("test")
	require.NotNil(t, tree)
	require.Equal(t, "v", string(tree.Get([]byte("k"))))

	require.NoError(t, copyB.ReleaseSnapshotRefs())
}

func TestTreeCopyConcurrentRewriteReload(t *testing.T) {
	db, err := OpenDB(0, Options{
		Config:          Config{SnapshotKeepRecent: 0},
		Dir:             t.TempDir(),
		CreateIfMissing: true,
		InitialStores:   []string{"test"},
	})
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	var pairs []*proto.KVPair
	for i := 0; i < 32; i++ {
		pairs = append(pairs, &proto.KVPair{
			Key:   []byte(fmt.Sprintf("key-%02d", i)),
			Value: []byte(fmt.Sprintf("value-%02d", i)),
		})
	}
	require.NoError(t, db.ApplyChangeSets([]*proto.NamedChangeSet{{
		Name:      "test",
		Changeset: proto.ChangeSet{Pairs: pairs},
	}}))
	_, err = db.Commit()
	require.NoError(t, err)

	errCh := make(chan error, 32)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 40; j++ {
				held := db.Copy()
				tree := held.TreeByName("test")
				if tree == nil {
					errCh <- fmt.Errorf("missing copied tree")
					_ = held.ReleaseSnapshotRefs()
					return
				}
				time.Sleep(time.Millisecond)
				if got := tree.Get([]byte("key-00")); len(got) == 0 {
					errCh <- fmt.Errorf("missing copied value")
					_ = held.ReleaseSnapshotRefs()
					return
				}
				if err := held.ReleaseSnapshotRefs(); err != nil {
					errCh <- err
					return
				}
			}
		}()
	}

	for i := 0; i < 10; i++ {
		require.NoError(t, db.RewriteSnapshot(context.Background()))
		require.NoError(t, db.Reload())
		require.NoError(t, db.ApplyChangeSets([]*proto.NamedChangeSet{{
			Name: "test",
			Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
				{Key: []byte(fmt.Sprintf("round-%02d", i)), Value: []byte("ok")},
			}},
		}}))
		_, err = db.Commit()
		require.NoError(t, err)
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		require.NoError(t, err)
	}
}

func TestSnapshotDoubleCloseReturnsError(t *testing.T) {
	snapshot := NewEmptySnapshot(1)
	require.NoError(t, snapshot.Close())
	require.Error(t, snapshot.Close())
	require.Panics(t, snapshot.Acquire)
}

func TestSnapshotBackedDeletionReleasesOwnership(t *testing.T) {
	repair1199Buffers(t)
	dir := t.TempDir()
	db, err := OpenDB(0, Options{Dir: dir, CreateIfMissing: true, InitialStores: []string{"bank", "layerxanchor"}})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	for _, name := range []string{"bank", "layerxanchor"} {
		require.NoError(t, db.ApplyChangeSet(name, proto.ChangeSet{Pairs: []*proto.KVPair{
			{Key: []byte("key"), Value: []byte(name)},
		}}))
	}
	_, err = db.Commit()
	require.NoError(t, err)
	require.NoError(t, db.RewriteSnapshot(context.Background()))
	require.NoError(t, db.Close())
	db, err = OpenDB(0, Options{Dir: dir, ZeroCopy: true})
	require.NoError(t, err)
	retired := db.TreeByName("layerxanchor")
	snapshot := retired.snapshot
	require.NotNil(t, snapshot)
	mappingFile := snapshot.kvsMap.file
	held := retired.Copy()
	t.Cleanup(func() { require.NoError(t, held.Close()) })
	require.Equal(t, int32(2), snapshot.refCount.Load())
	require.Error(t, db.ApplyUpgrades([]*proto.TreeNameUpgrade{
		{Name: "layerxanchor", Delete: true}, {Name: "invalid", RenameFrom: "missing"},
	}))
	require.Same(t, retired, db.TreeByName("layerxanchor"))
	require.Equal(t, int32(2), snapshot.refCount.Load())
	for i := 0; i < 128; i++ {
		retired.ApplyChangeSetAsync(proto.ChangeSet{Pairs: []*proto.KVPair{
			{Key: []byte(fmt.Sprintf("pending-%03d", i)), Value: []byte("written")},
		}})
	}
	require.NoError(t, db.ApplyUpgrades([]*proto.TreeNameUpgrade{{Name: "layerxanchor", Delete: true}}))
	require.Nil(t, db.TreeByName("layerxanchor"))
	require.Nil(t, retired.pendingChanges)
	finished := make(chan struct{})
	go func() { retired.pendingWg.Wait(); close(finished) }()
	select {
	case <-finished:
	case <-time.After(3 * time.Second):
		t.Fatal("deleted tree's replay worker did not exit")
	}
	require.Equal(t, int64(129), retired.Version(), "all queued writes must finish before release")
	require.Nil(t, retired.root)
	require.Nil(t, retired.snapshot)
	require.Equal(t, int32(1), snapshot.refCount.Load())
	require.Equal(t, []byte("layerxanchor"), held.Get([]byte("key")))
	require.Nil(t, held.Get([]byte("pending-127")))
	require.NoError(t, db.Close())
	require.Equal(t, []byte("layerxanchor"), held.Get([]byte("key")))
	require.NoError(t, held.Close())
	require.Zero(t, snapshot.refCount.Load())
	require.Nil(t, snapshot.kvsMap)
	require.Nil(t, snapshot.nodesMap)
	require.Nil(t, snapshot.leavesMap)
	_, err = mappingFile.Stat()
	require.ErrorIs(t, err, os.ErrClosed)
}
