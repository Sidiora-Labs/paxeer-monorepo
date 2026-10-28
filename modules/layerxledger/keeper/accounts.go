package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

const accountOpenActivity uint32 = 0x00010004

func AccountID(didKey, assetID [32]byte) ([32]byte, error) {
	_, id, err := lxphost.AssetAccountID(didKey, assetID)
	return id, ledgerReadError(err)
}

func (k *Keeper) OpenAccount(ctx sdk.Context, actor Actor, assetID, idempotencyKey [32]byte) (Result, error) {
	if err := k.checkPause(ctx, "openAccount"); err != nil {
		return Result{}, err
	}
	payload := types.EncodeOpenAccountPayload(assetID)
	result, err := k.execute(ctx, actor, accountOpenActivity, payload, idempotencyKey)
	if err != nil {
		return result, err
	}
	if err := k.Mirror(ctx, accountOpenActivity, result.Receipt); err != nil {
		return result, err
	}
	_, err = LedgerEvents(accountOpenActivity, result.Receipt, EventArgs{})
	return result, err
}
