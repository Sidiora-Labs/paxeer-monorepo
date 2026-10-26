package layerxledger

import (
	"errors"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

func KernelCodeText(code lxphost.Code) string {
	return pcommon.LXPCodeName(int32(code))
}

func RevertError(err error) error {
	if err == nil {
		return nil
	}
	var code string
	var registered *sdkerrors.Error
	if errors.As(err, &registered) && registered.Codespace() == types.ModuleName {
		code = registered.Error()
	} else {
		var result lxphost.Code
		if !errors.As(err, &result) {
			return err
		}
		code = KernelCodeText(result)
	}
	return revertError{cause: err, code: code}
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
