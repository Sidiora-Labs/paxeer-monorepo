package flatkv

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/ktype"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/vtype"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/types"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1198_ReadLifecycle(t *testing.T) {
	s := setupTestStore(t)
	defer func() { require.NoError(t, s.Close()) }()
	addr, slot := addrN(1), slotN(1)
	key := evmStorageKey(addr, slot)
	require.NoError(t, s.ApplyChangeSets([]*proto.NamedChangeSet{namedCS(storagePair(addr, slot, []byte{0x11}))}))
	require.Equal(t, int64(1), commitAndCheck(t, s))
	committed := s.CommittedRootHash()
	require.NoError(t, s.ApplyChangeSets([]*proto.NamedChangeSet{namedCS(storagePair(addr, slot, []byte{0x22}))}))
	value, found := s.Get(keys.EVMStoreKey, key)
	require.True(t, found)
	require.Equal(t, padLeft32(0x22), value)
	require.True(t, s.Has(keys.EVMStoreKey, key))
	height, found, err := s.GetBlockHeightModified(keys.EVMStoreKey, key)
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, int64(2), height)
	require.Equal(t, committed, s.CommittedRootHash())
	require.NotEqual(t, committed, s.RootHash())

	working, err := s.Iterator(keys.EVMStoreKey, nil, nil, true)
	require.NoError(t, err)
	defer func() { require.NoError(t, working.Close()) }()
	require.True(t, working.Valid())
	require.Equal(t, key, working.Key())
	require.Equal(t, padLeft32(0x22), working.Value())
	raw, err := s.RawGlobalIterator()
	require.NoError(t, err)
	defer func() { require.NoError(t, raw.Close()) }()
	require.True(t, raw.Valid())
	data, err := vtype.DeserializeStorageData(raw.Value())
	require.NoError(t, err)
	require.Equal(t, padLeft32(0x11), data.GetValue()[:])

	// Both exported nodes and historical handles read the committed snapshot.
	exp, err := s.Exporter(0)
	require.NoError(t, err)
	nodes := drainExporter(t, exp)
	require.NoError(t, exp.Close())
	require.Len(t, nodes, 1)
	require.Equal(t, int64(1), nodes[0].Version)
	require.Equal(t, bytes.Clone(raw.Value()), nodes[0].Value)
	view, err := s.LoadVersion(1, true)
	require.NoError(t, err)
	defer func() { require.NoError(t, view.Close()) }()
	value, found = view.Get(keys.EVMStoreKey, key)
	require.True(t, found)
	require.Equal(t, padLeft32(0x11), value)

	// Existing iterators remain frozen when the working store deletes the row.
	require.NoError(t, s.ApplyChangeSets([]*proto.NamedChangeSet{namedCS(storageDeletePair(addr, slot))}))
	require.False(t, s.Has(keys.EVMStoreKey, key))
	deleted, err := s.Iterator(keys.EVMStoreKey, nil, nil, true)
	require.NoError(t, err)
	require.False(t, deleted.Valid())
	require.NoError(t, deleted.Close())
	require.Equal(t, int64(2), commitAndCheck(t, s))
	require.Equal(t, padLeft32(0x22), working.Value())
	require.Equal(t, nodes[0].Value, raw.Value())
	require.NoError(t, working.Error())
	require.NoError(t, raw.Error())
}

func TestRepairM_1198_ImportDuplicateHash(t *testing.T) {
	for _, count := range []int{3, importBatchSize + 3} {
		name := "within_batch"
		if count > importBatchSize {
			name = "across_batches"
		}
		t.Run(name, func(t *testing.T) {
			s, imp := newKVImporterForTest(t, 7)
			defer func() { require.NoError(t, s.Close()) }()
			addr, slot := addrN(7), slotN(7)
			first := translatedStorageNode(t, 7, addr, slot, padLeft32(0x11))
			last := translatedStorageNode(t, 7, addr, slot, padLeft32(0x22))
			imp.AddNode(first)
			// Same-value repeats must not add extra hash contributions either.
			for i := 1; i < count-1; i++ {
				imp.AddNode(first)
			}
			imp.AddNode(last)
			require.NoError(t, imp.Close())
			_, pairs := imp.importStats()
			require.Equal(t, int64(count), pairs)
			assertImportedStorageReopen(t, s, 7, 1, map[string][]byte{
				string(evmStorageKey(addr, slot)): padLeft32(0x22),
			})
		})
	}
}

