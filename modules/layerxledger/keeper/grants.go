package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

// IssueGrant refuses the retired payer-grant route (decision 2340).
// No kernel activity, receipt, sequence or mirror state is written.
func (k *Keeper) IssueGrant(ctx sdk.Context, actor Actor, grant []byte, idempotencyKey [32]byte) (Result, error) {
	if err := k.checkPause(ctx, "issueGrant"); err != nil {
		return Result{}, err
	}
	return Result{}, types.ErrNoPayerGrant
}

// RevokeGrant retains its pause exemption but refuses the retired route
// without submitting activity 0x00010008 (decision 2340).
func (k *Keeper) RevokeGrant(ctx sdk.Context, actor Actor, grantID [32]byte, revocationSequence uint64, idempotencyKey [32]byte) (Result, error) {
	if err := k.checkPause(ctx, "revokeGrant"); err != nil {
		return Result{}, err
	}
	return Result{}, types.ErrNoPayerGrant
}
