package dbcache_test

import (
	"context"
	"testing"
	"time"

	"github.com/fortytw2/leaktest"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	errorutils "github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/common/threading"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/dbcache"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/pebbledb"
	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
)

func TestCacheConstructorMetricsValidation(t *testing.T) {
	defer leaktest.Check(t)()
	for _, interval := range []time.Duration{0, -time.Nanosecond, -time.Second} {
		t.Run(interval.String(), func(t *testing.T) {
			cfg := dbcache.DefaultCacheConfig()
			cfg.MetricsName = "cache-constructor-validation"
			cfg.MetricsScrapeInterval = interval
			expectedErr := cfg.Validate()
			require.Error(t, expectedErr)
			cache, err := dbcache.NewStandardCache(context.Background(), &cfg, nil, nil)
			require.ErrorContains(t, err, expectedErr.Error())
			require.Nil(t, cache, "invalid metrics configuration must not construct a cache or start a scraper")
		})
	}
	for _, interval := range []time.Duration{0, -time.Second} {
		t.Run("disabled-"+interval.String(), func(t *testing.T) {
			cfg := dbcache.DefaultCacheConfig()
			cfg.MetricsScrapeInterval = interval
			cache, err := dbcache.NewStandardCache(context.Background(), &cfg, nil, nil)
			require.NoError(t, err)
			require.NotNil(t, cache)
		})
	}
}

// This adapter translates the batch lifecycle to the real in-memory backend;
// Set, Delete, errors and storage are provided by its production batch.
type memoryBatchAdapter struct {
	dbm.Batch
	db    *dbm.MemDB
	count int
}

func (b *memoryBatchAdapter) Set(key, value []byte) error {
	if err := b.Batch.Set(key, value); err != nil {
		return err
	}
	b.count++
	return nil
}

func (b *memoryBatchAdapter) Delete(key []byte) error {
	if err := b.Batch.Delete(key); err != nil {
		return err
	}
	b.count++
	return nil
}

func (b *memoryBatchAdapter) Commit(opts types.WriteOptions) error {
	if opts.Sync {
		return b.Batch.WriteSync()
	}
	return b.Batch.Write()
}

func (b *memoryBatchAdapter) Len() int { return b.count }

func (b *memoryBatchAdapter) Reset() {
	if err := b.Batch.Close(); err != nil {
		panic(err)
	}
	b.Batch = b.db.NewBatch()
	b.count = 0
}

func newContractTestCache(t *testing.T, ctx context.Context) dbcache.Cache {
	t.Helper()
	pool := threading.NewAdHocPool()
	t.Cleanup(pool.Close)
	cache, err := dbcache.NewStandardCache(ctx, &dbcache.CacheConfig{
		ShardCount: 4, MaxSize: 1 << 20, EstimatedOverheadPerEntry: 16,
	}, pool, pool)
	require.NoError(t, err)
	return cache
}

func TestCachedBatchRejectsInvalidMutations(t *testing.T) {
	for _, operation := range []string{"set-nil-value", "set-empty-key", "delete-empty-key"} {
		t.Run(operation, func(t *testing.T) {
			backing := dbm.NewMemDB()
			t.Cleanup(func() { require.NoError(t, backing.Close()) })
			ctx, cancel := context.WithCancel(context.Background())
			t.Cleanup(cancel)
			cache := newContractTestCache(t, ctx)
			for key, value := range map[string]string{"keep": "original", "delete": "old"} {
				require.NoError(t, backing.Set([]byte(key), []byte(value)))
				cache.Set([]byte(key), []byte(value))
			}
			// Cache accepts empty keys, whereas the backing batch rejects them.
			// A rejected delete must not reach even an existing empty-key entry.
			cache.Set(nil, []byte("empty-key-sentinel"))
			inner := &memoryBatchAdapter{Batch: backing.NewBatch(), db: backing}
			batch := dbcache.NewCachedBatchForTest(inner, cache)
			t.Cleanup(func() { require.NoError(t, batch.Close()) })
			probe := backing.NewBatch()
			t.Cleanup(func() { require.NoError(t, probe.Close()) })
			var expectedErr, err error
			switch operation {
			case "set-nil-value":
				expectedErr = probe.Set([]byte("keep"), nil)
				err = batch.Set([]byte("keep"), nil)
			case "set-empty-key":
				expectedErr = probe.Set(nil, []byte("rejected"))
				err = batch.Set(nil, []byte("rejected"))
			case "delete-empty-key":
				expectedErr = probe.Delete(nil)
				err = batch.Delete(nil)
			}
			require.Error(t, expectedErr)
			require.Same(t, expectedErr, err, "the original backend error must be returned unchanged")
			require.Zero(t, batch.Len())
			require.NoError(t, batch.Set([]byte("valid"), []byte("committed")))
			require.NoError(t, batch.Delete([]byte("delete")))
			require.Equal(t, 2, batch.Len())
			require.NoError(t, batch.Commit(types.WriteOptions{Sync: true}))
			read := func(key []byte) ([]byte, bool, error) {
				value, err := backing.Get(key)
				return value, value != nil, err
			}
			for key, expected := range map[string][]byte{
				"keep": []byte("original"), "valid": []byte("committed"), "delete": nil,
			} {
				stored, err := backing.Get([]byte(key))
				require.NoError(t, err)
				require.Equal(t, expected, stored)
				cached, found, err := cache.Get(read, []byte(key), true)
				require.NoError(t, err)
				require.Equal(t, stored != nil, found)
				require.Equal(t, stored, cached)
			}
			empty, found, err := cache.Get(read, nil, true)
			require.NoError(t, err)
			require.True(t, found)
			require.Equal(t, []byte("empty-key-sentinel"), empty)
		})
	}
}

