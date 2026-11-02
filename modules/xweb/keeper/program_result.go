package keeper

import (
	"encoding/binary"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type ProgramResult struct {
	ContentDigest              types.Hash32
	FullLength, ResponseLength uint32
	Response                   []byte
}

func (k Keeper) GetProgramResult(ctx sdk.Context, programID types.Hash32, requestID uint64) (ProgramResult, error) {
	store := ctx.KVStore(k.storeKey)
	header := store.Get(ProgramAnswerKey(programID, requestID, 0))
	if header == nil {
		return ProgramResult{}, errProgramUnknownField
	}
	if len(header) != ProgramAnswerHeaderBytes {
		return ProgramResult{}, errProgramNonCanonical
	}
	result := ProgramResult{
		FullLength:     binary.BigEndian.Uint32(header[32:36]),
		ResponseLength: binary.BigEndian.Uint32(header[36:40]),
	}
	copy(result.ContentDigest[:], header[:32])
	if result.ResponseLength > types.MaxResponseBytes || result.ResponseLength > result.FullLength {
		return ProgramResult{}, errProgramNonCanonical
	}
	result.Response = make([]byte, int(result.ResponseLength))
	for part, offset := uint8(1), 0; offset < len(result.Response); part, offset = part+1, offset+ProgramAnswerChunkBytes {
		expected := min(ProgramAnswerChunkBytes, len(result.Response)-offset)
		chunk := store.Get(ProgramAnswerKey(programID, requestID, part))
		if len(chunk) != expected {
			return ProgramResult{}, errProgramNonCanonical
		}
		copy(result.Response[offset:offset+expected], chunk)
	}
	return result, nil
}
