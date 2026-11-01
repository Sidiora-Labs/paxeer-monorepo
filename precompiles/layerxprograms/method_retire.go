package layerxprograms

import (
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func init() {
	register("retireStorage", retireStorageHandler)
}

func retireStorageHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	request := decoded[0].([]byte)
	if err := CheckRetireStorageRequest(request); err != nil {
		return nil, err
	}
	receipt, err := submit(ctx, p, call, 0x0009000B, request, [16]byte{})
	if err != nil {
		return nil, err
	}
	if err := emitSandboxEvents(call.evm, receipt); err != nil {
		return nil, err
	}
	return call.method.Outputs.Pack()
}
