package keeper

import (
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type Result struct {
	lxphost.Receipt
}

func (k *Keeper) execute(ctx sdk.Context, actor Actor, payload []byte, idempotencyKey [32]byte) (Result, error) {
	if !actor.Authority.Bound {
		return Result{}, types.ErrAuthScope
	}
	if len(payload) < 4 || payload[0] != 0x71 || ctx.BlockTime().UnixMilli() < 0 {
		return Result{}, types.ErrNonCanonical
	}
	var hostKey sdk.StoreKey
	for _, key := range ctx.MultiStore().StoreKeys() {
		if key.Name() == "lxphost" {
			hostKey = key
			break
		}
	}
	if hostKey == nil {
		return Result{}, fmt.Errorf("layerxauth: lxphost store is not mounted")
	}
	sequence, err := lxphost.NextSequence(ctx, hostKey)
	if err != nil {
		return Result{}, authReadError(err)
	}
	accountSequence, err := k.host.ReadIdentitySequence(ctx, actor.DID)
	if err != nil {
		return Result{}, authReadError(err)
	}
	timestamp := uint64(ctx.BlockTime().UnixMilli())
	activity := lxphost.Activity{
		ProtocolVersion: 3,
		Type:            0x00070000 | uint32(payload[1]),
		ActorDID:        actor.DID,
		AuthorityKey:    actor.Authority.VerifiedKey,
		AccountSequence: accountSequence,
		NotBefore:       timestamp,
		NotAfter:        timestamp,
		IdempotencyKey:  idempotencyKey,
		Payload:         payload,
	}
	receipt, err := k.host.Execute(lxphost.WithSequence(ctx, sequence), activity, actor.Authority)
	if err != nil {
		return Result{Receipt: receipt}, authReadError(err)
	}
	return Result{Receipt: receipt}, FromKernel(receipt.ResultCode)
}
