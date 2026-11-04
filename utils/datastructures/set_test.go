package datastructures_test

import (
	"runtime"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/utils/datastructures"
	"github.com/stretchr/testify/require"
)

func TestSyncSetSequantial(t *testing.T) {
	set := datastructures.NewSyncSet([]uint64{1, 2, 3})
	require.Equal(t, 3, set.Size())
	set.Add(4)
	require.Equal(t, 4, set.Size())
	set.Add(1)
	require.Equal(t, 4, set.Size())
	for _, i := range []uint64{1, 2, 3, 4} {
		require.True(t, set.Contains(i))
	}
	set.Remove(4)
	require.Equal(t, 3, set.Size())
	set.RemoveAll([]uint64{2, 3})
	require.Equal(t, 1, set.Size())
}

func TestSyncSetParallel(t *testing.T) {
	set := datastructures.NewSyncSet([]uint64{})
	wg := sync.WaitGroup{}
	// a parallelism of 100 is likely to expose any potential data corruption
	for i := uint64(0); i < uint64(100); i++ {
		wg.Add(1)
		i := i
		go func() {
			set.Add(i)
			wg.Done()
		}()
	}
	wg.Wait()
	require.Equal(t, 100, set.Size())

	for i := uint64(0); i < uint64(100); i++ {
		wg.Add(1)
		i := i
		go func() {
			set.Remove(i)
			wg.Done()
		}()
	}
	wg.Wait()
	require.Equal(t, 0, set.Size())
}

func TestSyncSetMixedReadWrite(t *testing.T) {
	testSyncSetMixedReadWrite(t)
}

func testSyncSetMixedReadWrite(t *testing.T) {
	t.Helper()
	const iterations = 256
	set := datastructures.NewSyncSet([]int{-2, -1})
	start := make(chan struct{})
	finished := make(chan struct{})
	var workers, midpoint sync.WaitGroup
	workers.Add(5)
	midpoint.Add(5)
	for worker := 0; worker < 5; worker++ {
		go func(worker int) {
			defer workers.Done()
			<-start
			for i := 0; i < iterations; i++ {
				if i == iterations/2 {
					midpoint.Done()
					midpoint.Wait()
				}
				if worker < 2 {
					key := worker * 2
					set.Add(key)
					runtime.Gosched()
					set.AddAll([]int{key, key + 1})
					set.Remove(key)
					set.RemoveAll([]int{key, key + 1})
				} else {
					if !set.Contains(-2) || !set.Contains(-1) || set.Contains(99) {
						t.Error("concurrent mutation changed permanent or absent membership")
					}
					if size := set.Size(); size < 2 || size > 6 {
						t.Errorf("concurrent size = %d, want between 2 and 6", size)
					}
					snapshot := set.ToOrderedSlice(func(a, b int) bool { return a < b })
					if len(snapshot) < 2 || len(snapshot) > 6 {
						t.Errorf("snapshot length = %d, want between 2 and 6", len(snapshot))
					} else if snapshot[0] != -2 || snapshot[1] != -1 {
						t.Errorf("snapshot lost permanent members: %v", snapshot)
					}
					for j, key := range snapshot {
						if key < -2 || key > 3 || (j > 0 && snapshot[j-1] >= key) {
							t.Errorf("snapshot has invalid, duplicate or unsorted member: %v", snapshot)
							break
						}
					}
				}
				runtime.Gosched()
			}
		}(worker)
	}
	go func() {
		workers.Wait()
		close(finished)
	}()
	close(start)
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("mixed set readers and writers did not finish")
	}
	require.Equal(t, []int{-2, -1}, set.ToOrderedSlice(func(a, b int) bool { return a < b }))
	require.Equal(t, 2, set.Size())
	for _, key := range []int{0, 1, 2, 3} {
		require.False(t, set.Contains(key))
	}
}
