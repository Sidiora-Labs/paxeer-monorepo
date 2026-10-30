package streams

import (
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	paykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	agentcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxagent/common"
	payprecompile "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxpay"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	upgradekeeper "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/upgrade/keeper"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
)

const (
	StreamsAddress = "0x0000000000000000000000000000000000000906"
	PrecompileName = "streams"
	abiJSON        = `[
 {"type":"function","name":"open","stateMutability":"nonpayable","inputs":[{"name":"payee","type":"address"},{"name":"token","type":"address"},{"name":"ratePerSecond","type":"uint256"},{"name":"startTime","type":"uint64"},{"name":"stopTime","type":"uint64"},{"name":"cap","type":"uint256"}],"outputs":[{"name":"streamId","type":"uint256"}]},
 {"type":"function","name":"settle","stateMutability":"nonpayable","inputs":[{"name":"streamId","type":"uint256"}],"outputs":[{"name":"paid","type":"uint256"}]},
 {"type":"function","name":"close","stateMutability":"nonpayable","inputs":[{"name":"streamId","type":"uint256"}],"outputs":[{"name":"finalPaid","type":"uint256"}]},
 {"type":"function","name":"updateRate","stateMutability":"nonpayable","inputs":[{"name":"streamId","type":"uint256"},{"name":"newRate","type":"uint256"}],"outputs":[]},
 {"type":"function","name":"accrued","stateMutability":"view","inputs":[{"name":"streamId","type":"uint256"}],"outputs":[{"name":"amount","type":"uint256"}]},
 {"type":"function","name":"getStream","stateMutability":"view","inputs":[{"name":"streamId","type":"uint256"}],"outputs":[{"name":"stream","type":"tuple","components":[{"name":"id","type":"uint256"},{"name":"payer","type":"address"},{"name":"payee","type":"address"},{"name":"token","type":"address"},{"name":"ratePerSecond","type":"uint256"},{"name":"cap","type":"uint256"},{"name":"startTime","type":"uint64"},{"name":"stopTime","type":"uint64"},{"name":"settled","type":"uint256"},{"name":"active","type":"bool"}]}]},
 {"type":"event","name":"StreamOpened","anonymous":false,"inputs":[{"name":"streamId","type":"uint256","indexed":true},{"name":"payer","type":"address","indexed":true},{"name":"payee","type":"address","indexed":true},{"name":"token","type":"address","indexed":false},{"name":"ratePerSecond","type":"uint256","indexed":false},{"name":"cap","type":"uint256","indexed":false}]},
 {"type":"event","name":"StreamSettled","anonymous":false,"inputs":[{"name":"streamId","type":"uint256","indexed":true},{"name":"payee","type":"address","indexed":true},{"name":"paid","type":"uint256","indexed":false},{"name":"committed","type":"uint256","indexed":false}]},
 {"type":"event","name":"StreamClosed","anonymous":false,"inputs":[{"name":"streamId","type":"uint256","indexed":true},{"name":"finalPaid","type":"uint256","indexed":false},{"name":"refund","type":"uint256","indexed":false}]},
 {"type":"event","name":"StreamRateUpdated","anonymous":false,"inputs":[{"name":"streamId","type":"uint256","indexed":true},{"name":"oldRate","type":"uint256","indexed":false},{"name":"newRate","type":"uint256","indexed":false}]}
 ]`
)

type streamTuple struct {
	Id                  *big.Int
	Payer, Payee, Token common.Address
	RatePerSecond, Cap  *big.Int
	StartTime, StopTime uint64
	Settled             *big.Int
	Active              bool
}

type PrecompileExecutor struct {
	pay *paykeeper.Keeper
	abi abi.ABI
}

var _ pcommon.PrecompileExecutor = (*PrecompileExecutor)(nil)

func NewPrecompile(pay *paykeeper.Keeper, upgrade *upgradekeeper.Keeper, planName string) (*agentcommon.Precompile, error) {
	if pay == nil {
		return nil, fmt.Errorf("%w: pay keeper is unavailable", agentcommon.ErrNoContext)
	}
	if upgrade == nil || planName == "" {
		return nil, fmt.Errorf("%w: no merge upgrade activation reader bound", agentcommon.ErrNotActive)
	}
	a, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return nil, fmt.Errorf("%w: %v", agentcommon.ErrABIDecode, err)
	}
	return agentcommon.NewPrecompile(a, &PrecompileExecutor{pay, a}, common.HexToAddress(StreamsAddress), PrecompileName, upgrade, planName), nil
}

func (p *PrecompileExecutor) RequiredGas(input []byte, method *abi.Method) uint64 {
	if method == nil {
		return pcommon.UnknownMethodCallGas
	}
	if len(input) > math.MaxInt-4 {
		return math.MaxUint64
	}
	return agentcommon.MethodGas(method.ID, len(input)+4)
}

