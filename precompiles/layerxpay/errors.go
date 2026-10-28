package layerxpay

import (
	"errors"

	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
)

func RevertError(err error) []byte {
	if err == nil {
		return nil
	}
	var result lxphost.Code
	if errors.As(err, &result) {
		return pcommon.LXPRevert(pcommon.LXPCodeName(int32(result)), err.Error())
	}
	if code := paytypes.PayRevertCode(err); code != "" {
		return pcommon.LXPRevert(code, err.Error())
	}
	if errors.Is(err, govtypes.ErrPausedScope) {
		return pcommon.LXPRevert("LXP_PAUSED_SCOPE", err.Error())
	}
	return pcommon.LXPRevert("LXP_UNKNOWN", err.Error())
}
