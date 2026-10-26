package types_test

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	"github.com/stretchr/testify/require"
)

func programGenesis(pending, committed int) *types.GenesisState {
	g := types.DefaultGenesis()
	g.Params.KernelNetworkId = 125
	g.Params.ProgramFeeDenoms = []string{types.ProgramFeeDenomPAX}
	g.ProgramPendingCount = uint64(pending)
	g.ProgramCommittedCount = uint64(committed)
	asset := types.Hash32{0x31}
	feeAccount := sha256.Sum256(append([]byte("PAXEERX_WEB_FEES_V1"), asset[:]...))
	payloadHash := types.Keccak([]byte("https://example.com"))
	for i := 0; i < pending+committed; i++ {
		record := make([]byte, 123)
		record[0] = 1
		record[1] = types.KindFetch
		copy(record[2:34], payloadHash[:])
		copy(record[34:66], asset[:])
		copy(record[66:98], feeAccount[:])
		record[113] = 1
		binary.BigEndian.PutUint64(record[114:122], uint64(i+1))
		entry := types.ProgramPendingEntry{
			ProgramID: types.Hash32{0x21},
			RequestID: uint64(i + 1),
			Record:    record,
		}
		if i >= pending {
			record[122] = 1
			g.ProgramAnswers = append(g.ProgramAnswers, types.ProgramAnswerEntry{
				ProgramID: entry.ProgramID,
				RequestID: entry.RequestID,
				Part:      0,
				Value:     make([]byte, 40),
			})
		}
		g.ProgramPending = append(g.ProgramPending, entry)
	}
	if committed > 0 {
		g.ProgramFeeDenoms = []types.ProgramFeeDenomEntry{{Asset: asset, Denom: types.ProgramFeeDenomPAX}}
	}
	return g
}

func TestGenesisValidateRefusesPendingOverCapacity(t *testing.T) {
	require.Equal(t, 256, types.ProgramPendingCapacity)
	g := programGenesis(256, 0)
	require.NoError(t, g.Validate())
	g = programGenesis(257, 0)
	err := g.Validate()
	require.ErrorIs(t, err, types.ErrInvalidGenesis)
	require.Contains(t, err.Error(), "pending")
	require.Contains(t, err.Error(), "capacity 256")

	g.ProgramPendingCount = 256
	require.ErrorIs(t, g.Validate(), types.ErrInvalidGenesis)
}

func TestGenesisValidateRefusesCommittedOverCapacity(t *testing.T) {
	require.Equal(t, 64, types.ProgramCommittedCapacity)
	g := programGenesis(0, 64)
	require.NoError(t, g.Validate())
	g = programGenesis(0, 65)
	err := g.Validate()
	require.ErrorIs(t, err, types.ErrInvalidGenesis)
	require.Contains(t, err.Error(), "committed")
	require.Contains(t, err.Error(), "capacity 64")

	g.ProgramCommittedCount = 64
	require.ErrorIs(t, g.Validate(), types.ErrInvalidGenesis)

	g = programGenesis(256, 64)
	require.NoError(t, g.Validate())
}

