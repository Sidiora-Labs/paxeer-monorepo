package types

import (
	"crypto/sha256"
	"encoding/binary"
)

const (
	SeqHashDomain = "PAXEER/v1/sequence\x00"
)

func SeqHash(prev [32]byte, seq uint64, activityID [32]byte) [32]byte {
	var seqBytes [8]byte
	binary.BigEndian.PutUint64(seqBytes[:], seq)

	h := sha256.New()
	h.Write([]byte(SeqHashDomain))
	h.Write(prev[:])
	h.Write(seqBytes[:])
	h.Write(activityID[:])
	var out [32]byte
	copy(out[:], h.Sum(nil))
	return out
}
