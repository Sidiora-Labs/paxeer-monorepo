package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
)

var kernelErrors = map[lxphost.Code]error{
	-204: types.ErrAuthScope,
	-203: types.ErrAuthRevoked,
	-202: types.ErrAuthExpired,
	-222: types.ErrAuthNotMature,
	-304: types.ErrNotYetValid,
	-301: types.ErrSequenceReused,
	-3:   types.ErrNonCanonical,
	-207: types.ErrUnauthorizedDebit,
	-302: types.ErrIdempotentReplay,
}

func FromKernel(code lxphost.Code) error {
	if code == 0 {
		return nil
	}
	if err, ok := kernelErrors[code]; ok {
		return err
	}
	return code
}
