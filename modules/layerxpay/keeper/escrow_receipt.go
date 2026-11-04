//go:build cgo

package keeper

/*
#cgo CFLAGS: -I${SRCDIR}/../../../include
#cgo LDFLAGS: -L${SRCDIR}/../../../build -L${SRCDIR}/../../../programs/target/release -L${SRCDIR}/../../../programs/target/debug -llayerx -llayerx_programs_sandbox -llayerx -lssl -lcrypto -pthread -ldl -lm
#include "layerx/lxp_receipt.h"

lxp_result lx_escrow_bind_economic_receipt(lxp_receipt *receipt,
                                         const lxp_effect_buffer *effects);

static lxp_result pay_bind_escrow_receipt(const uint8_t *bytes, size_t length,
                                         uint8_t *output, size_t capacity,
                                         size_t *output_length)
{
    lxp_receipt receipt;
    lxp_arena arena;
    lxp_byte_span encoded;
    lxp_result status = lxp_receipt_decode(bytes, length, false, &receipt);
    if (status == LXP_OK)
        status = lx_escrow_bind_economic_receipt(&receipt, &receipt.effects);
    if (status == LXP_OK)
        status = lxp_arena_init(&arena, output, capacity);
    if (status == LXP_OK)
        status = lxp_receipt_encode(&receipt, false, &arena, &encoded);
    if (status == LXP_OK) *output_length = encoded.length;
    return status;
}
*/
import "C"

import (
	"bytes"
	"encoding/binary"
	"unsafe"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func escrowReceiptKey(sequence uint64) []byte {
	return binary.BigEndian.AppendUint64([]byte("layerxpay/escrow-receipt/v1/"), sequence)
}

// bindEscrowReceipt publishes the economic event's root before the unsigned
// receipt leaves Submit. The receipt and economic writes share its cache branch.
func (k Keeper) bindEscrowReceipt(ctx sdk.Context, receipt *lxphost.Receipt) error {
	decoded, err := codec.DecodeReceipt(receipt.Encoded)
	if err != nil {
		return err
	}
	if decoded.ModuleID != types.EscrowModuleID || decoded.ResultCode != 0 || decoded.SequencerSignature != nil {
		return lxphost.Code(-1001)
	}
	output := make([]byte, C.LXP_MAX_ACTIVITY_BYTES)
	var length C.size_t
	status := C.pay_bind_escrow_receipt((*C.uint8_t)(unsafe.Pointer(&receipt.Encoded[0])), C.size_t(len(receipt.Encoded)),
		(*C.uint8_t)(unsafe.Pointer(&output[0])), C.size_t(len(output)), &length)
	if status != C.LXP_OK {
		return lxphost.Code(status)
	}
	receipt.Encoded = bytes.Clone(output[:int(length)])
	ctx.KVStore(k.hostKey).Set(escrowReceiptKey(receipt.GlobalSequence), receipt.Encoded)
	return nil
}

// GetEscrowReceipt reads the canonical bytes published for one execution,
// including replays, from the same persistent store as its economic result.
func (k Keeper) GetEscrowReceipt(ctx sdk.Context, sequence uint64) (*codec.Receipt, bool, error) {
	wire := ctx.KVStore(k.hostKey).Get(escrowReceiptKey(sequence))
	if wire == nil {
		return nil, false, nil
	}
	receipt, err := codec.DecodeReceipt(wire)
	if err != nil {
		return nil, false, err
	}
	if receipt.GlobalSequence != sequence || receipt.ModuleID != types.EscrowModuleID || receipt.ResultCode != 0 {
		return nil, false, lxphost.Code(-1001)
	}
	return receipt, true, nil
}
