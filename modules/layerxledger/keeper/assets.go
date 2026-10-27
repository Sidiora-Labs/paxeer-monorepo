package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

const (
	assetRegisterActivity uint32 = 0x00010001
	assetPauseActivity    uint32 = 0x00010002
	assetUnpauseActivity  uint32 = 0x00010003
)

func (k *Keeper) RegisterAsset(ctx sdk.Context, actor Actor, args types.RegisterArgs, idempotencyKey [32]byte) (Result, error) {
	if err := k.checkPause(ctx, "register"); err != nil {
		return Result{}, err
	}
	payload, err := types.EncodeRegisterPayload(args)
	if err != nil {
		return Result{}, err
	}
	return k.executeAssetActivity(ctx, actor, assetRegisterActivity, payload, idempotencyKey)
}

func (k *Keeper) PauseAsset(ctx sdk.Context, actor Actor, assetID, idempotencyKey [32]byte) (Result, error) {
	if err := k.checkPause(ctx, "pause"); err != nil {
		return Result{}, err
	}
	return k.executeAssetActivity(ctx, actor, assetPauseActivity, types.EncodePausePayload(assetID), idempotencyKey)
}

func (k *Keeper) UnpauseAsset(ctx sdk.Context, actor Actor, assetID, idempotencyKey [32]byte) (Result, error) {
	if err := k.checkPause(ctx, "unpause"); err != nil {
		return Result{}, err
	}
	return k.executeAssetActivity(ctx, actor, assetUnpauseActivity, types.EncodePausePayload(assetID), idempotencyKey)
}

func (k *Keeper) executeAssetActivity(ctx sdk.Context, actor Actor, activityType uint32, payload []byte, idempotencyKey [32]byte) (Result, error) {
	result, err := k.execute(ctx, actor, activityType, payload, idempotencyKey)
	if err != nil {
		return result, err
	}
	if err := k.Mirror(ctx, activityType, result.Receipt); err != nil {
		return result, err
	}
	return result, nil
}
