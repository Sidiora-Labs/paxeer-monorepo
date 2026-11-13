package keeper

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
)

const StreamMetadataAskRef = "layerxpay/stream/ask-ref/v1/"

const askStreamPrefix = "layerxpay/ask-stream/v1/"

func streamAskKey(prefix string, id [32]byte) []byte {
	return append([]byte(prefix), id[:]...)
}

// StreamOpenForAsk funds a native-asset metered stream to the room's bound
// settlement account. A reading is the cumulative token count of the last
// settled lap batch; only the existing signed StreamMeter path advances it.
// The pay precompile supplies a streamID with sixteen leading zero bytes.
func (k Keeper) StreamOpenForAsk(ctx sdk.Context, caller common.Address, streamID, askRef [32]byte, settlementAccount common.Address, rate *big.Int, rateUnit uint8, totalCap *big.Int, hostAuthorityKey [32]byte) error {
	if err := k.CheckWrite(ctx, "streamOpen"); err != nil {
		return err
	}
	if streamID == ([32]byte{}) || askRef == ([32]byte{}) || hostAuthorityKey == ([32]byte{}) || settlementAccount == (common.Address{}) || rateUnit == 0 {
		return fmt.Errorf("LXP_NON_CANONICAL: ask stream identifiers or rate unit")
	}
	if ctx.BlockTime().UnixMilli() <= 0 {
		return fmt.Errorf("LXP_NON_CANONICAL: ask stream start timestamp")
	}
	rateAmount, err := agentStreamAmount(rate)
	if err != nil {
		return err
	}
	funding, err := agentStreamAmount(totalCap)
	if err != nil {
		return err
	}
	if ctx.KVStore(k.storeKey).Has(streamAskKey(askStreamPrefix, askRef)) {
		return fmt.Errorf("LXP_NON_CANONICAL: ask already has a stream")
	}
	if ctx.KVStore(k.storeKey).Has(streamAskKey(StreamMetadataAskRef, streamID)) {
		return fmt.Errorf("LXP_NON_CANONICAL: stream already has an ask")
	}
	payer, err := k.AuthorityFor(ctx, caller)
	if err != nil {
		return err
	}
	settlement, err := k.AuthorityFor(ctx, settlementAccount)
	if err != nil {
		return err
	}
	_, recipient, err := lxphost.MainAccountID(settlement.AuthorityKey)
	if err != nil {
		return err
	}
	asset, err := k.agentStreamAsset(ctx, common.Address{})
	if err != nil {
		return err
	}
	account, err := codec.DeriveAccountID([]byte("agent:" + string(payer.ActorDID) + ":stream:" + hex.EncodeToString(streamID[:])))
	if err != nil {
		return err
	}
	branch, commit := ctx.CacheContext()
	_, err = k.StreamOpen(branch, caller, types.StreamOpen{
		StreamID: streamID, StreamAccount: account, Recipient: recipient, AssetID: asset,
		Mode: types.StreamModeMetered, Rate: rateAmount, RateUnit: uint64(rateUnit),
		StartTimestamp: uint64(ctx.BlockTime().UnixMilli()),
		TotalCap:       funding, InitialFunding: funding, MeterAuthorities: [][32]byte{hostAuthorityKey},
	})
	if err != nil {
		return err
	}
	store := branch.KVStore(k.storeKey)
	store.Set(streamAskKey(StreamMetadataAskRef, streamID), askRef[:])
	store.Set(streamAskKey(askStreamPrefix, askRef), streamID[:])
	commit()
	return nil
}

func (k Keeper) AskStreamOf(ctx sdk.Context, askRef [32]byte) ([32]byte, error) {
	store := ctx.KVStore(k.storeKey)
	encoded := store.Get(streamAskKey(askStreamPrefix, askRef))
	if encoded == nil {
		return [32]byte{}, nil
	}
	if len(encoded) != 32 || [32]byte(encoded) == ([32]byte{}) {
		return [32]byte{}, fmt.Errorf("LXP_NON_CANONICAL: ask stream metadata")
	}
	id := [32]byte(encoded)
	ref := store.Get(streamAskKey(StreamMetadataAskRef, id))
	if len(ref) != 32 || [32]byte(ref) != askRef {
		return [32]byte{}, fmt.Errorf("LXP_NON_CANONICAL: ask stream metadata mismatch")
	}
	if _, err := k.GetStream(ctx, id); err != nil {
		return [32]byte{}, err
	}
	return id, nil
}

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
