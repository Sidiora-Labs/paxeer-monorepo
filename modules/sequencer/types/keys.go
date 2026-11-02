package types

import (
	"bytes"
	"encoding/binary"
)

const (
	// ModuleName is the sequencer module, mounted by the activation store
	// upgrade.
	ModuleName = "sequencer"
	StoreKey   = ModuleName
)

var (
	// LastSeqKey holds the last committed sequence position, u64 big-endian.
	LastSeqKey = []byte{0x01}
	// LastSeqHashKey holds the seqHash of LastSeq, 32 bytes.
	LastSeqHashKey = []byte{0x02}
	// ReceiptKeysKey holds the pinned receipt keys with their height ranges.
	ReceiptKeysKey = []byte{0x03}
	// LeaderListKey holds the pinned leader list.
	LeaderListKey = []byte{0x04}
	// StoredVotesKey is a prefix only; decided vote entries live under StoredVotesKeyAt.
	StoredVotesKey = []byte{0x05}
	// ActivationHeightKey holds the first sequenced height S, u64 big-endian.
	ActivationHeightKey = []byte{0x07}
)

// StoredVotesKeyAt returns the decided-votes key for the given height.
func StoredVotesKeyAt(height int64) []byte {
	var encoded [8]byte
	binary.BigEndian.PutUint64(encoded[:], uint64(height))
	return append(bytes.Clone(StoredVotesKey), encoded[:]...)
}
