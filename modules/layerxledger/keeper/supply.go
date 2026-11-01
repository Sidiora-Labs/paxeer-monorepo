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
	result, err := k.execute(ctx, actor, assetMintActivity, payload, idempotencyKey)
	if err != nil {
		return result, err
	}
	if err := k.Mirror(ctx, assetMintActivity, result.Receipt); err != nil {
		return result, err
	}
	_, err = LedgerEvents(assetMintActivity, result.Receipt, EventArgs{})
	return result, err
}

func (k *Keeper) Burn(ctx sdk.Context, actor Actor, assetID, fromAccount [32]byte, amount sdk.Int, idempotencyKey [32]byte) (Result, error) {
	if err := k.checkPause(ctx, "burn"); err != nil {
		return Result{}, err
	}
	payload, err := types.EncodeSupplyPayload(assetID, fromAccount, amount)
	if err != nil {
		return Result{}, err
	}
	result, err := k.execute(ctx, actor, assetBurnActivity, payload, idempotencyKey)
	if err != nil {
		return result, err
	}
	if err := k.Mirror(ctx, assetBurnActivity, result.Receipt); err != nil {
		return result, err
	}
	_, err = LedgerEvents(assetBurnActivity, result.Receipt, EventArgs{})
	return result, err
}
