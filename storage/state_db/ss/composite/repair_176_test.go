package composite

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"testing"

	commonerrors "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv"
	flatconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/config"
	sctypes "github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/wal"
	"github.com/stretchr/testify/require"
)

func TestRepair_176_FlatKVModuleKeyCollision(t *testing.T) {
	producerConfig := flatconfig.DefaultTestConfig(t)
	producer, err := flatkv.NewCommitStore(t.Context(), producerConfig)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, producer.Close()) })
	_, err = producer.LoadVersion(0, false)
	require.NoError(t, err)

	var expected []types.SnapshotNode
	var changesets []*proto.NamedChangeSet
	for _, module := range []string{"bank", "staking", "auth"} {
		cs := &proto.NamedChangeSet{Name: module}
		for _, length := range []int{21, 53} {
			for _, prefix := range []byte{0x03, 0x07, 0x08, 0x0a} {
				key := bytes.Repeat([]byte{0xab}, length)
				key[0] = prefix
				value := append([]byte{0x00, 0xff, prefix, byte(length)}, []byte(module)...)
				cs.Changeset.Pairs = append(cs.Changeset.Pairs, &proto.KVPair{Key: key, Value: value})
				expected = append(expected, types.SnapshotNode{StoreKey: module, Key: key, Value: value})
			}
		}
		changesets = append(changesets, cs)
	}

	address := bytes.Repeat([]byte{0xab}, 20)
	nonce := make([]byte, 8)
	binary.BigEndian.PutUint64(nonce, 42)
	evmCS := &proto.NamedChangeSet{Name: "evm"}
	for _, pair := range []*proto.KVPair{
		{Key: keys.BuildEVMKey(keys.EVMKeyNonce, address), Value: nonce},
		{Key: keys.BuildEVMKey(keys.EVMKeyCodeHash, address), Value: bytes.Repeat([]byte{0xcd}, 32)},
		{Key: keys.BuildEVMKey(keys.EVMKeyCode, address), Value: []byte{0x60, 0x00, 0xff}},
		{Key: keys.BuildEVMKey(keys.EVMKeyStorage, bytes.Repeat([]byte{0xab}, 52)), Value: bytes.Repeat([]byte{0xef}, 32)},
		{Key: []byte{0x15, 0x00, 0xff}, Value: []byte("evm legacy")},
	} {
		evmCS.Changeset.Pairs = append(evmCS.Changeset.Pairs, pair)
		expected = append(expected, types.SnapshotNode{StoreKey: "evm", Key: pair.Key, Value: pair.Value})
	}
	changesets = append(changesets, evmCS)
	require.NoError(t, producer.ApplyChangeSets(changesets))
	version, err := producer.Commit()
	require.NoError(t, err)
	require.Equal(t, int64(1), version)
	for _, node := range expected {
		value, found := producer.Get(node.StoreKey, node.Key)
		require.True(t, found)
		require.Equal(t, node.Value, value)
	}

	exporter, err := producer.Exporter(version)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, exporter.Close()) })
	header, err := exporter.Next()
	require.NoError(t, err)
	module, ok := header.(string)
	require.True(t, ok)
	require.Equal(t, keys.FlatKVStoreKey, module)
	var exported []types.SnapshotNode
	for {
		item, err := exporter.Next()
		if err != nil {
			require.ErrorIs(t, err, commonerrors.ErrorExportDone)
			break
		}
		node, ok := item.(*sctypes.SnapshotNode)
		require.True(t, ok, "unexpected export item %T", item)
		exported = append(exported, types.SnapshotNode{StoreKey: module, Key: node.Key, Value: node.Value})
	}
	// FlatKV merges the nonce and code hash into one physical account row.
	require.Len(t, exported, len(expected)-1)
	require.NoError(t, exporter.Close())
	require.NoError(t, producer.Close())

	for _, mode := range []struct{ split, separate bool }{{false, false}, {true, false}, {true, true}} {
		t.Run(fmt.Sprintf("split=%t/separate=%t", mode.split, mode.separate), func(t *testing.T) {
			dir := t.TempDir()
			cfg := repair176Config(dir, mode.separate)
			cfg.EVMSplit = mode.split
			consumer, err := NewCompositeStateStore(cfg, dir)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, consumer.Close()) })
			transport := make(chan types.SnapshotNode, len(exported))
			for _, node := range exported {
				transport <- node
			}
			close(transport)
			require.NoError(t, consumer.Import(version, transport))
			require.NoError(t, consumer.SetEarliestVersion(version, false))
			require.NoError(t, consumer.SetLatestVersion(version))
			require.NoError(t, consumer.Close())

			reopened, err := NewCompositeStateStore(cfg, dir)
			require.NoError(t, err)
			consumer = reopened
			require.Equal(t, version, consumer.GetLatestVersion())
			for _, node := range expected {
				value, err := consumer.Get(node.StoreKey, version, node.Key)
				require.NoError(t, err)
				require.Equal(t, node.Value, value, "module=%s key=%x", node.StoreKey, node.Key)
				if node.StoreKey != "evm" {
					wrongModule, err := consumer.Get("evm", version, node.Key)
					require.NoError(t, err)
					var evmValue []byte
					for _, evmNode := range expected {
						if evmNode.StoreKey == "evm" && bytes.Equal(evmNode.Key, node.Key) {
							evmValue = evmNode.Value
							break
						}
					}
					require.Equal(t, evmValue, wrongModule, "non-EVM key leaked into evm: %x", node.Key)
				}
			}
		})
	}
}

