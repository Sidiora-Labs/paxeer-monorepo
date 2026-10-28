package layerxprograms

import (
	"fmt"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func callHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	method := programsABI.Methods["call"]
	if call.method == nil || call.method.Name != method.Name {
		return nil, errGateUnknownMethod
	}
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	programID, programOK := decoded[0].([32]byte)
	abiVersion, versionOK := decoded[1].(uint16)
	entrypoint, entrypointOK := decoded[2].(string)
	calldata, calldataOK := decoded[3].([]byte)
	capabilities, capabilitiesOK := decoded[4].([]byte)
	access, accessOK := decoded[5].([]byte)
	responseCapacity, capacityOK := decoded[6].(uint32)
	budget, budgetOK := decoded[7].([7]uint64)
	feeLimit, feeLimitOK := decoded[8].([16]byte)
	if !programOK || !versionOK || !entrypointOK || !calldataOK || !capabilitiesOK || !accessOK || !capacityOK || !budgetOK || !feeLimitOK {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: invalid call argument types")
	}
	payload, err := EncodeCallPayload(programID, abiVersion, entrypoint, calldata, capabilities, access, responseCapacity, budget)
	if err != nil {
		return nil, err
	}
	if p == nil || p.host == nil || p.evmKeeper == nil || call.evm == nil || call.evm.StateDB == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: call requires host, EVM keeper and EVM state")
	}
	receipt, err := submit(ctx, p, call, 0x00090003, payload, feeLimit)
	if err != nil {
		return nil, err
	}
	outcome := receipt.ProgramOutcome
	if outcome == nil {
		return nil, fmt.Errorf("LXP_PROGRAM_REFUSED: missing call program outcome")
	}
	output, err := method.Outputs.Pack(outcome.Status, outcome.Kind, outcome.Response)
	if err != nil {
		return nil, err
	}
	if err := emitCallEvents(call.evm, receipt); err != nil {
		return nil, err
	}
	return output, nil
}

func init() {
	register("call", callHandler)
}
