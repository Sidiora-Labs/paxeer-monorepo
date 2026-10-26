package types_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

func TestPayErrorsRegisteredInCodespace(t *testing.T) {
	for _, tc := range []struct {
		err  *sdkerrors.Error
		code uint32
		text string
	}{
		{paytypes.ErrInvalidGenesis, 3, "invalid genesis"},
		{paytypes.ErrUnboundCaller, 4, "LXP_PAY_UNBOUND_CALLER"},
		{paytypes.ErrFamilyDisabled, 5, "LXP_MODULE_DISABLED"},
		{paytypes.ErrKernelGateMismatch, 6, "LXP_PAY_KERNEL_GATE_MISMATCH"},
	} {
		t.Run(tc.text, func(t *testing.T) {
			if got := tc.err.Codespace(); got != "layerxpay" {
				t.Errorf("codespace = %q, want layerxpay", got)
			}
			if got := tc.err.ABCICode(); got != tc.code {
				t.Errorf("ABCI code = %d, want %d", got, tc.code)
			}
			if got := tc.err.Error(); got != tc.text {
				t.Errorf("description = %q, want %q", got, tc.text)
			}
			if err := sdkerrors.ABCIError("layerxpay", tc.code, "remote refusal"); !errors.Is(err, tc.err) {
				t.Errorf("registered ABCI error = %v, want %v", err, tc.err)
			}
		})
	}
}

func TestPayRevertCodeMapsToRegistry(t *testing.T) {
	for _, tc := range []struct {
		err  *sdkerrors.Error
		code string
	}{
		{paytypes.ErrUnboundCaller, "LXP_PAY_UNBOUND_CALLER"},
		{paytypes.ErrFamilyDisabled, "LXP_MODULE_DISABLED"},
		{paytypes.ErrKernelGateMismatch, "LXP_PAY_KERNEL_GATE_MISMATCH"},
	} {
		t.Run(tc.code, func(t *testing.T) {
			for name, err := range map[string]error{
				"direct":         tc.err,
				"sdk_wrapped":    tc.err.Wrap("pay operation"),
				"stdlib_wrapped": fmt.Errorf("pay operation: %w", tc.err),
				"nested_wrapped": fmt.Errorf("execution: %w", tc.err.Wrap("pay operation")),
			} {
				t.Run(name, func(t *testing.T) {
					code := paytypes.PayRevertCode(err)
					if code != tc.code {
						t.Fatalf("PayRevertCode(%v) = %q, want %q", err, code, tc.code)
					}
					reason, unpackErr := abi.UnpackRevert(pcommon.LXPRevert(code, "pay operation"))
					if unpackErr != nil {
						t.Fatalf("decode revert: %v", unpackErr)
					}
					if !strings.HasPrefix(reason, tc.code+": ") || strings.HasPrefix(reason, "LXP_UNKNOWN") {
						t.Errorf("revert reason = %q, want prefix %q", reason, tc.code+": ")
					}
				})
			}
		})
	}
	for name, err := range map[string]error{
		"nil":                    nil,
		"genesis":                paytypes.ErrInvalidGenesis,
		"wrapped_genesis":        fmt.Errorf("initialization: %w", paytypes.ErrInvalidGenesis.Wrap("invalid records")),
		"unrelated":              errors.New("unrelated error"),
		"matching_unbound_text":  errors.New("LXP_PAY_UNBOUND_CALLER"),
		"matching_disabled_text": errors.New("LXP_MODULE_DISABLED"),
		"matching_gate_text":     errors.New("LXP_PAY_KERNEL_GATE_MISMATCH"),
		"other_codespace":        sdkerrors.New("other", 4, "LXP_PAY_UNBOUND_CALLER"),
		"other_code":             sdkerrors.New(paytypes.ModuleName, 7, "LXP_PAY_UNBOUND_CALLER"),
	} {
		t.Run(name, func(t *testing.T) {
			if got := paytypes.PayRevertCode(err); got != "" {
				t.Errorf("PayRevertCode(%v) = %q, want empty", err, got)
			}
		})
	}
}
