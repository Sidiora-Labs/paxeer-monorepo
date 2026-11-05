package layerxprograms

import (
	"encoding/binary"
	"fmt"
	"math"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
)

func lifecyclePayloadError(code int32, detail string) error {
	return fmt.Errorf("%s: %s", pcommon.LXPCodeName(code), detail)
}

func lifecyclePayloadSize(header int, parts ...[]byte) (int, error) {
	size := uint64(header)
	for _, part := range parts {
		size += uint64(len(part))
	}
	if size > uint64(int(^uint(0)>>1)) {
		return 0, lifecyclePayloadError(-5, "payload exceeds addressable length")
	}
	return int(size), nil
}

func EncodeDeployPayload(programID [32]byte, abiVersion uint16, policy uint8, authority, codeHash [32]byte, iface, wasm []byte) ([]byte, error) {
	if programID == ([32]byte{}) || policy > 1 || (policy == 0) != (authority == ([32]byte{})) {
		return nil, lifecyclePayloadError(-3, "invalid program ID, policy or authority")
	}
	if len(wasm) == 0 {
		return nil, lifecyclePayloadError(-3, "wasm is empty")
	}
	if uint64(len(wasm)) > math.MaxUint32 || len(iface) > 952 {
		return nil, lifecyclePayloadError(-5, "wasm or interface exceeds kernel length limit")
	}
	header := 104
	if len(iface) != 0 {
		header = 108
	}
	size, err := lifecyclePayloadSize(header, iface, wasm)
	if err != nil {
		return nil, err
	}
	payload := make([]byte, size)
	copy(payload, programID[:])
	binary.BigEndian.PutUint16(payload[32:34], abiVersion)
	payload[34] = policy
	copy(payload[36:68], authority[:])
	copy(payload[68:100], codeHash[:])
	binary.BigEndian.PutUint32(payload[100:104], uint32(len(wasm)))
	if len(iface) != 0 {
		binary.BigEndian.PutUint32(payload[104:108], uint32(len(iface)))
	}
	copy(payload[header:], iface)
	copy(payload[header+len(iface):], wasm)
	return payload, nil
}

func EncodeUpgradePayload(programID [32]byte, abiVersion uint16, flags uint8, oldHash, newHash [32]byte, hook, iface, wasm []byte) ([]byte, error) {
	if len(hook) > math.MaxUint16 || uint64(len(wasm)) > math.MaxUint32 || len(iface) > 952 {
		return nil, lifecyclePayloadError(-5, "hook, wasm or interface exceeds kernel length limit")
	}
	if programID == ([32]byte{}) || flags&0xfc != 0 || (flags&1 != 0) != (len(hook) != 0) {
		return nil, lifecyclePayloadError(-3, "invalid program ID, flags or hook presence")
	}
	if len(wasm) == 0 {
		return nil, lifecyclePayloadError(-3, "wasm is empty")
	}
	framed := len(iface) != 0 || flags&2 != 0
	header := 106
	if framed {
		header = 110
	}
	size, err := lifecyclePayloadSize(header, hook, iface, wasm)
	if err != nil {
		return nil, err
	}
	payload := make([]byte, size)
	copy(payload, programID[:])
	binary.BigEndian.PutUint16(payload[32:34], abiVersion)
	payload[34] = flags
	copy(payload[36:68], oldHash[:])
	copy(payload[68:100], newHash[:])
	binary.BigEndian.PutUint16(payload[100:102], uint16(len(hook)))
	binary.BigEndian.PutUint32(payload[102:106], uint32(len(wasm)))
	if framed {
		binary.BigEndian.PutUint32(payload[106:110], uint32(len(iface)))
	}
	offset := header
	offset += copy(payload[offset:], hook)
	offset += copy(payload[offset:], iface)
	copy(payload[offset:], wasm)
	return payload, nil
}

func EncodeRegistryReadPayload(programID [32]byte, body []byte) ([]byte, error) {
	if programID == ([32]byte{}) {
		return nil, lifecyclePayloadError(-3, "program ID is zero")
	}
	size, err := lifecyclePayloadSize(32, body)
	if err != nil {
		return nil, err
	}
	payload := make([]byte, size)
	copy(payload, programID[:])
	copy(payload[32:], body)
	return payload, nil
}
