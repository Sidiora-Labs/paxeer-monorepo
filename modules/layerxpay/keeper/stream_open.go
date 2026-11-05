package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

func (k Keeper) StreamOpen(ctx sdk.Context, caller common.Address, open types.StreamOpen) (lxphost.Receipt, error) {
	payload, err := types.EncodeStreamOpen(open)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "streamOpen", payload, open.StreamID)
}

func (k Keeper) StreamTopUp(ctx sdk.Context, caller common.Address, streamID [32]byte, amount codec.U128, idempotencyKey [32]byte) (lxphost.Receipt, error) {
	payload, err := types.EncodeStreamTopUp(streamID, amount)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	return k.Submit(ctx, caller, "streamTopUp", payload, idempotencyKey)
}
