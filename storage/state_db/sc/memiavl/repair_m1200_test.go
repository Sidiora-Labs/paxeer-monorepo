package memiavl

import (
	"context"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	ics23 "github.com/confio/ics23/go"
	"github.com/stretchr/testify/require"

	errorutils "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/types"
)

func TestRepairM_1200_UnloadedReads(t *testing.T) {
	cs := NewCommitStore(t.TempDir(), DefaultConfig())
	require.NoError(t, cs.Initialize([]string{"params"}))
	for name, store := range map[string]*CommitStore{"nil": nil, "before-load": cs} {
		t.Run(name, func(t *testing.T) {
			require.False(t, store.IsLoaded())
			var reader types.Committer = store
			for _, route := range []string{"params", "pramas"} {
				value, ok, err := reader.Get(route, []byte("key"))
				require.NoError(t, err)
				require.Nil(t, value)
				require.False(t, ok)
				has, err := reader.Has(route, []byte("key"))
				require.NoError(t, err)
				require.False(t, has)
				proof, err := reader.GetProof(route, []byte("key"))
				require.NoError(t, err)
				require.Nil(t, proof)
				for _, ascending := range []bool{true, false} {
					iter, err := reader.Iterator(route, nil, nil, ascending)
					require.NoError(t, err)
					require.Nil(t, iter)
				}
			}
			_, _, err := reader.Get("", []byte("key"))
			require.ErrorContains(t, err, "store name cannot be empty")
			_, _, err = reader.Get("params", nil)
			require.ErrorContains(t, err, "key cannot be nil")
			_, err = reader.Has("", []byte("key"))
			require.ErrorContains(t, err, "store name cannot be empty")
			_, err = reader.Has("params", nil)
			require.ErrorContains(t, err, "key cannot be nil")
			_, err = reader.GetProof("", []byte("key"))
			require.ErrorContains(t, err, "store name cannot be empty")
			_, err = reader.GetProof("params", nil)
			require.ErrorContains(t, err, "key cannot be nil")
			_, err = reader.Iterator("", nil, nil, true)
			require.ErrorContains(t, err, "store name cannot be empty")
		})
	}
}

func TestRepairM_1200_UnroutableRead(t *testing.T) {
	cs := NewCommitStore(t.TempDir(), Config{AsyncCommitBuffer: -1})
	require.NoError(t, cs.Initialize([]string{"params", "empty", "empty-value"}))
	reader, err := cs.LoadVersion(0, false)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, cs.Close()) })
	require.True(t, cs.IsLoaded())
	require.NoError(t, reader.ApplyChangeSets([]*proto.NamedChangeSet{{Name: "params", Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
		{Key: []byte("a"), Value: []byte("first")},
		{Key: []byte("b"), Value: []byte("second")},
	}}}, {Name: "empty-value", Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
		{Key: []byte("zero"), Value: []byte{}},
	}}}}))
	_, err = reader.Commit()
	require.NoError(t, err)
	for _, route := range []string{"pramas", "missing"} {
		t.Run(route, func(t *testing.T) {
			value, ok, err := reader.Get(route, []byte("a"))
			require.ErrorContains(t, err, fmt.Sprintf("store %q not found", route))
			require.Nil(t, value)
			require.False(t, ok)
			has, err := reader.Has(route, []byte("a"))
			require.ErrorContains(t, err, fmt.Sprintf("store %q not found", route))
			require.False(t, has)
			proof, err := reader.GetProof(route, []byte("a"))
			require.ErrorContains(t, err, fmt.Sprintf("store %q not found", route))
			require.Nil(t, proof)
			for _, ascending := range []bool{true, false} {
				iter, err := reader.Iterator(route, nil, nil, ascending)
				require.ErrorContains(t, err, fmt.Sprintf("store %q not found", route))
				require.Nil(t, iter)
			}
		})
	}
	for _, route := range []string{"params", "empty"} {
		value, ok, err := reader.Get(route, []byte("absent"))
		require.NoError(t, err)
		require.Nil(t, value)
		require.False(t, ok)
		has, err := reader.Has(route, []byte("absent"))
		require.NoError(t, err)
		require.False(t, has)
		proof, err := reader.GetProof(route, []byte("absent"))
		require.NoError(t, err)
		if route == "params" {
			require.True(t, ics23.VerifyNonMembership(ics23.IavlSpec, reader.GetChildStoreByName(route).RootHash(), proof, []byte("absent")))
		}
	}
	for key, expected := range map[string][]byte{"a": []byte("first"), "b": []byte("second")} {
		value, ok, err := reader.Get("params", []byte(key))
		require.NoError(t, err)
		require.True(t, ok)
		require.Equal(t, expected, value)
		has, err := reader.Has("params", []byte(key))
		require.NoError(t, err)
		require.True(t, has)
		proof, err := reader.GetProof("params", []byte(key))
		require.NoError(t, err)
		require.True(t, ics23.VerifyMembership(ics23.IavlSpec, reader.GetChildStoreByName("params").RootHash(), proof, []byte(key), expected))
	}
	value, ok, err := reader.Get("empty-value", []byte("zero"))
	require.NoError(t, err)
	require.True(t, ok)
	require.Equal(t, []byte{}, value)
	has, err := reader.Has("empty-value", []byte("zero"))
	require.NoError(t, err)
	require.True(t, has)
	proof, err := reader.GetProof("empty-value", []byte("zero"))
	require.NoError(t, err)
	require.NotNil(t, proof.GetExist())
	require.Equal(t, []byte("zero"), proof.GetExist().Key)
	require.Empty(t, proof.GetExist().Value)
	for _, ascending := range []bool{true, false} {
		iter, err := reader.Iterator("params", nil, nil, ascending)
		require.NoError(t, err)
		require.NotNil(t, iter)
		var keys []string
		for ; iter.Valid(); iter.Next() {
			keys = append(keys, string(iter.Key()))
		}
		require.NoError(t, iter.Error())
		require.NoError(t, iter.Close())
		expected := []string{"a", "b"}
		if !ascending {
			expected = []string{"b", "a"}
		}
		require.Equal(t, expected, keys)
		iter, err = reader.Iterator("empty", nil, nil, ascending)
		require.NoError(t, err)
		require.NotNil(t, iter)
		require.False(t, iter.Valid())
		require.NoError(t, iter.Error())
		require.NoError(t, iter.Close())
	}
}

