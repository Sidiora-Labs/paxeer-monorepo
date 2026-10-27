package keeper

import (
	"errors"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type Event = lxphost.ReceiptEvent

type Result struct {
	Receipt lxphost.Receipt
	Events  []Event
}

func WithSequence(ctx sdk.Context) (sdk.Context, error) {
	for _, key := range ctx.MultiStore().StoreKeys() {
		if key.Name() != "lxphost" {
			continue
		}
		sequence, err := lxphost.NextSequence(ctx, key)
		if err != nil {
			return ctx, ledgerReadError(err)
		}
		if sequence == 0 {
			return ctx, types.ErrSequenceExhausted
		}
		return lxphost.WithSequence(ctx, sequence), nil
	}
	return ctx, fmt.Errorf("layerxledger: lxphost store is not mounted")
}

func (k *Keeper) execute(ctx sdk.Context, actor Actor, activityType uint32, payload []byte, idempotencyKey [32]byte) (Result, error) {
	timestamp := ctx.BlockTime().UnixMilli()
	if timestamp < 0 {
		return Result{}, types.ErrNonCanonical
	}
	ctx, err := WithSequence(ctx)
	if err != nil {
		return Result{}, err
	}
	accountSequence, err := k.host.ReadIdentitySequence(ctx, actor.DID)
	if err != nil {
		return Result{}, ledgerReadError(err)
	}
	activity := lxphost.Activity{
		ProtocolVersion: 3,
		Type:            activityType,
		ActorDID:        actor.DID,
		AuthorityKey:    actor.Authority.VerifiedKey,
		AccountSequence: accountSequence,
		NotBefore:       uint64(timestamp),
		NotAfter:        uint64(timestamp),
		IdempotencyKey:  idempotencyKey,
		Payload:         payload,
	}
	receipt, err := k.host.Execute(ctx, activity, actor.Authority)
	if err != nil {
		err = ledgerReadError(err)
		if errors.Is(err, types.ErrIdempotentReplay) {
			stored, readErr := k.host.ReadIdempotentReceipt(ctx, actor.DID, idempotencyKey)
			if readErr != nil {
				return Result{}, ledgerReadError(readErr)
			}
			return Result{Receipt: stored, Events: stored.Events}, err
		}
		return Result{}, err
	}
	stored, err := k.host.ReadIdempotentReceipt(ctx, actor.DID, idempotencyKey)
	if err != nil {
		return Result{Receipt: receipt, Events: receipt.Events}, ledgerReadError(err)
	}
	receipt.Encoded = stored.Encoded
	return Result{Receipt: receipt, Events: receipt.Events}, FromKernel(receipt.ResultCode)
}
