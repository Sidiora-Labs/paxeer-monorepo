package layerxprograms

import (
	"errors"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func getWebRequestHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	if call.method == nil || call.method.Name != "getWebRequest" {
		return nil, errGateUnknownMethod
	}
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	programID, ok := decoded[0].([32]byte)
	if !ok {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: getWebRequest programId must be bytes32")
	}
	requestID, ok := decoded[1].(uint64)
	if !ok {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: getWebRequest requestId must be uint64")
	}
	if p == nil || p.host == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: programs host is unavailable")
	}
	record, err := p.host.ReadWebRequest(ctx, programID, requestID)
	if errors.Is(err, lxphost.Code(-7)) {
		return nil, fmt.Errorf("LXP_NOT_FOUND: getWebRequest: %w", err)
	}
	if err != nil {
		var code lxphost.Code
		if errors.As(err, &code) {
			return nil, kernelError(code, call.method.Name)
		}
		return nil, err
	}
	return call.method.Outputs.Pack(record)
}

func init() {
	register("getWebRequest", getWebRequestHandler)
}
