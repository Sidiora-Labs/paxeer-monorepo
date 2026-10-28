package layerxprograms

import sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"

func init() {
	register("deploy", deployHandler)
}

func deployHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	payload, err := EncodeDeployPayload(
		decoded[0].([32]byte), decoded[1].(uint16), decoded[2].(uint8),
		decoded[3].([32]byte), decoded[4].([32]byte), decoded[5].([]byte), decoded[6].([]byte),
	)
	if err != nil {
		return nil, err
	}
	const activityType uint32 = 0x00090001
	receipt, err := submit(ctx, p, call, activityType, payload, [16]byte{})
	if err != nil {
		return nil, err
	}
	if err := emitReceiptEvents(call.evm, activityType, [32]byte(payload[:32]), receipt); err != nil {
		return nil, err
	}
	return nil, nil
}
