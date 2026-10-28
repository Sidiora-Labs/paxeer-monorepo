package layerxprograms

import (
	"encoding/binary"
	"fmt"
	"math"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func init() {
	register("simulateCall", simulateHandler)
}

func simulateHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	if call.method == nil || call.method.Name != "simulateCall" {
		return nil, errGateUnknownMethod
	}
	if ctx.EVMPrecompileCalledFromDelegateCall() {
		return nil, fmt.Errorf("LXP_PROGRAM_REFUSED: delegatecall is not allowed")
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
	if p == nil || p.host == nil || p.evmKeeper == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: simulation requires host and EVM keeper")
	}
	authority, did, err := principalOf(ctx, p.evmKeeper, call.caller)
	if err != nil {
		return nil, err
	}
	accountSequence, err := p.host.ReadIdentitySequence(ctx, did)
	if err != nil {
		return nil, err
	}
	lastSequence, err := p.host.ReadLastSequence(ctx, did)
	if err != nil {
		return nil, err
	}
	if lastSequence == math.MaxUint64 {
		return nil, fmt.Errorf("LXP_OVERFLOW: host sequence exhausted")
	}
	txHash := [32]byte(ctx.EVMTxHash())
	ordinal, err := simulationOrdinal(ctx, txHash)
	if err != nil {
		return nil, err
	}
	var selector [4]byte
	copy(selector[:], programsABI.Methods["call"].ID)
	timestamp := uint64(ctx.BlockTime().UnixMilli())
	receipt, err := p.host.Simulate(lxphost.WithSequence(ctx, lastSequence+1), lxphost.Activity{
		ProtocolVersion: 3,
		Type:            0x00090003,
		ActorDID:        did,
		AuthorityKey:    authority.VerifiedKey,
		AccountSequence: accountSequence,
		NotBefore:       timestamp,
		NotAfter:        timestamp,
		IdempotencyKey:  IdempotencyKey(txHash, ordinal, selector),
		FeeLimit:        feeLimit,
		Payload:         payload,
	}, authority)
	if err != nil {
		return nil, err
	}
	if err := kernelError(receipt.ResultCode, call.method.Name); err != nil {
		return nil, err
	}
	if receipt.ProgramOutcome == nil {
		return nil, fmt.Errorf("LXP_FATAL_INVARIANT: missing simulation program outcome")
	}
	outcome := receipt.ProgramOutcome
	return call.method.Outputs.Pack(outcome.Status, outcome.Kind, outcome.Response)
}

func simulationOrdinal(ctx sdk.Context, txHash [32]byte) (uint32, error) {
	for _, key := range ctx.MultiStore().StoreKeys() {
		if key.Name() != evmtypes.TransientStoreKey {
			continue
		}
		transient, ok := key.(*sdk.TransientStoreKey)
		if !ok {
			break
		}
		previous := ctx.TransientStore(transient).Get(append([]byte(ordinalKeyPrefix), txHash[:]...))
		if previous == nil {
			return 0, nil
		}
		if len(previous) != 4 {
			return 0, fmt.Errorf("LXP_FATAL_INVARIANT: invalid call ordinal")
		}
		last := binary.BigEndian.Uint32(previous)
		if last == math.MaxUint32 {
			return 0, fmt.Errorf("LXP_OVERFLOW: call ordinal exhausted")
		}
		return last + 1, nil
	}
	return 0, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: EVM transient store is not mounted")
}
