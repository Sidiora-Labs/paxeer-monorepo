package datastructures_test

import (
	"fmt"
	"sync"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/utils/datastructures"
	"github.com/stretchr/testify/require"
)

func TestTypedSyncMapSequantial(t *testing.T) {
	m := datastructures.NewTypedSyncMap[int, string]()
	m.Store(1, "a")
	loaded, ok := m.Load(1)
	require.True(t, ok)
	require.Equal(t, "a", loaded)

	actual, exists := m.LoadOrStore(1, "b")
	require.True(t, exists)
	require.Equal(t, "a", actual)
	loaded, _ = m.Load(1)
	require.Equal(t, "a", loaded)

	actual, exists = m.LoadOrStore(2, "b")
	require.False(t, exists)
	require.Equal(t, "b", actual)
	loaded, ok = m.Load(2)
	require.True(t, ok)
	require.Equal(t, "b", loaded)

	require.Equal(t, 2, m.Len())

	m.Delete(1)

	loaded, ok = m.Load(1)
	require.False(t, ok)
	require.Empty(t, loaded)
	require.Equal(t, 1, m.Len())
}

func TestTypedSyncMapParallel(t *testing.T) {
	m := datastructures.NewTypedSyncMap[int, string]()
	wg := sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		i := i
		go func() {
			m.Store(i, "a")
			wg.Done()
		}()
	}
	wg.Wait()
	require.Equal(t, 100, m.Len())

	for i := 0; i < 100; i++ {
		wg.Add(1)
		i := i
		go func() {
			m.Delete(i)
			wg.Done()
		}()
	}
	wg.Wait()
	require.Equal(t, 0, m.Len())
}

func TestTypedSyncMapRangeStops(t *testing.T) {
	testTypedSyncMapRangeStops(t)
}

func testTypedSyncMapRangeStops(t *testing.T) {
	t.Helper()
	for _, stop := range []int{1, 2, 3} {
		t.Run(fmt.Sprintf("stop_at_%d", stop), func(t *testing.T) {
			m := datastructures.NewTypedSyncMap[int, string]()
			m.Store(3, "c")
			m.Store(1, "a")
			m.Store(2, "b")
			var keys []int
			var values []string
			m.Range(func(key int, value string) bool {
				keys = append(keys, key)
				values = append(values, value)
				return key != stop
			})
			require.Equal(t, []int{1, 2, 3}[:stop], keys)
			require.Equal(t, []string{"a", "b", "c"}[:stop], values)
			require.Equal(t, 3, m.Len())
		})
	}
	empty := datastructures.NewTypedSyncMap[int, string]()
	empty.Range(func(int, string) bool {
		t.Fatal("empty map invoked a callback")
		return false
	})
}

func TestTypedSyncMapRangeDeletes(t *testing.T) {
	testTypedSyncMapRangeDeletes(t)
}

func testTypedSyncMapRangeDeletes(t *testing.T) {
	t.Helper()
	m := datastructures.NewTypedSyncMap[int, string]()
	m.Store(3, "c")
	m.Store(2, "b")
	m.Store(1, "a")
	var keys []int
	var values []string
	m.Range(func(key int, value string) bool {
		keys = append(keys, key)
		values = append(values, value)
		if key == 1 {
			m.Delete(1)
			m.Delete(2)
		}
		return true
	})
	require.Equal(t, []int{1, 3}, keys)
	require.Equal(t, []string{"a", "c"}, values)
	require.Equal(t, 1, m.Len())
	_, ok := m.Load(2)
	require.False(t, ok)
	value, ok := m.Load(3)
	require.True(t, ok)
	require.Equal(t, "c", value)
}

func TestTypedSyncMapDeepCopy(t *testing.T) {
	m := datastructures.NewTypedSyncMap[int, []string]()
	val := []string{"a"}
	m.Store(1, val)
	copy := m.DeepCopy(utils.SliceCopy[string])
	require.Equal(t, 1, copy.Len())
	copiedVal, ok := copy.Load(1)
	require.True(t, ok)
	copiedVal[0] = "b"
	require.Equal(t, "a", val[0])
}

func TestTypedSyncMapDeepApply(t *testing.T) {
	m := datastructures.NewTypedSyncMap[string, int]()
	m.Store("a", 1)
	m.Store("c", 3)
	m.Store("b", 2)
	agg := 0
	lastSeen := -1
	m.DeepApply(func(i int) {
		agg += i
		// Require that entries are applied in sorted order
		require.Less(t, lastSeen, i)
		lastSeen = i
	})
	require.Equal(t, 3, m.Len())
	require.Equal(t, 6, agg)
}

func TestTypedNestedSyncMapSequantial(t *testing.T) {
	m := datastructures.NewTypedNestedSyncMap[int, int, string]()
	m.StoreNested(1, 1, "a")
	loaded, ok := m.LoadNested(1, 1)
	require.True(t, ok)
	require.Equal(t, "a", loaded)

	actual, exists := m.LoadOrStoreNested(1, 1, "b")
	require.True(t, exists)
	require.Equal(t, "a", actual)
	loaded, _ = m.LoadNested(1, 1)
	require.Equal(t, "a", loaded)

	actual, exists = m.LoadOrStoreNested(1, 2, "b")
	require.False(t, exists)
	require.Equal(t, "b", actual)
	loaded, ok = m.LoadNested(1, 2)
	require.True(t, ok)
	require.Equal(t, "b", loaded)

	require.Equal(t, 1, m.Len())

	actual, exists = m.LoadOrStoreNested(2, 1, "c")
	require.False(t, exists)
	require.Equal(t, "c", actual)
	loaded, ok = m.LoadNested(2, 1)
	require.True(t, ok)
	require.Equal(t, "c", loaded)

	require.Equal(t, 2, m.Len())

	m.DeleteNested(1, 1)

	loaded, ok = m.LoadNested(1, 1)
	require.False(t, ok)
	require.Empty(t, loaded)
	require.Equal(t, 2, m.Len())
}

func TestTypedNestedSyncMapParallel(t *testing.T) {
	m := datastructures.NewTypedNestedSyncMap[int, int, string]()
	wg := sync.WaitGroup{}
	for i := 0; i < 100; i++ {
		wg.Add(1)
		i := i
		go func() {
			m.StoreNested(i, i, "a")
			wg.Done()
		}()
	}
	wg.Wait()
	require.Equal(t, 100, m.Len())

	for i := 0; i < 100; i++ {
		wg.Add(1)
		i := i
		go func() {
			m.DeleteNested(i, i)
			wg.Done()
		}()
	}
	wg.Wait()
	require.Equal(t, 0, m.Len())
}

func TestTypedNestedSyncMapDeepCopy(t *testing.T) {
	m := datastructures.NewTypedNestedSyncMap[int, int, []string]()
	val := []string{"a"}
	m.StoreNested(1, 1, val)
	copy := m.DeepCopy(utils.SliceCopy[string])
	require.Equal(t, 1, copy.Len())
	copiedVal, ok := copy.LoadNested(1, 1)
	require.True(t, ok)
	copiedVal[0] = "b"
	require.Equal(t, "a", val[0])
}

func TestTypedNestedSyncMapDeepApply(t *testing.T) {
	m := datastructures.NewTypedNestedSyncMap[int, int, int]()
	m.StoreNested(1, 1, 1)
	m.StoreNested(2, 1, 2)
	m.StoreNested(2, 2, 3)
	agg := 0
	m.DeepApply(func(i int) { agg += i })
	require.Equal(t, 2, m.Len())
	require.Equal(t, 6, agg)
}
