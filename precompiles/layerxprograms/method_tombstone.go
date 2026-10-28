package layerxprograms

import (
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func tombstoneHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	payload, err := EncodeTombstonePayload(decoded[0].([32]byte))
	if err != nil {
		return nil, err
	}
	const activityType uint32 = 0x00090007
	receipt, err := submit(ctx, p, call, activityType, payload, [16]byte{})
	if err != nil {
		return nil, err
	}
	if err := emitReceiptEvents(call.evm, activityType, [32]byte(payload[:32]), receipt); err != nil {
		return nil, err
	}
	return nil, nil
}

func init() {
	register("tombstone", tombstoneHandler)
}
