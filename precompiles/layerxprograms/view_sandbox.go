package layerxprograms

import (
	"errors"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func getSandboxHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	leaseID, ok := decoded[0].([32]byte)
	if !ok {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: getSandbox requires a bytes32 leaseId")
	}
	kind, ok := decoded[1].(uint8)
	if !ok || kind > 2 {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: getSandbox requires a record kind from 0 to 2")
	}
	if p == nil || p.host == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: programs host is unavailable")
	}
	record, err := p.host.ReadSandbox(ctx, leaseID, kind)
	if errors.Is(err, lxphost.Code(-7)) {
		return nil, fmt.Errorf("LXP_NOT_FOUND: sandbox record: %w", err)
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

func getRetirementControlHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	if _, err := decodeArgs(call.method, args); err != nil {
		return nil, err
	}
	if p == nil || p.host == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: programs host is unavailable")
	}
	record, err := p.host.ReadRetirementControl(ctx)
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
	register("getSandbox", getSandboxHandler)
	register("getRetirementControl", getRetirementControlHandler)
}
