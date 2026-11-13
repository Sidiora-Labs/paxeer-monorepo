package bank_test

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

// TestBankDurableReopen checks ABI results and refusal reasons after committing
// and reopening bank storage after every call.
func TestBankDurableReopen(t *testing.T) {
	transcript := proofBankDurablePath(t)
	require.Len(t, transcript.Successes, 2)
	require.Len(t, transcript.Reasons, 12)
	require.Len(t, transcript.Malformed, 19)
	for _, row := range transcript.Reasons {
		data, err := hex.DecodeString(row.Data[2:])
		require.NoError(t, err)
		reason, err := abi.UnpackRevert(data)
		require.NoError(t, err)
		want := row.Code + ": " + row.Message
		if row.Detail != nil {
			want += "; " + *row.Detail
		}
		require.Equal(t, want, reason)
	}
}
