package layerxprograms

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	xwebkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/keeper"
	xwebtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

func callWithWebRequests(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
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
	cached, write := ctx.CacheContext()
	authority, did, err := principalOf(cached, p.evmKeeper, call.caller)
	if err != nil {
		return nil, err
	}
	sequence, err := p.host.ReadIdentitySequence(cached, did)
	if err != nil {
		return nil, err
	}
	_, main, err := lxphost.MainAccountID(authority.VerifiedKey)
	if err != nil {
		return nil, err
	}
	receipt, err := submit(cached, p, call, 0x00090003, payload, feeLimit)
	if err != nil {
		return nil, err
	}
	outcome := receipt.ProgramOutcome
	if outcome == nil {
		return nil, fmt.Errorf("LXP_PROGRAM_REFUSED: missing call program outcome")
	}
	programCall := &xwebkeeper.ProgramCall{ProgramID: xwebtypes.Hash32(programID), Principal: lxphost.BankAddress(main), Sequence: sequence}
	keeper, _ := cached.Value(xwebContextKey{}).(*xwebkeeper.Keeper)
	for _, leg := range receipt.Transfers {
		denom := ""
		if keeper != nil {
			denom, _ = keeper.ProgramFeeDenom(cached, xwebtypes.Hash32(leg.Asset))
		}
		programCall.Legs = append(programCall.Legs, xwebkeeper.ProgramLeg{From: lxphost.BankAddress(leg.From), To: lxphost.BankAddress(leg.To), Asset: xwebtypes.Hash32(leg.Asset), Denom: denom, Amount: leg.Amount})
	}
	if err := commitWebRequestReceipt(cached, p, programCall, receipt); err != nil {
		return nil, err
	}
	output, err := method.Outputs.Pack(outcome.Status, outcome.Kind, outcome.Response)
	if err != nil {
		return nil, err
	}
	if err := emitCallEvents(call.evm, receipt); err != nil {
		return nil, err
	}
	write()
	ctx.EventManager().EmitEvents(cached.EventManager().Events())
	return output, nil
}

type xwebContextKey struct{}

func configureXWeb(p *PrecompileExecutor, keeper *xwebkeeper.Keeper) {
	table := make(handlerTable, len(handlers))
	for name, h := range handlers {
		table[name] = h
	}
	table["call"] = func(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
		return callWithWebRequests(ctx.WithValue(xwebContextKey{}, keeper), p, call, args)
	}
	p.handlers = table
}

func commitWebRequestReceipt(ctx sdk.Context, p *PrecompileExecutor, call *xwebkeeper.ProgramCall, receipt lxphost.Receipt) error {
	cached, write := ctx.CacheContext()
	if err := forwardWebRequests(cached, p, call, receipt); err != nil {
		return err
	}
	keeper, _ := cached.Value(xwebContextKey{}).(*xwebkeeper.Keeper)
	if keeper != nil {
		for _, leg := range call.Legs {
			if leg.Amount.IsNil() || !leg.Amount.IsPositive() || leg.Denom == "" {
				return fmt.Errorf("LXP_NON_CANONICAL: invalid program transfer leg")
			}
			if err := p.evmKeeper.BankKeeper().SendCoins(cached, leg.From, leg.To, sdk.NewCoins(sdk.NewCoin(leg.Denom, leg.Amount))); err != nil {
				return err
			}
		}
	}
	write()
	ctx.EventManager().EmitEvents(cached.EventManager().Events())
	return nil
}

func forwardWebRequests(ctx sdk.Context, p *PrecompileExecutor, call *xwebkeeper.ProgramCall, receipt lxphost.Receipt) error {
	if receipt.ProgramOutcome == nil {
		return fmt.Errorf("LXP_PROGRAM_REFUSED: missing call program outcome")
	}
	if receipt.ResultCode != 0 {
		return kernelError(receipt.ResultCode, "call")
	}
	if receipt.ProgramOutcome.Kind != 1 {
		return nil
	}
	const domain = "LayerX/programs/events/v1\x00"
	raw := receipt.ProgramOutcome.Events
	if len(raw) < len(domain)+4 || !bytes.Equal(raw[:len(domain)], []byte(domain)) {
		return fmt.Errorf("LXP_NON_CANONICAL: invalid program event envelope")
	}
	count := binary.BigEndian.Uint32(raw[len(domain):])
	if count > 512 {
		return fmt.Errorf("LXP_LENGTH_LIMIT: too many program events")
	}
	raw = raw[len(domain)+4:]
	for i := uint32(0); i < count; i++ {
		if len(raw) < 77 {
			return fmt.Errorf("LXP_NON_CANONICAL: truncated program event")
		}
		program := xwebtypes.Hash32(raw[:32])
		depth := raw[72]
		if depth > 8 {
			return fmt.Errorf("LXP_NON_CANONICAL: invalid program event frame")
		}
		for j, component := range raw[64:72] {
			if (j < int(depth)) != (component != 0) {
				return fmt.Errorf("LXP_NON_CANONICAL: invalid program event frame")
			}
		}
		topicLength := binary.BigEndian.Uint32(raw[73:77])
		raw = raw[77:]
		if topicLength > 64 || uint64(topicLength)+4 > uint64(len(raw)) {
			return fmt.Errorf("LXP_NON_CANONICAL: invalid program event topic")
		}
		topic := raw[:int(topicLength)]
		raw = raw[int(topicLength):]
		dataLength := binary.BigEndian.Uint32(raw[:4])
		raw = raw[4:]
		if dataLength > 65536 || uint64(dataLength) > uint64(len(raw)) {
			return fmt.Errorf("LXP_NON_CANONICAL: invalid program event data")
		}
		data := raw[:int(dataLength)]
		raw = raw[int(dataLength):]
		if !bytes.Equal(topic, []byte(xwebkeeper.ProgramRequestTopic)) {
			continue
		}
		if p == nil || call == nil {
			return fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: program call is unavailable")
		}
		keeper, _ := ctx.Value(xwebContextKey{}).(*xwebkeeper.Keeper)
		if keeper == nil {
			return fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: xweb keeper is unavailable")
		}
		original := call.ProgramID
		call.ProgramID = program
		err := keeper.RequestFromProgram(ctx, call, topic, data)
		call.ProgramID = original
		if err != nil {
			return err
		}
	}
	if len(raw) != 0 {
		return fmt.Errorf("LXP_NON_CANONICAL: trailing program event bytes")
	}
	return nil
}
