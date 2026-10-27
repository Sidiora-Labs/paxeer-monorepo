package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

const (
	payerGrantIssueActivity  uint32 = 0x00010007
	payerGrantRevokeActivity uint32 = 0x00010008
)

// IssueGrant validates and issues a payer's signed grant through the kernel.
func (k *Keeper) IssueGrant(ctx sdk.Context, actor Actor, grant []byte, idempotencyKey [32]byte) (Result, error) {
	if err := k.checkPause(ctx, "issueGrant"); err != nil {
		return Result{}, err
	}
	if err := types.ValidateIssueGrantPayload(grant); err != nil {
		return Result{}, err
	}
	return k.executePayerGrant(ctx, actor, payerGrantIssueActivity, grant, idempotencyKey)
}

// RevokeGrant revokes a payer grant, including while the ledger is paused.
func (k *Keeper) RevokeGrant(ctx sdk.Context, actor Actor, grantID [32]byte, revocationSequence uint64, idempotencyKey [32]byte) (Result, error) {
	if err := k.checkPause(ctx, "revokeGrant"); err != nil {
		return Result{}, err
	}
	payload := types.EncodeRevokeGrantPayload(grantID, revocationSequence)
	return k.executePayerGrant(ctx, actor, payerGrantRevokeActivity, payload, idempotencyKey)
}

func (k *Keeper) executePayerGrant(ctx sdk.Context, actor Actor, activityType uint32, payload []byte, idempotencyKey [32]byte) (Result, error) {
	cache, write := ctx.CacheContext()
	result, err := k.execute(cache, actor, activityType, payload, idempotencyKey)
	if err != nil {
		// Kernel refusals retain their receipt and sequence, just as execute does.
		write()
		return result, err
	}
	if err := k.Mirror(cache, activityType, result.Receipt); err != nil {
		return result, err
	}
	// Validate the ABI event projection before committing, retaining the raw
	// receipt events required by Result for the caller's log encoding.
	if _, err := LedgerEvents(activityType, result.Receipt, EventArgs{}); err != nil {
		return result, err
	}
	write()
	return result, nil
}
