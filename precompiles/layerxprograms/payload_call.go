package layerxprograms

import (
	"encoding/binary"
	"errors"
	"math"
)

var (
	errCallNonCanonical = errors.New("LXP_NON_CANONICAL")
	errCallLengthLimit  = errors.New("LXP_LENGTH_LIMIT")
)

func EncodeCallPayload(programID [32]byte, abiVersion uint16, entrypoint string, calldata, capabilities, access []byte, responseCapacity uint32, budget [7]uint64) ([]byte, error) {
	if len(entrypoint) == 0 || len(entrypoint) > 128 {
		return nil, errCallNonCanonical
	}
	for i := 0; i < len(entrypoint); i++ {
		b := entrypoint[i]
		if !((b >= 'A' && b <= 'Z') || (b >= 'a' && b <= 'z') || (b >= '0' && b <= '9') || b == '_' || b == '.') {
			return nil, errCallNonCanonical
		}
	}
	if uint64(len(calldata)) > math.MaxUint32 || len(capabilities) > math.MaxUint16 || uint64(len(access)) > math.MaxUint32 {
		return nil, errCallLengthLimit
	}
	length := uint64(106+len(entrypoint)) + uint64(len(calldata)) + uint64(len(capabilities)) + uint64(len(access))
	if length > uint64(^uint(0)>>1) {
		return nil, errCallLengthLimit
	}
	payload := make([]byte, int(length))
	copy(payload[:32], programID[:])
	binary.BigEndian.PutUint16(payload[32:34], abiVersion)
	binary.BigEndian.PutUint16(payload[34:36], uint16(len(entrypoint)))
	binary.BigEndian.PutUint32(payload[36:40], uint32(len(calldata)))
	binary.BigEndian.PutUint16(payload[40:42], uint16(len(capabilities)))
	binary.BigEndian.PutUint32(payload[42:46], uint32(len(access)))
	binary.BigEndian.PutUint32(payload[46:50], responseCapacity)
	for i, word := range budget {
		binary.BigEndian.PutUint64(payload[50+i*8:58+i*8], word)
	}
	cursor := 106
	cursor += copy(payload[cursor:], entrypoint)
	cursor += copy(payload[cursor:], calldata)
	cursor += copy(payload[cursor:], capabilities)
	copy(payload[cursor:], access)
	return payload, nil
}
