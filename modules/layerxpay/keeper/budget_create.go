package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

type BudgetCreateArgs struct {
	BudgetID           [32]byte
	BudgetAccount      [32]byte
	AssetID            [32]byte
	PurposeHash        [32]byte
	PerPeriodLimit     codec.U128
	CarryCap           codec.U128
	Amount             codec.U128
	PeriodLengthMs     uint64
	PeriodStartMs      uint64
	ExpiryMs           uint64
	RevocationSequence uint64
	RolloverPolicy     uint8
}

func (k Keeper) BudgetCreate(ctx sdk.Context, caller common.Address, a BudgetCreateArgs) (lxphost.Receipt, error) {
	payload, err := types.EncodeBudgetCreate(a.BudgetID, a.BudgetAccount, a.AssetID, a.PurposeHash,
		a.PerPeriodLimit, a.CarryCap, a.Amount, a.PeriodLengthMs, a.PeriodStartMs,
		a.ExpiryMs, a.RevocationSequence, a.RolloverPolicy)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "budgetCreate", payload, a.BudgetID)
}

func (k Keeper) BudgetCreateNative(ctx sdk.Context, caller common.Address, a BudgetCreateArgs, sourceAccount [32]byte, sourceSequence uint64) (lxphost.Receipt, error) {
	payload, err := types.EncodeBudgetCreateNative(a.BudgetID, a.BudgetAccount, a.AssetID, a.PurposeHash,
		a.PerPeriodLimit, a.CarryCap, a.Amount, a.PeriodLengthMs, a.PeriodStartMs,
		a.ExpiryMs, a.RevocationSequence, a.RolloverPolicy, sourceAccount, sourceSequence)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "budgetCreateNative", payload, a.BudgetID)
}

func (k Keeper) BudgetFund(ctx sdk.Context, caller common.Address, budgetID [32]byte, amount codec.U128, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeBudgetFund(budgetID, amount)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "budgetFund", payload, idempotencyKey)
}

func (k Keeper) BudgetFundNative(ctx sdk.Context, caller common.Address, budgetID [32]byte, amount codec.U128, sourceSequence uint64, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeBudgetFundNative(budgetID, amount, sourceSequence)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "budgetFundNative", payload, idempotencyKey)
}

func (k Keeper) BudgetAmend(ctx sdk.Context, caller common.Address, budgetID [32]byte, perPeriodLimit, carryCap codec.U128, expiryMs uint64, rolloverPolicy uint8, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeBudgetAmend(budgetID, perPeriodLimit, carryCap, expiryMs, rolloverPolicy)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "budgetAmend", payload, idempotencyKey)
}
