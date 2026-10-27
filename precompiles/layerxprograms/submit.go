package layerxprograms

import (
	"fmt"
	"math"
	"math/big"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
)

type PrecompileExecutor struct {
	host      *lxphost.Host
	evmKeeper *evmkeeper.Keeper
	handlers  handlerTable
}

type callContext struct {
	caller common.Address
	method *abi.Method
	evm    *vm.EVM
}

type handler func(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error)

type handlerTable map[string]handler

func (t handlerTable) register(name string, h handler) {
	if _, exists := t[name]; exists {
		panic(fmt.Sprintf("layerxprograms: duplicate handler registration: %s", name))
	}
	t[name] = h
}

var handlers = handlerTable{}

func register(name string, h handler) { handlers.register(name, h) }

func (p *PrecompileExecutor) table() handlerTable {
	if p.handlers != nil {
		return p.handlers
	}
	return handlers
}

func submit(ctx sdk.Context, p *PrecompileExecutor, call callContext, activityType uint32, payload []byte, feeLimit [16]byte) (lxphost.Receipt, error) {
	if call.method == nil || len(call.method.ID) != 4 {
		return lxphost.Receipt{}, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: missing submission selector")
	}
	authority, did, err := principalOf(ctx, p.evmKeeper, call.caller)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	accountSequence, err := p.host.ReadIdentitySequence(ctx, did)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	lastSequence, err := p.host.ReadLastSequence(ctx, did)
	if err != nil {
		return lxphost.Receipt{}, err
	}
	if lastSequence == math.MaxUint64 {
		return lxphost.Receipt{}, fmt.Errorf("LXP_OVERFLOW: host sequence exhausted")
	}
	var selector [4]byte
	copy(selector[:], call.method.ID)
	txHash := [32]byte(ctx.EVMTxHash())
	timestamp := uint64(ctx.BlockTime().UnixMilli())
	activity := lxphost.Activity{
		ProtocolVersion: 3,
		Type:            activityType,
		ActorDID:        did,
		AuthorityKey:    authority.VerifiedKey,
		AccountSequence: accountSequence,
		NotBefore:       timestamp,
		NotAfter:        timestamp,
		IdempotencyKey:  IdempotencyKey(txHash, nextOrdinal(ctx, txHash), selector),
		FeeLimit:        feeLimit,
		Payload:         payload,
	}
	receipt, err := p.host.Execute(lxphost.WithSequence(ctx, lastSequence+1), activity, authority)
	if err != nil {
		return receipt, err
	}
	return receipt, kernelError(receipt.ResultCode, call.method.Name)
}

func (p *PrecompileExecutor) RequiredGas(input []byte, method *abi.Method) uint64 {
	return gasTable[method.Name].Gas(input)
}

func (p *PrecompileExecutor) Execute(ctx sdk.Context, method *abi.Method, caller common.Address, _ common.Address, args []interface{}, value *big.Int, readOnly bool, evm *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	if method == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: missing ABI method")
	}
	if readOnly && gasTable[method.Name].Write {
		return nil, ErrReadOnlyWrite
	}
	if ctx.EVMPrecompileCalledFromDelegateCall() {
		return nil, fmt.Errorf("LXP_PROGRAM_REFUSED: delegatecall is not allowed")
	}
	if err := pcommon.ValidateNonPayable(value); err != nil {
		return nil, err
	}
	h, ok := p.table()[method.Name]
	if !ok {
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: %s", method.Name)
	}
	return h(ctx, p, callContext{caller: caller, method: method, evm: evm}, args)
}

func registryReadHandler(ctx sdk.Context, p *PrecompileExecutor, call callContext, args []interface{}) ([]byte, error) {
	decoded, err := decodeArgs(call.method, args)
	if err != nil {
		return nil, err
	}
	payload, err := EncodeRegistryReadPayload(decoded[0].([32]byte), decoded[1].([]byte))
	if err != nil {
		return nil, err
	}
	_, err = submit(ctx, p, call, 0x00090004, payload, [16]byte{})
	return nil, err
}

func init() {
	register("registryRead", registryReadHandler)
}
