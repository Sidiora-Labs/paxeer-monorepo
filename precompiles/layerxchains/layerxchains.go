package layerxchains

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/state"
	chainskeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/keeper"
	chains "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxchains/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
)

//go:embed abi.json
var abiJSON []byte

type Executor struct {
	abi    abi.ABI
	keeper *chainskeeper.Keeper
	evm    chainsEVMKeeper
}

type Precompile struct {
	*pcommon.Precompile
	executor *Executor
	evm      utils.EVMKeeper
}

var _ pcommon.PrecompileExecutor = (*Executor)(nil)
var _ vm.DynamicGasPrecompiledContract = (*Precompile)(nil)

func NewPrecompile(keeper *chainskeeper.Keeper) (*pcommon.Precompile, error) {
	if keeper == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: layerxchains keeper is unavailable")
	}
	a, err := abi.JSON(bytes.NewReader(abiJSON))
	if err != nil {
		return nil, err
	}
	executor := &Executor{abi: a, keeper: keeper}
	return pcommon.NewPrecompile(a, executor, common.HexToAddress(ChainsAddress), PrecompileName).WithLXPReasons().WithGasTable(methodGas), nil
}

func NewServedPrecompile(keeper *chainskeeper.Keeper, evm utils.EVMKeeper) (*Precompile, error) {
	p, err := NewPrecompile(keeper)
	if err != nil {
		return nil, err
	}
	wired, ok := evm.(chainsEVMKeeper)
	if !ok || evm == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: EVM payment and minimum fee keeper is required")
	}
	executor := p.GetExecutor().(*Executor)
	executor.evm = wired
	return &Precompile{Precompile: p, executor: executor, evm: evm}, nil
}

func NewMergePrecompile(keepers utils.Keepers, _ string) (precompiles.IPrecompile, error) {
	wired, ok := keepers.(LayerXChainsKeepers)
	if !ok {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: LayerXChainsKeepers is required")
	}
	p, err := NewServedPrecompile(wired.LayerXChainsK(), keepers.EVMK())
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (p *Precompile) RunAndCalculateGas(evm *vm.EVM, caller, callingContract common.Address, input []byte,
	suppliedGas uint64, value *big.Int, hooks *tracing.Hooks, readOnly bool, isFromDelegateCall bool,
) (ret []byte, remainingGas uint64, err error) {
	operation := p.GetName() + "_unknown"
	defer func() {
		if recovered := recover(); recovered != nil {
			if _, outOfGas := recovered.(sdk.ErrorOutOfGas); !outOfGas {
				panic(recovered)
			}
			ret, remainingGas, err = nil, 0, vm.ErrOutOfGas
		}
		pcommon.HandlePrecompileError(err, evm, operation)
		if err != nil && !errors.Is(err, vm.ErrOutOfGas) {
			ret, err = RevertError(err), vm.ErrExecutionReverted
		}
	}()
	ctx, method, args, err := p.Prepare(evm, input)
	if err != nil {
		if suppliedGas < pcommon.UnknownMethodCallGas {
			return nil, 0, vm.ErrOutOfGas
		}
		return nil, suppliedGas - pcommon.UnknownMethodCallGas, err
	}
	operation = method.Name
	charge := methodGas[method.Name].Gas(input[4:])
	if suppliedGas < charge {
		return nil, 0, vm.ErrOutOfGas
	}
	limit := p.evm.GetCosmosGasLimitFromEVMGas(ctx.WithGasMeter(sdk.NewInfiniteGasMeterWithMultiplier(ctx)), suppliedGas-charge)
	if limit == 0 {
		return nil, 0, vm.ErrOutOfGas
	}
	ctx = ctx.WithGasMeter(sdk.NewGasMeterWithMultiplier(ctx, limit))
	em := ctx.EventManager()
	ctx = ctx.WithEventManager(sdk.NewEventManager()).WithEVMPrecompileCalledFromDelegateCall(isFromDelegateCall)
	ret, err = p.executor.Execute(ctx, method, caller, callingContract, args, value, readOnly, evm, hooks)
	if errors.Is(err, vm.ErrOutOfGas) {
		return nil, 0, err
	}
	remainingGas = p.evm.GetEVMGasLimitFromCtx(ctx)
	if err != nil {
		return ret, remainingGas, err
	}
	em.EmitEvents(ctx.EventManager().Events())
	return ret, remainingGas, nil
}

func init() {
	if err := precompiles.RegisterMergeConstructor(common.HexToAddress(ChainsAddress), NewMergePrecompile); err != nil {
		panic(err)
	}
}

func (p *Executor) RequiredGas(input []byte, method *abi.Method) uint64 {
	if method != nil {
		if row, ok := methodGas[method.Name]; ok {
			return row.Gas(input)
		}
	}
	return pcommon.UnknownMethodCallGas
}

func (p *Executor) Execute(ctx sdk.Context, method *abi.Method, caller, _ common.Address, args []interface{}, value *big.Int, readOnly bool, evm *vm.EVM, hooks *tracing.Hooks) ([]byte, error) {
	if p == nil || p.keeper == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: layerxchains keeper is unavailable")
	}
	if method == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: missing layerxchains method")
	}
	canonical, err := p.abi.MethodById(method.ID)
	if err != nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: %w", err)
	}
	method = canonical
	if method.StateMutability != "payable" {
		if err := pcommon.ValidateNonPayable(value); err != nil {
			return nil, err
		}
	}
	if ctx.EVMPrecompileCalledFromDelegateCall() {
		return nil, fmt.Errorf("LXP_UNAUTHORIZED_DEBIT: cannot delegatecall layerxchains")
	}
	if readOnly && !method.IsConstant() {
		return nil, fmt.Errorf("LXP_UNAUTHORIZED_DEBIT: layerxchains write under staticcall")
	}
	if err := pcommon.ValidateArgsLength(args, len(method.Inputs)); err != nil {
		return nil, err
	}
	if _, err := method.Inputs.Pack(args...); err != nil {
		return nil, fmt.Errorf("LXP_NON_CANONICAL: %w", err)
	}
	if method.IsConstant() {
		return p.view(ctx, method, args)
	}
	if evm == nil || evm.StateDB == nil || p.evm == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: layerxchains EVM is unavailable")
	}
	amount := sdk.ZeroInt()
	if method.StateMutability == "payable" {
		if value == nil || value.Sign() < 0 {
			return nil, chains.ErrBondTooLow
		}
		coin, err := pcommon.HandlePaymentUhpx(ctx, p.evm.GetPaxAddressOrDefault(ctx, common.HexToAddress(ChainsAddress)), p.evm.GetPaxAddressOrDefault(ctx, caller), value, p.evm.BankKeeper(), p.evm, hooks, evm.GetDepth())
		if err != nil {
			return nil, err
		}
		amount = coin.Amount
	}
	var id uint32
	if method.Name == "createChain" {
		domain, err := p.keeper.Create(ctx, caller, args[0].(string), args[1].(uint64), args[2].(uint64), args[3].(uint16), amount, p.evm.GetMinimumFeePerGas(ctx))
		if err != nil {
			return nil, err
		}
		if err := p.emitEvent(evm, "ChainCreated", domain.Id, &caller, domain.Name, chains.ChainIDOf(domain.Id).Uint64()); err != nil {
			return nil, err
		}
		return method.Outputs.Pack(domain.Id)
	}
	id = args[0].(uint32)
	switch method.Name {
	case "fundChain":
		err = p.keeper.Fund(ctx, id, caller, amount)
	case "pauseChain":
		err = p.keeper.Pause(ctx, caller, id)
	case "resumeChain":
		err = p.keeper.Resume(ctx, caller, id)
	case "retireChain":
		err = p.keeper.Retire(ctx, caller, id)
	case "governancePause":
		err = p.keeper.GovernancePause(ctx, caller, id, args[1].(bool))
	case "setOperator":
		err = p.keeper.SetOperator(ctx, caller, id, args[1].(common.Address))
	default:
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: %s", method.Name)
	}
	if err != nil {
		return nil, err
	}
	switch method.Name {
	case "fundChain":
		err = p.emitEvent(evm, "ChainFunded", id, &caller, chainsWei(amount))
	case "setOperator":
		next := args[1].(common.Address)
		err = p.emitEvent(evm, "ChainOperatorChanged", id, &next)
	default:
		domain, ok := p.keeper.GetDomain(ctx, id)
		if !ok {
			return nil, chains.ErrUnknownDomain
		}
		err = p.emitEvent(evm, "ChainStatusChanged", id, nil, domain.Status)
	}
	if err != nil {
		return nil, err
	}
	return method.Outputs.Pack()
}

