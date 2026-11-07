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

// GetProgramResult reassembles the canonical stored parts for query and EVM
// consumers. A missing chunk is corruption, while a missing header is unknown.
func (k Keeper) GetProgramResult(ctx sdk.Context, programID types.Hash32, requestID uint64) (ProgramResult, error) {
	store := ctx.KVStore(k.storeKey)
	key := ProgramAnswerKey
	header := store.Get(key(programID, requestID, 0))
	if header == nil {
		key = programArchiveKey
		header = store.Get(key(programID, requestID, 0))
	}
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
		chunk := store.Get(key(programID, requestID, part))
		if len(chunk) != expected {
			return ProgramResult{}, errProgramNonCanonical
		}
		copy(result.Response[offset:offset+expected], chunk)
	}
	return result, nil
}

// Archived answers retain their canonical header and chunks indefinitely in
// consensus state. Only the bounded hot arena is reclaimed; fulfilled records
// remain replay tombstones. Eviction follows store-key order deterministically.
const programArchivePrefix = "web/archive/"

func programArchiveKey(programID types.Hash32, requestID uint64, part uint8) []byte {
	key := append([]byte(programArchivePrefix), programID[:]...)
	key = binary.BigEndian.AppendUint64(key, requestID)
	return append(key, part)
}

func (k Keeper) reserveProgramAnswer(ctx sdk.Context, reservations uint64) error {
	if reservations > ProgramCommittedCapacity {
		return errProgramArenaExhausted
	}
	count := k.ProgramCommittedCount(ctx)
	if count > ProgramCommittedCapacity {
		return errProgramFatalInvariant
	}
	for count > ProgramCommittedCapacity-reservations {
		store := ctx.KVStore(k.storeKey)
		iterator := store.Iterator([]byte(programAnswerPrefix), sdk.PrefixEndBytes([]byte(programAnswerPrefix)))
		if !iterator.Valid() {
			_ = iterator.Close()
			return errProgramFatalInvariant
		}
		key := append([]byte(nil), iterator.Key()...)
		if err := iterator.Close(); err != nil {
			return err
		}
		if len(key) != len(programAnswerPrefix)+41 || key[len(key)-1] != 0 {
			return errProgramNonCanonical
		}
		var program types.Hash32
		copy(program[:], key[len(programAnswerPrefix):])
		id := binary.BigEndian.Uint64(key[len(programAnswerPrefix)+32:])
		result, err := k.GetProgramResult(ctx, program, id)
		if err != nil {
			return err
		}
		record, found, err := k.GetProgramPendingRecord(ctx, program, id)
		if err != nil {
			return err
		}
		if !found || !record.Fulfilled || store.Has(programArchiveKey(program, id, 0)) {
			return errProgramFatalInvariant
		}
		parts := (result.ResponseLength + ProgramAnswerChunkBytes - 1) / ProgramAnswerChunkBytes
		for part := uint32(0); part <= parts; part++ {
			hot := ProgramAnswerKey(program, id, uint8(part))
			store.Set(programArchiveKey(program, id, uint8(part)), store.Get(hot))
			store.Delete(hot)
		}
		count--
		k.setProgramCommittedCount(ctx, count)
	}
	return nil
}
