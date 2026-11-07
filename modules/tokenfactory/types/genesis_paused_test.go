package types_test

import (
	"encoding/binary"
	"strings"
	"testing"

	"github.com/gogo/protobuf/proto"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/tokenfactory/types"
)

func TestGenesisPausedDenomsWireField(t *testing.T) {
	type PausedDenoms = []string

	state := types.GenesisState{
		Params: types.Params{DenomAllowlistMaxSize: 42},
		FactoryDenoms: []types.GenesisDenom{{
			Denom:             "denom",
			AuthorityMetadata: types.DenomAuthorityMetadata{Admin: "admin"},
		}},
		PausedDenoms: PausedDenoms{"factory/admin/one", "factory/admin/" + strings.Repeat("a", 130)},
	}

	legacyWire := []byte("\x0a\x02\x08\x2a\x12\x10\x0a\x05denom\x12\x07\x0a\x05admin")
	wantWire := append([]byte(nil), legacyWire...)
	for _, denom := range state.PausedDenoms {
		wantWire = append(wantWire, 0x1a)
		wantWire = binary.AppendUvarint(wantWire, uint64(len(denom)))
		wantWire = append(wantWire, denom...)
	}

	wire, err := proto.Marshal(&state)
	require.NoError(t, err)
	require.Equal(t, wantWire, wire, "paused denoms must use repeated length-delimited field 3 (tag 0x1a)")
	require.Equal(t, len(wire), state.Size())

	var decoded types.GenesisState
	require.NoError(t, proto.Unmarshal(wire, &decoded))
	require.Equal(t, state, decoded)
	require.Equal(t, state.PausedDenoms, decoded.GetPausedDenoms())

	var legacy types.GenesisState
	require.NoError(t, proto.Unmarshal(legacyWire, &legacy))
	require.Empty(t, legacy.GetPausedDenoms())
	require.Equal(t, state.Params, legacy.Params)
	require.Equal(t, state.FactoryDenoms, legacy.FactoryDenoms)
	legacyRoundTrip, err := proto.Marshal(&legacy)
	require.NoError(t, err)
	require.Equal(t, legacyWire, legacyRoundTrip)

	var empty types.GenesisState
	require.NoError(t, proto.Unmarshal(nil, &empty))
	require.Empty(t, empty.GetPausedDenoms())
	require.Nil(t, (*types.GenesisState)(nil).GetPausedDenoms())
}