func repair176Config(dir string, separate bool) config.StateStoreConfig {
	return config.StateStoreConfig{
		Backend:           "pebbledb",
		ImportNumWorkers:  1,
		EVMSplit:          true,
		SeparateEVMSubDBs: separate,
		EVMDBDirectory:    filepath.Join(dir, "evm_ss"),
	}
}

// Emit the recovery input through the real commit producer and its on-disk WAL.
func repair176History(t *testing.T, initialVersion int64) []proto.ChangelogEntry {
	t.Helper()
	cfg := flatconfig.DefaultTestConfig(t)
	producer, err := flatkv.NewCommitStore(t.Context(), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, producer.Close()) })
	_, err = producer.LoadVersion(0, false)
	require.NoError(t, err)
	require.NoError(t, producer.SetInitialVersion(initialVersion))
	for version := initialVersion; version < initialVersion+5; version++ {
		changesets := []*proto.NamedChangeSet{{Name: "bank", Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
			{Key: []byte{0x01, byte(version)}, Value: []byte{byte(version), 0xff}},
		}}}}
		if version != 3 {
			changesets = append(changesets, &proto.NamedChangeSet{Name: "evm", Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
				{Key: []byte{0x15, byte(version)}, Value: []byte{0x00, byte(version)}},
			}}})
		}
		require.NoError(t, producer.ApplyChangeSets(changesets))
		committed, err := producer.Commit()
		require.NoError(t, err)
		require.Equal(t, version, committed)
	}
	require.NoError(t, producer.Close())
	log, err := wal.NewChangelogWAL(filepath.Join(cfg.DataDir, "changelog"), wal.Config{})
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, log.Close()) })
	first, err := log.FirstOffset()
	require.NoError(t, err)
	last, err := log.LastOffset()
	require.NoError(t, err)
	var entries []proto.ChangelogEntry
	require.NoError(t, log.Replay(first, last, func(_ uint64, entry proto.ChangelogEntry) error {
		entries = append(entries, entry)
		return nil
	}))
	require.Len(t, entries, 5)
	require.NoError(t, log.Close())
	return entries
}

