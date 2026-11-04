package flatkv

import (
	"bytes"
	"maps"
	"slices"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/keys"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/lthash"
	"github.com/stretchr/testify/require"
)

func repair124Rows(t *testing.T, s *CommitStore) map[string][]byte {
	t.Helper()
	it, err := s.RawGlobalIterator()
	require.NoError(t, err)
	defer func() { require.NoError(t, it.Close()) }()
	rows := make(map[string][]byte)
	for ; it.Valid(); it.Next() {
		rows[string(it.Key())] = bytes.Clone(it.Value())
	}
	require.NoError(t, it.Error())
	return rows
}

func TestRepair_124_RejectedWrite(t *testing.T) {
	for _, withPending := range []bool{false, true} {
		name := "committed_only"
		if withPending {
			name = "existing_pending"
		}
		t.Run(name, func(t *testing.T) {
			cfg := config.DefaultTestConfig(t)
			cfg.SnapshotInterval = 0
			s := reopenTestStore(t, cfg)
			defer func() { require.NoError(t, s.Close()) }()
			commitMixedState(t, s, 1)
			addr, fresh, slot := addrN(1), addrN(99), slotN(1)
			if withPending {
				require.NoError(t, s.ApplyChangeSets([]*proto.NamedChangeSet{
					namedCS(noncePair(addr, 7), storagePair(addr, slot, []byte{7}), codePair(addr, []byte{7})),
					{Name: "bank", Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{{Key: []byte("pending"), Value: []byte("accepted")}}}},
				}))
			}
			accounts := maps.Clone(s.accountWrites)
			storage := maps.Clone(s.storageWrites)
			code := maps.Clone(s.codeWrites)
			legacy := maps.Clone(s.legacyWrites)
			pending := slices.Clone(s.pendingChangeSets)
			working := s.workingLtHash.Clone()
			committed := s.committedLtHash.Clone()
			hashes := make(map[string]*lthash.LtHash)
			for dir, hash := range s.perDBWorkingLtHash {
				hashes[dir] = hash.Clone()
			}
			offset, err := s.changelog.LastOffset()
			require.NoError(t, err)
			rejected := []*proto.NamedChangeSet{
				namedCS(noncePair(addr, 999), noncePair(fresh, 999), codePair(addr, []byte("rejected")),
					&proto.KVPair{Key: evmStorageKey(addr, slot), Value: []byte{1}}),
				{Name: "bank", Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{{Key: []byte("rejected"), Value: []byte("rejected")}}}},
			}
			require.ErrorContains(t, s.ApplyChangeSets(rejected), "failed to parse storage")
			require.Equal(t, accounts, s.accountWrites)
			require.Equal(t, storage, s.storageWrites)
			require.Equal(t, code, s.codeWrites)
			require.Equal(t, legacy, s.legacyWrites)
			require.Equal(t, pending, s.pendingChangeSets)
			require.True(t, working.Equal(s.workingLtHash))
			require.True(t, committed.Equal(s.committedLtHash))
			for dir, hash := range hashes {
				require.True(t, hash.Equal(s.perDBWorkingLtHash[dir]), dir)
			}
			afterOffset, err := s.changelog.LastOffset()
			require.NoError(t, err)
			require.Equal(t, offset, afterOffset)
			require.False(t, s.Has(keys.EVMStoreKey, noncePair(fresh, 0).Key))
			require.False(t, s.Has("bank", []byte("rejected")))

			accepted := namedCS(storagePair(addr, slotN(2), []byte{42}))
			require.NoError(t, s.ApplyChangeSets([]*proto.NamedChangeSet{accepted}))
			require.Equal(t, int64(2), commitAndCheck(t, s))
			wantNonce := uint64(1)
			if withPending {
				wantNonce = 7
			}
			value, found := s.Get(keys.EVMStoreKey, noncePair(addr, 0).Key)
			require.True(t, found)
			require.Equal(t, nonceBytes(wantNonce), value)
			require.NoError(t, VerifyLtHash(s))
			verifyPerDBLtHash(t, s)
			rows := repair124Rows(t, s)
			root := s.CommittedRootHash()
			perDB := fullScanPerDBLtHash(t, s)

			first, err := s.changelog.FirstOffset()
			require.NoError(t, err)
			last, err := s.changelog.LastOffset()
			require.NoError(t, err)
			var versions []int64
			require.NoError(t, s.changelog.Replay(first, last, func(_ uint64, entry proto.ChangelogEntry) error {
				versions = append(versions, entry.Version)
				if entry.Version == 2 {
					require.Equal(t, append(pending, accepted), entry.Changesets)
				}
				return nil
			}))
			require.Equal(t, []int64{1, 2}, versions)
			require.NoError(t, s.Close())

			parent, err := NewCommitStore(t.Context(), cfg)
			require.NoError(t, err)
			defer func() { require.NoError(t, parent.Close()) }()
			recoveredStore, err := parent.LoadVersion(0, true)
			require.NoError(t, err)
			recovered := recoveredStore.(*CommitStore)
			defer func() { require.NoError(t, recovered.Close()) }()
			require.Equal(t, int64(2), recovered.Version())
			require.Equal(t, rows, repair124Rows(t, recovered))
			require.Equal(t, root, recovered.CommittedRootHash())
			require.NoError(t, VerifyLtHash(recovered))
			verifyPerDBLtHash(t, recovered)
			for dir, hash := range perDB {
				require.True(t, hash.Equal(recovered.perDBWorkingLtHash[dir]), dir)
			}
		})
	}
}

func TestRepair_124_LatestExport(t *testing.T) {
	testLatestExportRoundTrip(t)
}
