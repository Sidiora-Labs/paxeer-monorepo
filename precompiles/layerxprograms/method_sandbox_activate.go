package layerxprograms

import (
	"fmt"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func init() {
	register("sandboxActivate", sandboxActivateHandler)
}

func sandboxActivateHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	if call.method == nil || call.method.Name != "sandboxActivate" {
		return nil, errGateUnknownMethod
	}
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	leaseID, leaseOK := decoded[0].([32]byte)
	digest, digestOK := decoded[1].([32]byte)
	sequence, sequenceOK := decoded[2].(uint64)
	if !leaseOK || !digestOK || !sequenceOK {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: sandboxActivate arguments")
	}
	payload, err := EncodeSandboxActivatePayload(leaseID, digest, sequence)
	if err != nil {
		return nil, err
	}
	receipt, err := submit(ctx, p, call, 0x00090009, payload, [16]byte{})
	if err != nil {
		return nil, err
	}
	return nil, emitSandboxEvents(call.evm, receipt)
}
