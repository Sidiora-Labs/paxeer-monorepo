package mvcc

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/cockroachdb/pebble/v2"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
)

func TestRepairM_1175_AscendingPruneDirtyMapRestart(t *testing.T) {
	for _, keepLast := range []bool{true, false} {
		t.Run(fmt.Sprintf("keep-last-%t", keepLast), func(t *testing.T) {
			dir := t.TempDir()
			cfg := config.DefaultStateStoreConfig()
			cfg.KeepLastVersion = keepLast
			modules := []string{"bank", "staking"}
			keys := [][]byte{[]byte("a"), []byte("b"), []byte("z")}
			value := func(module string, key []byte, version int64) []byte {
				return []byte(fmt.Sprintf("%s/%s/%d", module, key, version))
			}

			// The real legacy batch producer creates an unmarked database.
			raw, err := pebble.Open(dir, &pebble.Options{Comparer: MVCCComparer})
			require.NoError(t, err)
			for _, version := range []int64{10, 20, 30, 40} {
				batch, err := NewBatch(raw, version, false)
				require.NoError(t, err)
				for _, module := range modules {
					for _, key := range keys {
						require.NoError(t, batch.Set(module, key, value(module, key, version)))
					}
				}
				require.NoError(t, batch.Write())
			}
			require.NoError(t, raw.Close())

			open := func() *Database {
				store, err := OpenDB(dir, cfg)
				require.NoError(t, err)
				db := store.(*Database)
				db.metricsCancel()
				db.metricsWG.Wait()
				t.Cleanup(func() { require.NoError(t, db.Close()) })
				require.False(t, db.descending)
				_, closer, err := db.storage.Get([]byte(descendingMVCCMarkerKey))
				if closer != nil {
					require.NoError(t, closer.Close())
				}
				require.ErrorIs(t, err, pebble.ErrNotFound)
				db.storeKeyDirty.Range(func(key, value any) bool {
					t.Errorf("reopened dirty map contains %v: %v", key, value)
					return true
				})
				return db
			}
			assertPhysical := func(db *Database, want []int64) {
				t.Helper()
				itr, err := db.storage.NewIter(nil)
				require.NoError(t, err)
				defer func() { require.NoError(t, itr.Close()) }()
				seen := make(map[string][]int64)
				for itr.First(); itr.Valid(); itr.Next() {
					if isMetadataKey(itr.Key()) {
						continue
					}
					key, versionBytes, ok := SplitMVCCKey(itr.Key())
					require.True(t, ok)
					version, err := decodeUint64Ascending(versionBytes)
					require.NoError(t, err)
					seen[string(key)] = append(seen[string(key)], version)
				}
				require.NoError(t, itr.Error())
				require.Len(t, seen, len(modules)*len(keys))
				for _, module := range modules {
					for _, key := range keys {
						require.Equal(t, want, seen[string(prependStoreKey(module, key))], "%s/%s", module, key)
						for _, version := range want {
							stored, closer, err := db.storage.Get(MVCCEncodeAscending(prependStoreKey(module, key), version))
							require.NoError(t, err)
							require.Equal(t, MVCCEncodeAscending(value(module, key, version), 0), bytes.Clone(stored))
							require.NoError(t, closer.Close())
						}
						got, err := db.Get(module, 40, key)
						require.NoError(t, err)
						require.Equal(t, value(module, key, 40), got)
					}
				}
			}

			db := open()
			assertPhysical(db, []int64{10, 20, 30, 40})
			require.NoError(t, db.Close())
			db = open()
			for _, boundary := range []int64{25, 35} {
				// No writes repopulate the map before either prune.
				require.NoError(t, db.Prune(boundary))
				want := []int64{40}
				if boundary == 25 {
					want = []int64{30, 40}
				}
				if keepLast {
					want = append([]int64{boundary - 5}, want...)
				}
				assertPhysical(db, want)
				require.Equal(t, boundary+1, db.GetEarliestVersion())
				require.Equal(t, int64(40), db.GetLatestVersion())
				require.NoError(t, db.Close())
				db = open()
				assertPhysical(db, want)
				require.Equal(t, boundary+1, db.GetEarliestVersion())
				require.Equal(t, int64(40), db.GetLatestVersion())
			}
		})
	}
}

func TestRepairM_1175_MetricsProofAcceptsWrongUnits(t *testing.T) {
	proofNodeMetrics(t)
}
