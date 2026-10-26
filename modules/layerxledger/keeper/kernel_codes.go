package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
)

var kernelErrors = map[lxphost.Code]error{
	3:    types.ErrNonCanonical,
	107:  types.ErrMalformedSend,
	207:  types.ErrUnauthorizedDebit,
	213:  types.ErrContextMismatch,
	214:  types.ErrNoPayerGrant,
	302:  types.ErrIdempotentReplay,
	303:  types.ErrExpired,
	305:  types.ErrSequenceMismatch,
	307:  types.ErrSequenceExhausted,
	400:  types.ErrInsufficientBalance,
	401:  types.ErrZeroAmount,
	402:  types.ErrAssetMismatch,
	403:  types.ErrAssetPaused,
	411:  types.ErrAssetAlreadyRegistered,
	735:  types.ErrPausedScope,
	905:  types.ErrArenaExhausted,
	1003: types.ErrFatalSupplyMismatch,
}

func FromKernel(code lxphost.Code) error {
	if code == 0 {
		return nil
	}
	if err, ok := kernelErrors[-code]; ok {
		return err
	}
	return code
}
