package memiavl

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	errorutils "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/types"
	"github.com/stretchr/testify/require"
)

func repair1199Buffers(t *testing.T) {
	t.Helper()
	nodes, buffer := nodeChanSize, bufIOSize
	SetPipelineBufferSize(100)
	bufIOSize = 4096
	t.Cleanup(func() { nodeChanSize, bufIOSize = nodes, buffer })
}

func repair1199Open(t *testing.T, opts Options) *DB {
	t.Helper()
	db, err := OpenDB(0, opts)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db
}

func repair1199Set(t *testing.T, db *DB, name, value string) {
	t.Helper()
	require.NoError(t, db.ApplyChangeSet(name, proto.ChangeSet{Pairs: []*proto.KVPair{
		{Key: []byte("key"), Value: []byte(value)},
	}}))
}

func repair1199NoOpenResources(t *testing.T, dir string) {
	t.Helper()
	fds, err := os.ReadDir("/proc/self/fd")
	require.NoError(t, err)
	for _, fd := range fds {
		target, err := os.Readlink(filepath.Join("/proc/self/fd", fd.Name()))
		if os.IsNotExist(err) {
			continue
		}
		require.NoError(t, err)
		require.False(t, strings.HasPrefix(target, dir+string(os.PathSeparator)), "leaked descriptor: %s", target)
	}
	maps, err := os.ReadFile("/proc/self/maps")
	require.NoError(t, err)
	require.NotContains(t, string(maps), dir+string(os.PathSeparator), "failed open retained a mapping")
}

func TestRepairM_1199_RetirementTestContract(t *testing.T) {
	TestApplyUpgradesDeletesAnchorTreeAndKeepsMergeTrees(t)
}

func TestRepairM_1199_DeleteTreeRelease(t *testing.T) {
	TestSnapshotBackedDeletionReleasesOwnership(t)
}

