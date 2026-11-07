package layerxauth

import (
	"errors"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
)

func KernelCodeText(code lxphost.Code) string {
	return pcommon.LXPCodeName(int32(code))
}

// RevertError exposes the canonical LXP name of a registered auth, governance,
// or kernel error while retaining its identity through Unwrap and Cause.
// Unknown Go errors pass through for the precompile's LXPReason transport.
func RevertError(err error) error {
	if err == nil {
		return nil
	}
	for _, registered := range []error{
		types.ErrAuthScope,
		types.ErrAuthRevoked,
		types.ErrAuthExpired,
		types.ErrAuthNotMature,
		types.ErrNotYetValid,
		types.ErrSequenceReused,
		types.ErrNonCanonical,
		types.ErrUnauthorizedDebit,
		types.ErrIdempotentReplay,
		govtypes.ErrPausedScope,
	} {
		if errors.Is(err, registered) {
			return revertError{cause: err, code: registered.Error()}
		}
	}
	var code lxphost.Code
	if !errors.As(err, &code) {
		return err
	}
	return revertError{cause: err, code: KernelCodeText(code)}
}

type revertError struct {
	cause error
	code  string
}

// Error returns the code consumers match; diagnostic context stays in the cause.
// The precompile packs this text as Solidity Error(string) through LXPReason.
func (e revertError) Error() string {
	return e.code
}

func (e revertError) Unwrap() error {
	return e.cause
}

func (e revertError) Cause() error {
	return e.cause
}