func (p *PrecompileExecutor) Execute(ctx sdk.Context, method *abi.Method, caller common.Address, _ common.Address, args []interface{}, value *big.Int, readOnly bool, evm *vm.EVM, _ *tracing.Hooks) (output []byte, err error) {
	defer func() {
		if err != nil {
			err = agentcommon.RevertError(err)
		}
	}()
	if err = agentcommon.HeightGate(ctx); err != nil {
		return nil, err
	}
	if p == nil || p.pay == nil {
		return nil, agentcommon.ErrNoContext
	}
	if method == nil {
		return nil, agentcommon.ErrUnknownMethod
	}
	method, err = p.abi.MethodById(method.ID)
	if err != nil {
		return nil, agentcommon.ErrUnknownMethod
	}
	if !method.IsConstant() {
		if err = agentcommon.GuardWrite(ctx, readOnly, false); err != nil {
			return nil, err
		}
	}
	if err = agentcommon.ValidateNonPayable(value); err != nil {
		return nil, err
	}
	if err = agentcommon.ValidateArgsLength(args, len(method.Inputs)); err != nil {
		return nil, err
	}
	encoded, err := method.Inputs.Pack(args...)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", agentcommon.ErrABIDecode, err)
	}
	args, err = method.Inputs.Unpack(encoded)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", agentcommon.ErrABIDecode, err)
	}
	if method.Name == "open" {
		rate, cap := args[2].(*big.Int), args[5].(*big.Int)
		if rate.BitLen() > 128 || cap.BitLen() > 128 {
			return nil, agentcommon.ErrOverflow
		}
		id, err := p.pay.OpenAgentStream(ctx, caller, args[0].(common.Address), args[1].(common.Address), rate, args[3].(uint64), args[4].(uint64), cap)
		if err != nil {
			return nil, err
		}
		if err = p.emit(evm, "StreamOpened", []common.Hash{common.BigToHash(id), common.BytesToHash(caller.Bytes()), common.BytesToHash(args[0].(common.Address).Bytes())}, args[1], rate, cap); err != nil {
			return nil, err
		}
		return method.Outputs.Pack(id)
	}
	id := args[0].(*big.Int)
	if method.Name == "updateRate" && args[1].(*big.Int).BitLen() > 128 {
		return nil, agentcommon.ErrOverflow
	}
	var stream paykeeper.AgentStream
	found := false
	if id.Sign() != 0 {
		stream, found, err = p.pay.AgentStreamView(ctx, id)
		if err != nil {
			return nil, err
		}
	}
	if method.Name == "getStream" {
		if !found {
			stream = paykeeper.AgentStream{ID: new(big.Int), RatePerSecond: new(big.Int), Cap: new(big.Int), Settled: new(big.Int)}
		}
		return method.Outputs.Pack(streamTuple{stream.ID, stream.Payer, stream.Payee, stream.Token, stream.RatePerSecond, stream.Cap, stream.StartTime, stream.StopTime, stream.Settled, stream.Active})
	}
	var kernelID [32]byte
	id.FillBytes(kernelID[:])
	if method.Name == "accrued" {
		if !found {
			return method.Outputs.Pack(new(big.Int))
		}
		accrued, _, err := p.pay.StreamAccrued(ctx, kernelID)
		if err != nil {
			return nil, err
		}
		return method.Outputs.Pack(accrued.BigInt())
	}
	switch method.Name {
	case "settle":
		paid, err := p.pay.SettleAgentStream(ctx, caller, id)
		if err != nil {
			return nil, err
		}
		if err = p.emit(evm, "StreamSettled", []common.Hash{common.BigToHash(id), common.BytesToHash(stream.Payee.Bytes())}, paid, paid); err != nil {
			return nil, err
		}
		return method.Outputs.Pack(paid)
	case "close":
		paid, refund, err := p.pay.CloseAgentStream(ctx, caller, id)
		if err != nil {
			return nil, err
		}
		if err = p.emit(evm, "StreamClosed", []common.Hash{common.BigToHash(id)}, paid, refund); err != nil {
			return nil, err
		}
		return method.Outputs.Pack(paid)
	case "updateRate":
		rate := args[1].(*big.Int)
		oldRate, err := p.pay.UpdateAgentStreamRate(ctx, caller, id, rate)
		if err != nil {
			return nil, err
		}
		after, _, err := p.pay.AgentStreamView(ctx, id)
		if err != nil {
			return nil, err
		}
		paid := new(big.Int).Sub(after.Settled, stream.Settled)
		old, err := streamU128(oldRate)
		if err != nil {
			return nil, err
		}
		next, err := streamU128(rate)
		if err != nil {
			return nil, err
		}
		committed, err := streamU128(paid)
		if err != nil {
			return nil, err
		}
		if err = p.emit(evm, "StreamRateUpdated", []common.Hash{common.BigToHash(id)}, oldRate, rate); err != nil {
			return nil, err
		}
		if err = payprecompile.EmitStreamRateUpdated(evm, kernelID, old, next, committed); err != nil {
			return nil, err
		}
		return method.Outputs.Pack()
	}
	return nil, agentcommon.ErrUnknownMethod
}

func streamU128(n *big.Int) (codec.U128, error) {
	if n == nil || n.Sign() < 0 || n.BitLen() > 128 {
		return codec.U128{}, agentcommon.ErrOverflow
	}
	return codec.U128{Hi: new(big.Int).Rsh(new(big.Int).Set(n), 64).Uint64(), Lo: n.Uint64()}, nil
}

func (p *PrecompileExecutor) emit(evm *vm.EVM, name string, indexed []common.Hash, values ...interface{}) error {
	event := p.abi.Events[name]
	data, err := event.Inputs.NonIndexed().Pack(values...)
	if err != nil {
		return err
	}
	return pcommon.EmitEVMLog(evm, common.HexToAddress(StreamsAddress), append([]common.Hash{event.ID}, indexed...), data)
}
