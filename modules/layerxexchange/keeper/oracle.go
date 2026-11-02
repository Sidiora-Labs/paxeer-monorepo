package keeper

/*
#cgo CFLAGS: -I${SRCDIR}/../../../include -I${SRCDIR}/../../../build/generated
#include <stdlib.h>
#include <string.h>
#include "layerx/lxp_module_ctx.h"
#include "layerx/lx_oracle.h"

static int32_t exchange_oracle_read(const uint8_t feed[32],
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
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"unsafe"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxexchange/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
)

type OraclePrice struct {
	Price               sdk.Int
	ObservedAt          uint64
	ObservationSequence uint64
	SourceSetDigest     [32]byte
}

type oracleReplayKey struct{}

type oracleReplay struct {
	host      *lxphost.Host
	storeKey  sdk.StoreKey
	validator sdk.ValAddress
	activity  lxphost.Activity
	authority lxphost.KernelAuthority
}

// WithOracleReplay binds the authenticated validator and signed replay input
// supplied by the caller to the host store used for execution and reads.
func WithOracleReplay(ctx sdk.Context, host *lxphost.Host, storeKey sdk.StoreKey,
	validator sdk.ValAddress, activity lxphost.Activity, authority lxphost.KernelAuthority) sdk.Context {
	activity.ActorDID = bytes.Clone(activity.ActorDID)
	activity.Payload = bytes.Clone(activity.Payload)
	activity.Signature = bytes.Clone(activity.Signature)
	return ctx.WithValue(oracleReplayKey{}, oracleReplay{host, storeKey, bytes.Clone(validator), activity, authority})
}

func oracleReplayFrom(ctx sdk.Context, validator sdk.ValAddress) (oracleReplay, error) {
	replay, ok := ctx.Value(oracleReplayKey{}).(oracleReplay)
	if !ok || replay.host == nil || replay.storeKey == nil {
		return oracleReplay{}, sdkerrors.Wrap(types.ErrKeeperMissing, "oracle replay host")
	}
	if len(validator) == 0 || !bytes.Equal(validator, replay.validator) {
		return oracleReplay{}, sdkerrors.Wrap(sdkerrors.ErrUnauthorized, "oracle validator")
	}
	return replay, nil
}

func (k *Keeper) SubmitOracleObservation(ctx sdk.Context, validator sdk.ValAddress,
	feedID [32]byte, price sdk.Int, confidence uint64) error {
	if retired, err := k.Retired(ctx); err != nil {
		return err
	} else if retired {
		return k.retirementError(ctx)
	}
	replay, err := oracleReplayFrom(ctx, validator)
	if err != nil {
		return err
	}
	activity := replay.activity
	if feedID == ([32]byte{}) || !layerxAmount(price) || activity.Type != uint32(C.LX_ORACLE_PUSH_ACTIVITY) ||
		len(activity.Payload) != int(C.LX_ORACLE_TRANSPORT_BYTES) || activity.Payload[0] != 1 ||
		!bytes.Equal(activity.Payload[1:33], feedID[:]) ||
		new(big.Int).SetBytes(activity.Payload[41:57]).Cmp(price.BigInt()) != 0 {
		return sdkerrors.Wrap(types.ErrInvalidIntent, "signed oracle observation does not match submission")
	}
	_, err = atomically(ctx, func(branch sdk.Context) (struct{}, error) {
		receipt, err := replay.host.Execute(branch, activity, replay.authority)
		if err != nil {
			return struct{}{}, err
		}
		if receipt.ResultCode != 0 {
			return struct{}{}, receipt.ResultCode
		}
		record, err := json.Marshal(struct {
			Validator  sdk.ValAddress
			Confidence uint64
			Activity   lxphost.Activity
			Receipt    lxphost.Receipt
		}{validator, confidence, activity, receipt})
		if err != nil {
			return struct{}{}, err
		}
		key := append([]byte("oracle/activity/"), activity.IdempotencyKey[:]...)
		k.store(branch).Set(key, record)
		return struct{}{}, nil
	})
	return err
}

func (k *Keeper) CommittedOraclePrice(ctx sdk.Context, validator sdk.ValAddress,
	feedID [32]byte) (OraclePrice, bool, error) {
	replay, err := oracleReplayFrom(ctx, validator)
	if err != nil {
		return OraclePrice{}, false, err
	}
	store := ctx.KVStore(replay.storeKey)
	read := func(prefix string) []byte {
		key := append([]byte{1, 0, 6}, []byte(prefix)...)
		return store.Get(append(key, feedID[:]...))
	}
	market, oracle := read("market:"), read("oracle:")
	var marketPtr, oraclePtr unsafe.Pointer
	if market != nil {
		marketPtr = C.CBytes(market)
		defer C.free(marketPtr)
	}
	if oracle != nil {
		oraclePtr = C.CBytes(oracle)
		defer C.free(oraclePtr)
	}
	var encoded [64]byte
	status := C.exchange_oracle_read((*C.uint8_t)(unsafe.Pointer(&feedID[0])),
		(*C.uint8_t)(marketPtr), C.size_t(len(market)),
		(*C.uint8_t)(oraclePtr), C.size_t(len(oracle)),
		(*C.uint8_t)(unsafe.Pointer(&encoded[0])))
	if status == C.LXP_ERR_UNKNOWN_FIELD {
		return OraclePrice{}, false, nil
	}
	if status != C.LXP_OK {
		return OraclePrice{}, false, lxphost.Code(status)
	}
	var priceBytes [16]byte
	for i := range priceBytes {
		priceBytes[15-i] = encoded[i]
	}
	result := OraclePrice{
		Price:               sdk.NewIntFromBigInt(new(big.Int).SetBytes(priceBytes[:])),
		ObservedAt:          binary.LittleEndian.Uint64(encoded[16:24]),
		ObservationSequence: binary.LittleEndian.Uint64(encoded[24:32]),
	}
	copy(result.SourceSetDigest[:], encoded[32:])
	return result, true, nil
}
