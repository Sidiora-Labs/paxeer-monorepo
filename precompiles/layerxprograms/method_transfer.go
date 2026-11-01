package layerxprograms

import (
	"fmt"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func init() {
	register("transfer", transferHandler)
}

func transferHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	if call.method == nil || call.method.Name != "transfer" {
		return nil, errGateUnknownMethod
	}
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	programID, ok := decoded[0].([32]byte)
	if !ok {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: expected bytes32 program id, got %T", decoded[0])
	}
	payload, err := EncodeTransferPayload(programID, decoded[1].([]TransferLeg))
	if err != nil {
		return nil, err
	}
	receipt, err := submit(ctx, p, call, 0x00090005, payload, [16]byte{})
	if err != nil {
		return nil, err
	}
	return nil, emitReceiptEvents(call.evm, 0x00090005, [32]byte(payload[:32]), receipt)
}
