package layerxprograms

import sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"

func registryReadHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	programID := decoded[0].([32]byte)
	payload, err := EncodeRegistryReadPayload(programID, decoded[1].([]byte))
	if err != nil {
		return nil, err
	}
	receipt, err := submit(ctx, p, call, 0x00090004, payload, [16]byte{})
	if err != nil {
		return nil, err
	}
	if err := emitReceiptEvents(call.evm, 0x00090004, programID, receipt); err != nil {
		return nil, err
	}
	return nil, nil
}

func init() {
	register("registryRead", registryReadHandler)
}