func TestRepairM_1198_ImportBatchRelease(t *testing.T) {
	// Retains the real Pebble batches across seven flushes and checks that
	// their representations were released, then reopens and scans all hashes.
	TestKVImporter_LargeImportTriggersMultipleFlushes(t)
}

func TestRepairM_1198_ImportValidation(t *testing.T) {
	valid := translatedStorageNode(t, 7, addrN(1), slotN(1), padLeft32(0x11))
	accountKey := accountPhysKey(addrN(1))
	addr := addrN(1)
	codeKey := ktype.EVMPhysicalKey(keys.EVMKeyCode, addr[:])
	legacyKey := ktype.ModulePhysicalKey("bank", []byte("supply"))
	with := func(key, value []byte) *types.SnapshotNode {
		return &types.SnapshotNode{Key: bytes.Clone(key), Value: bytes.Clone(value), Version: 7}
	}
	badVersion := with(valid.Key, valid.Value)
	badVersion.Version = 6
	branch := with(valid.Key, valid.Value)
	branch.Height = 1
	unknownVersion := bytes.Clone(valid.Value)
	unknownVersion[0] = 0xFF
	future := bytes.Clone(valid.Value)
	binary.BigEndian.PutUint64(future[1:9], 8)
	negative := bytes.Clone(valid.Value)
	binary.BigEndian.PutUint64(negative[1:9], ^uint64(0))
	aliasKey := ktype.ModulePhysicalKey(keys.EVMStoreKey, noncePair(addrN(1), 0).Key)
	aliasKey[len("evm/")] = 0x08
	cases := []struct {
		name string
		node *types.SnapshotNode
	}{
		{"nil_node", nil},
		{"wrong_version", badVersion},
		{"non_leaf", branch},
		{"nil_key", with(nil, valid.Value)},
		{"empty_key", with([]byte{}, valid.Value)},
		{"missing_module", with([]byte("unprefixed"), valid.Value)},
		{"empty_module", with([]byte("/key"), valid.Value)},
		{"empty_evm_key", with([]byte("evm/"), valid.Value)},
		{"reserved_metadata_key", with(ktype.MetaVersionKey, valid.Value)},
		{"short_storage_key", with(valid.Key[:len(valid.Key)-1], valid.Value)},
		{"long_storage_key", with(append(bytes.Clone(valid.Key), 0), valid.Value)},
		{"short_account_key", with(accountKey[:len(accountKey)-1], valid.Value)},
		{"long_code_key", with(append(bytes.Clone(codeKey), 0), valid.Value)},
		{"account_alias", with(aliasKey, vtype.NewAccountData().SetNonce(1).Serialize())},
		{"raw_storage_value", with(valid.Key, padLeft32(0x11))},
		{"nil_value", with(valid.Key, nil)},
		{"unsupported_value_version", with(valid.Key, unknownVersion)},
		{"truncated_account", with(accountKey, []byte{0, 1})},
		{"truncated_code", with(codeKey, []byte{0, 1})},
		{"truncated_legacy", with(legacyKey, []byte{0, 1})},
		{"future_value_height", with(valid.Key, future)},
		{"negative_value_height", with(valid.Key, negative)},
		{"deleted_storage", with(valid.Key, vtype.NewStorageData().SetBlockHeight(7).Serialize())},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, imp := newKVImporterForTest(t, 7)
			defer func() { require.NoError(t, s.Close()) }()
			imp.AddNode(valid)
			imp.AddNode(tc.node)
			first := imp.Err()
			require.Error(t, first, "validation must fail before enqueueing")
			imp.AddNode(nil)
			imp.AddNode(valid)
			require.Same(t, first, imp.Err(), "retain first error")
			require.ErrorIs(t, imp.Close(), first)
			require.ErrorIs(t, imp.Close(), first)
			require.Equal(t, int64(0), s.Version(), "invalid import must not finalize")
			_, err := os.Stat(filepath.Join(s.config.DataDir, snapshotName(7)))
			require.True(t, os.IsNotExist(err), "no snapshot for a rejected import")
			cfg := s.config
			require.NoError(t, s.Close())
			reopened := reopenTestStore(t, &cfg)
			defer func() { require.NoError(t, reopened.Close()) }()
			require.Equal(t, int64(0), reopened.Version())
			require.False(t, reopened.Has(keys.EVMStoreKey, evmStorageKey(addrN(1), slotN(1))))
			require.NoError(t, VerifyLtHash(reopened))
		})
	}
	for _, version := range []int64{0, -1} {
		t.Run("invalid_target_"+fmt.Sprint(version), func(t *testing.T) {
			s, imp := newKVImporterForTest(t, version)
			defer func() { require.NoError(t, s.Close()) }()
			require.ErrorContains(t, imp.Err(), "target version must be positive")
			require.Error(t, imp.Close())
			require.Zero(t, s.Version())
		})
	}
	t.Run("rejected_after_full_batch", func(t *testing.T) {
		s, imp := newKVImporterForTest(t, 7)
		defer func() { require.NoError(t, s.Close()) }()
		flushing, release := make(chan struct{}), make(chan struct{})
		hook := func(dir string) {
			if dir == storageDBDir {
				close(flushing)
				<-release
			}
		}
		flushHookForTest.Store(&hook)
		defer flushHookForTest.Store(nil)
		// Release a gated flush even if the assertion fails, then drain workers
		// before the store closes. This hook never replaces the real commit.
		defer func() {
			select {
			case <-release:
			default:
				close(release)
			}
			_ = imp.Close()
		}()
		for i := 0; i < importBatchSize; i++ {
			imp.AddNode(valid)
		}
		select {
		case <-flushing:
		case <-time.After(5 * time.Second):
			t.Fatal("real worker did not reach a full-batch flush")
		}
		imp.AddNode(nil)
		first := imp.Err()
		require.Error(t, first)
		close(release)
		require.ErrorIs(t, imp.Close(), first)
		_, pairs := imp.importStats()
		require.Equal(t, int64(importBatchSize), pairs, "the real in-flight batch committed before shutdown")
		cfg := s.config
		require.NoError(t, s.Close())
		reopened := reopenTestStore(t, &cfg)
		defer func() { require.NoError(t, reopened.Close()) }()
		require.Zero(t, reopened.Version())
		require.False(t, reopened.Has(keys.EVMStoreKey, evmStorageKey(addrN(1), slotN(1))))
		require.NoError(t, VerifyLtHash(reopened))
	})
	// Preserve the former skip tests as stricter whole-import rejection checks.
	t.Run("non_leaf_rejection", TestImporterHeightNonZeroSkipped)
	t.Run("nil_key_rejection", TestImporterNilKeySkipped)
}