func TestRepairM_1199_OpenErrorCleanup(t *testing.T) {
	repair1199Buffers(t)
	t.Run("pruned-target", func(t *testing.T) {
		dir := t.TempDir()
		db := repair1199Open(t, Options{Dir: dir, CreateIfMissing: true, InitialVersion: 100, InitialStores: []string{"test"}})
		repair1199Set(t, db, "test", "survives")
		version, err := db.Commit()
		require.NoError(t, err)
		require.Equal(t, int64(100), version)
		hash := db.TreeByName("test").RootHash()
		require.NoError(t, db.RewriteSnapshot(context.Background()))
		require.NoError(t, db.Close())
		require.NoError(t, os.RemoveAll(filepath.Join(dir, snapshotName(0))))
		failed, err := OpenDB(1, Options{Dir: dir})
		require.Nil(t, failed)
		require.ErrorContains(t, err, "target version is pruned")
		repair1199NoOpenResources(t, dir)
		reopened := repair1199Open(t, Options{Dir: dir})
		require.Equal(t, version, reopened.Version())
		require.Equal(t, hash, reopened.TreeByName("test").RootHash())
		require.Equal(t, []byte("survives"), reopened.TreeByName("test").Get([]byte("key")))
	})
	t.Run("drifted-rollback", func(t *testing.T) {
		dir := t.TempDir()
		db := repair1199Open(t, Options{Dir: dir, CreateIfMissing: true, InitialStores: []string{"test"}})
		for i := 0; i < 3; i++ {
			require.NoError(t, db.ApplyChangeSet("test", ChangeSets[i]))
			_, err := db.Commit()
			require.NoError(t, err)
			if i == 0 {
				require.NoError(t, db.RewriteSnapshot(context.Background()))
			}
		}
		for _, index := range []uint64{2, 3} {
			entry, err := db.GetWAL().ReadAt(index)
			require.NoError(t, err)
			require.NoError(t, db.GetWAL().Write(entry))
		}
		require.NoError(t, db.ApplyChangeSet("test", ChangeSets[3]))
		_, err := db.Commit()
		require.NoError(t, err)
		hash := db.TreeByName("test").RootHash()
		require.NoError(t, db.Close())
		previous, err := os.Readlink(currentPath(dir))
		require.NoError(t, err)
		failed, err := OpenDB(3, Options{Dir: dir, LoadForOverwriting: true})
		require.Nil(t, failed)
		require.ErrorContains(t, err, "index offset is inconsistent")
		repair1199NoOpenResources(t, dir)
		current, err := os.Readlink(currentPath(dir))
		require.NoError(t, err)
		require.Equal(t, previous, current)
		reopened := repair1199Open(t, Options{Dir: dir})
		require.Equal(t, int64(4), reopened.Version())
		require.Equal(t, hash, reopened.TreeByName("test").RootHash())
		last, err := reopened.GetWAL().LastOffset()
		require.NoError(t, err)
		require.Equal(t, uint64(6), last, "rejected rollback must not truncate the log")
	})
	t.Run("partial-snapshot-load", func(t *testing.T) {
		dir := t.TempDir()
		db := repair1199Open(t, Options{Dir: dir, CreateIfMissing: true, InitialStores: []string{"a", "z"}})
		for _, name := range []string{"a", "z"} {
			repair1199Set(t, db, name, name)
		}
		_, err := db.Commit()
		require.NoError(t, err)
		require.NoError(t, db.RewriteSnapshot(context.Background()))
		require.NoError(t, db.Close())
		metadataPath := filepath.Join(dir, snapshotName(1), "z", FileNameMetadata)
		metadata, err := os.ReadFile(metadataPath)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(metadataPath, []byte("broken"), 0600))
		failed, err := OpenDB(0, Options{Dir: dir})
		require.Nil(t, failed)
		require.ErrorContains(t, err, "metadata file size")
		repair1199NoOpenResources(t, dir)
		require.NoError(t, os.WriteFile(metadataPath, metadata, 0600))
		reopened := repair1199Open(t, Options{Dir: dir})
		for _, name := range []string{"a", "z"} {
			require.Equal(t, []byte(name), reopened.TreeByName(name).Get([]byte("key")))
		}
	})
	t.Run("wal-open-error", func(t *testing.T) {
		dir := t.TempDir()
		db := repair1199Open(t, Options{Dir: dir, CreateIfMissing: true, InitialStores: []string{"test"}})
		repair1199Set(t, db, "test", "snapshot")
		_, err := db.Commit()
		require.NoError(t, err)
		require.NoError(t, db.RewriteSnapshot(context.Background()))
		require.NoError(t, db.Close())
		logPath := utils.GetChangelogPath(dir)
		require.NoError(t, os.Rename(logPath, logPath+"-saved"))
		require.NoError(t, os.WriteFile(logPath, []byte("obstruction"), 0600))
		failed, err := OpenDB(0, Options{Dir: dir})
		require.Nil(t, failed)
		require.ErrorContains(t, err, "failed to open changelog WAL")
		repair1199NoOpenResources(t, dir)
		require.NoError(t, os.Remove(logPath))
		require.NoError(t, os.Rename(logPath+"-saved", logPath))
		reopened := repair1199Open(t, Options{Dir: dir})
		require.Equal(t, []byte("snapshot"), reopened.TreeByName("test").Get([]byte("key")))
	})
}

