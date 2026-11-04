package bank_test

import (
	"encoding/hex"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

// Task 22.33.1280 closes bank-durable-reopen-proof with this test and the
// shared durable path in proof_0x1001_bank_test.go. All original ABI and error
// assertions execute against a committed application that is closed and reopened
// after every call. This gate uses Go consumers without subprocess toolchains.
// Prior attempt: verification could not compile the auth, staking and
// tokenfactory CLI callers of tx.NewFactoryCLI, so no repair test executed.
// Task 22.31.463 repaired those callers; this continuation retains the proof
// and qualifies it in the required hf-build, then hf-done order.
// Wiring requests: none; all repair code is inside precompiles/bank.
func TestRepairM_1280_BankDurableReopen(t *testing.T) {
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