func TestGenesisValidateRefusesCountersThatDisagreeWithRecords(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*types.GenesisState)
	}{
		{"pending counter too low", func(g *types.GenesisState) { g.ProgramPendingCount = 0 }},
		{"pending counter too high", func(g *types.GenesisState) { g.ProgramPendingCount = 2 }},
		{"committed counter too low", func(g *types.GenesisState) { g.ProgramCommittedCount = 0 }},
		{"committed counter too high", func(g *types.GenesisState) { g.ProgramCommittedCount = 2 }},
		{"missing answer header", func(g *types.GenesisState) { g.ProgramAnswers = nil }},
		{"answer chunk without header", func(g *types.GenesisState) { g.ProgramAnswers[0].Part = 1 }},
		{"answer without record", func(g *types.GenesisState) { g.ProgramAnswers[0].RequestID = 3 }},
		{"answer for another program", func(g *types.GenesisState) { g.ProgramAnswers[0].ProgramID[0]++ }},
		{"answer for unfulfilled record", func(g *types.GenesisState) { g.ProgramAnswers[0].RequestID = 1 }},
		{"orphan answer chunk", func(g *types.GenesisState) {
			chunk := g.ProgramAnswers[0]
			chunk.RequestID = 3
			chunk.Part = 1
			chunk.Value = []byte("answer")
			g.ProgramAnswers = append(g.ProgramAnswers, chunk)
		}},
		{"missing fee denom", func(g *types.GenesisState) { g.ProgramFeeDenoms = nil }},
		{"fee denom for another asset", func(g *types.GenesisState) { g.ProgramFeeDenoms[0].Asset[0]++ }},
		{"empty fee denom", func(g *types.GenesisState) { g.ProgramFeeDenoms[0].Denom = "" }},
		{"unmapped fee denom", func(g *types.GenesisState) { g.ProgramFeeDenoms[0].Denom = "uatom" }},
		{"duplicate fee asset", func(g *types.GenesisState) {
			g.ProgramFeeDenoms = append(g.ProgramFeeDenoms, g.ProgramFeeDenoms[0])
		}},
		{"conflicting fee asset", func(g *types.GenesisState) {
			entry := g.ProgramFeeDenoms[0]
			entry.Denom = types.ProgramFeeDenomSID
			g.ProgramFeeDenoms = append(g.ProgramFeeDenoms, entry)
		}},
		{"duplicate pending key", func(g *types.GenesisState) {
			g.ProgramPending = append(g.ProgramPending, g.ProgramPending[0])
			g.ProgramPendingCount++
		}},
		{"duplicate fulfilled key", func(g *types.GenesisState) {
			g.ProgramPending = append(g.ProgramPending, g.ProgramPending[1])
			g.ProgramCommittedCount++
		}},
		{"duplicate answer header", func(g *types.GenesisState) {
			g.ProgramAnswers = append(g.ProgramAnswers, g.ProgramAnswers[0])
		}},
		{"duplicate answer chunk", func(g *types.GenesisState) {
			chunk := g.ProgramAnswers[0]
			chunk.Part = 1
			chunk.Value = []byte("answer")
			g.ProgramAnswers = append(g.ProgramAnswers, chunk, chunk)
		}},
		{"empty record", func(g *types.GenesisState) { g.ProgramPending[0].Record = nil }},
		{"short record", func(g *types.GenesisState) { g.ProgramPending[0].Record = g.ProgramPending[0].Record[:122] }},
		{"long record", func(g *types.GenesisState) { g.ProgramPending[0].Record = append(g.ProgramPending[0].Record, 0) }},
		{"wrong record version", func(g *types.GenesisState) { g.ProgramPending[0].Record[0] = 2 }},
		{"invalid fulfilled byte", func(g *types.GenesisState) { g.ProgramPending[0].Record[122] = 2 }},
		{"invalid program kind", func(g *types.GenesisState) { g.ProgramPending[0].Record[1] = types.KindApi }},
		{"short answer header", func(g *types.GenesisState) { g.ProgramAnswers[0].Value = make([]byte, 39) }},
		{"long answer header", func(g *types.GenesisState) { g.ProgramAnswers[0].Value = make([]byte, 41) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			g := programGenesis(1, 1)
			require.NoError(t, g.Validate())
			tc.mutate(g)
			require.ErrorIs(t, g.Validate(), types.ErrInvalidGenesis)
		})
	}

	t.Run("mapped fee denoms", func(t *testing.T) {
		for _, denom := range []string{types.ProgramFeeDenomPAX, types.ProgramFeeDenomSID,
			types.ProgramFeeDenomUSDC, types.ProgramFeeDenomUSDL} {
			g := programGenesis(1, 1)
			g.Params.ProgramFeeDenoms = []string{denom}
			g.ProgramFeeDenoms[0].Denom = denom
			require.NoError(t, g.Validate(), denom)
		}
	})
	t.Run("request ids scoped by program", func(t *testing.T) {
		g := programGenesis(0, 2)
		g.ProgramPending[1].ProgramID[0]++
		g.ProgramPending[1].RequestID = g.ProgramPending[0].RequestID
		g.ProgramAnswers[1].ProgramID = g.ProgramPending[1].ProgramID
		g.ProgramAnswers[1].RequestID = g.ProgramPending[1].RequestID
		require.NoError(t, g.Validate())
	})
	t.Run("multiple answer parts count once", func(t *testing.T) {
		g := programGenesis(1, 1)
		binary.BigEndian.PutUint32(g.ProgramAnswers[0].Value[32:36], 6)
		binary.BigEndian.PutUint32(g.ProgramAnswers[0].Value[36:40], 6)
		g.ProgramAnswers = append(g.ProgramAnswers, types.ProgramAnswerEntry{
			ProgramID: g.ProgramAnswers[0].ProgramID,
			RequestID: g.ProgramAnswers[0].RequestID,
			Part:      1,
			Value:     []byte("answer"),
		})
		require.NoError(t, g.Validate())
	})
	t.Run("default genesis JSON shape", func(t *testing.T) {
		g := types.DefaultGenesis()
		require.NoError(t, g.Validate())
		raw, err := json.Marshal(g)
		require.NoError(t, err)
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &fields))
		require.Len(t, fields, 6)
		for _, field := range []string{"params", "paused", "attestors", "nonce", "requests", "results"} {
			require.Contains(t, fields, field)
		}
		var restored types.GenesisState
		require.NoError(t, json.Unmarshal(raw, &restored))
		require.NoError(t, restored.Validate())
	})
	t.Run("origin 2 JSON round trip", func(t *testing.T) {
		g := programGenesis(1, 1)
		raw, err := json.Marshal(g)
		require.NoError(t, err)
		var fields map[string]json.RawMessage
		require.NoError(t, json.Unmarshal(raw, &fields))
		for _, field := range []string{"program_pending", "program_answers", "program_fee_denoms_by_asset",
			"program_pending_count", "program_committed_count"} {
			require.Contains(t, fields, field)
		}
		var restored types.GenesisState
		require.NoError(t, json.Unmarshal(raw, &restored))
		require.Equal(t, *g, restored)
		require.NoError(t, restored.Validate())
	})
}
