package keeper

import (
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func ProgramCountersInvariant(k Keeper) sdk.Invariant {
	return func(ctx sdk.Context) (string, bool) {
		var pending, fulfilled, headers uint64
		for _, entry := range k.ProgramPendingEntries(ctx) {
			record, err := DecodeProgramPendingRecord(entry.Record)
			if err != nil {
				return sdk.FormatInvariant(types.ModuleName, "program-counters", fmt.Sprintf(
					"invalid program pending record for program %x request %d: %v", entry.ProgramID, entry.RequestID, err)), true
			}
			if record.Fulfilled {
				fulfilled++
			} else {
				pending++
			}
		}
		for _, entry := range k.ProgramAnswerEntries(ctx) {
			if entry.Part == 0 {
				headers++
			}
		}
		storedPending := k.ProgramPendingCount(ctx)
		storedCommitted := k.ProgramCommittedCount(ctx)
		broken := pending != storedPending || fulfilled != storedCommitted || headers != storedCommitted
		return sdk.FormatInvariant(types.ModuleName, "program-counters", fmt.Sprintf(
			"pending: counted=%d stored=%d; committed: counted fulfilled=%d counted answer headers=%d stored=%d",
			pending, storedPending, fulfilled, headers, storedCommitted)), broken
	}
}
