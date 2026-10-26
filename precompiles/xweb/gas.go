package xweb

import (
	"encoding/binary"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
)

var programFulfilArguments = pcommon.MustGetABI(f, "abi.json").Methods[FulfilProgramMethod].Inputs

func ProgramFulfilGas(input []byte) uint64 {
	signatures := uint64(types.ObservationMaxSignatures)
	args, err := programFulfilArguments.Unpack(input)
	if err == nil && len(args) == 1 {
		observation, ok := args[0].([]byte)
		if ok && len(observation) >= types.ObservationHeaderBytes+1 {
			responseLength := binary.BigEndian.Uint32(observation[types.ObservationHeaderBytes-4 : types.ObservationHeaderBytes])
			offset := uint64(types.ObservationHeaderBytes) + uint64(responseLength)
			if offset < uint64(len(observation)) {
				count := observation[offset]
				if count > 0 && count <= types.ObservationMaxSignatures {
					signatures = uint64(count)
				}
			}
		}
	}
	return WriteBaseGas + GasPerByte*uint64(len(input)) + GasPerSignature*signatures
}