func TestRepairM_1198_ImportProducerRoundTrip(t *testing.T) {
	source := setupTestStore(t)
	defer func() { require.NoError(t, source.Close()) }()
	addr := addrN(1)
	changes := []*proto.NamedChangeSet{
		namedCS(noncePair(addr, 42), codeHashPair(addr, codeHashN(3)), codePair(addr, []byte{0x60, 0x01}),
			storagePair(addr, slotN(1), []byte{0x11}), noncePair(addrN(2), 9),
			&proto.KVPair{Key: []byte{0x09, 0x01}, Value: []byte("legacy")}),
		{Name: "bank", Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
			{Key: []byte("supply"), Value: []byte("100")}, {Key: []byte("empty"), Value: []byte{}},
		}}},
	}
	require.NoError(t, source.ApplyChangeSets(changes))
	require.Equal(t, int64(1), commitAndCheck(t, source))
	require.Equal(t, int64(2), commitAndCheck(t, source))
	exp, err := source.Exporter(0)
	require.NoError(t, err)
	nodes := drainExporter(t, exp)
	require.NoError(t, exp.Close())
	require.Len(t, nodes, 7)
	root, rows := source.CommittedRootHash(), repair124Rows(t, source)
	target, imp := newKVImporterForTest(t, 2)
	defer func() { require.NoError(t, target.Close()) }()
	require.NoError(t, imp.AddModule(keys.FlatKVStoreKey))
	for _, node := range nodes {
		require.Equal(t, int64(2), node.Version)
		imp.AddNode(node)
	}
	require.NoError(t, imp.Close())
	require.NoError(t, imp.Close())
	require.Equal(t, root, target.CommittedRootHash())
	require.Equal(t, rows, repair124Rows(t, target))
	cfg := target.config
	require.NoError(t, target.Close())
	reopened := reopenTestStore(t, &cfg)
	defer func() { require.NoError(t, reopened.Close()) }()
	require.Equal(t, int64(2), reopened.Version())
	require.Equal(t, root, reopened.CommittedRootHash())
	require.Equal(t, rows, repair124Rows(t, reopened))
	for _, cs := range changes {
		for _, pair := range cs.Changeset.Pairs {
			value, found := reopened.Get(cs.Name, pair.Key)
			require.True(t, found, "module=%s key=%x", cs.Name, pair.Key)
			require.Equal(t, pair.Value, value)
		}
	}
	require.NoError(t, VerifyLtHash(reopened))
	verifyPerDBLtHash(t, reopened)
}

