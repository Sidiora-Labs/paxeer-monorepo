package datastructures

import (
	"sort"
	"sync"
)

// SyncSet supports concurrent reads and writes. ToOrderedSlice copies membership
// under the lock, then sorts the detached snapshot without the lock held, so its
// comparator may access or mutate the set. AddAll adds each value separately;
// RemoveAll removes its values under one lock.
type SyncSet[T comparable] struct {
	dict map[T]bool
	mu   *sync.Mutex
}

func NewSyncSet[T comparable](initial []T) SyncSet[T] {
	res := SyncSet[T]{
		dict: map[T]bool{},
		mu:   &sync.Mutex{},
	}
	for _, s := range initial {
		res.dict[s] = true
	}
	return res
}

func (s *SyncSet[T]) Add(val T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dict[val] = true
}

func (s *SyncSet[T]) AddAll(vals []T) {
	for _, val := range vals {
		s.Add(val)
	}
}

func (s *SyncSet[T]) Remove(val T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.dict, val)
}

func (s *SyncSet[T]) RemoveAll(vals []T) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, val := range vals {
		delete(s.dict, val)
	}
}

func (s *SyncSet[T]) Contains(val T) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.dict[val]
	return ok
}

func (s *SyncSet[T]) ToOrderedSlice(comparator func(T, T) bool) []T {
	s.mu.Lock()
	res := make([]T, 0, len(s.dict))
	for s := range s.dict {
		res = append(res, s)
	}
	s.mu.Unlock()
	sort.SliceStable(res, func(i, j int) bool {
		return comparator(res[i], res[j])
	})

	return res
}

func (s *SyncSet[T]) Size() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.dict)
}

func StringComparator(s1 string, s2 string) bool {
	return s1 < s2
}
