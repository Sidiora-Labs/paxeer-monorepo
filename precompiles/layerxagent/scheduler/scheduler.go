package scheduler

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"

	evmkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/keeper"
	agentkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxagent/keeper"
	govtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxgov/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
)

const (
	SchedulerAddress = "0x0000000000000000000000000000000000000905"
	PrecompileName   = "scheduler"
	abiJSON          = `[
{"type":"function","name":"schedule","stateMutability":"payable","inputs":[{"name":"target","type":"address"},{"name":"callData","type":"bytes"},{"name":"executeAtBlock","type":"uint64"},{"name":"gasLimit","type":"uint64"}],"outputs":[{"name":"jobId","type":"uint256"}]},
{"type":"function","name":"cancel","stateMutability":"nonpayable","inputs":[{"name":"jobId","type":"uint256"}],"outputs":[]},
{"type":"function","name":"reschedule","stateMutability":"nonpayable","inputs":[{"name":"jobId","type":"uint256"},{"name":"newBlock","type":"uint64"}],"outputs":[]},
{"type":"function","name":"getJob","stateMutability":"view","inputs":[{"name":"jobId","type":"uint256"}],"outputs":[{"name":"id","type":"uint256"},{"name":"creator","type":"address"},{"name":"target","type":"address"},{"name":"callData","type":"bytes"},{"name":"executeAtBlock","type":"uint64"},{"name":"gasLimit","type":"uint64"},{"name":"deposit","type":"uint256"},{"name":"active","type":"bool"}]},
{"type":"function","name":"pending","stateMutability":"view","inputs":[{"name":"creator","type":"address"}],"outputs":[{"name":"jobIds","type":"uint256[]"}]},
{"type":"event","name":"JobScheduled","anonymous":false,"inputs":[{"name":"jobId","type":"uint256","indexed":true},{"name":"creator","type":"address","indexed":true},{"name":"target","type":"address","indexed":false},{"name":"executeAtBlock","type":"uint64","indexed":false},{"name":"gasLimit","type":"uint64","indexed":false},{"name":"deposit","type":"uint256","indexed":false}]},
{"type":"event","name":"JobCancelled","anonymous":false,"inputs":[{"name":"jobId","type":"uint256","indexed":true},{"name":"creator","type":"address","indexed":true},{"name":"refund","type":"uint256","indexed":false}]},
{"type":"event","name":"JobRescheduled","anonymous":false,"inputs":[{"name":"jobId","type":"uint256","indexed":true},{"name":"newBlock","type":"uint64","indexed":false}]}
]`
)

type SchedulerPrecompile struct{ *agentcommon.Precompile }

type PrecompileExecutor struct {
	keeper agentkeeper.Keeper
	evm    *evmkeeper.Keeper
	abi    abi.ABI
}

var _ pcommon.PrecompileExecutor = PrecompileExecutor{}
var _ vm.PrecompiledContract = (*SchedulerPrecompile)(nil)

func NewPrecompile(keeper agentkeeper.Keeper, evm *evmkeeper.Keeper, upgrade *upgradekeeper.Keeper, planName string) (*SchedulerPrecompile, error) {
	if evm == nil {
		return nil, fmt.Errorf("%w: scheduler needs an EVM keeper", agentcommon.ErrNoContext)
	}
	a, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", agentcommon.ErrABIDecode, err)
	}
	return &SchedulerPrecompile{agentcommon.NewPrecompile(a, PrecompileExecutor{keeper, evm, a}, common.HexToAddress(SchedulerAddress), PrecompileName, upgrade, planName)}, nil
}

func (p PrecompileExecutor) RequiredGas(input []byte, method *abi.Method) uint64 {
	if method == nil {
		return pcommon.UnknownMethodCallGas
	}
	if len(input) > math.MaxInt-4 {
		return math.MaxUint64
	}
	return agentcommon.MethodGas(method.ID, len(input)+4)
}

