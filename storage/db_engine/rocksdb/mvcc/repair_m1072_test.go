//go:build rocksdbBackend
// +build rocksdbBackend

package mvcc

import (
	"fmt"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/proto"
)

func repairM1072Open(t *testing.T, dir string, buffer int) *Database {
	t.Helper()
	cfg := config.DefaultStateStoreConfig()
	cfg.Backend = config.RocksDBBackend
	cfg.AsyncWriteBuffer = buffer
	cfg.PruneIntervalSeconds = 0
	db, err := OpenDB(dir, cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, db.Close()) })
	return db
}

func repairM1072Changes(key, value string) []*proto.NamedChangeSet {
	return []*proto.NamedChangeSet{{
		Name: "repair",
		Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{{
			Key: []byte(key), Value: []byte(value),
		}}},
	}}
}

func repairM1072Value(t *testing.T, db *Database, version int64, key, value string) {
	t.Helper()
	got, err := db.Get("repair", version, []byte(key))
	require.NoError(t, err)
	require.Equal(t, []byte(value), got)
}

func TestRepairM_1072_ConfigPathPersistence(t *testing.T) {
	for _, buffer := range []int{-8, 0, 4} {
		t.Run(fmt.Sprintf("buffer_%d", buffer), func(t *testing.T) {
			dir := t.TempDir()
			db := repairM1072Open(t, dir, buffer)
			if buffer <= 0 {
				require.Nil(t, db.pendingChanges)
			} else {
				require.NotNil(t, db.pendingChanges)
				require.Equal(t, buffer, cap(db.pendingChanges))
			}
			require.NoError(t, db.ApplyChangesetAsync(7, repairM1072Changes("key", "value")))
			if buffer <= 0 {
				require.EqualValues(t, 7, db.GetLatestVersion())
				persisted, err := retrieveLatestVersion(db.storage)
				require.NoError(t, err)
				require.EqualValues(t, 7, persisted)
				repairM1072Value(t, db, 7, "key", "value")
			}
			require.NoError(t, db.Close())
			reopened := repairM1072Open(t, dir, buffer)
			require.EqualValues(t, 7, reopened.GetLatestVersion())
			repairM1072Value(t, reopened, 7, "key", "value")
			entry, err := reopened.streamHandler.ReadAt(1)
			require.NoError(t, err)
			require.EqualValues(t, 7, entry.Version)
			require.Equal(t, repairM1072Changes("key", "value"), entry.Changesets)
		})
	}
}

func TestRepairM_1072_HasRepeatedOwnedReads(t *testing.T) {
	db := repairM1072Open(t, t.TempDir(), 0)
	require.NoError(t, db.ApplyChangesetAsync(1, repairM1072Changes("present", "value")))
	require.NoError(t, db.ApplyChangesetAsync(2, []*proto.NamedChangeSet{{
		Name: "repair", Changeset: proto.ChangeSet{Pairs: []*proto.KVPair{
			{Key: []byte("present"), Delete: true},
			{Key: []byte("empty"), Value: []byte{}},
		}},
	}}))
	for i := 0; i < 4096; i++ {
		for _, lookup := range []struct {
			version int64
			key     string
			exists  bool
		}{
			{1, "present", true},
			{2, "present", false},
			{2, "missing", false},
			{2, "empty", true},
		} {
			got, err := db.Has("repair", lookup.version, []byte(lookup.key))
			require.NoError(t, err)
			require.Equal(t, lookup.exists, got)
		}
	}
	repairM1072Value(t, db, 1, "present", "value")
	require.NoError(t, db.SetEarliestVersion(2, false))
	exists, err := db.Has("repair", 1, []byte("present"))
	require.NoError(t, err)
	require.False(t, exists)
}

func TestRepairM_1072_WALAppendFailureRestart(t *testing.T) {
	for _, buffer := range []int{0, 4} {
		t.Run(fmt.Sprintf("buffer_%d", buffer), func(t *testing.T) {
			dir := t.TempDir()
			db := repairM1072Open(t, dir, buffer)
			require.NoError(t, db.ApplyChangesetAsync(1, repairM1072Changes("accepted", "value")))
			require.NoError(t, db.Close())
			db = repairM1072Open(t, dir, buffer)
			// A closed production WAL rejects append without replacing the WAL peer.
			require.NoError(t, db.streamHandler.Close())
			require.Error(t, db.ApplyChangesetAsync(2, repairM1072Changes("rejected", "value")))
			require.EqualValues(t, 1, db.GetLatestVersion())
			persisted, err := retrieveLatestVersion(db.storage)
			require.NoError(t, err)
			require.EqualValues(t, 1, persisted)
			rejected, err := db.Get("repair", 2, []byte("rejected"))
			require.NoError(t, err)
			require.Nil(t, rejected)
			require.NoError(t, db.Close())
			reopened := repairM1072Open(t, dir, buffer)
			require.EqualValues(t, 1, reopened.GetLatestVersion())
			repairM1072Value(t, reopened, 1, "accepted", "value")
			got, err := reopened.Get("repair", 2, []byte("rejected"))
			require.NoError(t, err)
			require.Nil(t, got)
			last, err := reopened.streamHandler.LastOffset()
			require.NoError(t, err)
			require.EqualValues(t, 1, last)
			entry, err := reopened.streamHandler.ReadAt(last)
			require.NoError(t, err)
			require.EqualValues(t, 1, entry.Version)
			require.Equal(t, repairM1072Changes("accepted", "value"), entry.Changesets)
		})
	}
}