func repair1200Tree(t *testing.T, leaves int) *Tree {
	t.Helper()
	tree := New(0)
	t.Cleanup(func() { require.NoError(t, tree.Close()) })
	for i := 0; i < leaves; i++ {
		tree.Set([]byte(fmt.Sprintf("key-%08d", i)), []byte(fmt.Sprintf("value-%08d", i)))
	}
	_, _, err := tree.SaveVersion(true)
	require.NoError(t, err)
	return tree
}

func repair1200Await(t *testing.T, done <-chan struct{}, operation string) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("%s did not complete within 2 seconds", operation)
	}
}

func TestRepairM_1200_ScanStopConvention(t *testing.T) {
	tree := repair1200Tree(t, 32768)
	for _, stop := range []int{1, 2, 3, 17, 64} {
		calls := 0
		tree.ScanPostOrder(func(node Node) bool {
			calls++
			require.NotNil(t, node)
			return calls == stop
		})
		require.Equal(t, stop, calls, "true must stop the scan")
	}
	var scan []*types.SnapshotNode
	tree.ScanPostOrder(func(node Node) bool {
		scan = append(scan, &types.SnapshotNode{Key: node.Key(), Value: node.Value(), Height: int8(node.Height()), Version: int64(node.Version())})
		return false
	})
	require.Len(t, scan, 2*32768-1, "false must continue through every node")
	exporter := tree.Export()
	defer exporter.Close()
	for _, expected := range scan {
		node, err := exporter.Next()
		require.NoError(t, err)
		require.Equal(t, expected, node)
	}
	_, err := exporter.Next()
	require.ErrorIs(t, err, errorutils.ErrorExportDone)
	for _, leaves := range []int{0, 1} {
		small := repair1200Tree(t, leaves)
		calls := 0
		small.ScanPostOrder(func(Node) bool { calls++; return true })
		require.Equal(t, leaves, calls)
	}
}

