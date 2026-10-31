package layerxanchor_test

import (
	"math/big"
	"reflect"
	"testing"

	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/layerxanchor"
	"github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/utils"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/require"
)

func TestUnwiredAnchorRevertsModuleDisabled(t *testing.T) {
	precompile, err := layerxanchor.NewPrecompile(&utils.EmptyKeepers{})
	require.NoError(t, err)
	ctx := sdk.Context{}.WithBlockHeight(1)
	require.NotEmpty(t, precompile.GetABI().Methods)
	for name, method := range precompile.GetABI().Methods {
		t.Run(name, func(t *testing.T) {
			args := make([]interface{}, len(method.Inputs))
			for i, input := range method.Inputs {
				args[i] = reflect.Zero(input.Type.GetType()).Interface()
				if input.Type.T == abi.UintTy && input.Type.Size > 64 {
					args[i] = new(big.Int)
				}
			}
			input, err := method.Inputs.Pack(args...)
			require.NoError(t, err)
			args, err = method.Inputs.Unpack(input)
			require.NoError(t, err)
			for _, readOnly := range []bool{false, true} {
				ret, err := precompile.GetExecutor().Execute(ctx, &method, common.Address{}, common.Address{}, args, big.NewInt(0), readOnly, nil, nil)
				require.ErrorIs(t, err, vm.ErrExecutionReverted)
				require.Equal(t, pcommon.RetiredRevert(layerxanchor.PrecompileName), ret)
				reason, err := abi.UnpackRevert(ret)
				require.NoError(t, err)
				require.Equal(t, "LXP_MODULE_DISABLED: module disabled; layerxAnchor retired", reason)
			}
		})
	}
}