func (p *Executor) view(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	switch method.Name {
	case "getChain":
		d, ok := p.keeper.GetDomain(ctx, args[0].(uint32))
		if !ok {
			return nil, chains.ErrUnknownDomain
		}
		return method.Outputs.Pack(d.Id, d.Name, d.Operator, d.Status, chains.ChainIDOf(d.Id).Uint64(), d.GasAllowance, d.ByteAllowance, d.FeeShareBps, chainsWei(d.Bond), p.keeper.NextBaseFee(ctx, d.Id).TruncateInt().BigInt(), uint64(d.CreatedHeight))
	case "chainByName":
		return method.Outputs.Pack(p.chainByName(ctx, args[0].(string)))
	case "chainCount":
		return method.Outputs.Pack(p.keeper.DomainCount(ctx))
	case "chainIdOf":
		return method.Outputs.Pack(chains.ChainIDOf(args[0].(uint32)).Uint64())
	case "params":
		params := p.keeper.GetParams(ctx)
		left := params.MaxTotalGasAllowance
		p.keeper.IterateDomains(ctx, func(d chains.Domain) bool {
			if d.Status == chains.StatusActive {
				left -= d.GasAllowance
			}
			return false
		})
		return method.Outputs.Pack(params.Authority, chainsWei(params.MinBond), chainsWei(params.CreationFee), params.MinGasAllowance, params.MaxGasAllowance, left, params.MaxChains)
	default:
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: %s", method.Name)
	}
}

func (p *Executor) chainByName(ctx sdk.Context, name string) uint32 {
	d, ok := p.keeper.GetDomainByName(ctx, name)
	if !ok {
		return uint32(0)
	}
	return d.Id
}

func chainsWei(amount sdk.Int) *big.Int {
	return new(big.Int).Mul(amount.BigInt(), state.UhpxToSweiMultiplier)
}