func (p PrecompileExecutor) Execute(ctx sdk.Context, method *abi.Method, caller common.Address, _ common.Address, args []interface{}, value *big.Int, readOnly bool, evm *vm.EVM, hooks *tracing.Hooks) ([]byte, error) {
	if err := agentcommon.HeightGate(ctx); err != nil {
		return nil, err
	}
	if method == nil {
		return nil, agentcommon.ErrUnknownMethod
	}
	counts := map[string]int{"schedule": 4, "cancel": 1, "reschedule": 2, "getJob": 1, "pending": 1}
	count, known := counts[method.Name]
	if !known {
		return nil, agentcommon.ErrUnknownMethod
	}
	if method.Name != "schedule" {
		if err := agentcommon.ValidateNonPayable(value); err != nil {
			return nil, err
		}
	}
	if method.Name == "schedule" || method.Name == "cancel" || method.Name == "reschedule" {
		if err := agentcommon.GuardWrite(ctx, readOnly, false); err != nil {
			return nil, err
		}
	}
	if err := agentcommon.ValidateArgsLength(args, count); err != nil {
		return nil, err
	}
	switch method.Name {
	case "schedule":
		target, okTarget := args[0].(common.Address)
		data, okData := args[1].([]byte)
		at, okAt := args[2].(uint64)
		gas, okGas := args[3].(uint64)
		if !okTarget || !okData || !okAt || !okGas || value == nil || value.Sign() < 0 || value.BitLen() > 256 {
			return nil, agentcommon.ErrABIDecode
		}
		if evm == nil || p.evm == nil {
			return nil, agentcommon.ErrNoContext
		}
		account := p.evm.GetPaxAddressOrDefault(ctx, common.HexToAddress(SchedulerAddress))
		// EVM CALL already credited value here; keep the helper's refund in the payer account used for escrow.
		if _, _, err := pcommon.HandlePaymentUhpxWei(ctx, account, account, value, p.evm.BankKeeper(), p.evm, hooks, evm.GetDepth()); err != nil {
			return nil, keeperError(err)
		}
		id, err := p.keeper.Schedule(ctx, account, caller, target, data, at, gas, value)
		if err != nil {
			return nil, keeperError(err)
		}
		jobID := new(big.Int).SetUint64(id)
		if err := p.emit(evm, "JobScheduled", jobID, &caller, target, at, gas, value); err != nil {
			return nil, err
		}
		return pack(method, jobID)
	case "cancel", "reschedule", "getJob":
		id, ok := args[0].(*big.Int)
		if !ok || id == nil || id.Sign() < 0 || id.BitLen() > 256 {
			return nil, agentcommon.ErrABIDecode
		}
		if method.Name == "getJob" {
			job := agentkeeper.Job{Deposit: sdk.ZeroInt()}
			if id.IsUint64() {
				if stored, found := p.keeper.GetJob(ctx, id.Uint64()); found {
					job = stored
				}
			}
			return pack(method, new(big.Int).SetUint64(job.ID), job.Creator, job.Target, job.CallData, job.ExecuteAtBlock, job.GasLimit, job.Deposit.BigInt(), job.Active)
		}
		if !id.IsUint64() {
			return nil, agentcommon.ErrABIDecode
		}
		if method.Name == "cancel" {
			refund, err := p.keeper.Cancel(ctx, caller, id.Uint64())
			if err != nil {
				return nil, keeperError(err)
			}
			if err := p.emit(evm, "JobCancelled", id, &caller, refund.BigInt()); err != nil {
				return nil, err
			}
		} else {
			at, ok := args[1].(uint64)
			if !ok {
				return nil, agentcommon.ErrABIDecode
			}
			if err := p.keeper.Reschedule(ctx, caller, id.Uint64(), at); err != nil {
				return nil, keeperError(err)
			}
			if err := p.emit(evm, "JobRescheduled", id, nil, at); err != nil {
				return nil, err
			}
		}
		return pack(method)
	case "pending":
		creator, ok := args[0].(common.Address)
		if !ok {
			return nil, agentcommon.ErrABIDecode
		}
		ids := p.keeper.Pending(ctx, creator)
		out := make([]*big.Int, len(ids))
		for i, id := range ids {
			out[i] = new(big.Int).SetUint64(id)
		}
		return pack(method, out)
	}
	return nil, agentcommon.ErrUnknownMethod
}

func keeperError(err error) error {
	if errors.Is(err, govtypes.ErrPausedScope) {
		err = fmt.Errorf("%s: %w", govtypes.ErrPausedScope.Error(), err)
	}
	return agentcommon.RevertError(err)
}

func pack(method *abi.Method, values ...interface{}) ([]byte, error) {
	data, err := method.Outputs.Pack(values...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", agentcommon.ErrABIDecode, err)
	}
	return data, nil
}

func (p PrecompileExecutor) emit(evm *vm.EVM, name string, id *big.Int, creator *common.Address, values ...interface{}) error {
	event := p.abi.Events[name]
	data, err := event.Inputs.NonIndexed().Pack(values...)
	if err != nil {
		return fmt.Errorf("%w: %v", agentcommon.ErrABIDecode, err)
	}
	topics := []common.Hash{event.ID, common.BigToHash(id)}
	if creator != nil {
		topics = append(topics, common.BytesToHash(creator.Bytes()))
	}
	return pcommon.EmitEVMLog(evm, common.HexToAddress(SchedulerAddress), topics, data)
}
