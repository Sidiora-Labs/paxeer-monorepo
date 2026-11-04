package dbcache_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	errorutils "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/threading"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/dbcache"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/pebbledb"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
)

// This timing decorator pauses the first publication; every cache operation
// still runs through the real standard cache backed by a real Pebble database.
type repair106PublicationPause struct {
	dbcache.Cache
	calls   atomic.Int32
	entered chan struct{}
	release chan struct{}
}

func (p *repair106PublicationPause) pause() {
	if p.calls.Add(1) == 1 {
		close(p.entered)
		<-p.release
	}
}

func (p *repair106PublicationPause) Set(key, value []byte) {
	p.pause()
	p.Cache.Set(key, value)
}

func (p *repair106PublicationPause) Delete(key []byte) {
	p.pause()
	p.Cache.Delete(key)
}

func (p *repair106PublicationPause) BatchSet(updates []dbcache.CacheUpdate) error {
	p.pause()
	return p.Cache.BatchSet(updates)
}

func TestRepair_106_SetPublicationOrder(t *testing.T) {
	repair106PublicationOrder(t, "set")
}

func TestRepair_106_DeletePublicationOrder(t *testing.T) {
	repair106PublicationOrder(t, "delete")
}

func TestRepair_106_BatchSetPublicationOrder(t *testing.T) {
	repair106PublicationOrder(t, "batch-set")
}

func TestRepair_106_BatchDeletePublicationOrder(t *testing.T) {
	repair106PublicationOrder(t, "batch-delete")
}

func repair106Write(t *testing.T, db types.KeyValueDB, operation string, value []byte) func() error {
	t.Helper()
	key := []byte("shared")
	opts := types.WriteOptions{Sync: true}
	switch operation {
	case "set":
		return func() error { return db.Set(key, value, opts) }
	case "delete":
		return func() error { return db.Delete(key, opts) }
	case "batch-set", "batch-delete":
		batch := db.NewBatch()
		t.Cleanup(func() { require.NoError(t, batch.Close()) })
		if operation == "batch-set" {
			require.NoError(t, batch.Set(key, value))
		} else {
			require.NoError(t, batch.Delete(key))
		}
		return func() error { return batch.Commit(opts) }
	default:
		t.Fatalf("unknown write operation %q", operation)
		return nil
	}
}

func repair106PublicationOrder(t *testing.T, firstOperation string) {
	t.Helper()
	for _, secondOperation := range []string{"set", "delete", "batch-set", "batch-delete"} {
		t.Run(secondOperation, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			backing, err := pebbledb.Open(ctx, &pebbledb.PebbleDBConfig{DataDir: t.TempDir()})
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, backing.Close()) })
			require.NoError(t, backing.Set([]byte("shared"), []byte("initial"), types.WriteOptions{Sync: true}))

			pool := threading.NewAdHocPool()
			t.Cleanup(pool.Close)
			standard, err := dbcache.NewStandardCache(ctx, &dbcache.CacheConfig{
				ShardCount: 4, MaxSize: 1 << 20, EstimatedOverheadPerEntry: 16,
			}, pool, pool)
			require.NoError(t, err)
			pause := &repair106PublicationPause{
				Cache: standard, entered: make(chan struct{}), release: make(chan struct{}),
			}
			var releaseOnce sync.Once
			release := func() { releaseOnce.Do(func() { close(pause.release) }) }
			t.Cleanup(release)
			cached := dbcache.NewCachedKeyValueDB(backing, pause)
			initial, err := cached.Get([]byte("shared"))
			require.NoError(t, err)
			require.Equal(t, []byte("initial"), initial)

			firstWrite := repair106Write(t, cached, firstOperation, []byte("first"))
			secondWrite := repair106Write(t, cached, secondOperation, []byte("second"))
			firstDone := make(chan error, 1)
			go func() { firstDone <- firstWrite() }()
			select {
			case <-pause.entered:
			case err := <-firstDone:
				t.Fatalf("first writer returned before cache publication: %v", err)
			case <-time.After(10 * time.Second):
				t.Fatal("first writer did not reach cache publication")
			}

			secondStarted := make(chan struct{})
			secondDone := make(chan error, 1)
			go func() {
				close(secondStarted)
				secondDone <- secondWrite()
			}()
			<-secondStarted
			var secondErr error
			var secondFinished bool
			select {
			case secondErr = <-secondDone:
				secondFinished = true
			case <-time.After(100 * time.Millisecond):
			}
			// The first DB write is already visible, but the second must wait for
			// its cache publication before changing the backing database.
			pausedValue, pausedErr := backing.Get([]byte("shared"))
			release()
			select {
			case err = <-firstDone:
				require.NoError(t, err)
			case <-time.After(10 * time.Second):
				t.Fatal("first writer did not finish after publication was released")
			}
			if !secondFinished {
				select {
				case secondErr = <-secondDone:
				case <-time.After(10 * time.Second):
					t.Fatal("second writer did not finish after publication was released")
				}
			}
			require.NoError(t, secondErr)
			if firstOperation == "delete" || firstOperation == "batch-delete" {
				require.True(t, errorutils.IsNotFound(pausedErr), "second writer changed DB before first cache publication")
			} else {
				require.NoError(t, pausedErr)
				require.Equal(t, []byte("first"), pausedValue, "second writer changed DB before first cache publication")
			}
			expected := []byte("second")
			if secondOperation == "delete" || secondOperation == "batch-delete" {
				expected = nil
			}
			repair106AssertState(t, backing, expected)
			repair106AssertState(t, cached, expected)
			require.False(t, secondFinished, "second writer completed while the first cache publication was paused")
		})
	}
}

func repair106AssertState(t *testing.T, db types.KeyValueDB, expected []byte) {
	t.Helper()
	value, err := db.Get([]byte("shared"))
	if expected == nil {
		require.True(t, errorutils.IsNotFound(err))
	} else {
		require.NoError(t, err)
	}
	require.Equal(t, expected, value)
	keys := map[string]types.BatchGetResult{"shared": {}}
	require.NoError(t, db.BatchGet(keys))
	require.NoError(t, keys["shared"].Error)
	require.Equal(t, expected, keys["shared"].Value)
	require.Equal(t, expected != nil, keys["shared"].IsFound())

	iter, err := db.NewIter(nil)
	require.NoError(t, err)
	defer func() { require.NoError(t, iter.Close()) }()
	if expected != nil {
		require.True(t, iter.Valid())
		require.Equal(t, []byte("shared"), iter.Key())
		require.Equal(t, expected, iter.Value())
		iter.Next()
	}
	require.False(t, iter.Valid())
	require.NoError(t, iter.Error())
}
