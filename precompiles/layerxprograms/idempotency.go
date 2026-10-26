package layerxprograms

import (
	"crypto/sha256"
	"encoding/binary"
	"math"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

const ordinalKeyPrefix = "layerxprograms/ordinal/v1/"

func IdempotencyKey(txHash [32]byte, ordinal uint32, selector [4]byte) [32]byte {
	const domain = "layerxprograms/idempotency/v1"
	var preimage [len(domain) + 32 + 4 + 4]byte
	copy(preimage[:], domain)
	copy(preimage[len(domain):], txHash[:])
	binary.BigEndian.PutUint32(preimage[len(domain)+32:], ordinal)
	copy(preimage[len(domain)+32+4:], selector[:])
	digest := sha256.Sum256(preimage[:])
	if digest == ([32]byte{}) {
		return sha256.Sum256(digest[:])
	}
	return digest
}

func nextOrdinal(ctx sdk.Context, txHash [32]byte) uint32 {
	var transientKey *sdk.TransientStoreKey
	for _, key := range ctx.MultiStore().StoreKeys() {
		if key.Name() == evmtypes.TransientStoreKey {
			transientKey, _ = key.(*sdk.TransientStoreKey)
			break
		}
	}
	if transientKey == nil {
		panic("layerxprograms: EVM transient store is not mounted")
	}
	store := ctx.TransientStore(transientKey)
	key := append([]byte(ordinalKeyPrefix), txHash[:]...)
	var ordinal uint32
	if previous := store.Get(key); previous != nil {
		if len(previous) != 4 {
			panic("layerxprograms: invalid call ordinal")
		}
		last := binary.BigEndian.Uint32(previous)
		if last == math.MaxUint32 {
			panic("layerxprograms: call ordinal exhausted")
		}
		ordinal = last + 1
	}
	var encoded [4]byte
	binary.BigEndian.PutUint32(encoded[:], ordinal)
	store.Set(key, encoded[:])
	return ordinal
}
