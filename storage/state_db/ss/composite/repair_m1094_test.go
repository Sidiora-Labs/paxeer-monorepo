package composite

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/pebbledb/mvcc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv"
	flatconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/ss/evm"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/wal"
	"github.com/stretchr/testify/require"
)

func repairM1094Config(dir string, separate bool) config.StateStoreConfig {
	return config.StateStoreConfig{
		Backend: "pebbledb", ImportNumWorkers: 1, EVMSplit: true,
		SeparateEVMSubDBs: separate, EVMDBDirectory: filepath.Join(dir, "evm_ss"),
	}
}

type repairM1094Row struct {
	Key, Value string
	Version    int64
}

func repairM1094Raw(t *testing.T, store types.StateStore, module string) []repairM1094Row {
	t.Helper()
	var rows []repairM1094Row
	stopped, err := store.RawIterate(module, func(key, value []byte, version int64) bool {
		rows = append(rows, repairM1094Row{string(key), string(value), version})
		return false
	})
	require.NoError(t, err)
	require.False(t, stopped)
	return rows
}

// FlatKV commits produce both the changesets and the durable recovery stream.
func repairM1094History(t *testing.T) []proto.ChangelogEntry {
	t.Helper()
	cfg := flatconfig.DefaultTestConfig(t)
	producer, err := flatkv.NewCommitStore(t.Context(), cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, producer.Close()) })
	_, err = producer.LoadVersion(0, false)
	require.NoError(t, err)
	for version := int64(1); version <= 4; version++ {
		address := bytes.Repeat([]byte{0xab}, 20)
		nonce := make([]byte, 8)
		binary.BigEndian.PutUint64(nonce, uint64(version))
		legacyValue := []byte{byte(version)}
		if version == 3 {
			legacyValue = nil
		}
		changesets := []*proto.NamedChangeSet{
			{Name: "bank", Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{{Key: []byte("supply"), Value: []byte{byte(version)}}}}},
			{Name: "evm", Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
				{Key: keys.BuildEVMKey(keys.EVMKeyNonce, address), Value: nonce},
				{Key: keys.BuildEVMKey(keys.EVMKeyCodeHash, address), Value: bytes.Repeat([]byte{byte(version)}, 32)},
				{Key: keys.BuildEVMKey(keys.EVMKeyCode, address), Value: []byte{0x60, byte(version)}},
				{Key: keys.BuildEVMKey(keys.EVMKeyStorage, bytes.Repeat([]byte{0xab}, 52)), Value: bytes.Repeat([]byte{byte(version)}, 32)},
				{Key: []byte{0x15, 0xff}, Value: legacyValue, Delete: version == 3},
			}}},
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
	require.NoError(t, log.Close())
	require.Len(t, entries, 4)
	return entries
}

func TestRepairM_1094_RawHistory(t *testing.T) {
	entries := repairM1094History(t)
	for _, mode := range []struct{ split, separate bool }{{false, false}, {true, false}, {true, true}} {
		t.Run(fmt.Sprintf("split=%t/separate=%t", mode.split, mode.separate), func(t *testing.T) {
			dir := t.TempDir()
			cfg := repairM1094Config(dir, mode.separate)
			cfg.EVMSplit = mode.split
			store, err := NewCompositeStateStore(cfg, dir)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, store.Close()) })
			want := map[string][]repairM1094Row{}
			for _, entry := range entries {
				require.NoError(t, store.ApplyChangesetSync(entry.Version, entry.Changesets))
				require.NoError(t, store.SetLatestVersion(entry.Version))
				for _, cs := range entry.Changesets {
					for _, pair := range cs.Changeset.Pairs {
						if !pair.Delete {
							want[cs.Name] = append(want[cs.Name], repairM1094Row{string(pair.Key), string(pair.Value), entry.Version})
						}
					}
				}
			}
			require.Len(t, want["evm"], 19)
			require.Len(t, want["bank"], 4)
			check := func() {
				for _, module := range []string{"evm", "bank"} {
					require.ElementsMatch(t, want[module], repairM1094Raw(t, store, module))
					for _, row := range want[module] {
						value, err := store.Get(module, row.Version, []byte(row.Key))
						require.NoError(t, err)
						require.Equal(t, []byte(row.Value), value)
					}
				}
				deleted, err := store.Get("evm", 3, []byte{0x15, 0xff})
				require.NoError(t, err)
				require.Nil(t, deleted)
				calls := 0
				stopped, err := store.RawIterate("evm", func(_, _ []byte, _ int64) bool { calls++; return true })
				require.NoError(t, err)
				require.True(t, stopped)
				require.Equal(t, 1, calls, "stop must propagate across every sub-DB")
				require.Empty(t, repairM1094Raw(t, store, "missing"))
				if cfg.EVMSplit {
					require.Empty(t, repairM1094Raw(t, store.cosmosStore, "evm"))
				}
			}
			check()
			require.NoError(t, store.Close())
			store, err = NewCompositeStateStore(cfg, dir)
			require.NoError(t, err)
			check()
		})
	}
}

