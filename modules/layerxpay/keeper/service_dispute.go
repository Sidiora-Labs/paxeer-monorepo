package keeper

import (
	"crypto/sha256"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

const (
	serviceArbiterRequired     lxphost.Code = -737
	serviceUnauthorizedArbiter lxphost.Code = -738
	serviceEscrowLinkMismatch  lxphost.Code = -739
)

func (k Keeper) serviceDisputeEscrow(ctx sdk.Context, agreementID [32]byte, method string) (types.EscrowRecord, error) {
	agreement, found := k.GetAgreement(ctx, agreementID)
	if !found {
		return types.EscrowRecord{}, submitCodeError(method, lxphost.Code(-706))
	}
	escrow, found := k.GetEscrow(ctx, agreement.EscrowID)
	if !found || escrow.EscrowID != agreement.EscrowID || escrow.Owner != agreement.Buyer || escrow.Beneficiary != agreement.Provider {
		return types.EscrowRecord{}, submitCodeError(method, serviceEscrowLinkMismatch)
	}
	offer, found := k.GetOffer(ctx, agreement.OfferID)
	if !found || escrow.AssetID != offer.AssetID {
		return types.EscrowRecord{}, submitCodeError(method, serviceEscrowLinkMismatch)
	}
	if escrow.Arbiter == ([32]byte{}) {
		return types.EscrowRecord{}, submitCodeError(method, serviceArbiterRequired)
	}
	return escrow, nil
}

func (k Keeper) ServiceDisputeOpen(ctx sdk.Context, caller common.Address, disputeID, agreementID [32]byte, evidenceHashes [][32]byte) (lxphost.Receipt, error) {
	const method = "serviceDisputeOpen"
	if err := k.CheckWrite(ctx, method); err != nil {
		return lxphost.Receipt{}, err
	}
	if _, err := k.AuthorityFor(ctx, caller); err != nil {
		return lxphost.Receipt{}, err
	}
	payload, err := types.EncodeServiceDisputeOpen(disputeID, agreementID, evidenceHashes)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	if _, err := k.serviceDisputeEscrow(ctx, agreementID, method); err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, method, payload, disputeID)
}

func (k Keeper) ServiceDisputeResolve(ctx sdk.Context, caller common.Address, disputeID [32]byte, ruling uint16, providerBPS uint32, escrowResolutionID [32]byte) (lxphost.Receipt, error) {
	const method = "serviceDisputeResolve"
	if err := k.CheckWrite(ctx, method); err != nil {
		return lxphost.Receipt{}, err
	}
	authority, err := k.AuthorityFor(ctx, caller)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	dispute, found := k.GetDispute(ctx, disputeID)
	if !found {
		return lxphost.Receipt{}, submitCodeError(method, lxphost.Code(-706))
	}
	escrow, err := k.serviceDisputeEscrow(ctx, dispute.AgreementID, method)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	_, principal, err := lxphost.MainAccountID(authority.AuthorityKey)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	if principal != escrow.Arbiter {
		return lxphost.Receipt{}, submitCodeError(method, serviceUnauthorizedArbiter)
	}
	if escrowResolutionID != escrow.EscrowID {
		return lxphost.Receipt{}, submitCodeError(method, serviceEscrowLinkMismatch)
	}
	payload, err := types.EncodeServiceDisputeResolve(disputeID, ruling, providerBPS, escrowResolutionID)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	if err := k.CheckWrite(ctx, "escrowDisputeResolve"); err != nil {
		return lxphost.Receipt{}, err
	}
	branch, commit := ctx.CacheContext()
	receipt, err := k.Submit(branch, caller, method, payload, sha256.Sum256(payload))
	if err != nil {
		return receipt, err
	}
	escrowKey := sha256.Sum256(append([]byte("layerxpay:serviceDisputeResolve:escrow:"), payload...))
	if settlement, err := k.EscrowDisputeResolve(branch, caller, escrow.EscrowID, providerBPS, escrowKey); err != nil {
		return settlement, err
	}
	commit()
	return receipt, nil
}
