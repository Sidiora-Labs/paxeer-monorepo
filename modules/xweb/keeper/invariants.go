package keeper

import (
	"encoding/binary"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func ProgramCountersInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var pending, fulfilled, headers uint64
		var invalidAnswer string
		for _, entry := range k.ProgramPendingEntries(ctx) {
			record, err := DecodeProgramPendingRecord(entry.Record)
			if err != nil {
				return sdk.FormatInvariant(types.ModuleName, "program-counters", fmt.Sprintf(
					"invalid program pending record for program %x request %d: %v", entry.ProgramID, entry.RequestID, err)), true
			}
			if record.Fulfilled {
				if !ctx.KVStore(k.storeKey).Has(programArchiveKey(entry.ProgramID, entry.RequestID, 0)) {
					fulfilled++
				}
				if _, err := k.GetProgramResult(ctx, entry.ProgramID, entry.RequestID); err != nil {
					invalidAnswer = fmt.Sprintf("; invalid program answer for program %x request %d: %v", entry.ProgramID, entry.RequestID, err)
				}
			} else {
				pending++
			}
		}
		for _, entry := range k.ProgramAnswerEntries(ctx) {
			if entry.Part == 0 {
				headers++
				if _, err := k.GetProgramResult(ctx, entry.ProgramID, entry.RequestID); err != nil {
					invalidAnswer = fmt.Sprintf("; invalid program answer for program %x request %d: %v", entry.ProgramID, entry.RequestID, err)
				}
			}
		}
		k.iterate(ctx, []byte(programArchivePrefix), func(key, value []byte) {
			if len(key) != 41 {
				invalidAnswer = "; invalid program archive key"
				return
			}
			var program types.Hash32
			copy(program[:], key[:32])
			id := binary.BigEndian.Uint64(key[32:40])
			header := ctx.KVStore(k.storeKey).Get(programArchiveKey(program, id, 0))
			if len(header) != ProgramAnswerHeaderBytes {
				invalidAnswer = "; invalid program archive header"
				return
			}
			parts := (uint64(binary.BigEndian.Uint32(header[36:40])) + ProgramAnswerChunkBytes - 1) / ProgramAnswerChunkBytes
			if uint64(key[40]) > parts {
				invalidAnswer = "; invalid program archive part"
			}
			if key[40] != 0 {
				return
			}
			record, found, err := k.GetProgramPendingRecord(ctx, program, id)
			if err != nil || !found || !record.Fulfilled || ctx.KVStore(k.storeKey).Has(ProgramAnswerKey(program, id, 0)) {
				invalidAnswer = "; invalid program archive tombstone"
			}
			if _, err := k.GetProgramResult(ctx, program, id); err != nil {
				invalidAnswer = fmt.Sprintf("; invalid program archived answer: %v", err)
			}
		})
		storedPending := k.ProgramPendingCount(ctx)
		storedCommitted := k.ProgramCommittedCount(ctx)
		broken := storedPending > ProgramPendingCapacity || storedCommitted > ProgramCommittedCapacity || pending != storedPending || fulfilled != storedCommitted || headers != storedCommitted || invalidAnswer != ""
		return sdk.FormatInvariant(types.ModuleName, "program-counters", fmt.Sprintf(
			"pending: counted=%d stored=%d; committed: counted fulfilled=%d counted answer headers=%d stored=%d",
			pending, storedPending, fulfilled, headers, storedCommitted)+invalidAnswer), broken
	}
}
