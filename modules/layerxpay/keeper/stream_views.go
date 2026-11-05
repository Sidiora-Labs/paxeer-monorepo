//go:build cgo

package keeper

/*
#cgo CFLAGS: -I${SRCDIR}/../../../include -I${SRCDIR}/../../../build/generated
#include "layerx/lx_stream.h"
*/
import "C"

import (
	"math/big"
	"unsafe"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func streamViewKey(id [32]byte, economicResult bool) ([]byte, error) {
	var key [C.LX_STREAM_KEY_BYTES]byte
	var status C.lxp_result
	if economicResult {
		status = C.lx_stream_result_key((*C.uint8_t)(unsafe.Pointer(&id[0])), (*C.uint8_t)(unsafe.Pointer(&key[0])))
	} else {
		status = C.lx_stream_state_key((*C.uint8_t)(unsafe.Pointer(&id[0])), (*C.uint8_t)(unsafe.Pointer(&key[0])))
	}
	if status != C.LXP_OK {
		return nil, lxphost.Code(status)
	}
	return types.KernelKVKey(types.StreamModuleID, key[:]), nil
}

func (k Keeper) GetStream(ctx sdk.Context, streamID [32]byte) (types.StreamRecord, error) {
	key, err := streamViewKey(streamID, false)
	if err != nil {
		return types.StreamRecord{}, err
	}
	encoded := ctx.KVStore(k.hostKey).Get(key)
	if encoded == nil {
		return types.StreamRecord{}, lxphost.Code(C.LXP_ERR_UNKNOWN_FIELD)
	}
	record, err := types.DecodeStreamRecord(encoded)
	if err != nil {
		return types.StreamRecord{}, err
	}
	if record.StreamID != streamID {
		return types.StreamRecord{}, lxphost.Code(C.LXP_FATAL_INVARIANT)
	}
	return record, nil
}

func (k Keeper) GetStreamResult(ctx sdk.Context, idempotencyKey [32]byte) (types.StreamResult, error) {
	key, err := streamViewKey(idempotencyKey, true)
	if err != nil {
		return types.StreamResult{}, err
	}
	encoded := ctx.KVStore(k.hostKey).Get(key)
	if encoded == nil {
		return types.StreamResult{}, lxphost.Code(C.LXP_ERR_UNKNOWN_FIELD)
	}
	return types.DecodeStreamResult(encoded)
}

func (k Keeper) StreamAccrued(ctx sdk.Context, streamID [32]byte) (accruedTotal, unsettled sdk.Int, err error) {
	timestamp := ctx.BlockTime().UnixMilli()
	if timestamp < 0 {
		return sdk.ZeroInt(), sdk.ZeroInt(), lxphost.Code(C.LXP_ERR_NON_CANONICAL)
	}
	record, err := k.GetStream(ctx, streamID)
	if err != nil {
		return sdk.ZeroInt(), sdk.ZeroInt(), err
	}
	newlyAccrued := sdk.ZeroInt()
	if record.Mode == types.StreamModeTime {
		newlyAccrued, err = k.host.ReadStreamAccrued(ctx, streamID, uint64(timestamp))
		if err != nil {
			return sdk.ZeroInt(), sdk.ZeroInt(), err
		}
	}
	accruedBytes := record.AccruedTotal.Bytes()
	settledBytes := record.SettledTotal.Bytes()
	accruedTotal = sdk.NewIntFromBigInt(new(big.Int).SetBytes(accruedBytes[:])).Add(newlyAccrued)
	settled := sdk.NewIntFromBigInt(new(big.Int).SetBytes(settledBytes[:]))
	return accruedTotal, accruedTotal.Sub(settled), nil
}
