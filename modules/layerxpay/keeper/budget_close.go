package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func (k Keeper) BudgetClose(ctx sdk.Context, caller common.Address, budgetID [32]byte, revocationSequence uint64, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeBudgetClose(budgetID, revocationSequence)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "budgetClose", payload, idempotencyKey)
}

func (k Keeper) BudgetDefund(ctx sdk.Context, caller common.Address, budgetID [32]byte, amount codec.U128, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeBudgetDefund(budgetID, amount)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "budgetDefund", payload, idempotencyKey)
}

func (k Keeper) BudgetRevoke(ctx sdk.Context, caller common.Address, budgetID [32]byte, revocationSequence uint64, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeBudgetRevoke(budgetID, revocationSequence)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "budgetRevoke", payload, idempotencyKey)
}
