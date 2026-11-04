package keeper

import (
	"bytes"
	"encoding/binary"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

const assetSendActivity uint32 = 0x00010005

func (k *Keeper) Send(ctx sdk.Context, actor Actor, envelope []byte, idempotencyKey [32]byte) (Result, error) {
	if err := k.checkPause(ctx, "send"); err != nil {
		return Result{}, err
	}
	if err := types.ValidateSendEnvelope(envelope); err != nil {
		return Result{}, err
	}
	var from, assetID [32]byte
	copy(from[:], envelope[4:36])
	copy(assetID[:], envelope[68:100])
	owner, err := k.host.ReadAccountOwner(ctx, from)
	if err != nil {
		return Result{}, ledgerReadError(err)
	}
	if owner != actor.Authority.Actor {
		return Result{}, types.ErrUnauthorizedDebit
	}
	source, err := k.accountByID(ctx, from)
	if err != nil {
		return Result{}, err
	}
	if !source.HasAsset || source.AssetID != assetID {
		return Result{}, types.ErrAssetMismatch
	}
	mainName, _, err := lxphost.MainAccountID(actor.DIDKey)
	if err != nil {
		return Result{}, ledgerReadError(err)
	}
	assetName, _, err := lxphost.AssetAccountID(actor.DIDKey, assetID)
	if err != nil {
		return Result{}, ledgerReadError(err)
	}
	if source.Name != mainName && source.Name != assetName {
		return Result{}, types.ErrUnauthorizedDebit
	}
	var zeroAmount [16]byte
	if bytes.Equal(envelope[100:116], zeroAmount[:]) {
		return Result{}, types.ErrZeroAmount
	}
	timestamp := ctx.BlockTime().UnixMilli()
	if timestamp < 0 {
		return Result{}, types.ErrNonCanonical
	}
	if uint64(timestamp) > binary.BigEndian.Uint64(envelope[156:164]) {
		return Result{}, types.ErrExpired
	}
	result, err := k.execute(ctx, actor, assetSendActivity, envelope, idempotencyKey)
	if err != nil {
		return result, err
	}
	if err := k.Mirror(ctx, assetSendActivity, result.Receipt); err != nil {
		return result, err
	}
	_, err = LedgerEvents(assetSendActivity, result.Receipt, EventArgs{})
	return result, err
}
