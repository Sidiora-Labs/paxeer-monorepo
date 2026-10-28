package keeper

import (
	"fmt"
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k Keeper) GetBudget(ctx sdk.Context, budgetID [32]byte) (types.BudgetRecord, bool, error) {
	var key [39]byte
	copy(key[:7], "budget:")
	copy(key[7:], budgetID[:])
	data := ctx.KVStore(k.hostKey).Get(types.KernelKVKey(types.BudgetModuleID, key[:]))
	if data == nil {
		return types.BudgetRecord{}, false, nil
	}
	record, err := types.DecodeBudgetRecord(data)
	if err != nil {
		return types.BudgetRecord{}, false, fmt.Errorf("layerxpay: decode budget: %w", err)
	}
	if record.BudgetID != budgetID {
		return types.BudgetRecord{}, false, lxphost.Code(-213)
	}
	return record, true, nil
}

func (k Keeper) BudgetRemaining(ctx sdk.Context, budgetID [32]byte) (sdk.Int, error) {
	record, found, err := k.GetBudget(ctx, budgetID)
	if err != nil || !found {
		return sdk.ZeroInt(), err
	}
	allowance, ok := record.PerPeriodLimit.CheckedSub(record.SpentThisPeriod)
	if !ok {
		return sdk.ZeroInt(), lxphost.Code(-1001)
	}
	if allowance.IsZero() {
		return sdk.ZeroInt(), nil
	}
	encoded := allowance.Bytes()
	remaining := sdk.NewIntFromBigInt(new(big.Int).SetBytes(encoded[:]))
	balance, err := k.host.ReadBalance(ctx, record.BudgetAccount, record.AssetID)
	if err != nil {
		return sdk.ZeroInt(), err
	}
	if balance.LT(remaining) {
		remaining = balance
	}
	return remaining, nil
}
