package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

const (
	assetMintActivity uint32 = 0x0001000a
	assetBurnActivity uint32 = 0x0001000b
)

func (k *Keeper) Mint(ctx sdk.Context, actor Actor, assetID, toAccount [32]byte, amount sdk.Int, idempotencyKey [32]byte) (Result, error) {
	if err := k.checkPause(ctx, "mint"); err != nil {
		return Result{}, err
	}
	payload, err := types.EncodeSupplyPayload(assetID, toAccount, amount)
	if err != nil {
		return Result{}, err
	}
	return k.executeSupplyActivity(ctx, actor, assetMintActivity, payload, idempotencyKey)
}

func (k *Keeper) Burn(ctx sdk.Context, actor Actor, assetID, fromAccount [32]byte, amount sdk.Int, idempotencyKey [32]byte) (Result, error) {
	if err := k.checkPause(ctx, "burn"); err != nil {
		return Result{}, err
	}
	payload, err := types.EncodeSupplyPayload(assetID, fromAccount, amount)
	if err != nil {
		return Result{}, err
	}
	return k.executeSupplyActivity(ctx, actor, assetBurnActivity, payload, idempotencyKey)
}

// executeSupplyActivity commits kernel and bank transfers together. A rejected
// kernel activity still records its refusal, while a failed mirror publishes
// neither the accepted kernel transfer nor any bank state or events.
func (k *Keeper) executeSupplyActivity(ctx sdk.Context, actor Actor, activityType uint32, payload []byte, idempotencyKey [32]byte) (Result, error) {
	cached, write := ctx.CacheContext()
	result, err := k.execute(cached, actor, activityType, payload, idempotencyKey)
	if err != nil {
		write()
		ctx.EventManager().EmitEvents(cached.EventManager().Events())
		return result, err
	}
	if err := k.Mirror(cached, activityType, result.Receipt); err != nil {
		return result, err
	}
	if _, err := LedgerEvents(activityType, result.Receipt, EventArgs{}); err != nil {
		return result, err
	}
	write()
	ctx.EventManager().EmitEvents(cached.EventManager().Events())
	return result, nil
}
