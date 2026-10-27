package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func (k *Keeper) GrantSession(ctx sdk.Context, actor Actor, grant []byte, expirySequence uint64, actionKey, idempotencyKey [32]byte) (Result, error) {
	if err := k.checkPause(ctx, "grantSession"); err != nil {
		return Result{}, err
	}
	payload, err := types.EncodeSessionPayload(grant, expirySequence, actionKey, nil)
	if err != nil {
		return Result{}, err
	}
	result, err := k.execute(ctx, actor, payload, idempotencyKey)
	if err != nil {
		return result, err
	}
	_, err = AuthEvents(result.Receipt)
	return result, err
}

func (k *Keeper) GrantSessionSuccessor(ctx sdk.Context, actor Actor, grant []byte, expirySequence uint64, actionKey, predecessorID, expectedCommitment, idempotencyKey [32]byte) (Result, error) {
	if err := k.checkPause(ctx, "grantSessionSuccessor"); err != nil {
		return Result{}, err
	}
	payload, err := types.EncodeSessionPayload(grant, expirySequence, actionKey, &types.SessionSuccessor{
		PredecessorID: predecessorID, ExpectedCommitment: expectedCommitment,
	})
	if err != nil {
		return Result{}, err
	}
	result, err := k.execute(ctx, actor, payload, idempotencyKey)
	if err != nil {
		return result, err
	}
	_, err = AuthEvents(result.Receipt)
	return result, err
}
