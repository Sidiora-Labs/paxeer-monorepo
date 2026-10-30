package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func (k Keeper) StreamMeter(ctx sdk.Context, caller common.Address, streamID [32]byte, cumulativeReading uint64, authorityKey [32]byte, signature [64]byte, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeStreamMeter(streamID, cumulativeReading, authorityKey, signature)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "streamMeter", payload, idempotencyKey)
}

func (k Keeper) StreamSettle(ctx sdk.Context, caller common.Address, streamID, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeStreamKeyed(streamID, idempotencyKey)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "streamSettle", payload, idempotencyKey)
}

func (k Keeper) StreamClose(ctx sdk.Context, caller common.Address, streamID, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeStreamKeyed(streamID, idempotencyKey)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "streamClose", payload, idempotencyKey)
}
