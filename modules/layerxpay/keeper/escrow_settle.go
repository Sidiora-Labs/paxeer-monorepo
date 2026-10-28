package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func (k Keeper) EscrowRelease(ctx sdk.Context, caller common.Address, escrowID, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	return k.Submit(ctx, caller, "escrowRelease", types.EncodeEscrowRelease(escrowID, idempotencyKey), idempotencyKey)
}

func (k Keeper) EscrowTimeout(ctx sdk.Context, caller common.Address, escrowID, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	return k.Submit(ctx, caller, "escrowTimeout", types.EncodeEscrowRelease(escrowID, idempotencyKey), idempotencyKey)
}

func (k Keeper) EscrowDisputeOpen(ctx sdk.Context, caller common.Address, escrowID, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	return k.Submit(ctx, caller, "escrowDisputeOpen", types.EncodeEscrowDisputeOpen(escrowID), idempotencyKey)
}

func (k Keeper) EscrowDisputeResolve(ctx sdk.Context, caller common.Address, escrowID [32]byte, beneficiaryBps uint32, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	return k.Submit(ctx, caller, "escrowDisputeResolve", types.EncodeEscrowDisputeResolve(escrowID, beneficiaryBps, idempotencyKey), idempotencyKey)
}
