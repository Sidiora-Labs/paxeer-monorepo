package types

import (
	"errors"

	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

var (
	ErrUnboundCaller      = sdkerrors.Register(ModuleName, 4, "LXP_PAY_UNBOUND_CALLER")
	ErrFamilyDisabled     = sdkerrors.Register(ModuleName, 5, "LXP_MODULE_DISABLED")
	ErrKernelGateMismatch = sdkerrors.Register(ModuleName, 6, "LXP_PAY_KERNEL_GATE_MISMATCH")
)

func PayRevertCode(err error) string {
	switch {
	case errors.Is(err, ErrUnboundCaller):
		return "LXP_PAY_UNBOUND_CALLER"
	case errors.Is(err, ErrFamilyDisabled):
		return "LXP_MODULE_DISABLED"
	case errors.Is(err, ErrKernelGateMismatch):
		return "LXP_PAY_KERNEL_GATE_MISMATCH"
	default:
		return ""
	}
}
