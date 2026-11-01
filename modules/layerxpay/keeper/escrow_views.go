package keeper

import (
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k Keeper) GetEscrow(ctx sdk.Context, escrowID [32]byte) (types.EscrowRecord, bool) {
	key := append([]byte("hold:"), escrowID[:]...)
	data := ctx.KVStore(k.hostKey).Get(types.KernelKVKey(types.EscrowModuleID, key))
	if data == nil {
		return types.EscrowRecord{}, false
	}
	record, err := types.DecodeEscrowRecord(data)
	if err != nil {
		panic(fmt.Errorf("layerxpay: decode escrow record: %w", err))
	}
	return record, true
}

func (k Keeper) EscrowRemaining(ctx sdk.Context, escrowID [32]byte) (codec.U128, bool) {
	record, found := k.GetEscrow(ctx, escrowID)
	return record.LockedAmount, found
}

func (k Keeper) GetEscrowResult(ctx sdk.Context, idempotencyKey [32]byte) (types.EscrowResult, bool) {
	key := append([]byte("result:"), idempotencyKey[:]...)
	data := ctx.KVStore(k.hostKey).Get(types.KernelKVKey(types.EscrowModuleID, key))
	if data == nil {
		return types.EscrowResult{}, false
	}
	result, err := types.DecodeEscrowResult(data)
	if err != nil {
		panic(fmt.Errorf("layerxpay: decode escrow result: %w", err))
	}
	return result, true
}
