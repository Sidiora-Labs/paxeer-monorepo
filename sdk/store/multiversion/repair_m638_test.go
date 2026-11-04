package multiversion

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/dbadapter"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/occ"
	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"
)

func repairM638Iterator(store types.KVStore, ascending bool) types.Iterator {
	if ascending {
		return store.Iterator(nil, nil)
	}
	return store.ReverseIterator(nil, nil)
}

func TestRepairM_638_IteratorErrors(t *testing.T) {
	for _, ascending := range []bool{true, false} {
		t.Run(fmt.Sprintf("ascending=%t", ascending), func(t *testing.T) {
			parent := dbadapter.Store{DB: dbm.NewMemDB()}
			store := NewMultiVersionStore(parent)
			consumer := store.VersionedIndexedStore(1, 0, make(chan occ.Abort, 1))
			empty := repairM638Iterator(consumer, ascending)
			require.NoError(t, empty.Error())
			require.False(t, empty.Valid())
			require.NoError(t, empty.Error())
			require.Panics(t, func() { empty.Key() })
			require.Panics(t, func() { empty.Value() })
			require.Panics(t, func() { empty.Next() })
			require.NoError(t, empty.Close())

			parent.Set([]byte("a"), []byte("parent"))
			parent.Set([]byte("b"), []byte("hidden"))
			producer := store.VersionedIndexedStore(0, 0, make(chan occ.Abort, 1))
			producer.Delete([]byte("b"))
			producer.Set([]byte("c"), []byte("version"))
			producer.WriteToMultiVersionStore()
			consumer = store.VersionedIndexedStore(1, 1, make(chan occ.Abort, 1))
			iter := repairM638Iterator(consumer, ascending)
			values := map[string]string{}
			var keys []string
			for ; iter.Valid(); iter.Next() {
				require.NoError(t, iter.Error())
				keys = append(keys, string(iter.Key()))
				values[string(iter.Key())] = string(iter.Value())
			}
			require.Equal(t, map[string]string{"a": "parent", "c": "version"}, values)
			wantKeys := []string{"a", "c"}
			if !ascending {
				wantKeys = []string{"c", "a"}
			}
			require.Equal(t, wantKeys, keys)
			require.NoError(t, iter.Error(), "exhaustion is not a backing failure")
			require.Panics(t, func() { iter.Next() })
			require.NoError(t, iter.Close())

			backend, err := dbm.NewGoLevelDB("closed", t.TempDir())
			require.NoError(t, err)
			require.NoError(t, backend.Close())
			failedParent := dbadapter.Store{DB: backend}
			failedStore := NewMultiVersionStore(failedParent)
			failedConsumer := failedStore.VersionedIndexedStore(0, 0, make(chan occ.Abort, 1))
			failedConsumer.Set([]byte("local"), []byte("still-valid"))
			failed := repairM638Iterator(failedConsumer, ascending)
			merged := failed.(*trackedIterator).Iterator.(*mvsMergeIterator)
			backingErr := merged.parent.Error()
			require.Error(t, backingErr, "the real closed backend must produce the error")
			require.ErrorIs(t, failed.Error(), backingErr)
			require.True(t, failed.Valid(), "the versioned write is still available")
			require.ErrorIs(t, failed.Error(), backingErr)
			require.Equal(t, []byte("still-valid"), failed.Value())
			failed.Next()
			require.False(t, failed.Valid())
			require.ErrorIs(t, failed.Error(), backingErr)
			require.NoError(t, failed.Close())
			failedConsumer.WriteToMultiVersionStore()
			valid, _ := failedStore.ValidateTransactionState(0)
			require.False(t, valid, "a backend failure must invalidate the recorded scan")

			cacheFailure := repairM638Iterator(failedParent, ascending)
			backingErr = cacheFailure.Error()
			require.Error(t, backingErr)
			parentIter := repairM638Iterator(parent, ascending)
			merged = NewMVSMergeIterator(parentIter, cacheFailure, ascending, NoOpHandler{})
			require.ErrorIs(t, merged.Error(), backingErr, "cache errors must also survive the merge")
			require.NoError(t, merged.Close())
		})
	}
}

func TestRepairM_638_NilSet(t *testing.T) {
	parent := dbadapter.Store{DB: dbm.NewMemDB()}
	key := []byte("key")
	parent.Set(key, []byte("parent"))
	store := NewMultiVersionStore(parent)
	reader := store.VersionedIndexedStore(0, 0, make(chan occ.Abort, 1))
	require.Equal(t, []byte("parent"), reader.Get(key))
	require.Panics(t, func() { reader.Set(key, nil) })
	require.Empty(t, reader.GetWriteset())
	require.Equal(t, ReadSet{"key": {[]byte("parent")}}, ReadSet(reader.GetReadset()))
	require.Equal(t, []byte("parent"), reader.Get(key))
	reader.WriteToMultiVersionStore()
	store.WriteLatestToStore()
	require.Equal(t, []byte("parent"), parent.Get(key))

	writer := store.VersionedIndexedStore(1, 0, make(chan occ.Abort, 1))
	writer.Set(key, []byte("existing"))
	require.Panics(t, func() { writer.Set(key, nil) })
	require.Equal(t, WriteSet{"key": []byte("existing")}, WriteSet(writer.GetWriteset()))
	require.Empty(t, writer.GetReadset())
	require.Equal(t, []byte("existing"), writer.Get(key))
	writer.Set([]byte("empty"), []byte{})
	writer.WriteToMultiVersionStore()
	consumer := store.VersionedIndexedStore(2, 0, make(chan occ.Abort, 1))
	require.Equal(t, []byte("existing"), consumer.Get(key))
	require.NotNil(t, consumer.Get([]byte("empty")))
	store.WriteLatestToStore()
	require.Equal(t, []byte("existing"), parent.Get(key))

	consumer.Delete(key)
	consumer.WriteToMultiVersionStore()
	require.True(t, store.GetLatest(key).IsDeleted())
	require.Nil(t, store.VersionedIndexedStore(3, 0, make(chan occ.Abort, 1)).Get(key))
	store.WriteLatestToStore()
	require.False(t, parent.Has(key))
	require.NotNil(t, parent.Get([]byte("empty")))
}

