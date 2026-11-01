package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func (k Keeper) ServiceAgreementPropose(ctx sdk.Context, caller common.Address, agreementID, offerID, termsHash, escrowID, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeServicePropose(agreementID, offerID, termsHash, escrowID)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "serviceAgreementPropose", payload, idempotencyKey)
}

func (k Keeper) ServiceAgreementAccept(ctx sdk.Context, caller common.Address, agreementID, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeServiceIdentifier(agreementID)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "serviceAgreementAccept", payload, idempotencyKey)
}

func (k Keeper) ServiceCommitTask(ctx sdk.Context, caller common.Address, commitmentID, agreementID, taskHash, escrowID [32]byte, deadline, resourceBound uint64, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeServiceCommit(commitmentID, agreementID, taskHash, escrowID, deadline, resourceBound)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "serviceCommitTask", payload, idempotencyKey)
}

func (k Keeper) ServiceCommitAbandon(ctx sdk.Context, caller common.Address, commitmentID [32]byte, reason uint16, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeServiceAbandon(commitmentID, reason)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "serviceCommitAbandon", payload, idempotencyKey)
}
