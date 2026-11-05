package app

import (
	"testing"

	seqtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/sequencer/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
)

func TestSequencedHaltWrapsBatchError(t *testing.T) {
	err := sequencedHalt(7, seqtypes.ErrBadSignature)

	require.ErrorIs(t, err, ErrSequencedBlockHalt)
	require.ErrorIs(t, err, seqtypes.ErrBadSignature)
	require.Equal(t, "sequenced block halt at height 7: "+seqtypes.ErrBadSignature.Error(), err.Error())
}

func TestSequencedHaltKeepsABCICode(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
	}{
		{name: "direct", err: seqtypes.ErrBadSignature},
		{name: "wrapped", err: seqtypes.ErrBadSignature.Wrap("batch delivery failed")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := sequencedHalt(7, tc.err)

			require.ErrorIs(t, err, ErrSequencedBlockHalt)
			require.ErrorIs(t, err, tc.err)
			require.ErrorIs(t, err, seqtypes.ErrBadSignature)
			codespace, code, _ := sdkerrors.ABCIInfo(err, false)
			require.Equal(t, seqtypes.ModuleName, codespace)
			require.Equal(t, seqtypes.ErrBadSignature.ABCICode(), code)
			require.Equal(t, uint32(3), code)
		})
	}
}
