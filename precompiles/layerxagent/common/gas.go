package common

import (
	"encoding/binary"
	"math"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
)

const (
	eip712HashGas     uint64 = 1000
	eip712RecoverGas  uint64 = 4000
	teeVerifyGas      uint64 = 46000
	orobBaseGas       uint64 = 1000
	orobPerByteGas    uint64 = 3
	pofqBaseGas       uint64 = 1000
	pofqPerByteGas    uint64 = 3
	schedulerReadGas  uint64 = 1000
	schedulerWriteGas uint64 = 20000
	schedulerOpenGas  uint64 = 25000
	oracleReadGas     uint64 = 1000
	oracleSubmitGas   uint64 = 30000
	streamsOpenGas    uint64 = 50000
	streamsSettleGas  uint64 = 60000
	streamsCloseGas   uint64 = 80000
)

func MethodGas(selector []byte, inputLen int) uint64 {
	base, perByte, known := methodGas(selector)
	if !known {
		return pcommon.UnknownMethodCallGas
	}
	if inputLen < 0 {
		return math.MaxUint64
	}
	length := uint64(inputLen)
	if length > (math.MaxUint64-base)/perByte {
		return math.MaxUint64
	}
	return base + perByte*length
}

func methodGas(selector []byte) (base, perByte uint64, known bool) {
	if len(selector) != 4 {
		return 0, 0, false
	}
	switch binary.BigEndian.Uint32(selector) {
	case 0x454ad3a1, 0x3df597e3, 0x3f779aef:
		return orobBaseGas, orobPerByteGas, true
	case 0xe087a6ba, 0x1b1fba48, 0xa8f32b0e:
		return pofqBaseGas, pofqPerByteGas, true
	case 0xf52981f4,
		0x894e9a0d, 0x01ec24b8, 0x26f02502:
		return 1000, 3, true
	case 0xbf22c457, 0x5eebea20:
		return schedulerReadGas, 3, true
	case 0x8397e9f8:
		return 6000, 3, true
	case 0x81097a95, 0x99812615: // aggregate, getValidatorPrice
		return oracleReadGas, 3, true
	case 0xdc9c2875: // submitPrice; the write rate also enforces staticcall refusal
		return oracleSubmitGas, 30, true
	case 0x849ba0a3:
		return schedulerOpenGas, 30, true
	case 0x40e58ee5, 0x08e6f770:
		return schedulerWriteGas, 30, true
	case 0x93e5b08b:
		return streamsOpenGas, 30, true
	case 0x8df82800, 0x405abb41:
		return streamsSettleGas, 30, true
	case 0x0aebeb4e:
		return streamsCloseGas, 30, true
	case 0x5fa55ca7, 0x71ded4b3:
		return teeVerifyGas, 3, true
	case 0xd61c3c34, 0xe6d341cd:
		return eip712HashGas, 3, true
	case 0xac580b5c:
		return eip712RecoverGas, 3, true
	default:
		return 0, 0, false
	}
}
