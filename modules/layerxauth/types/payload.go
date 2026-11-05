package types

import "encoding/binary"

type SessionSuccessor struct {
	PredecessorID      [32]byte
	ExpectedCommitment [32]byte
}

func EncodeSessionPayload(grant []byte, expirySequence uint64, actionKey [32]byte, successor *SessionSuccessor) ([]byte, error) {
	length := sessionMinBytes
	variant, fields := byte(sessionVariantGrant), byte(sessionGrantFields)
	if successor != nil {
		length += 2 * (4 + 32)
		variant, fields = sessionVariantSuccessor, sessionSuccessorFields
	}
	if len(grant) == 0 || len(grant) > governancePayloadMaxBytes-length {
		return nil, ErrNonCanonical.Wrapf("session grant length %d outside 1..%d", len(grant), governancePayloadMaxBytes-length)
	}
	payload := make([]byte, length+len(grant))
	payload[0], payload[1], payload[2], payload[3] = governancePayloadTag, sessionOrdinal, variant, fields
	binary.BigEndian.PutUint32(payload[4:8], uint32(len(grant)))
	copy(payload[8:], grant)
	offset := 8 + len(grant)
	binary.BigEndian.PutUint64(payload[offset:offset+8], expirySequence)
	offset += 8
	binary.BigEndian.PutUint32(payload[offset:offset+4], uint32(len(actionKey)))
	copy(payload[offset+4:], actionKey[:])
	offset += 4 + len(actionKey)
	if successor != nil {
		binary.BigEndian.PutUint32(payload[offset:offset+4], uint32(len(successor.PredecessorID)))
		copy(payload[offset+4:], successor.PredecessorID[:])
		offset += 4 + len(successor.PredecessorID)
		binary.BigEndian.PutUint32(payload[offset:offset+4], uint32(len(successor.ExpectedCommitment)))
		copy(payload[offset+4:], successor.ExpectedCommitment[:])
	}
	if err := ValidateSessionPayload(payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func EncodeCapabilityPayload(grant []byte) ([]byte, error) {
	const overhead = 4 + 4
	if len(grant) == 0 || len(grant) > governancePayloadMaxBytes-overhead {
		return nil, ErrNonCanonical.Wrapf("capability grant length %d outside 1..%d", len(grant), governancePayloadMaxBytes-overhead)
	}
	payload := make([]byte, overhead+len(grant))
	payload[0], payload[1], payload[2], payload[3] = governancePayloadTag, 8, 1, 1
	binary.BigEndian.PutUint32(payload[4:8], uint32(len(grant)))
	copy(payload[8:], grant)
	return payload, nil
}

func EncodeRevokePayload(grantID [32]byte, reason uint8, sequence uint64) []byte {
	payload := make([]byte, revokeBytes)
	payload[0], payload[1], payload[2], payload[3] = governancePayloadTag, revokeOrdinal, revokeVariant, revokeFields
	copy(payload[4:revokeReasonIndex], grantID[:])
	payload[revokeReasonIndex] = reason
	binary.BigEndian.PutUint64(payload[revokeReasonIndex+1:], sequence)
	return payload
}