func TestRepairM_1199_StaleCurrentLink(t *testing.T) {
	repair1199Buffers(t)
	dir := t.TempDir()
	db := repair1199Open(t, Options{Dir: dir, CreateIfMissing: true, InitialStores: []string{"test"}})
	repair1199Set(t, db, "test", "before")
	_, err := db.Commit()
	require.NoError(t, err)
	require.NoError(t, db.RewriteSnapshot(context.Background()))
	require.NoError(t, db.Close())
	require.NoError(t, os.Symlink(snapshotName(999), currentTmpPath(dir)))
	restarted := repair1199Open(t, Options{Dir: dir})
	require.Equal(t, []byte("before"), restarted.TreeByName("test").Get([]byte("key")))
	repair1199Set(t, restarted, "test", "after")
	_, err = restarted.Commit()
	require.NoError(t, err)
	hash := restarted.TreeByName("test").RootHash()
	require.NoError(t, restarted.RewriteSnapshot(context.Background()))
	current, err := os.Readlink(currentPath(dir))
	require.NoError(t, err)
	require.Equal(t, snapshotName(2), current)
	_, err = os.Lstat(currentTmpPath(dir))
	require.True(t, os.IsNotExist(err))
	require.NoError(t, restarted.Close())
	reopened := repair1199Open(t, Options{Dir: dir})
	require.Equal(t, int64(2), reopened.SnapshotVersion())
	require.Equal(t, hash, reopened.TreeByName("test").RootHash())
	require.Equal(t, []byte("after"), reopened.TreeByName("test").Get([]byte("key")))

	// A failed atomic rename removes only the temporary link created by publication.
	require.NoError(t, os.Remove(currentPath(dir)))
	require.NoError(t, os.Mkdir(currentPath(dir), 0700))
	require.NoError(t, os.WriteFile(filepath.Join(currentPath(dir), "keep"), []byte("keep"), 0600))
	require.Error(t, updateCurrentSymlink(dir, snapshotName(2)))
	_, err = os.Lstat(currentTmpPath(dir))
	require.True(t, os.IsNotExist(err))
	data, err := os.ReadFile(filepath.Join(currentPath(dir), "keep"))
	require.NoError(t, err)
	require.Equal(t, []byte("keep"), data)
	require.NoError(t, os.RemoveAll(currentPath(dir)))
	require.NoError(t, os.Symlink(snapshotName(2), currentPath(dir)))
	require.NoError(t, os.WriteFile(currentTmpPath(dir), []byte("keep"), 0600))
	require.ErrorContains(t, updateCurrentSymlink(dir, snapshotName(2)), "not a symlink")
	data, err = os.ReadFile(currentTmpPath(dir))
	require.NoError(t, err)
	require.Equal(t, []byte("keep"), data)
	current, err = os.Readlink(currentPath(dir))
	require.NoError(t, err)
	require.Equal(t, snapshotName(2), current)
}

func repair1199WaitImporter(t *testing.T, importer *TreeImporter) {
	t.Helper()
	select {
	case <-importer.done:
	case <-time.After(3 * time.Second):
		t.Fatal("import worker did not finish independently of Close")
	}
}

func TestRepairM_1199_ImportErrorBackpressure(t *testing.T) {
	repair1199Buffers(t)
	for _, failure := range []string{"filesystem", "version"} {
		t.Run(failure, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "tree")
			version := int64(-1)
			if failure == "filesystem" {
				version = 1
				require.NoError(t, os.WriteFile(path, []byte("not a directory"), 0600))
				path = filepath.Join(path, "child")
			}
			importer := NewTreeImporter(context.Background(), path, version)
			repair1199WaitImporter(t, importer)
			first := importer.err
			require.Error(t, first)
			result := make(chan error, 1)
			go func() {
				for i := 0; i < nodeChanSize+32; i++ {
					if err := importer.Add(&types.SnapshotNode{Key: []byte("key"), Version: 1}); !errors.Is(err, first) {
						result <- fmt.Errorf("Add did not retain worker error: %v", err)
						return
					}
				}
				result <- nil
			}()
			select {
			case err := <-result:
				require.NoError(t, err)
			case <-time.After(3 * time.Second):
				t.Fatal("failed import blocked its producer past channel capacity")
			}
			require.ErrorIs(t, importer.Close(), first)
			require.ErrorIs(t, importer.Close(), first)
		})
	}
	t.Run("multi-tree-error-transport", func(t *testing.T) {
		dir := t.TempDir()
		importer, err := NewMultiTreeImporter(dir, 1)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(importer.tmpDir(), []byte("not a directory"), 0600))
		require.NoError(t, importer.AddModule("test"))
		repair1199WaitImporter(t, importer.importer)
		first := importer.importer.err
		require.Error(t, first)
		for i := 0; i < nodeChanSize+32; i++ {
			require.ErrorIs(t, importer.Add(&types.SnapshotNode{Key: []byte("key"), Version: 1}), first)
		}
		require.ErrorIs(t, importer.Close(), first)
		repair1199NoOpenResources(t, dir)
		_, err = os.Lstat(currentPath(dir))
		require.True(t, os.IsNotExist(err))
		retry, err := NewMultiTreeImporter(dir, 1)
		require.NoError(t, err)
		require.NoError(t, retry.AddModule("test"))
		require.NoError(t, retry.Add(&types.SnapshotNode{Key: []byte("key"), Value: []byte("retried"), Version: 1}))
		require.NoError(t, retry.Close())
		db := repair1199Open(t, Options{Dir: dir})
		require.Equal(t, []byte("retried"), db.TreeByName("test").Get([]byte("key")))
	})
	t.Run("idle-cancellation", func(t *testing.T) {
		dir := t.TempDir()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		importer, err := NewMultiTreeImporter(dir, 1)
		require.NoError(t, err)
		importer.ctx = ctx
		require.NoError(t, importer.AddModule("test"))
		require.Eventually(t, func() bool {
			_, err := os.Stat(filepath.Join(importer.tmpDir(), "test", FileNameKVs))
			return err == nil
		}, 3*time.Second, time.Millisecond)
		cancel()
		repair1199WaitImporter(t, importer.importer)
		require.ErrorIs(t, importer.Add(&types.SnapshotNode{Key: []byte("key"), Version: 1}), context.Canceled)
		require.ErrorIs(t, importer.Close(), context.Canceled)
		require.ErrorIs(t, importer.Close(), context.Canceled)
		_, err = os.Lstat(currentPath(dir))
		require.True(t, os.IsNotExist(err))
		repair1199NoOpenResources(t, dir)
	})
}

