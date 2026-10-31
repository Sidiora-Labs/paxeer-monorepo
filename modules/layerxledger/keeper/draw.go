package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type DrawArgs = types.DrawArgs

func (k *Keeper) Draw(ctx sdk.Context, actor Actor, args types.DrawArgs) (Result, error) {
	if err := k.checkPause(ctx, "draw"); err != nil {
		return Result{}, err
	}
	if !actor.Authority.Bound || actor.DIDKey != actor.Authority.VerifiedKey {
		return Result{}, types.ErrUnauthorizedDebit
	}
	return Result{}, types.ErrNoPayerGrant
}