func TestRepairM_1198_ImportFixtureShape(t *testing.T) {
	t.Run("idempotent_success_and_reopen", TestKVImporter_CloseIdempotent_HappyPath)
	t.Run("idempotent_error", TestKVImporter_CloseIdempotent_AfterError)
	t.Run("error_lifecycle", TestKVImporter_ErrLifecycle)
	t.Run("backpressure_and_reopen", TestKVImporter_BackpressureBlocksProducerUntilWorkersDrain)
	t.Run("fail_fast", TestKVImporter_AddNodeAfterDoneDoesNotBlock)
	t.Run("abort", TestKVImporter_AbortSkipsFinalize)
	t.Run("abort_nil", TestKVImporter_AbortNilReasonStillAborts)
	t.Run("abort_after_close", TestKVImporter_AbortAfterCloseIsNoop)
}

func TestRepairM_1198_WALRecovery(t *testing.T) {
	for _, fsync := range []bool{false, true} {
		name := "pebble_no_fsync"
		if fsync {
			name = "pebble_fsync"
		}
		t.Run(name, func(t *testing.T) {
			cfg := config.DefaultTestConfig(t)
			cfg.Fsync = fsync
			cfg.SnapshotInterval = 0
			s := reopenTestStore(t, cfg)
			defer func() { require.NoError(t, s.Close()) }()
			key := evmStorageKey(addrN(1), slotN(1))
			require.NoError(t, s.ApplyChangeSets([]*proto.NamedChangeSet{makeChangeSet(key, padLeft32(0x11), false)}))
			require.Equal(t, int64(1), commitAndCheck(t, s))
			// Stop between the real synchronous WAL append and the DB write.
			require.NoError(t, s.ApplyChangeSets([]*proto.NamedChangeSet{makeChangeSet(key, padLeft32(0x22), false)}))
			expectedRoot := s.RootHash()
			require.NoError(t, s.changelog.Write(proto.ChangelogEntry{Version: 2, Changesets: s.pendingChangeSets}))
			require.Equal(t, int64(1), s.Version())
			s.clearPendingWrites()
			require.NoError(t, s.Close())
			reopened := reopenTestStore(t, cfg)
			defer func() { require.NoError(t, reopened.Close()) }()
			require.Equal(t, int64(2), reopened.Version())
			require.Equal(t, expectedRoot, reopened.CommittedRootHash())
			value, found := reopened.Get(keys.EVMStoreKey, key)
			require.True(t, found)
			require.Equal(t, padLeft32(0x22), value)
			require.NoError(t, VerifyLtHash(reopened))
		})
	}
}

func TestRepairM_1198_RejectedWrite(t *testing.T) {
	TestApplyChangeSetsErrorRecoveryPartialState(t)
}