func repair1199Export(t *testing.T) (*Tree, []*types.SnapshotNode) {
	t.Helper()
	tree := New(0)
	t.Cleanup(func() { require.NoError(t, tree.Close()) })
	for _, key := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		tree.Set([]byte(key), []byte("value-"+key))
	}
	_, _, err := tree.SaveVersion(true)
	require.NoError(t, err)
	tree.Remove([]byte("c"))
	tree.Set([]byte("f"), []byte("updated"))
	_, _, err = tree.SaveVersion(true)
	require.NoError(t, err)
	exporter := tree.Export()
	defer exporter.Close()
	var nodes []*types.SnapshotNode
	for {
		node, err := exporter.Next()
		if errors.Is(err, errorutils.ErrorExportDone) {
			break
		}
		require.NoError(t, err)
		nodes = append(nodes, node)
	}
	return tree, nodes
}

func TestRepairM_1199_ImportStackValidation(t *testing.T) {
	repair1199Buffers(t)
	tree, nodes := repair1199Export(t)
	root := len(nodes) - 1
	require.Positive(t, nodes[root].Height)
	require.Zero(t, nodes[0].Height)
	require.Zero(t, nodes[1].Height)
	cases := []string{"first-branch", "nil-node", "truncated-tree", "negative-height", "child-height", "branch-order", "leaf-order", "future-version", "negative-version", "overflow-version", "parent-version"}
	for _, name := range cases {
		t.Run(name, func(t *testing.T) {
			stream := make([]*types.SnapshotNode, len(nodes))
			for i, node := range nodes {
				copy := *node
				stream[i] = &copy
			}
			switch name {
			case "first-branch":
				stream = stream[root:]
			case "nil-node":
				stream = []*types.SnapshotNode{nil}
			case "truncated-tree":
				stream = stream[:2]
			case "negative-height":
				stream[0].Height = -1
			case "child-height":
				stream[root].Height++
			case "branch-order":
				stream[root].Key = []byte("zz")
			case "leaf-order":
				stream[1].Key = stream[0].Key
			case "future-version":
				stream[0].Version = tree.Version() + 1
			case "negative-version":
				stream[0].Version = -1
			case "overflow-version":
				stream[0].Version = math.MaxUint32 + 1
			case "parent-version":
				stream[root].Version = 0
			}
			dir := t.TempDir()
			importer, err := NewMultiTreeImporter(dir, uint64(tree.Version()))
			require.NoError(t, err)
			require.NoError(t, importer.Add("test"))
			for _, node := range stream {
				if err := importer.Add(node); err != nil {
					break
				}
			}
			require.Error(t, importer.Close())
			for _, path := range []string{currentPath(dir), filepath.Join(dir, snapshotName(tree.Version())), importer.tmpDir()} {
				_, err := os.Lstat(path)
				require.True(t, os.IsNotExist(err), "malformed snapshot published or retained: %s", path)
			}
			repair1199NoOpenResources(t, dir)
		})
	}
	t.Run("valid-producer-consumer", func(t *testing.T) {
		dir := t.TempDir()
		importer, err := NewMultiTreeImporter(dir, uint64(tree.Version()))
		require.NoError(t, err)
		require.NoError(t, importer.Add("test"))
		for _, node := range nodes {
			require.NoError(t, importer.Add(node))
		}
		require.NoError(t, importer.Close())
		db := repair1199Open(t, Options{Dir: dir})
		require.Equal(t, tree.Version(), db.Version())
		require.Equal(t, tree.RootHash(), db.TreeByName("test").RootHash())
		for _, key := range []string{"a", "b", "d", "e", "f", "g", "h"} {
			require.Equal(t, tree.Get([]byte(key)), db.TreeByName("test").Get([]byte(key)))
			proof, err := db.TreeByName("test").GetMembershipProof([]byte(key))
			require.NoError(t, err)
			require.True(t, tree.VerifyMembership(proof, []byte(key)))
		}
		require.Nil(t, db.TreeByName("test").Get([]byte("c")))
	})
}