func TestRepairM_1094_MaintenanceErrors(t *testing.T) {
	for _, separate := range []bool{false, true} {
		t.Run(fmt.Sprintf("separate=%t", separate), func(t *testing.T) {
			dir := t.TempDir()
			cfg := repairM1094Config(dir, separate)
			store, err := NewCompositeStateStore(cfg, dir)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, store.Close()) })
			for version := int64(1); version <= 3; version++ {
				require.NoError(t, store.ApplyChangesetSync(version, []*proto.NamedChangeSet{
					{Name: "bank", Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{{Key: []byte("balance"), Value: []byte{byte(version)}}}}},
					{Name: "evm", Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{{Key: []byte{0x15}, Value: []byte{byte(version)}}}}},
				}))
				require.NoError(t, store.SetLatestVersion(version))
			}
			require.NoError(t, store.SetEarliestVersion(1, false))
			// These errors originate in the real MVCC backend's version validation.
			err = store.SetLatestVersion(-1)
			require.ErrorContains(t, err, "EVM store latest version")
			require.ErrorContains(t, err, "non-negative")
			err = store.SetEarliestVersion(-1, true)
			require.ErrorContains(t, err, "EVM store earliest version")
			require.ErrorContains(t, err, "non-negative")
			require.Equal(t, int64(3), store.GetLatestVersion())
			require.Equal(t, int64(1), store.GetEarliestVersion())
			require.NoError(t, store.evmStore.Close())
			require.ErrorContains(t, store.SetLatestVersion(4), "EVM store latest version")
			require.ErrorContains(t, store.SetEarliestVersion(2, false), "EVM store earliest version")
			err = store.Prune(1)
			require.ErrorContains(t, err, "EVM store")
			require.ErrorContains(t, err, "closed")
			require.Equal(t, int64(1), store.cosmosStore.GetEarliestVersion(), "failed EVM maintenance must leave Cosmos history intact")
			require.Equal(t, int64(3), store.cosmosStore.GetLatestVersion())
			_, err = store.RawIterate("evm", func(_, _ []byte, _ int64) bool { t.Fatal("closed backend called callback"); return false })
			require.ErrorContains(t, err, "closed")
			require.NoError(t, store.Close())
			store, err = NewCompositeStateStore(cfg, dir)
			require.NoError(t, err)
			require.Equal(t, int64(3), store.GetLatestVersion())
			require.Equal(t, int64(1), store.GetEarliestVersion())
			for _, module := range []string{"bank", "evm"} {
				require.Len(t, repairM1094Raw(t, store, module), 3)
			}
			require.NoError(t, store.Prune(1))
			require.Equal(t, int64(2), store.GetEarliestVersion())
		})
	}
}

