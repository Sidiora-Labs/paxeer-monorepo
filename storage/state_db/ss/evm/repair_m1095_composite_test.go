package evm_test

import (
	"bytes"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/require"

	commonerrors "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv"
	flatconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/config"
	sctypes "github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/ss/composite"
)

func TestRepairM_1095_CompositeFlatKVRoundTrip(t *testing.T) {
	t.Cleanup(leaktest.CheckTimeout(t, 5*time.Second))
	producer, err := flatkv.NewCommitStore(t.Context(), flatconfig.DefaultTestConfig(t))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, producer.Close()) })
	_, err = producer.LoadVersion(0, false)
	require.NoError(t, err)
	var changesets []*proto.NamedChangeSet
	var expected []types.SnapshotNode
	for _, module := range []string{"evm", "bank"} {
		cs := &proto.NamedChangeSet{Name: module}
		for i := 0; i < 512; i++ {
			key := []byte(fmt.Sprintf("\x15snapshot/%06d", i))
			value := []byte(fmt.Sprintf("%s-value-%d", module, i))
			cs.Changeset.Pairs = append(cs.Changeset.Pairs, &proto.KVPair{Key: key, Value: value})
			expected = append(expected, types.SnapshotNode{StoreKey: module, Key: key, Value: value})
		}
		changesets = append(changesets, cs)
	}
	require.NoError(t, producer.ApplyChangeSets(changesets))
	version, err := producer.Commit()
	require.NoError(t, err)
	exporter, err := producer.Exporter(version)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, exporter.Close()) })
	header, err := exporter.Next()
	require.NoError(t, err)
	require.Equal(t, keys.FlatKVStoreKey, header)
	var nodes []types.SnapshotNode
	for {
		item, err := exporter.Next()
		if err != nil {
			require.ErrorIs(t, err, commonerrors.ErrorExportDone)
			break
		}
		node, ok := item.(*sctypes.SnapshotNode)
		require.True(t, ok)
		nodes = append(nodes, types.SnapshotNode{
			StoreKey: keys.FlatKVStoreKey, Key: bytes.Clone(node.Key), Value: bytes.Clone(node.Value),
		})
	}
	require.Len(t, nodes, len(expected))
	require.NoError(t, exporter.Close())

	dir := t.TempDir()
	cfg := config.StateStoreConfig{
		Backend: "pebbledb", ImportNumWorkers: 2, EVMSplit: true,
		EVMDBDirectory: filepath.Join(dir, "evm"),
	}
	store, err := composite.NewCompositeStateStore(cfg, dir)
	require.NoError(t, err)
	t.Cleanup(func() {
		if store != nil {
			require.NoError(t, store.Close())
		}
	})
	input := make(chan types.SnapshotNode)
	producerDone := make(chan struct{})
	result := make(chan error, 1)
	abort := make(chan struct{})
	defer close(abort)
	go func() {
		defer close(producerDone)
		defer close(input)
		for _, node := range nodes {
			select {
			case input <- node:
			case <-abort:
				return
			}
		}
	}()
	go func() { result <- store.Import(version, input) }()
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	select {
	case err := <-result:
		require.NoError(t, err)
	case <-timer.C:
		t.Fatal("FlatKV producer/composite router/EVM import did not finish within five seconds")
	}
	select {
	case <-producerDone:
	case <-timer.C:
		t.Fatal("snapshot producer did not finish within five seconds")
	}
	check := func() {
		for _, node := range expected {
			value, err := store.Get(node.StoreKey, version, node.Key)
			require.NoError(t, err)
			require.Equal(t, node.Value, value)
		}
	}
	check()
	require.NoError(t, store.Close())
	store = nil
	store, err = composite.NewCompositeStateStore(cfg, dir)
	require.NoError(t, err)
	check()
}