func TestCachedKeyValueDBValueOwnership(t *testing.T) {
	for _, mode := range []string{"standard", "passthrough"} {
		for _, source := range []string{"backing", "set", "batch"} {
			for _, firstRead := range []string{"get", "batch-get"} {
				t.Run(mode+"/"+source+"/"+firstRead, func(t *testing.T) {
					ctx, cancel := context.WithCancel(context.Background())
					t.Cleanup(cancel)
					backing, err := pebbledb.Open(ctx, &pebbledb.PebbleDBConfig{DataDir: t.TempDir()})
					require.NoError(t, err)
					t.Cleanup(func() { require.NoError(t, backing.Close()) })
					cache := dbcache.NewNoOpCache()
					if mode == "standard" {
						cache = newContractTestCache(t, ctx)
					}
					cached := dbcache.NewCachedKeyValueDB(backing, cache)
					key := []byte("key")
					expected := []byte("original-value")
					switch source {
					case "backing":
						require.NoError(t, backing.Set(key, expected, types.WriteOptions{}))
					case "set":
						require.NoError(t, cached.Set(key, expected, types.WriteOptions{}))
					case "batch":
						batch := cached.NewBatch()
						t.Cleanup(func() { require.NoError(t, batch.Close()) })
						require.NoError(t, batch.Set(key, expected))
						require.NoError(t, batch.Commit(types.WriteOptions{}))
					}
					var first []byte
					if firstRead == "get" {
						first, err = cached.Get(key)
						require.NoError(t, err)
					} else {
						results := map[string]types.BatchGetResult{"key": {}, "missing": {}}
						require.NoError(t, cached.BatchGet(results))
						require.NoError(t, results["key"].Error)
						require.NoError(t, results["missing"].Error)
						require.False(t, results["missing"].IsFound())
						first = results["key"].Value
					}
					require.Equal(t, []byte("original-value"), first)
					second, err := cached.Get(key)
					require.NoError(t, err)
					first[0] = 'X'
					require.Equal(t, []byte("original-value"), second, "consumers must own independent writable buffers")
					second[1] = 'Y'
					uncached, err := backing.Get(key)
					require.NoError(t, err)
					require.Equal(t, []byte("original-value"), uncached)
					uncached[0] = 'Z'
					for i := 0; i < 2; i++ {
						value, err := cached.Get(key)
						require.NoError(t, err)
						require.Equal(t, []byte("original-value"), value)
						results := map[string]types.BatchGetResult{"key": {}, "missing": {}}
						require.NoError(t, cached.BatchGet(results))
						require.NoError(t, results["key"].Error)
						require.Equal(t, []byte("original-value"), results["key"].Value)
						require.NoError(t, results["missing"].Error)
						require.Nil(t, results["missing"].Value)
						results["key"].Value[0] = 'B'
						stored, err := backing.Get(key)
						require.NoError(t, err)
						require.Equal(t, []byte("original-value"), stored)
					}
					missing, err := cached.Get([]byte("missing"))
					require.ErrorIs(t, err, errorutils.ErrNotFound)
					require.Nil(t, missing)
				})
			}
		}
	}
}
