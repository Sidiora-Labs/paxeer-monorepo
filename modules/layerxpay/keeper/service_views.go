package keeper

import (
	"encoding/binary"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

const serviceKeyBytes = 40

func serviceKey(prefix string, id [32]byte) []byte {
	key := make([]byte, serviceKeyBytes)
	copy(key[:8], prefix)
	copy(key[8:], id[:])
	return types.KernelKVKey(types.ServiceModuleID, key)
}

func decodeServiceView[T any](bz []byte, decode func([]byte) (T, error)) (T, bool) {
	if bz == nil {
		var zero T
		return zero, false
	}
	record, err := decode(bz)
	if err != nil {
		panic(fmt.Errorf("layerxpay: decode service record: %w", err))
	}
	return record, true
}

func (k Keeper) GetOffer(ctx sdk.Context, id [32]byte) (types.ServiceOffer, bool) {
	return decodeServiceView(ctx.KVStore(k.hostKey).Get(serviceKey("offer::1", id)), types.DecodeServiceOffer)
}

func (k Keeper) GetAgreement(ctx sdk.Context, id [32]byte) (types.ServiceAgreement, bool) {
	return decodeServiceView(ctx.KVStore(k.hostKey).Get(serviceKey("agree::1", id)), types.DecodeServiceAgreement)
}

func (k Keeper) GetCommitment(ctx sdk.Context, id [32]byte) (types.ServiceCommitment, bool) {
	return decodeServiceView(ctx.KVStore(k.hostKey).Get(serviceKey("commit:1", id)), types.DecodeServiceCommitment)
}

func (k Keeper) GetExecution(ctx sdk.Context, id [32]byte) (types.ServiceExecution, bool) {
	return decodeServiceView(ctx.KVStore(k.hostKey).Get(serviceKey("execut:1", id)), types.DecodeServiceExecution)
}

func (k Keeper) GetDelivery(ctx sdk.Context, id [32]byte) (types.ServiceDelivery, bool) {
	store := ctx.KVStore(k.hostKey)
	header := store.Get(serviceKey("delivr:1", id))
	if header == nil {
		return types.ServiceDelivery{}, false
	}
	if len(header) != 138 || header[0] != 1 || header[129] == 0 || int(header[129]) > types.ServiceMaxDeliverables {
		panic("layerxpay: invalid service delivery header")
	}
	// Kernel delivery items live at separate 41-byte keys; the decoder takes
	// the header followed by the ordered, versioned item records.
	bz := make([]byte, len(header), len(header)+73*int(header[129]))
	copy(bz, header)
	key := append(serviceKey("delitm:1", id), byte(0))
	for i := byte(0); i < header[129]; i++ {
		key[len(key)-1] = i
		item := store.Get(key)
		if len(item) != 73 {
			panic("layerxpay: missing or invalid service delivery item")
		}
		bz = append(bz, item...)
	}
	return decodeServiceView(bz, types.DecodeServiceDelivery)
}

func (k Keeper) GetProgress(ctx sdk.Context, id [32]byte) (types.ServiceProgress, bool) {
	return decodeServiceView(ctx.KVStore(k.hostKey).Get(serviceKey("progrs:1", id)), types.DecodeServiceProgress)
}

func (k Keeper) GetDispute(ctx sdk.Context, id [32]byte) (types.ServiceDispute, bool) {
	return decodeServiceView(ctx.KVStore(k.hostKey).Get(serviceKey("disput:1", id)), types.DecodeServiceDispute)
}

func (k Keeper) LatestDelivery(ctx sdk.Context, agreementID [32]byte) (types.ServiceDelivery, bool) {
	iterator := sdk.KVStorePrefixIterator(ctx.KVStore(k.hostKey), types.KernelKVKey(types.ServiceModuleID, []byte("delivr:1")))
	defer iterator.Close()
	var latestID [32]byte
	var sequence uint64
	found := false
	for ; iterator.Valid(); iterator.Next() {
		key := iterator.Key()
		if len(key) != 3+serviceKeyBytes {
			panic("layerxpay: invalid service delivery key")
		}
		// Scan headers first, then load the selected delivery's items.
		header := iterator.Value()
		if len(header) != 138 || header[0] != 1 || header[129] == 0 || int(header[129]) > types.ServiceMaxDeliverables {
			panic("layerxpay: invalid service delivery header")
		}
		currentSequence := binary.BigEndian.Uint64(header[130:138])
		if [32]byte(header[65:97]) == agreementID && (!found || currentSequence >= sequence) {
			copy(latestID[:], key[3+8:])
			sequence, found = currentSequence, true
		}
	}
	if !found {
		return types.ServiceDelivery{}, false
	}
	return k.GetDelivery(ctx, latestID)
}

func (k Keeper) ProgressHighWater(ctx sdk.Context, commitmentID [32]byte) (uint32, bool) {
	iterator := sdk.KVStorePrefixIterator(ctx.KVStore(k.hostKey), types.KernelKVKey(types.ServiceModuleID, []byte("progrs:1")))
	defer iterator.Close()
	var highWater uint32
	found := false
	for ; iterator.Valid(); iterator.Next() {
		if len(iterator.Key()) != 3+serviceKeyBytes {
			panic("layerxpay: invalid service progress key")
		}
		progress, _ := decodeServiceView(iterator.Value(), types.DecodeServiceProgress)
		if progress.CommitmentID == commitmentID {
			if progress.ProgressBPS > highWater {
				highWater = progress.ProgressBPS
			}
			found = true
		}
	}
	return highWater, found
}
