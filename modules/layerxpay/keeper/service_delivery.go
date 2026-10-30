package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

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
