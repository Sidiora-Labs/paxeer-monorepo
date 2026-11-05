package oracle

/*
#cgo CFLAGS: -I${SRCDIR}/../../../include -I${SRCDIR}/../../../build/generated
#include <stdlib.h>
#include <string.h>
#include "layerx/lxp_module_ctx.h"
#include "layerx/lx_oracle.h"

static int32_t oracle_committed_read(const uint8_t feed[32],
    const uint8_t *market, size_t market_length,
    const uint8_t *oracle, size_t oracle_length, uint8_t out[64])
{
    lxp_kernel *kernel = calloc(1U, sizeof(*kernel));
    lxp_module_ctx *ctx = calloc(1U, sizeof(*ctx));
    lx_oracle_committed committed;
    int32_t status = LXP_ERR_ARENA_EXHAUSTED;
    if (kernel == NULL || ctx == NULL) goto done;
    ctx->kernel = kernel;
    ctx->module_id = LXP_MODULE_PERPS;
    const uint8_t *values[] = {market, oracle};
    const size_t lengths[] = {market_length, oracle_length};
    const char *prefixes[] = {"market:", "oracle:"};
    for (size_t i = 0U; i < 2U; ++i) {
        if (values[i] == NULL) continue;
        if (lengths[i] > LXP_MODULE_MAX_VALUE_BYTES) {
            status = LXP_ERR_NON_CANONICAL;
            goto done;
        }
        lxp_module_kv_entry *entry = &kernel->module_kv[kernel->module_kv_count++];
        entry->module_id = LXP_MODULE_PERPS;
        entry->key_length = 39U;
        memcpy(entry->key, prefixes[i], 7U);
        memcpy(entry->key + 7U, feed, 32U);
        entry->value_length = (uint32_t)lengths[i];
        memcpy(entry->value, values[i], lengths[i]);
    }
    status = lx_oracle_committed_read(ctx, feed, &committed);
    if (status == LXP_OK) status = lx_oracle_committed_encode(&committed, out);
done:
    free(ctx);
    free(kernel);
    return status;
}
*/
import "C"

import (
	"encoding/binary"
	"math/big"
	"unsafe"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type OraclePrice struct {
	Price               sdk.Int
	ObservedAt          uint64
	ObservationSequence uint64
	SourceSetDigest     [32]byte
}

func (k *Keeper) CommittedOraclePrice(ctx sdk.Context, feed [32]byte) (OraclePrice, bool, error) {
	key, err := oracleStoreKey(ctx, k.hostStoreName)
	if err != nil {
		return OraclePrice{}, false, err
	}
	store := ctx.KVStore(key)
	market := store.Get(append([]byte{1, 0, 6, 'm', 'a', 'r', 'k', 'e', 't', ':'}, feed[:]...))
	observation := store.Get(append([]byte{1, 0, 6, 'o', 'r', 'a', 'c', 'l', 'e', ':'}, feed[:]...))
	var marketPtr, oraclePtr unsafe.Pointer
	if market != nil {
		marketPtr = C.CBytes(market)
		defer C.free(marketPtr)
	}
	if observation != nil {
		oraclePtr = C.CBytes(observation)
		defer C.free(oraclePtr)
	}
	var encoded [64]byte
	status := C.oracle_committed_read((*C.uint8_t)(unsafe.Pointer(&feed[0])),
		(*C.uint8_t)(marketPtr), C.size_t(len(market)),
		(*C.uint8_t)(oraclePtr), C.size_t(len(observation)),
		(*C.uint8_t)(unsafe.Pointer(&encoded[0])))
	if status == C.LXP_ERR_UNKNOWN_FIELD {
		return OraclePrice{}, false, nil
	}
	if status != C.LXP_OK {
		return OraclePrice{}, false, lxphost.Code(status)
	}
	var amount [16]byte
	for i := range amount {
		amount[15-i] = encoded[i]
	}
	result := OraclePrice{
		Price:               sdk.NewIntFromBigInt(new(big.Int).SetBytes(amount[:])),
		ObservedAt:          binary.LittleEndian.Uint64(encoded[16:24]),
		ObservationSequence: binary.LittleEndian.Uint64(encoded[24:32]),
	}
	copy(result.SourceSetDigest[:], encoded[32:])
	return result, true, nil
}
