package layerxauth

import (
	"errors"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
)

func KernelCodeText(code lxphost.Code) string {
	return pcommon.LXPCodeName(int32(code))
}

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
		types.ErrPausedScope,
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

func (e revertError) Error() string {
	return e.code
}

func (e revertError) Unwrap() error {
	return e.cause
}

func (e revertError) Cause() error {
	return e.cause
}
