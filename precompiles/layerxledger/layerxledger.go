package layerxledger

import (
	"errors"
	"fmt"
	"math/big"

	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
)

const (
	LedgerAddress  = "0x000000000000000000000000000000000000101A"
	PrecompileName = "layerxledger"
)

type LedgerKeepers interface {
	LayerXLedgerK() *ledgerkeeper.Keeper
}

type PrecompileExecutor struct {
	abi     abi.ABI
	address common.Address
	keeper  *ledgerkeeper.Keeper
}

type ledgerMethod func(*PrecompileExecutor, sdk.Context, *abi.Method, ledgerkeeper.Actor, []interface{}, *vm.EVM, *tracing.Hooks) ([]byte, error)

var ledgerMethods = map[string]ledgerMethod{}

var _ pcommon.PrecompileExecutor = (*PrecompileExecutor)(nil)

func NewPrecompile(keepers utils.Keepers) (*pcommon.Precompile, error) {
	a := GetABI()
	p := &PrecompileExecutor{abi: a, address: common.HexToAddress(LedgerAddress)}
	if provider, ok := keepers.(LedgerKeepers); ok {
		p.keeper = provider.LayerXLedgerK()
	}
	return pcommon.NewPrecompile(a, p, p.address, PrecompileName).WithLXPReasons().WithGasTable(GasTable), nil
}

func NewMergePrecompile(keepers utils.Keepers, planName string) (precompiles.IPrecompile, error) {
	return NewPrecompile(keepers)
}

func init() {
	if err := precompiles.RegisterMergeConstructor(common.HexToAddress(LedgerAddress), NewMergePrecompile); err != nil {
		panic(err)
	}
}

func (p *PrecompileExecutor) RequiredGas(input []byte, method *abi.Method) uint64 {
	if method != nil {
		if row, ok := GasTable[method.Name]; ok {
			return row.Gas(input)
		}
	}
	return pcommon.UnknownMethodCallGas
}

func (p *PrecompileExecutor) Execute(ctx sdk.Context, method *abi.Method, caller common.Address, _ common.Address, args []interface{}, value *big.Int, readOnly bool, evm *vm.EVM, hooks *tracing.Hooks) ([]byte, error) {
	if ctx.EVMPrecompileCalledFromDelegateCall() {
		return nil, fmt.Errorf("LXP_UNAUTHORIZED_DEBIT: cannot delegatecall layerxledger")
	}
	if err := pcommon.ValidateNonPayable(value); err != nil {
		return nil, err
	}
	if method == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: missing layerxledger method")
	}
	canonical, err := p.abi.MethodById(method.ID)
	if err != nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: %w", err)
	}
	method = canonical
	if readOnly && !method.IsConstant() {
		return nil, fmt.Errorf("LXP_UNAUTHORIZED_DEBIT: cannot call a layerxledger state change from staticcall")
	}
	if p.keeper == nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_NO_CONTEXT: layerxledger keeper is unavailable")
	}
	var actor ledgerkeeper.Actor
	if !method.IsConstant() {
		actor, err = p.keeper.Actor(ctx, caller)
		if err != nil {
			return nil, RevertError(err)
		}
	}
	if err := pcommon.ValidateArgsLength(args, len(method.Inputs)); err != nil {
		return nil, err
	}
	handler, ok := ledgerMethods[method.Name]
	if !ok {
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: layerxledger method %s is unavailable", method.Name)
	}
	result, err := handler(p, ctx, method, actor, args, evm, hooks)
	if errors.Is(err, ledgerkeeper.ErrPausedScope) {
		err = revertError{cause: err, code: "LXP_PAUSED_SCOPE"}
	}
	return result, RevertError(err)
}
