package types

import (
	"testing"

	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/stretchr/testify/require"
)

func TestErrorCodesTwentyEightToThirty(t *testing.T) {
	for _, tc := range []struct {
		name    string
		err     *sdkerrors.Error
		code    uint32
		message string
	}{
		{
			name:    "ErrIssuerMismatch",
			err:     ErrIssuerMismatch,
			code:    28,
			message: "LXP_UNAUTHORIZED_DEBIT: caller is not the denom issuer",
		},
		{
			name:    "ErrEncodeTokenFactorySetDenomPaused",
			err:     ErrEncodeTokenFactorySetDenomPaused,
			code:    29,
			message: "Error while encoding tokenfactory set denom paused msg in wasmd",
		},
		{
			name:    "ErrEncodingDenomPaused",
			err:     ErrEncodingDenomPaused,
			code:    30,
			message: "Error encoding denom paused as JSON",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.NotNil(t, tc.err)
			require.Equal(t, "tokenfactory", tc.err.Codespace())
			require.Equal(t, tc.code, tc.err.ABCICode())
			require.EqualError(t, tc.err, tc.message)
			require.True(t, tc.err.Is(sdkerrors.ABCIError("tokenfactory", tc.code, "registry lookup")), "code must resolve to the registered sentinel")
		})
	}
}
