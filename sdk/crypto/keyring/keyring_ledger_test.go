//go:build !ledger
// +build !ledger

package keyring

import (
	"errors"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/stretchr/testify/require"
)

// Unavailable support is an error-path assertion, not a successful device gate.
// Positive device tests retain their original tags in keyring_ledger_device_test.go.
func TestKeyringLedgerUnavailable(t *testing.T) {
	kr := NewInMemory()
	info, err := kr.SaveLedgerKey("device", hd.Secp256k1, "cosmos", 118, 0, 0)
	require.Nil(t, info)
	require.ErrorContains(t, err, "failed to generate ledger key: failed to retrieve device: ledger nano S: support for ledger devices is not available in this executable")
	require.NotNil(t, errors.Unwrap(err))
	cause := err
	for errors.Unwrap(cause) != nil {
		cause = errors.Unwrap(cause)
	}
	require.EqualError(t, cause, "support for ledger devices is not available in this executable")
	infos, listErr := kr.List()
	require.NoError(t, listErr)
	require.Empty(t, infos)
}
