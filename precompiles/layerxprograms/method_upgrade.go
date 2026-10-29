package layerxprograms

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func upgradeHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	if call.method == nil || call.method.Name != "upgrade" {
		return nil, errGateUnknownMethod
	}
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	programID := decoded[0].([32]byte)
	payload, err := EncodeUpgradePayload(programID, decoded[1].(uint16), decoded[2].(uint8),
		decoded[3].([32]byte), decoded[4].([32]byte), decoded[5].([]byte), decoded[6].([]byte), decoded[7].([]byte))
	if err != nil {
		return nil, err
	}
	receipt, err := submit(ctx, p, call, 0x00090002, payload, decoded[8].([16]byte))
	if err != nil {
		if code, ok := err.(lxphost.Code); ok {
			return nil, kernelError(code, call.method.Name)
		}
		return nil, err
	}
	return nil, emitReceiptEvents(call.evm, 0x00090002, programID, receipt)
}

func init() {
	register("upgrade", upgradeHandler)
}
