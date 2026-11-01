package layerxprograms

import (
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

const windDownActivityType uint32 = 0x00090007

func init() {
	register("setExitRoute", setExitRouteHandler)
	register("deprecate", deprecateHandler)
}

func setExitRouteHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	payload, err := EncodeSetExitRoutePayload(decoded[0].([32]byte), decoded[1].([32]byte), decoded[2].([32]byte), decoded[3].([32]byte), decoded[4].([]byte))
	if err != nil {
		return nil, err
	}
	receipt, err := submit(ctx, p, call, windDownActivityType, payload, [16]byte{})
	if err != nil {
		return nil, err
	}
	return nil, emitReceiptEvents(call.evm, windDownActivityType, [32]byte(payload[:32]), receipt)
}

func deprecateHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	payload, err := EncodeDeprecatePayload(decoded[0].([32]byte), decoded[1].([32]byte), decoded[2].(uint64))
	if err != nil {
		return nil, err
	}
	receipt, err := submit(ctx, p, call, windDownActivityType, payload, [16]byte{})
	if err != nil {
		return nil, err
	}
	return nil, emitReceiptEvents(call.evm, windDownActivityType, [32]byte(payload[:32]), receipt)
}
