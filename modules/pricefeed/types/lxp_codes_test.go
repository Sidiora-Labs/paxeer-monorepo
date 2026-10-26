package types_test

import (
	"errors"
	"fmt"
	"testing"

	perpstypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/perps/types"
	pricefeedtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/pricefeed/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
)

func TestLXPCodesCoverCodes2To16(t *testing.T) {
	want := []pricefeedtypes.LXPCode{
		{pricefeedtypes.ErrUnauthorized, "LXP_PRICEFEED_UNAUTHORIZED"},
		{pricefeedtypes.ErrInvalidParams, "LXP_PRICEFEED_INVALID_PARAMS"},
		{pricefeedtypes.ErrInvalidGenesis, "LXP_PRICEFEED_INVALID_GENESIS"},
		{pricefeedtypes.ErrNotValidator, "LXP_PRICEFEED_NOT_VALIDATOR"},
		{pricefeedtypes.ErrInvalidKey, "LXP_PRICEFEED_INVALID_KEY"},
		{pricefeedtypes.ErrKeyInUse, "LXP_PRICEFEED_KEY_IN_USE"},
		{pricefeedtypes.ErrNoKey, "LXP_PRICEFEED_NO_KEY"},
		{pricefeedtypes.ErrUnknownMarket, "LXP_PRICEFEED_UNKNOWN_MARKET"},
		{pricefeedtypes.ErrUnregisteredFeeder, "LXP_PRICEFEED_UNREGISTERED_FEEDER"},
		{pricefeedtypes.ErrRateLimited, "LXP_PRICEFEED_RATE_LIMITED"},
		{pricefeedtypes.ErrTooManyObservations, "LXP_PRICEFEED_TOO_MANY_OBSERVATIONS"},
		{pricefeedtypes.ErrSubmissionTooLarge, "LXP_PRICEFEED_SUBMISSION_TOO_LARGE"},
		{pricefeedtypes.ErrDuplicateMarket, "LXP_PRICEFEED_DUPLICATE_MARKET"},
		{pricefeedtypes.ErrSubmissionAlone, "LXP_PRICEFEED_SUBMISSION_ALONE"},
		{pricefeedtypes.ErrInvalidObservation, "LXP_PRICEFEED_INVALID_OBSERVATION"},
	}
	codes := pricefeedtypes.LXPCodes()
	require.Len(t, codes, 15)
	for i, code := range codes {
		t.Run(want[i].Name, func(t *testing.T) {
			require.NotNil(t, code.Err)
			require.Same(t, want[i].Err, code.Err)
			require.Equal(t, pricefeedtypes.ModuleName, code.Err.Codespace())
			require.Equal(t, uint32(i+2), code.Err.ABCICode())
			require.Equal(t, want[i].Name, code.Name)
			for _, err := range []error{
				code.Err,
				fmt.Errorf("submit: %w", code.Err),
				fmt.Errorf("deliver: %w", sdkerrors.Wrap(code.Err, "submit")),
			} {
				name, ok := pricefeedtypes.LXPName(err)
				require.True(t, ok)
				require.Equal(t, want[i].Name, name)
			}
		})
	}
}

func TestLXPNameRefusesForeignError(t *testing.T) {
	var nilRegistered *sdkerrors.Error
	cases := map[string]error{
		"foreign":         perpstypes.ErrInvalidParams,
		"wrapped foreign": fmt.Errorf("submit: %w", perpstypes.ErrInvalidParams),
		"plain":           errors.New("LXP_PRICEFEED_INVALID_PARAMS"),
		"nil":             nil,
		"typed nil":       nilRegistered,
	}
	for label, err := range cases {
		t.Run(label, func(t *testing.T) {
			name, ok := pricefeedtypes.LXPName(err)
			require.False(t, ok)
			require.Empty(t, name)
		})
	}
}
