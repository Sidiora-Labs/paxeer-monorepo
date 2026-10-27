package layerxprograms

import (
	"encoding/binary"
	"errors"
	"math/big"
)

func EncodeSetExitRoutePayload(programId, account, asset, destination [32]byte, seed []byte) ([]byte, error) {
	if len(seed) > 128 {
		return nil, errors.New("LXP_LENGTH_LIMIT: exit route seed exceeds 128 bytes")
	}
	payload, err := encodeWindDownPayload(programId, 1, 131+len(seed))
	if err != nil {
		return nil, err
	}
	copy(payload[33:65], account[:])
	copy(payload[65:97], asset[:])
	copy(payload[97:129], destination[:])
	binary.BigEndian.PutUint16(payload[129:131], uint16(len(seed)))
	copy(payload[131:], seed)
	return payload, nil
}

func EncodeDeprecatePayload(programId, exitProgram [32]byte, deadline uint64) ([]byte, error) {
	payload, err := encodeWindDownPayload(programId, 2, 73)
	if err != nil {
		return nil, err
	}
	if exitProgram != programId {
		return nil, errors.New("LXP_PROGRAM_REFUSED: exit program must equal program ID")
	}
	copy(payload[33:65], exitProgram[:])
	binary.BigEndian.PutUint64(payload[65:73], deadline)
	return payload, nil
}

func EncodeTombstonePayload(programId [32]byte) ([]byte, error) {
	return encodeWindDownPayload(programId, 3, 33)
}

func EncodeExitValuePayload(programId, account [32]byte) ([]byte, error) {
	payload, err := encodeWindDownPayload(programId, 4, 65)
	if err != nil {
		return nil, err
	}
	copy(payload[33:65], account[:])
	return payload, nil
}

func EncodeExitValueBoundedPayload(programId, account [32]byte, maxAmount *big.Int) ([]byte, error) {
	if maxAmount == nil || maxAmount.Sign() <= 0 {
		return nil, errors.New("LXP_NON_CANONICAL: maximum exit amount must be positive")
	}
	if maxAmount.BitLen() > 128 {
		return nil, errors.New("LXP_OVERFLOW: maximum exit amount exceeds u128")
	}
	payload, err := encodeWindDownPayload(programId, 5, 81)
	if err != nil {
		return nil, err
	}
	copy(payload[33:65], account[:])
	maxAmount.FillBytes(payload[65:81])
	return payload, nil
}

func encodeWindDownPayload(programId [32]byte, operation byte, length int) ([]byte, error) {
	if programId == ([32]byte{}) {
		return nil, errors.New("LXP_NON_CANONICAL: program ID must be nonzero")
	}
	payload := make([]byte, length)
	copy(payload[:32], programId[:])
	payload[32] = operation
	return payload, nil
}