func TestRepairM_1200_ExportCancellation(t *testing.T) {
	tree := repair1200Tree(t, 32768)
	var calls atomic.Int64
	blocked := make(chan struct{})
	exporter := newExporter(func(callback func(*types.SnapshotNode) bool) {
		tree.ScanPostOrder(func(node Node) bool {
			if calls.Add(1) == exportBufferSize+1 {
				close(blocked)
			}
			return callback(&types.SnapshotNode{Key: node.Key(), Value: node.Value(), Height: int8(node.Height()), Version: int64(node.Version())})
		})
	})
	defer exporter.Close()
	repair1200Await(t, blocked, "export reaching channel backpressure")
	wrote := make(chan struct{})
	go func() { tree.Set([]byte("after-close"), []byte("written")); close(wrote) }()
	closed := make(chan struct{})
	go func() { exporter.Close(); close(closed) }()
	repair1200Await(t, closed, "canceled export")
	repair1200Await(t, wrote, "write after canceled export")
	require.GreaterOrEqual(t, calls.Load(), int64(exportBufferSize+1))
	require.LessOrEqual(t, calls.Load(), int64(exportBufferSize+2), "cancellation must bound callbacks independently of tree size")
	require.Equal(t, []byte("written"), tree.Get([]byte("after-close")))
	_, err := exporter.Next()
	require.ErrorIs(t, err, errorutils.ErrorExportDone)
	exporter.Close()

	// Exercise Tree.Export itself with a queued writer and a partially consumed stream.
	direct := tree.Export()
	defer direct.Close()
	node, err := direct.Next()
	require.NoError(t, err)
	require.NotNil(t, node)
	wrote = make(chan struct{})
	go func() { tree.Set([]byte("after-direct-close"), []byte("written")); close(wrote) }()
	closed = make(chan struct{})
	go func() { direct.Close(); close(closed) }()
	repair1200Await(t, closed, "Tree.Export close")
	repair1200Await(t, wrote, "write after Tree.Export close")
	require.Equal(t, []byte("written"), tree.Get([]byte("after-direct-close")))
	_, err = direct.Next()
	require.ErrorIs(t, err, errorutils.ErrorExportDone)
	direct.Close()
}

func TestRepairM_1200_SnapshotExportConvention(t *testing.T) {
	for _, leaves := range []int{0, 1, 32768} {
		t.Run(fmt.Sprint(leaves), func(t *testing.T) {
			tree := repair1200Tree(t, leaves)
			dir := t.TempDir()
			require.NoError(t, tree.WriteSnapshot(context.Background(), dir))
			snapshot, err := OpenSnapshot(dir, Options{})
			require.NoError(t, err)
			persisted := NewFromSnapshot(snapshot, Options{})
			t.Cleanup(func() { require.NoError(t, persisted.Close()) })
			total := 2*leaves - 1
			if leaves == 0 {
				total = 0
			}
			for _, stop := range []int{1, 2, 3, 17, 64} {
				calls := 0
				snapshot.export(func(node *types.SnapshotNode) bool {
					calls++
					require.NotNil(t, node)
					return calls == stop
				})
				expected := stop
				if total < expected {
					expected = total
				}
				require.Equal(t, expected, calls)
			}
			mutableExport, persistedExport := tree.Export(), persisted.Export()
			defer mutableExport.Close()
			defer persistedExport.Close()
			for i := 0; i < total; i++ {
				expected, err := mutableExport.Next()
				require.NoError(t, err)
				node, err := persistedExport.Next()
				require.NoError(t, err)
				require.Equal(t, expected, node)
			}
			_, err = mutableExport.Next()
			require.ErrorIs(t, err, errorutils.ErrorExportDone)
			_, err = persistedExport.Next()
			require.ErrorIs(t, err, errorutils.ErrorExportDone)
			if leaves == 0 {
				return
			}
			var calls atomic.Int64
			blocked := make(chan struct{})
			counted := newExporter(func(callback func(*types.SnapshotNode) bool) {
				snapshot.export(func(node *types.SnapshotNode) bool {
					if calls.Add(1) == exportBufferSize+1 {
						close(blocked)
					}
					return callback(node)
				})
			})
			defer counted.Close()
			if total > exportBufferSize {
				repair1200Await(t, blocked, "snapshot export reaching backpressure")
			}
			closed := make(chan struct{})
			go func() { counted.Close(); close(closed) }()
			repair1200Await(t, closed, "snapshot export close")
			require.LessOrEqual(t, calls.Load(), int64(exportBufferSize+2))
			early := persisted.Export()
			defer early.Close()
			_, err = early.Next()
			require.NoError(t, err)
			closed = make(chan struct{})
			go func() { early.Close(); close(closed) }()
			repair1200Await(t, closed, "snapshot-backed Tree.Export close")
			persisted.Set([]byte("after-snapshot-close"), []byte("written"))
			require.Equal(t, []byte("written"), persisted.Get([]byte("after-snapshot-close")))
			_, err = early.Next()
			require.ErrorIs(t, err, errorutils.ErrorExportDone)
			early.Close()
		})
	}
}