func TestRepairM_1072_ConcurrentWALOrderRestart(t *testing.T) {
	dir := t.TempDir()
	db := repairM1072Open(t, dir, 4)
	const writers = 64
	start := make(chan struct{})
	results := make(chan error, writers)
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			changes := repairM1072Changes("shared", fmt.Sprintf("writer_%d", i))
			changes[0].Changeset.Pairs = append(changes[0].Changeset.Pairs, &proto.KVPair{
				Key: []byte(fmt.Sprintf("key_%d", i)), Value: []byte(fmt.Sprintf("value_%d", i)),
			})
			results <- db.ApplyChangesetAsync(1, changes)
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	for err := range results {
		require.NoError(t, err)
	}
	require.NoError(t, db.Close())
	reopened := repairM1072Open(t, dir, 4)
	replayed := repairM1072Open(t, filepath.Join(t.TempDir(), "replayed"), 0)
	last, err := reopened.streamHandler.LastOffset()
	require.NoError(t, err)
	require.EqualValues(t, writers, last)
	var final []byte
	require.NoError(t, reopened.streamHandler.Replay(1, last, func(_ uint64, entry proto.ChangelogEntry) error {
		final = append([]byte(nil), entry.Changesets[0].Changeset.Pairs[0].Value...)
		return replayed.ApplyChangesetSync(entry.Version, entry.Changesets)
	}))
	require.NotEmpty(t, final)
	for _, store := range []*Database{reopened, replayed} {
		require.EqualValues(t, 1, store.GetLatestVersion())
		repairM1072Value(t, store, 1, "shared", string(final))
		for i := 0; i < writers; i++ {
			repairM1072Value(t, store, 1, fmt.Sprintf("key_%d", i), fmt.Sprintf("value_%d", i))
		}
	}
}

func TestRepairM_1072_ImmediateClose(t *testing.T) {
	previous := runtime.GOMAXPROCS(1)
	defer runtime.GOMAXPROCS(previous)
	for _, writes := range []int{0, 32} {
		t.Run(fmt.Sprintf("writes_%d", writes), func(t *testing.T) {
			for iteration := 0; iteration < 16; iteration++ {
				dir := t.TempDir()
				db := repairM1072Open(t, dir, 32)
				for i := 1; i <= writes; i++ {
					require.NoError(t, db.ApplyChangesetAsync(int64(i), repairM1072Changes(fmt.Sprintf("key_%d", i), "value")))
				}
				require.NoError(t, db.Close())
				require.NoError(t, db.Close())
				require.ErrorContains(t, db.ApplyChangesetAsync(99, repairM1072Changes("late", "value")), "closed")
				require.ErrorContains(t, db.ApplyChangesetSync(99, repairM1072Changes("late", "value")), "closed")
				reopened := repairM1072Open(t, dir, 32)
				require.EqualValues(t, writes, reopened.GetLatestVersion())
				for i := 1; i <= writes; i++ {
					repairM1072Value(t, reopened, int64(writes), fmt.Sprintf("key_%d", i), "value")
				}
				require.NoError(t, reopened.Close())
			}
		})
	}
}

func TestRepairM_1072_CloseConcurrentAcceptedWrites(t *testing.T) {
	for _, buffer := range []int{0, 1, 16} {
		t.Run(fmt.Sprintf("buffer_%d", buffer), func(t *testing.T) {
			dir := t.TempDir()
			db := repairM1072Open(t, dir, buffer)
			require.NoError(t, db.ApplyChangesetAsync(1, repairM1072Changes("initial", "value")))
			const writers = 32
			type result struct {
				key string
				err error
			}
			results := make(chan result, writers)
			start := make(chan struct{})
			for i := 0; i < writers; i++ {
				go func(i int) {
					<-start
					key := fmt.Sprintf("key_%d", i)
					results <- result{key, db.ApplyChangesetAsync(1, repairM1072Changes(key, "value"))}
				}(i)
			}
			closed := make(chan error, 1)
			go func() { <-start; closed <- db.Close() }()
			close(start)
			accepted := []string{"initial"}
			rejected := []string{}
			for i := 0; i < writers; i++ {
				r := <-results
				if r.err == nil {
					accepted = append(accepted, r.key)
				} else {
					require.ErrorContains(t, r.err, "closed")
					rejected = append(rejected, r.key)
				}
			}
			require.NoError(t, <-closed)
			reopened := repairM1072Open(t, dir, buffer)
			require.EqualValues(t, 1, reopened.GetLatestVersion())
			for _, key := range accepted {
				repairM1072Value(t, reopened, 1, key, "value")
			}
			for _, key := range rejected {
				got, err := reopened.Get("repair", 1, []byte(key))
				require.NoError(t, err)
				require.Nil(t, got)
			}
			last, err := reopened.streamHandler.LastOffset()
			require.NoError(t, err)
			require.EqualValues(t, len(accepted), last)
		})
	}
}
