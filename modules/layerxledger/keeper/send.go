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
	_, main, err := lxphost.MainAccountID(actor.DIDKey)
	if err != nil {
		return Result{}, ledgerReadError(err)
	}
	if !bytes.Equal(envelope[4:36], main[:]) {
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
