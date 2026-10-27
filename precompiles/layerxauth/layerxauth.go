package layerxauth

import (
	"errors"
	"fmt"
	"math/big"

	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
)

const AuthAddress = "0x000000000000000000000000000000000000101C"
const PrecompileName = "layerxauth"

type AuthKeepers interface {
	LayerXAuthK() *authkeeper.Keeper
}

type PrecompileExecutor struct {
	abi     abi.ABI
	address common.Address
	keeper  *authkeeper.Keeper
}

type authHandler func(*PrecompileExecutor, sdk.Context, *abi.Method, authkeeper.Actor, []interface{}, *vm.EVM, *tracing.Hooks) ([]byte, error)

var authHandlers = map[string]authHandler{}

var _ pcommon.PrecompileExecutor = (*PrecompileExecutor)(nil)

func init() {
	if err := precompiles.RegisterMergeConstructor(common.HexToAddress(AuthAddress), NewMergePrecompile); err != nil {
		panic(err)
	}
}

func NewPrecompile(keepers utils.Keepers) (*pcommon.Precompile, error) {
	a := GetABI()
	p := &PrecompileExecutor{abi: a, address: common.HexToAddress(AuthAddress)}
	if provider, ok := keepers.(AuthKeepers); ok {
		p.keeper = provider.LayerXAuthK()
	}
	return pcommon.NewPrecompile(a, p, p.address, PrecompileName).WithLXPReasons().WithGasTable(GasTable), nil
}

func NewMergePrecompile(keepers utils.Keepers, _ string) (precompiles.IPrecompile, error) {
	return NewPrecompile(keepers)
}

func (p *PrecompileExecutor) RequiredGas(input []byte, method *abi.Method) uint64 {
	row, ok := GasTable[method.Name]
	if !ok {
		return pcommon.UnknownMethodCallGas
	}
	return row.Gas(input)
}

func (p *PrecompileExecutor) Execute(ctx sdk.Context, method *abi.Method, caller common.Address, _ common.Address, args []interface{}, value *big.Int, readOnly bool, evm *vm.EVM, hooks *tracing.Hooks) ([]byte, error) {
	if ctx.EVMPrecompileCalledFromDelegateCall() {
		return nil, fmt.Errorf("%w: cannot delegatecall layerxauth", types.ErrAuthScope)
	}
	if err := pcommon.ValidateNonPayable(value); err != nil {
		return nil, err
	}
	if readOnly && !method.IsConstant() {
		return nil, fmt.Errorf("%w: cannot call a layerxauth state change from staticcall", types.ErrAuthScope)
	}
	if p.keeper == nil {
		return nil, errors.New("LXP_PRECOMPILE_NO_CONTEXT: layerxauth keeper is unavailable")
	}
	actor := authkeeper.Actor{}
	if !method.IsConstant() {
		var err error
		actor, err = p.keeper.Actor(ctx, caller)
		if err != nil {
			return nil, RevertError(err)
		}
	}
	handler, ok := authHandlers[method.Name]
	if !ok {
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: layerxauth method %s is unavailable", method.Name)
	}
	ret, err := handler(p, ctx, method, actor, args, evm, hooks)
	return ret, RevertError(err)
}