type repairM638ObservedStore struct {
	types.KVStore
	opened       atomic.Int32
	closed       atomic.Int32
	closeStarted chan struct{}
	closeRelease chan struct{}
}

type repairM638ObservedIterator struct {
	types.Iterator
	store *repairM638ObservedStore
}

func (store *repairM638ObservedStore) Iterator(start, end []byte) types.Iterator {
	store.opened.Add(1)
	return &repairM638ObservedIterator{Iterator: store.KVStore.Iterator(start, end), store: store}
}

func (store *repairM638ObservedStore) ReverseIterator(start, end []byte) types.Iterator {
	store.opened.Add(1)
	return &repairM638ObservedIterator{Iterator: store.KVStore.ReverseIterator(start, end), store: store}
}

func (iter *repairM638ObservedIterator) Close() error {
	close(iter.store.closeStarted)
	<-iter.store.closeRelease
	err := iter.Iterator.Close()
	iter.store.closed.Add(1)
	return err
}

func TestRepairM_638_ParticipantAbort(t *testing.T) {
	for _, ascending := range []bool{true, false} {
		t.Run(fmt.Sprintf("ascending=%t", ascending), func(t *testing.T) {
			backing := dbadapter.Store{DB: dbm.NewMemDB()}
			parent := &repairM638ObservedStore{KVStore: backing}
			store := NewMultiVersionStore(parent)
			producer := store.VersionedIndexedStore(0, 0, make(chan occ.Abort, 1))
			for _, key := range []string{"a", "b", "c", "d", "e"} {
				backing.Set([]byte(key), []byte("parent"))
				producer.Set([]byte(key), []byte("predecessor"))
			}
			producer.WriteToMultiVersionStore()
			consumer := store.VersionedIndexedStore(1, 0, make(chan occ.Abort, 1))
			parent.closeStarted = make(chan struct{})
			parent.closeRelease = make(chan struct{})
			close(parent.closeRelease)
			iter := repairM638Iterator(consumer, ascending)
			for ; iter.Valid(); iter.Next() {
				require.Equal(t, []byte("predecessor"), iter.Value())
			}
			require.NoError(t, iter.Close())
			consumer.WriteToMultiVersionStore()
			store.InvalidateWriteset(0, 0)
			for _, key := range []string{"a", "b", "c", "d", "e"} {
				require.True(t, store.GetLatestBeforeIndex(1, []byte(key)).IsEstimate())
			}

			for attempt := 0; attempt < 24; attempt++ {
				parent.closeStarted = make(chan struct{})
				parent.closeRelease = make(chan struct{})
				result := make(chan bool, 1)
				go func() {
					valid, _ := store.ValidateTransactionState(1)
					result <- valid
				}()
				select {
				case <-parent.closeStarted:
				case <-time.After(3 * time.Second):
					close(parent.closeRelease)
					t.Fatal("aborted participant did not reach iterator cleanup")
				}
				select {
				case <-result:
					close(parent.closeRelease)
					t.Fatal("validation returned before joining iterator cleanup")
				default:
				}
				close(parent.closeRelease)
				select {
				case valid := <-result:
					require.False(t, valid)
				case <-time.After(3 * time.Second):
					t.Fatal("validation did not join its aborted participant")
				}
				require.Equal(t, int32(attempt+2), parent.opened.Load())
				require.Equal(t, parent.opened.Load(), parent.closed.Load())
			}
			store.SetWriteset(0, 1, WriteSet{
				"a": []byte("predecessor"), "b": []byte("predecessor"),
				"c": []byte("predecessor"), "d": []byte("predecessor"), "e": []byte("predecessor"),
			})
			parent.closeStarted = make(chan struct{})
			parent.closeRelease = make(chan struct{})
			close(parent.closeRelease)
			valid, conflicts := store.ValidateTransactionState(1)
			require.True(t, valid, "validation must succeed once the predecessor resolves its estimates")
			require.Empty(t, conflicts)
			require.Equal(t, parent.opened.Load(), parent.closed.Load())
		})
	}
}

func TestRepairM_638_TerminalEstimate(t *testing.T) {
	parent := dbadapter.Store{DB: dbm.NewMemDB()}
	store := NewMultiVersionStore(parent)
	producer := store.VersionedIndexedStore(0, 0, make(chan occ.Abort, 1))
	producer.Set([]byte("a"), []byte("value"))
	producer.Set([]byte("b"), []byte("value"))
	producer.WriteEstimatesToMultiVersionStore()
	aborts := make(chan occ.Abort, 1)
	iter := store.newMVSValidationIterator(1, nil, nil, store.CollectIteratorItems(1), true, nil, parent, aborts)
	defer func() { require.NoError(t, iter.Close()) }()
	abort := occ.NewEstimateAbort(0)
	require.PanicsWithValue(t, abort, func() { iter.Value() })
	require.Equal(t, abort, <-aborts)
	require.PanicsWithValue(t, abort, func() { iter.Value() })
	require.Empty(t, aborts, "the terminal abort must not be published twice")
}
