package layerxpay

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/stretchr/testify/require"
)

func assertPayRevert(t *testing.T, err error, code, message string) {
	t.Helper()
	data := RevertError(err)
	require.GreaterOrEqual(t, len(data), 4)
	require.Equal(t, []byte{0x08, 0xc3, 0x79, 0xa0}, data[:4])
	reason, decodeErr := abi.UnpackRevert(data)
	require.NoError(t, decodeErr)
	require.Equal(t, code+": "+message+"; "+err.Error(), reason)
	require.True(t, strings.HasPrefix(reason, code+": "))
	require.Equal(t, pcommon.LXPRevert(code, err.Error()), data)
	if code != "LXP_UNKNOWN" {
		require.False(t, strings.HasPrefix(reason, "LXP_UNKNOWN"))
	}
}

func TestPayRevertFromKernelCode(t *testing.T) {
	for _, tc := range []struct {
		value   lxphost.Code
		code    string
		message string
	}{
		{-708, "LXP_UNAUTHORIZED_CAPTURE", "unauthorized capture"},
		{-707, "LXP_CAPTURE_EXCEEDS_HOLD", "capture exceeds hold"},
		{-700, "LXP_ESCROW_STATE", "escrow state"},
		{-709, "LXP_HOLD_EXPIRED", "hold expired"},
		{-307, "LXP_SEQUENCE_EXHAUSTED", "sequence exhausted"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			require.Equal(t, tc.code, pcommon.LXPCodeName(int32(tc.value)))
			submitErr := fmt.Errorf("%s: layerxpay %s: %w", tc.code, "escrowCapture", tc.value)
			for name, err := range map[string]error{
				"direct":       tc.value,
				"submit":       submitErr,
				"executor":     fmt.Errorf("pay executor: %w", submitErr),
				"sdk_wrapped":  sdkerrors.Wrap(submitErr, "pay executor"),
				"nested":       fmt.Errorf("executor: %w", sdkerrors.Wrap(submitErr, "pay call")),
				"numeric_only": fmt.Errorf("escrowCapture refused: %w", tc.value),
			} {
				t.Run(name, func(t *testing.T) {
					assertPayRevert(t, err, tc.code, tc.message)
				})
			}
		})
	}
	t.Run("unregistered_kernel_code", func(t *testing.T) {
		err := fmt.Errorf("escrowCapture refused: %w", lxphost.Code(-999))
		assertPayRevert(t, err, "LXP_UNKNOWN", "unknown")
	})
}

func TestPayRevertFromModuleErrors(t *testing.T) {
	for _, tc := range []struct {
		err     error
		code    string
		message string
	}{
		{paytypes.ErrUnboundCaller, "LXP_PAY_UNBOUND_CALLER", "unbound caller"},
		{paytypes.ErrFamilyDisabled, "LXP_MODULE_DISABLED", "module disabled"},
		{paytypes.ErrKernelGateMismatch, "LXP_PAY_KERNEL_GATE_MISMATCH", "kernel gate mismatch"},
		{govtypes.ErrPausedScope, "LXP_PAUSED_SCOPE", "paused scope"},
		{lxphost.Code(-737), "LXP_ARBITER_REQUIRED", "arbiter required"},
		{lxphost.Code(-738), "LXP_UNAUTHORIZED_ARBITER", "unauthorized arbiter"},
		{lxphost.Code(-739), "LXP_ESCROW_LINK_MISMATCH", "escrow link mismatch"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			for name, err := range map[string]error{
				"direct":         tc.err,
				"stdlib_wrapped": fmt.Errorf("layerxpay call: %w", tc.err),
				"sdk_wrapped":    sdkerrors.Wrap(tc.err, "layerxpay call"),
				"nested_wrapped": fmt.Errorf("executor: %w", sdkerrors.Wrap(tc.err, "layerxpay call")),
			} {
				t.Run(name, func(t *testing.T) {
					assertPayRevert(t, err, tc.code, tc.message)
				})
			}
		})
	}
	t.Run("unregistered_errors", func(t *testing.T) {
		for _, err := range []error{
			errors.New("pay execution failed"),
			errors.New("LXP_PAY_UNBOUND_CALLER"),
			errors.New("LXP_PAUSED_SCOPE"),
			sdkerrors.New("other", 4, "LXP_PAY_UNBOUND_CALLER"),
			sdkerrors.New(paytypes.ModuleName, 99, "LXP_PAY_UNBOUND_CALLER"),
		} {
			assertPayRevert(t, err, "LXP_UNKNOWN", "unknown")
		}
	})
	require.Nil(t, RevertError(nil))
}
