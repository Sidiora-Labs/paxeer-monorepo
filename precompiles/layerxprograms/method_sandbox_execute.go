package layerxprograms

import (
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func sandboxExecuteHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	if call.method == nil || call.method.Name != "sandboxExecute" {
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: expected sandboxExecute")
	}
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	payload, err := EncodeSandboxExecutePayload(
		decoded[0].([32]byte), decoded[1].(uint64), decoded[2].([32]byte),
		decoded[3].([32]byte), decoded[4].([32]byte), decoded[5].([32]byte),
		decoded[6].(uint32), decoded[7].([7]uint64), decoded[8].([]byte),
	)
	if err != nil {
		return nil, err
	}
	receipt, err := submit(ctx, p, call, 0x00090009, payload, decoded[9].([16]byte))
	if err != nil {
		return nil, err
	}
	outcome := receipt.ProgramOutcome
	if outcome == nil || outcome.Kind < 1 || outcome.Kind > 3 {
		return nil, fmt.Errorf("%w: sandboxExecute missing terminal outcome", lxphost.Code(-1001))
	}
	output, err := call.method.Outputs.Pack(outcome.Status, outcome.Kind, outcome.Response)
	if err != nil {
		return nil, err
	}
	if err := emitSandboxEvents(call.evm, receipt); err != nil {
		return nil, err
	}
	return output, nil
}

func init() {
	register("sandboxExecute", sandboxExecuteHandler)
}
