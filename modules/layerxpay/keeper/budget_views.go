package keeper

import (
	"fmt"
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
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
	now := ctx.BlockTime().UnixMilli()
	if now < 0 {
		return sdk.ZeroInt(), lxphost.Code(-802)
	}
	if record.Revoked || record.Closed || record.ExpiryMs <= uint64(now) {
		return sdk.ZeroInt(), nil
	}
	record, err = projectBudgetPeriod(record, uint64(now))
	if err != nil {
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

// projectBudgetPeriod mirrors lx_budget_rollover on a detached record. Skipped
// periods advance the clock once; they do not accumulate additional carry.
func projectBudgetPeriod(record types.BudgetRecord, timestamp uint64) (types.BudgetRecord, error) {
	if record.PeriodLengthMs == 0 {
		return types.BudgetRecord{}, lxphost.Code(-213)
	}
	if timestamp < record.PeriodStartMs {
		return types.BudgetRecord{}, lxphost.Code(-802)
	}
	periods := (timestamp - record.PeriodStartMs) / record.PeriodLengthMs
	if periods == 0 {
		return record, nil
	}
	unspent, ok := record.PerPeriodLimit.CheckedSub(record.SpentThisPeriod)
	if !ok {
		return types.BudgetRecord{}, lxphost.Code(-1001)
	}
	configured := record.ConfiguredPeriodLimit
	if configured.IsZero() {
		configured = record.PerPeriodLimit
	}
	carry := codec.U128{}
	if record.RolloverPolicy == 2 {
		carry = unspent
		if carry.Hi > record.CarryCap.Hi || (carry.Hi == record.CarryCap.Hi && carry.Lo > record.CarryCap.Lo) {
			carry = record.CarryCap
		}
	}
	limit, ok := configured.CheckedAdd(carry)
	if !ok {
		return types.BudgetRecord{}, lxphost.Code(-500)
	}
	// The advance is bounded by timestamp - PeriodStartMs, so neither
	// multiplication nor addition can overflow uint64.
	record.PeriodStartMs += periods * record.PeriodLengthMs
	record.SpentThisPeriod = codec.U128{}
	record.Carried = carry
	record.PerPeriodLimit = limit
	return record, nil
}
