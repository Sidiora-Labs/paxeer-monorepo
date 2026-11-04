package store

import sdkkv "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/kv"

// Gets the first item.
func First(st KVStore, start, end []byte) (kv sdkkv.Pair, ok bool) {
	iter := st.Iterator(start, end)
	defer func() { _ = iter.Close() }()
	if !iter.Valid() {
		return kv, false
	}

	return sdkkv.Pair{Key: iter.Key(), Value: iter.Value()}, true
}

// Gets the last item.  `end` is exclusive.
func Last(st KVStore, start, end []byte) (kv sdkkv.Pair, ok bool) {
	iter := st.ReverseIterator(start, end)
	defer func() { _ = iter.Close() }()
	if !iter.Valid() {
		return kv, false
	}

	return sdkkv.Pair{Key: iter.Key(), Value: iter.Value()}, true
}