func TestRepair_176_SplitRecoveryIncompleteWAL(t *testing.T) {
	entries := repair176History(t, 1)
	for _, separate := range []bool{false, true} {
		for _, lagging := range []string{"evm", "cosmos"} {
			for _, tc := range []struct {
				name                          string
				versions                      []int
				truncateBefore, truncateAfter uint64
				wantError                     bool
				synchronized                  bool
			}{
				{name: "empty", wantError: true},
				{name: "missing_leading", versions: []int{1, 2, 3, 4, 5}, truncateBefore: 3, wantError: true},
				{name: "missing_tail", versions: []int{1, 2, 3, 4, 5}, truncateAfter: 2, wantError: true},
				{name: "missing_interior", versions: []int{1, 2, 4, 5}, wantError: true},
				{name: "complete_retained", versions: []int{1, 2, 3, 4, 5}, truncateBefore: 2},
				{name: "synchronized_empty", synchronized: true},
			} {
				t.Run(fmt.Sprintf("separate=%t/lagging=%s/%s", separate, lagging, tc.name), func(t *testing.T) {
					dir := t.TempDir()
					cfg := repair176Config(dir, separate)
					store, err := NewCompositeStateStore(cfg, dir)
					require.NoError(t, err)
					t.Cleanup(func() { require.NoError(t, store.Close()) })
					for _, entry := range entries[:4] {
						if tc.synchronized || lagging != "cosmos" || entry.Version == 1 {
							require.NoError(t, store.cosmosStore.ApplyChangesetSync(entry.Version, stripEVMFromChangesets(entry.Changesets)))
							require.NoError(t, store.cosmosStore.SetLatestVersion(entry.Version))
						}
						if tc.synchronized || lagging != "evm" || entry.Version == 1 {
							require.NoError(t, store.evmStore.ApplyChangesetSync(entry.Version, filterEVMChangesets(entry.Changesets)))
							require.NoError(t, store.evmStore.SetLatestVersion(entry.Version))
						}
					}
					require.NoError(t, store.SetEarliestVersion(1, false))
					require.Equal(t, store.cosmosStore.GetEarliestVersion(), store.evmStore.GetEarliestVersion())
					if !tc.synchronized {
						require.NotEqual(t, store.cosmosStore.GetLatestVersion(), store.evmStore.GetLatestVersion())
						err := store.validateEVMSSPostRecovery()
						require.Error(t, err)
						require.Contains(t, err.Error(), "latest version")
						require.Contains(t, err.Error(), "state sync")
					}
					require.NoError(t, store.Close())

					log, err := wal.NewChangelogWAL(utils.GetChangelogPath(utils.GetStateStorePath(dir, cfg.Backend)), wal.Config{})
					require.NoError(t, err)
					for _, version := range tc.versions {
						require.NoError(t, log.Write(entries[version-1]))
					}
					if tc.truncateBefore > 0 {
						require.NoError(t, log.TruncateBefore(tc.truncateBefore))
					}
					if tc.truncateAfter > 0 {
						require.NoError(t, log.TruncateAfter(tc.truncateAfter))
					}
					require.NoError(t, log.Close())

					reopened, err := NewCompositeStateStore(cfg, dir)
					if tc.wantError {
						if reopened != nil {
							t.Cleanup(func() { require.NoError(t, reopened.Close()) })
						}
						require.Error(t, err)
						require.Contains(t, err.Error(), "state sync")
						require.Contains(t, err.Error(), "incomplete split SS WAL")
						require.Nil(t, reopened, "incomplete history must never reach query consumers")
						return
					}
					require.NoError(t, err)
					t.Cleanup(func() { require.NoError(t, reopened.Close()) })
					wantLatest := int64(5)
					if tc.synchronized {
						wantLatest = 4
					}
					require.Equal(t, wantLatest, reopened.cosmosStore.GetLatestVersion())
					require.Equal(t, wantLatest, reopened.evmStore.GetLatestVersion())
					for _, entry := range entries[:wantLatest] {
						for _, cs := range entry.Changesets {
							for _, pair := range cs.Changeset.Pairs {
								value, err := reopened.Get(cs.Name, entry.Version, pair.Key)
								require.NoError(t, err)
								require.Equal(t, pair.Value, value, "module=%s version=%d key=%x", cs.Name, entry.Version, pair.Key)
							}
						}
					}
					require.NoError(t, reopened.Close())
					again, err := NewCompositeStateStore(cfg, dir)
					require.NoError(t, err)
					reopened = again
					require.Equal(t, wantLatest, reopened.cosmosStore.GetLatestVersion())
					require.Equal(t, wantLatest, reopened.evmStore.GetLatestVersion())
				})
			}
		}
	}
}

func TestRepair_176_InitialHeightRecovery(t *testing.T) {
	entries := repair176History(t, 100)
	dir := t.TempDir()
	cfg := repair176Config(dir, true)
	log, err := wal.NewChangelogWAL(utils.GetChangelogPath(utils.GetStateStorePath(dir, cfg.Backend)), wal.Config{})
	require.NoError(t, err)
	for _, entry := range entries {
		require.NoError(t, log.Write(entry))
	}
	require.NoError(t, log.Close())
	store, err := NewCompositeStateStore(cfg, dir)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.Equal(t, int64(104), store.cosmosStore.GetLatestVersion())
	require.Equal(t, int64(104), store.evmStore.GetLatestVersion())
	for _, entry := range entries {
		for _, cs := range entry.Changesets {
			for _, pair := range cs.Changeset.Pairs {
				value, err := store.Get(cs.Name, entry.Version, pair.Key)
				require.NoError(t, err)
				require.Equal(t, pair.Value, value)
			}
		}
	}
}
