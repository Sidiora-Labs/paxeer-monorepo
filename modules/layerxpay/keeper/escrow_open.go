package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

type EscrowOpenArgs struct {
	EscrowID           [32]byte
	Owner              [32]byte
	EscrowAccount      [32]byte
	Beneficiary        [32]byte
	Arbiter            [32]byte
	AssetID            [32]byte
	Amount             codec.U128
	ExpiryMs           uint64
	DisputeWindowMs    uint64
	TermsHash          [32]byte
	AgreementReference [32]byte
}

func (k Keeper) EscrowOpen(ctx sdk.Context, caller common.Address, a EscrowOpenArgs) (lxphost.Receipt, error) {
	payload := types.EncodeEscrowOpen(a.EscrowID, a.Owner, a.EscrowAccount, a.Beneficiary,
		a.Arbiter, a.AssetID, a.Amount, a.ExpiryMs, a.DisputeWindowMs, a.TermsHash, a.AgreementReference)
	return k.Submit(ctx, caller, "escrowOpen", payload, a.EscrowID)
}

func (k Keeper) EscrowCapture(ctx sdk.Context, caller common.Address, escrowID [32]byte, amount codec.U128, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	return k.Submit(ctx, caller, "escrowCapture", types.EncodeEscrowCapture(escrowID, amount, idempotencyKey), idempotencyKey)
}

func (k Keeper) EscrowPartialCapture(ctx sdk.Context, caller common.Address, escrowID [32]byte, amount codec.U128, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	return k.Submit(ctx, caller, "escrowPartialCapture", types.EncodeEscrowCapture(escrowID, amount, idempotencyKey), idempotencyKey)
}
