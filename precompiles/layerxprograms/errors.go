package layerxprograms

import (
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
)

type refusalCode string

func (e refusalCode) Error() string { return string(e) }

const (
	ErrModuleDisabled refusalCode = "LXP_MODULE_DISABLED: module disabled"
	ErrUnboundCaller  refusalCode = "LXP_ADDR_EVM_NOT_BOUND: EVM address is not bound to a LayerX DID"
	ErrReadOnlyWrite  refusalCode = "LXP_PROGRAM_REFUSED: program refused; write under staticcall"
)

func kernelError(code lxphost.Code, op string) error {
	if code == 0 {
		return nil
	}
	name := pcommon.LXPCodeName(int32(code))
	// Fatal kernel results halt execution and must never become EVM reverts.
	if name == "" {
		panic(code)
	}
	return fmt.Errorf("%s: %s: %w", name, op, code)
}
