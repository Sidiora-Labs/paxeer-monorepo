package keeper

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	sdktestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/stretchr/testify/require"
)

func genesisProgramStoreSnapshot(t *testing.T, k Keeper, ctx sdk.Context) map[string][]byte {
	t.Helper()
	out := map[string][]byte{}
	iterator := ctx.KVStore(k.storeKey).Iterator(nil, nil)
	for ; iterator.Valid(); iterator.Next() {
		out[string(iterator.Key())] = bytes.Clone(iterator.Value())
	}
	require.NoError(t, iterator.Close())
	return out
}

func TestGenesisRoundTripCarriesOriginTwoState(t *testing.T) {
	s := newProgramFulfilSuite(t, 4)
	otherProgram := types.Hash32{0x71, 0x22}
	s.record(programFulfilP, 3, types.KindSearch, programFulfilPayload, paxProgramAsset, sdk.NewInt(75))
	s.record(programFulfilP, 5, types.KindSearch, programFulfilPayload, sidProgramAsset, sdk.NewInt(29))
	s.record(otherProgram, ^uint64(0), types.KindFetch, []byte("pending fetch"), paxProgramAsset, sdk.NewInt(11))
	response := bytes.Repeat([]byte{0xab}, 2*ProgramAnswerChunkBytes+17)
	observation := s.observation(programFulfilP, 3, response, uint32(len(response)+100))
	require.NoError(t, s.k.FulfilProgram(s.ctx, s.signed(observation, s.keys[:3]...)))
	observation = s.observation(programFulfilP, 5, nil, 0)
	require.NoError(t, s.k.FulfilProgram(s.ctx, s.signed(observation, s.keys[:3]...)))
	s.k.setProgramFeeDenom(s.ctx, types.Hash32{0x99}, types.ProgramFeeDenomUSDC)
	s.k.setNonce(s.ctx, 19)
	s.k.setPaused(s.ctx, true)
	before := genesisProgramStoreSnapshot(t, s.k, s.ctx)

	exported := s.k.ExportGenesis(s.ctx)
	require.NoError(t, exported.Validate())
	require.Len(t, exported.ProgramPending, 3)
	require.Len(t, exported.ProgramAnswers, 5)
	require.Len(t, exported.ProgramFeeDenoms, 3)
	require.Equal(t, uint64(1), exported.ProgramPendingCount)
	require.Equal(t, uint64(2), exported.ProgramCommittedCount)
	require.Equal(t, before, genesisProgramStoreSnapshot(t, s.k, s.ctx))
	raw, err := json.Marshal(exported)
	require.NoError(t, err)
	var imported types.GenesisState
	require.NoError(t, json.Unmarshal(raw, &imported))

	key := sdk.NewKVStoreKey(types.StoreKey)
	ctx := sdktestutil.DefaultContext(key, sdk.NewTransientStoreKey("transient_genesis"))
	k := Keeper{storeKey: key}
	require.Empty(t, genesisProgramStoreSnapshot(t, k, ctx))
	k.InitGenesis(ctx, imported)
	require.Equal(t, before, genesisProgramStoreSnapshot(t, k, ctx))
	require.Equal(t, exported, k.ExportGenesis(ctx))
}

func TestInitGenesisPanicsOnACorruptPendingRecord(t *testing.T) {
	s := newProgramFulfilSuite(t, 4)
	s.record(programFulfilP, 3, types.KindSearch, programFulfilPayload, paxProgramAsset, sdk.NewInt(75))
	s.record(programFulfilP, 5, types.KindSearch, programFulfilPayload, sidProgramAsset, sdk.NewInt(29))
	exported := s.k.ExportGenesis(s.ctx)
	require.NoError(t, exported.Validate())
	for _, corruption := range []struct {
		name   string
		mutate func([]byte) []byte
	}{
		{"short", func(raw []byte) []byte { return raw[:len(raw)-1] }},
		{"version", func(raw []byte) []byte { raw[0] = 2; return raw }},
		{"kind", func(raw []byte) []byte { raw[1] = 0xff; return raw }},
		{"fulfilled", func(raw []byte) []byte { raw[122] = 2; return raw }},
	} {
		t.Run(corruption.name, func(t *testing.T) {
			genesis := exported
			genesis.ProgramPending = append([]types.ProgramPendingEntry(nil), exported.ProgramPending...)
			genesis.ProgramPending[1].Record = corruption.mutate(bytes.Clone(genesis.ProgramPending[1].Record))
			_, err := DecodeProgramPendingRecord(genesis.ProgramPending[1].Record)
			require.ErrorIs(t, err, errProgramNonCanonical)
			key := sdk.NewKVStoreKey(types.StoreKey)
			ctx := sdktestutil.DefaultContext(key, sdk.NewTransientStoreKey("transient_genesis"))
			k := Keeper{storeKey: key}
			before := genesisProgramStoreSnapshot(t, k, ctx)
			require.Panics(t, func() { k.InitGenesis(ctx, genesis) })
			require.Equal(t, before, genesisProgramStoreSnapshot(t, k, ctx))
		})
	}
}

func TestExportGenesisWithoutOriginTwoStateKeepsTheOldShape(t *testing.T) {
	s := newProgramFulfilSuite(t, 4)
	before := genesisProgramStoreSnapshot(t, s.k, s.ctx)
	exported := s.k.ExportGenesis(s.ctx)
	require.Nil(t, exported.ProgramPending)
	require.Nil(t, exported.ProgramAnswers)
	require.Nil(t, exported.ProgramFeeDenoms)
	require.Zero(t, exported.ProgramPendingCount)
	require.Zero(t, exported.ProgramCommittedCount)
	legacy := struct {
		Params    types.Params      `json:"params"`
		Paused    bool              `json:"paused"`
		Attestors types.AttestorSet `json:"attestors"`
		Nonce     uint64            `json:"nonce"`
		Requests  []types.Request   `json:"requests"`
		Results   []types.Result    `json:"results"`
	}{exported.Params, exported.Paused, exported.Attestors, exported.Nonce, exported.Requests, exported.Results}
	want, err := json.Marshal(legacy)
	require.NoError(t, err)
	got, err := json.Marshal(exported)
	require.NoError(t, err)
	require.Equal(t, string(want), string(got))
	require.Equal(t, before, genesisProgramStoreSnapshot(t, s.k, s.ctx))
	var imported types.GenesisState
	require.NoError(t, json.Unmarshal(want, &imported))
	key := sdk.NewKVStoreKey(types.StoreKey)
	ctx := sdktestutil.DefaultContext(key, sdk.NewTransientStoreKey("transient_genesis"))
	k := Keeper{storeKey: key}
	k.InitGenesis(ctx, imported)
	require.Equal(t, before, genesisProgramStoreSnapshot(t, k, ctx))
}
