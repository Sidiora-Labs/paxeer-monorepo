package layerxledger_test

import (
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxledger"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

func assertLxpRevert(t *testing.T, data []byte, text string) {
	t.Helper()
	require.GreaterOrEqual(t, len(data), 4)
	require.Equal(t, []byte{0x08, 0xc3, 0x79, 0xa0}, data[:4])
	decoded, err := abi.UnpackRevert(data)
	require.NoError(t, err)
	require.Equal(t, text, decoded)
	require.True(t, strings.HasPrefix(decoded, "LXP_"))
	require.Equal(t, pcommon.RevertReason(errors.New(text)), data)
}

func TestLedgerRevertReasons(t *testing.T) {
	t.Run("ledgerRegistry", func(t *testing.T) {
		cases := []struct {
			err    *sdkerrors.Error
			number uint32
			text   string
		}{
			{types.ErrNonCanonical, 3, "LXP_NON_CANONICAL"},
			{types.ErrMalformedSend, 107, "LXP_MALFORMED_SEND"},
			{types.ErrUnauthorizedDebit, 207, "LXP_UNAUTHORIZED_DEBIT"},
			{types.ErrContextMismatch, 213, "LXP_CONTEXT_MISMATCH"},
			{types.ErrNoPayerGrant, 214, "LXP_NO_PAYER_GRANT"},
			{types.ErrIdempotentReplay, 302, "LXP_IDEMPOTENT_REPLAY"},
			{types.ErrExpired, 303, "LXP_EXPIRED"},
			{types.ErrSequenceMismatch, 305, "LXP_SEQUENCE_MISMATCH"},
			{types.ErrSequenceExhausted, 307, "LXP_SEQUENCE_EXHAUSTED"},
			{types.ErrInsufficientBalance, 400, "LXP_INSUFFICIENT_BALANCE"},
			{types.ErrZeroAmount, 401, "LXP_ZERO_AMOUNT"},
			{types.ErrAssetMismatch, 402, "LXP_ASSET_MISMATCH"},
			{types.ErrAssetPaused, 403, "LXP_ASSET_PAUSED"},
			{types.ErrAssetAlreadyRegistered, 411, "LXP_ASSET_ALREADY_REGISTERED"},
			{types.ErrPausedScope, 735, "LXP_PAUSED_SCOPE"},
			{types.ErrArenaExhausted, 905, "LXP_ARENA_EXHAUSTED"},
			{types.ErrFatalSupplyMismatch, 1003, "LXP_FATAL_SUPPLY_MISMATCH"},
		}
		for _, test := range cases {
			t.Run(test.text, func(t *testing.T) {
				require.Equal(t, test.number, test.err.ABCICode())
				require.Equal(t, types.ModuleName, test.err.Codespace())
				require.Equal(t, test.text, test.err.Error())
				for _, err := range []error{test.err, fmt.Errorf("ledger refused: %w", test.err), sdkerrors.Wrap(test.err, "ledger refused")} {
					converted := layerxledger.RevertError(err)
					require.Equal(t, test.text, converted.Error())
					require.Equal(t, err, errors.Unwrap(converted))
					require.ErrorIs(t, converted, test.err)
					var registered *sdkerrors.Error
					require.ErrorAs(t, converted, &registered)
					require.Same(t, test.err, registered)
					require.Equal(t, err, converted.(interface{ Cause() error }).Cause())
					assertLxpRevert(t, pcommon.LXPReason(converted), test.text)
				}
			})
		}
	})

	t.Run("kernelRegistry", func(t *testing.T) {
		header, err := os.ReadFile("../../include/layerx/lxp_result.h")
		require.NoError(t, err)
		pattern := regexp.MustCompile(`(?m)^\s*X\(\s*(LXP_[A-Z0-9_]+)\s*,\s*(-?\d+)\s*\)`)
		declarations := pattern.FindAllSubmatch(header, -1)
		require.NotEmpty(t, declarations)
		for _, declaration := range declarations {
			numeric, err := strconv.ParseInt(string(declaration[2]), 10, 32)
			require.NoError(t, err)
			code := lxphost.Code(numeric)
			name := strings.Replace(string(declaration[1]), "LXP_ERR_", "LXP_", 1)
			t.Run(name, func(t *testing.T) {
				if numeric <= -1000 {
					require.True(t, strings.HasPrefix(name, "LXP_FATAL_"))
					require.Empty(t, layerxledger.KernelCodeText(code))
					return
				}
				require.Equal(t, name, layerxledger.KernelCodeText(code))
				for _, err := range []error{code, fmt.Errorf("ledger refused: %w", code), sdkerrors.Wrap(code, "ledger refused")} {
					converted := layerxledger.RevertError(err)
					require.Equal(t, name, converted.Error())
					require.Equal(t, err, errors.Unwrap(converted))
					require.ErrorIs(t, converted, code)
					var result lxphost.Code
					require.ErrorAs(t, converted, &result)
					require.Equal(t, code, result)
					require.Equal(t, err, converted.(interface{ Cause() error }).Cause())
					assertLxpRevert(t, pcommon.LXPReason(converted), name)
				}
			})
		}
		for _, code := range []lxphost.Code{-999, 1, math.MaxInt32} {
			t.Run(fmt.Sprintf("unknown_%d", code), func(t *testing.T) {
				require.Equal(t, "LXP_UNKNOWN", layerxledger.KernelCodeText(code))
				assertLxpRevert(t, pcommon.LXPReason(layerxledger.RevertError(code)), "LXP_UNKNOWN")
			})
		}
		for _, code := range []lxphost.Code{-1000, math.MinInt32} {
			require.Empty(t, layerxledger.KernelCodeText(code))
		}
	})

	t.Run("unregisteredErrors", func(t *testing.T) {
		require.Nil(t, layerxledger.RevertError(nil))
		unknown := errors.New("unregistered failure")
		for _, err := range []error{unknown, sdkerrors.ErrUnauthorized, fmt.Errorf("detail: %w", sdkerrors.ErrUnauthorized)} {
			require.Same(t, err, layerxledger.RevertError(err))
		}
		assertLxpRevert(t, pcommon.LXPReason(layerxledger.RevertError(unknown)), "LXP_UNKNOWN: unknown; unregistered failure")
	})
}
