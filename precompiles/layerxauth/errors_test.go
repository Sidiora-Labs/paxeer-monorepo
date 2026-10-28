package layerxauth_test

import (
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxauth"
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

func TestAuthRevertReasons(t *testing.T) {
	t.Run("authRegistry", func(t *testing.T) {
		cases := []struct {
			err    *sdkerrors.Error
			number uint32
			text   string
		}{
			{types.ErrAuthScope, 2, "LXP_AUTH_SCOPE"},
			{types.ErrAuthRevoked, 3, "LXP_AUTH_REVOKED"},
			{types.ErrAuthExpired, 4, "LXP_AUTH_EXPIRED"},
			{types.ErrAuthNotMature, 5, "LXP_AUTH_NOT_MATURE"},
			{types.ErrNotYetValid, 6, "LXP_NOT_YET_VALID"},
			{types.ErrSequenceReused, 7, "LXP_SEQUENCE_REUSED"},
			{types.ErrNonCanonical, 8, "LXP_NON_CANONICAL"},
			{types.ErrUnauthorizedDebit, 9, "LXP_UNAUTHORIZED_DEBIT"},
			{types.ErrIdempotentReplay, 10, "LXP_IDEMPOTENT_REPLAY"},
		}
		for _, test := range cases {
			t.Run(test.text, func(t *testing.T) {
				require.Equal(t, test.number, test.err.ABCICode())
				require.Equal(t, types.ModuleName, test.err.Codespace())
				require.Equal(t, test.text, test.err.Error())
				require.True(t, test.err.Is(layerxauth.RevertError(test.err)))
				require.True(t, test.err.Is(layerxauth.RevertError(sdkerrors.Wrap(test.err, "auth refused"))))
				for _, err := range []error{
					test.err,
					fmt.Errorf("auth refused: %w", test.err),
					sdkerrors.Wrap(test.err, "auth refused"),
					fmt.Errorf("outer: %w", sdkerrors.Wrap(test.err, "auth refused")),
				} {
					converted := layerxauth.RevertError(err)
					require.Equal(t, test.text, converted.Error())
					require.Same(t, err, errors.Unwrap(converted))
					require.Same(t, err, converted.(interface{ Cause() error }).Cause())
					require.ErrorIs(t, converted, test.err)
					var registered *sdkerrors.Error
					require.ErrorAs(t, converted, &registered)
					require.Same(t, test.err, registered)
					assertLxpRevert(t, pcommon.LXPReason(converted), test.text)
				}
			})
		}
	})

	t.Run("governanceRegistry", func(t *testing.T) {
		registered := govtypes.ErrPausedScope
		require.Equal(t, uint32(735), registered.ABCICode())
		require.Equal(t, govtypes.ModuleName, registered.Codespace())
		require.Equal(t, "LXP_PAUSED_SCOPE", registered.Error())
		require.True(t, registered.Is(layerxauth.RevertError(registered)))
		require.True(t, registered.Is(layerxauth.RevertError(sdkerrors.Wrap(registered, "auth refused"))))
		for _, err := range []error{
			registered,
			fmt.Errorf("auth refused: %w", registered),
			sdkerrors.Wrap(registered, "auth refused"),
			fmt.Errorf("outer: %w", sdkerrors.Wrap(registered, "auth refused")),
		} {
			converted := layerxauth.RevertError(err)
			require.Equal(t, "LXP_PAUSED_SCOPE", converted.Error())
			require.Same(t, err, errors.Unwrap(converted))
			require.Same(t, err, converted.(interface{ Cause() error }).Cause())
			require.ErrorIs(t, converted, registered)
			var result *sdkerrors.Error
			require.ErrorAs(t, converted, &result)
			require.Same(t, registered, result)
			assertLxpRevert(t, pcommon.LXPReason(converted), "LXP_PAUSED_SCOPE")
		}
	})

	t.Run("kernelRegistry", func(t *testing.T) {
		header, err := os.ReadFile("../../include/layerx/lxp_result.h")
		require.NoError(t, err)
		declarations := regexp.MustCompile(`(?m)^\s*X\(\s*(LXP_[A-Z0-9_]+)\s*,\s*(-?\d+)\s*\)`).FindAllSubmatch(header, -1)
		require.NotEmpty(t, declarations)
		for _, declaration := range declarations {
			numeric, err := strconv.ParseInt(string(declaration[2]), 10, 32)
			require.NoError(t, err)
			code := lxphost.Code(numeric)
			name := strings.Replace(string(declaration[1]), "LXP_ERR_", "LXP_", 1)
			t.Run(name, func(t *testing.T) {
				if numeric <= -1000 {
					require.Empty(t, layerxauth.KernelCodeText(code))
					require.Equal(t, pcommon.LXPCodeName(int32(code)), layerxauth.KernelCodeText(code))
					return
				}
				require.Equal(t, name, layerxauth.KernelCodeText(code))
				require.Equal(t, pcommon.LXPCodeName(int32(code)), layerxauth.KernelCodeText(code))
				for _, err := range []error{code, fmt.Errorf("auth refused: %w", code), sdkerrors.Wrap(code, "auth refused")} {
					converted := layerxauth.RevertError(err)
					require.Equal(t, name, converted.Error())
					require.Equal(t, err, errors.Unwrap(converted))
					require.Equal(t, err, converted.(interface{ Cause() error }).Cause())
					require.ErrorIs(t, converted, code)
					var result lxphost.Code
					require.ErrorAs(t, converted, &result)
					require.Equal(t, code, result)
					assertLxpRevert(t, pcommon.LXPReason(converted), name)
				}
			})
		}
		for _, code := range []lxphost.Code{-999, 1, math.MaxInt32} {
			require.Equal(t, "LXP_UNKNOWN", layerxauth.KernelCodeText(code))
			assertLxpRevert(t, pcommon.LXPReason(layerxauth.RevertError(code)), "LXP_UNKNOWN")
		}
		for _, code := range []lxphost.Code{-1000, math.MinInt32} {
			require.Empty(t, layerxauth.KernelCodeText(code))
		}
	})

	t.Run("unregisteredErrors", func(t *testing.T) {
		require.Nil(t, layerxauth.RevertError(nil))
		for _, original := range []error{
			errors.New("unregistered failure"),
			errors.New("LXP_AUTH_SCOPE"),
			sdkerrors.ErrUnauthorized,
			sdkerrors.New(types.ModuleName, 99, "unregistered auth failure"),
		} {
			require.Same(t, original, layerxauth.RevertError(original))
		}
	})
}