func TestRepairM_1199_ZeroRateConfig(t *testing.T) {
	repair1199Buffers(t)
	for _, rate := range []int{0, DefaultSnapshotWriteRateMBps, 7} {
		t.Run(fmt.Sprint(rate), func(t *testing.T) {
			config := DefaultConfig()
			if rate != DefaultSnapshotWriteRateMBps {
				config.SnapshotWriteRateMBps = rate
			}
			config.AsyncCommitBuffer = 0
			store := NewCommitStore(t.TempDir(), config)
			t.Cleanup(func() { require.NoError(t, store.Close()) })
			require.NoError(t, store.Initialize([]string{"test"}))
			_, err := store.LoadVersion(0, false)
			require.NoError(t, err)
			require.Equal(t, rate, store.db.opts.SnapshotWriteRateMBps)
			limiter := NewGlobalRateLimiter(store.db.opts.SnapshotWriteRateMBps)
			if rate == 0 {
				require.Nil(t, limiter)
			} else {
				require.NotNil(t, limiter)
				require.Equal(t, float64(rate*1024*1024), float64(limiter.Limit()))
			}
			require.NoError(t, store.ApplyChangeSets([]*proto.NamedChangeSet{{Name: "test", Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
				{Key: []byte("key"), Value: []byte("value")},
			}}}}))
			_, err = store.Commit()
			require.NoError(t, err)
			hash := store.db.TreeByName("test").RootHash()
			require.NoError(t, store.db.RewriteSnapshot(context.Background()))
			require.NoError(t, store.Close())
			_, err = store.LoadVersion(0, false)
			require.NoError(t, err)
			require.Equal(t, rate, store.db.opts.SnapshotWriteRateMBps)
			require.Equal(t, int64(1), store.db.SnapshotVersion())
			require.Equal(t, hash, store.db.TreeByName("test").RootHash())
			value, ok, err := store.Get("test", []byte("key"))
			require.NoError(t, err)
			require.True(t, ok)
			require.Equal(t, []byte("value"), value)
		})
	}
}

func TestRepairM_1199_EmptyMembershipProof(t *testing.T) {
	repair1199Buffers(t)
	tree := New(0)
	t.Cleanup(func() { require.NoError(t, tree.Close()) })
	proof, err := tree.GetMembershipProof([]byte("a"))
	require.Error(t, err)
	require.Nil(t, proof)
	for _, key := range []string{"a", "b", "c"} {
		tree.Set([]byte(key), []byte("value-"+key))
	}
	_, _, err = tree.SaveVersion(true)
	require.NoError(t, err)
	for _, key := range []string{"a", "b", "c"} {
		proof, err = tree.GetMembershipProof([]byte(key))
		require.NoError(t, err)
		require.True(t, tree.VerifyMembership(proof, []byte(key)))
	}
	proof, err = tree.GetMembershipProof([]byte("absent"))
	require.Error(t, err)
	require.Nil(t, proof)
	for _, key := range []string{"a", "b", "c"} {
		tree.Remove([]byte(key))
	}
	_, _, err = tree.SaveVersion(true)
	require.NoError(t, err)
	require.True(t, tree.IsEmpty())
	proof, err = tree.GetMembershipProof([]byte("a"))
	require.Error(t, err)
	require.Nil(t, proof)
}
