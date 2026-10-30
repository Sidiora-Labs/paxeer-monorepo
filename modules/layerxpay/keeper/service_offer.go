package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func (k Keeper) ServiceOfferPublish(ctx sdk.Context, caller common.Address, offerID, assetID [32]byte, price codec.U128, termsHash, deliverableSpecHash [32]byte, deliveryDeadline, acceptanceWindow, disputeWindow uint64, defaultOutcome uint8, offerExpiry uint64) (lxphost.Receipt, error) {
	payload, err := types.EncodeServiceOffer(offerID, assetID, price, termsHash, deliverableSpecHash, deliveryDeadline, acceptanceWindow, disputeWindow, defaultOutcome, offerExpiry)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "serviceOfferPublish", payload, offerID)
}

func (k Keeper) ServiceOfferWithdraw(ctx sdk.Context, caller common.Address, offerID [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeServiceIdentifier(offerID)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "serviceOfferWithdraw", payload, offerID)
}
