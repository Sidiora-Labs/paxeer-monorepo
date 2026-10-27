package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"

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
	grant, err := k.GetPayerGrant(ctx, args.GrantID)
	if err != nil {
		return Result{}, err
	}
	_, receiver, err := lxphost.AssetAccountID(actor.DIDKey, args.Asset)
	if err != nil {
		return Result{}, ledgerReadError(err)
	}
	networkID := config.GetEVMChainID(ctx.ChainID())
	if networkID.Sign() <= 0 || networkID.BitLen() > 32 {
		return Result{}, types.ErrNonCanonical
	}
	payload, err := types.EncodeReceivePayload(types.ReceiveArgs{
		From: args.From, To: receiver, Asset: args.Asset, Amount: args.Amount,
		GrantID: args.GrantID, ReceiverSequence: args.ReceiverSequence,
		IdempotencyKey: args.IdempotencyKey, ContextHash: args.ContextHash,
		PublicKey: actor.Authority.VerifiedKey, NetworkID: uint32(networkID.Uint64()),
		ProtocolVersion: codec.StateCommitmentProtocolVersion, PayerGrant: grant.Grant,
	})
	if err != nil {
		return Result{}, err
	}
	branch, write := ctx.CacheContext()
	const activityType uint32 = 0x00010006
	result, err := k.execute(branch, actor, activityType, payload, args.IdempotencyKey)
	if err != nil {
		return result, err
	}
	if err := k.Mirror(branch, activityType, result.Receipt); err != nil {
		return result, err
	}
	_, err = LedgerEvents(activityType, result.Receipt, EventArgs{
		GrantID: args.GrantID, IdempotencyKey: args.IdempotencyKey, ContextHash: args.ContextHash,
	})
	if err != nil {
		return result, err
	}
	write()
	ctx.EventManager().EmitEvents(branch.EventManager().Events())
	return result, nil
}
