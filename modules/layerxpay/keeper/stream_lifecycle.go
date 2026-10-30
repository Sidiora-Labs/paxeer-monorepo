package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func init() {
	types.MethodActivity["streamRateChange"] = types.KernelActivity{
		Type: types.ActivityStreamRateChange, Module: types.StreamModuleID,
		PayloadBytes: types.StreamRateChangePayloadBytes,
	}
}

func (k Keeper) StreamPause(ctx sdk.Context, caller common.Address, streamID, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeStreamID(streamID)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "streamPause", payload, idempotencyKey)
}

func (k Keeper) StreamResume(ctx sdk.Context, caller common.Address, streamID, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeStreamID(streamID)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "streamResume", payload, idempotencyKey)
}

func (k Keeper) StreamRateChange(ctx sdk.Context, caller common.Address, streamID [32]byte, newRate codec.U128, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeStreamRateChange(streamID, newRate, idempotencyKey)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "streamRateChange", payload, idempotencyKey)
}
