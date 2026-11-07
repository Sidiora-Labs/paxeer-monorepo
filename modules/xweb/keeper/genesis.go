package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

// InitGenesis loads a validated genesis. An active (unpaused) genesis that
// carries a program-origin pair field must carry a valid pair; an active
// genesis that carries neither field, the state exported before the program
// origin existed, replays as it is.
func (k Keeper) InitGenesis(ctx sdk.Context, genesis types.GenesisState) {
	if err := genesis.Validate(); err != nil {
		panic(err)
	}
	for _, entry := range genesis.ProgramPending {
		if _, err := DecodeProgramPendingRecord(entry.Record); err != nil {
			panic(err)
		}
	}
	if !genesis.Paused && (genesis.Params.KernelNetworkId != 0 || genesis.Params.ProgramFeeDenoms != nil) {
		if err := types.ValidateProgramSettings(genesis.Params.KernelNetworkId, genesis.Params.ProgramFeeDenoms); err != nil {
			panic(err)
		}
	}
	if err := k.SetParams(ctx, genesis.Params); err != nil {
		panic(err)
	}
	k.setPaused(ctx, genesis.Paused)
	k.setAttestorSet(ctx, genesis.Attestors)
	k.setNonce(ctx, genesis.Nonce)
	for _, request := range genesis.Requests {
		k.setRequest(ctx, request)
	}
	for _, result := range genesis.Results {
		k.setResult(ctx, result)
	}
	for _, entry := range genesis.ProgramPending {
		k.setProgramPendingRecord(ctx, ProgramPendingKey(entry.ProgramID, entry.RequestID), entry.Record)
	}
	for _, entry := range genesis.ProgramAnswers {
		ctx.KVStore(k.storeKey).Set(ProgramAnswerKey(entry.ProgramID, entry.RequestID, entry.Part), entry.Value)
	}
	for _, entry := range genesis.ProgramFeeDenoms {
		k.setProgramFeeDenom(ctx, entry.Asset, entry.Denom)
	}
	k.setProgramPendingCount(ctx, genesis.ProgramPendingCount)
	k.setProgramCommittedCount(ctx, genesis.ProgramCommittedCount)
}

// ExportGenesis returns owned entries for the module JSON export; replay through
// InitGenesis preserves program answers, fee denoms and counters after restart.
func (k Keeper) ExportGenesis(ctx sdk.Context) types.GenesisState {
	// The owning types schema must carry archived answer parts before JSON
	// export can represent this state. Refuse a lossy export in the meantime.
	iterator := ctx.KVStore(k.storeKey).Iterator([]byte(programArchivePrefix), sdk.PrefixEndBytes([]byte(programArchivePrefix)))
	hasArchive := iterator.Valid()
	if err := iterator.Close(); err != nil {
		panic(err)
	}
	if hasArchive {
		panic(types.ErrInvalidGenesis.Wrap("program archive requires modules/xweb/types/genesis.go archive schema wiring"))
	}
	return types.GenesisState{
		Params:                k.GetParams(ctx),
		Paused:                k.IsPaused(ctx),
		Attestors:             k.GetAttestorSet(ctx),
		Nonce:                 k.Nonce(ctx),
		Requests:              k.GetRequests(ctx),
		Results:               k.GetResults(ctx),
		ProgramPending:        k.ProgramPendingEntries(ctx),
		ProgramAnswers:        k.ProgramAnswerEntries(ctx),
		ProgramFeeDenoms:      k.ProgramFeeDenomEntries(ctx),
		ProgramPendingCount:   k.ProgramPendingCount(ctx),
		ProgramCommittedCount: k.ProgramCommittedCount(ctx),
	}
}
