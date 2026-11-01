package keeper

import (
	"math"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func (k Keeper) ServiceApplyDefaultOutcomes(ctx sdk.Context) (lxphost.Receipt, error) {
	const method = "serviceApplyDefaultOutcomes"
	if !k.GetParams(ctx).ServiceEnabled {
		return lxphost.Receipt{}, types.ErrFamilyDisabled.Wrap(method)
	}
	sequence, err := lxphost.NextSequence(ctx, k.hostKey)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	if sequence == 0 || sequence == math.MaxUint64 {
		return lxphost.Receipt{}, submitCodeError(method, lxphost.Code(-307))
	}
	branch, commit := ctx.CacheContext()
	receipt, err := k.host.ServiceApplyDefaultOutcomes(lxphost.WithSequence(branch, sequence))
	if err != nil {
		return receipt, err
	}
	if receipt.ResultCode != 0 {
		return receipt, submitCodeError(method, receipt.ResultCode)
	}
	commit()
	return receipt, nil
}

func (k Keeper) ServiceToolExecAttest(ctx sdk.Context, caller common.Address, execution []byte, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeServiceToolExecAttest(execution)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "serviceToolExecAttest", payload, idempotencyKey)
}

func (k Keeper) ServiceProgressReport(ctx sdk.Context, caller common.Address, reportID, commitmentID, noteHash, availabilityReference [32]byte, progressBPS uint32, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeServiceProgress(reportID, commitmentID, noteHash, availabilityReference, progressBPS)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "serviceProgressReport", payload, idempotencyKey)
}

func (k Keeper) ServiceDeliver(ctx sdk.Context, caller common.Address, deliveryID, agreementID [32]byte, deliverables []types.ServiceDeliverable, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeServiceDeliver(deliveryID, agreementID, deliverables)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "serviceDeliver", payload, idempotencyKey)
}

func (k Keeper) ServiceAccept(ctx sdk.Context, caller common.Address, agreementID, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeServiceIdentifier(agreementID)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "serviceAccept", payload, idempotencyKey)
}

func (k Keeper) ServiceReject(ctx sdk.Context, caller common.Address, agreementID [32]byte, reason uint16, contestedHashes [][32]byte, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeServiceReject(agreementID, reason, contestedHashes)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "serviceReject", payload, idempotencyKey)
}
