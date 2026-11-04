package datastructures_test

import (
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/utils/datastructures"
	"github.com/stretchr/testify/require"
)

func TestRepairM_868_SyncSetReadLocks(t *testing.T) {
	set := datastructures.NewSyncSet([]int{3, 1, 2})
	type result struct {
		snapshot []int
		contains bool
		size     int
	}
	finished := make(chan result, 1)
	go func() {
		var once sync.Once
		var observed result
		observed.snapshot = set.ToOrderedSlice(func(a, b int) bool {
			once.Do(func() {
				observed.contains = set.Contains(2)
				observed.size = set.Size()
				set.Remove(2)
				set.Add(4)
				set.AddAll([]int{5, 6})
				set.RemoveAll([]int{5, 6})
			})
			return a < b
		})
		finished <- observed
	}()
	select {
	case observed := <-finished:
		require.True(t, observed.contains)
		require.Equal(t, 3, observed.size)
		require.Equal(t, []int{1, 2, 3}, observed.snapshot)
		observed.snapshot[0] = 99
	case <-time.After(5 * time.Second):
		t.Fatal("sorting held the set lock while invoking its comparator")
	}
	require.Equal(t, []int{1, 3, 4}, set.ToOrderedSlice(func(a, b int) bool { return a < b }))
	require.False(t, set.Contains(99))
	require.False(t, set.Contains(2))
	require.Equal(t, 3, set.Size())
}

func TestRepairM_868_SyncSetMixedCoverage(t *testing.T) {
	testSyncSetMixedReadWrite(t)
}

func TestRepairM_868_TypedMapRangeDelete(t *testing.T) {
	testTypedSyncMapRangeDeletes(t)
}

func TestRepairM_868_TypedMapRangeStop(t *testing.T) {
	testTypedSyncMapRangeStops(t)
}

func TestRepairM_868_TypedMapRangeCoverage(t *testing.T) {
	m := datastructures.NewTypedSyncMap[int, *string]()
	first, removed, original, updated, added := "first", "removed", "original", "updated", "added"
	m.Store(3, &original)
	m.Store(2, &removed)
	m.Store(1, &first)
	m.Store(5, nil)
	var keys []int
	var values []*string
	m.Range(func(key int, value *string) bool {
		keys = append(keys, key)
		values = append(values, value)
		if key == 1 {
			finished := make(chan struct{})
			go func() {
				m.Delete(2)
				m.Store(3, &updated)
				m.Store(4, &added)
				close(finished)
			}()
			select {
			case <-finished:
			case <-time.After(5 * time.Second):
				t.Fatal("map mutation blocked during a Range callback")
			}
		}
		return true
	})
	require.Equal(t, []int{1, 3, 5}, keys)
	require.Equal(t, []*string{&first, &updated, nil}, values)
	_, ok := m.Load(2)
	require.False(t, ok)
	value, ok := m.Load(4)
	require.True(t, ok)
	require.Same(t, &added, value)
	require.Equal(t, 4, m.Len())
}
