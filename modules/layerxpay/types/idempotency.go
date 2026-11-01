package types

import (
	"crypto/sha256"
	"encoding/binary"
)

const (
	idempotencyKeyDomain = "layerxpay/idempotency/v1"
	streamIDDomain       = "layerxpay/stream-id/v1"
)

func DeriveIdempotencyKey(txHash [32]byte, callOrdinal uint32, selector [4]byte) [32]byte {
	var preimage [len(idempotencyKeyDomain) + 32 + 4 + 4]byte
	n := copy(preimage[:], idempotencyKeyDomain)
	n += copy(preimage[n:], txHash[:])
	binary.BigEndian.PutUint32(preimage[n:n+4], callOrdinal)
	copy(preimage[n+4:], selector[:])
	return nonzeroDerivedDigest(sha256.Sum256(preimage[:]))
}

func DeriveStreamID(txHash [32]byte, callOrdinal uint32, payer [32]byte) [32]byte {
	var preimage [len(streamIDDomain) + 32 + 4 + 32]byte
	n := copy(preimage[:], streamIDDomain)
	n += copy(preimage[n:], txHash[:])
	binary.BigEndian.PutUint32(preimage[n:n+4], callOrdinal)
	copy(preimage[n+4:], payer[:])
	return nonzeroDerivedDigest(sha256.Sum256(preimage[:]))
}

func nonzeroDerivedDigest(digest [32]byte) [32]byte {
	if digest == ([32]byte{}) {
		return sha256.Sum256(digest[:])
	}
	return digest
}