func TestRepairM_1094_JointProgressAndRecovery(t *testing.T) {
	entries := repairM1094History(t)
	for _, separate := range []bool{false, true} {
		for _, lagging := range []string{"evm", "cosmos"} {
			t.Run(fmt.Sprintf("separate=%t/lagging=%s", separate, lagging), func(t *testing.T) {
				dir := t.TempDir()
				cfg := repairM1094Config(dir, separate)
				store, err := NewCompositeStateStore(cfg, dir)
				require.NoError(t, err)
				t.Cleanup(func() { require.NoError(t, store.Close()) })
				for _, entry := range entries {
					if lagging != "cosmos" || entry.Version == 1 {
						require.NoError(t, store.cosmosStore.ApplyChangesetSync(entry.Version, stripEVMFromChangesets(entry.Changesets)))
						require.NoError(t, store.cosmosStore.SetLatestVersion(entry.Version))
					}
					if lagging != "evm" || entry.Version == 1 {
						require.NoError(t, store.evmStore.ApplyChangesetSync(entry.Version, filterEVMChangesets(entry.Changesets)))
						require.NoError(t, store.evmStore.SetLatestVersion(entry.Version))
					}
				}
				require.NoError(t, store.SetEarliestVersion(1, false))
				require.Equal(t, int64(1), store.GetLatestVersion())
				require.ErrorContains(t, store.Prune(2), "preserve split SS recovery history")
				require.ErrorContains(t, store.SetEarliestVersion(3, false), "preserve split SS recovery history")
				require.Equal(t, int64(1), store.cosmosStore.GetEarliestVersion())
				require.Equal(t, int64(1), store.evmStore.GetEarliestVersion())
				require.NoError(t, store.Close())
				log, err := wal.NewChangelogWAL(utils.GetChangelogPath(utils.GetStateStorePath(dir, cfg.Backend)), wal.Config{})
				require.NoError(t, err)
				for _, entry := range entries {
					require.NoError(t, log.Write(entry))
				}
				require.NoError(t, log.Close())
				store, err = NewCompositeStateStore(cfg, dir)
				require.NoError(t, err)
				require.Equal(t, int64(4), store.GetLatestVersion())
				require.Equal(t, int64(4), store.cosmosStore.GetLatestVersion())
				require.Equal(t, int64(4), store.evmStore.GetLatestVersion())
				require.Len(t, repairM1094Raw(t, store, "evm"), 19)
				require.Len(t, repairM1094Raw(t, store, "bank"), 4)
				require.NoError(t, store.Close())
				store, err = NewCompositeStateStore(cfg, dir)
				require.NoError(t, err)
				require.Equal(t, int64(4), store.GetLatestVersion())
				require.Len(t, repairM1094Raw(t, store, "evm"), 19)
			})
		}
	}
}

func TestRepairM_1094_JointEarliest(t *testing.T) {
	for _, separate := range []bool{false, true} {
		t.Run(fmt.Sprintf("separate=%t", separate), func(t *testing.T) {
			dir := t.TempDir()
			cfg := repairM1094Config(dir, separate)
			store, err := NewCompositeStateStore(cfg, dir)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, store.Close()) })
			require.NoError(t, store.SetLatestVersion(5))
			require.NoError(t, store.SetEarliestVersion(1, false))
			require.NoError(t, store.evmStore.SetEarliestVersion(3, false))
			require.Equal(t, int64(3), store.GetEarliestVersion())
			require.NoError(t, store.cosmosStore.SetEarliestVersion(4, false))
			require.Equal(t, int64(4), store.GetEarliestVersion())
			require.NoError(t, store.Close())
			reopened, err := NewCompositeStateStore(cfg, dir)
			if reopened != nil {
				t.Cleanup(func() { require.NoError(t, reopened.Close()) })
			}
			require.ErrorContains(t, err, "earliest version")
			require.ErrorContains(t, err, "state sync")
			require.Nil(t, reopened)
		})
	}
}

func TestRepairM_1094_SubDBProgress(t *testing.T) {
	dir := t.TempDir()
	cfg := repairM1094Config(dir, true)
	store, err := evm.NewEVMStateStore(cfg.EVMDBDirectory, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	require.NoError(t, store.SetLatestVersion(5))
	require.NoError(t, store.SetEarliestVersion(1, false))
	require.NoError(t, store.Close())
	// Persist a partial maintenance result in the last family DB, then reopen
	// the production EVM aggregator over the unequal durable boundaries.
	subCfg := cfg
	subCfg.UseDefaultComparer = true
	sub, err := mvcc.OpenDB(filepath.Join(cfg.EVMDBDirectory, evm.StoreTypeName(evm.StoreLegacy)), subCfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, sub.Close()) })
	require.NoError(t, sub.SetLatestVersion(4))
	require.NoError(t, sub.SetEarliestVersion(3, false))
	require.NoError(t, sub.Close())
	store, err = evm.NewEVMStateStore(cfg.EVMDBDirectory, cfg)
	require.NoError(t, err)
	require.Equal(t, int64(4), store.GetLatestVersion())
	require.Equal(t, int64(3), store.GetEarliestVersion())
}

func TestRepairM_1094_FlatKVCollisions(t *testing.T) {
	repairM1094FlatKVRoundTrip(t)
}

func TestRepairM_1094_ImportFailure(t *testing.T) {
	repairM1094FlatKVImportFailure(t)
}
