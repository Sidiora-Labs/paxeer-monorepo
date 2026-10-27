package xweb

import (
	"fmt"

	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/xweb/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

func (p PrecompileExecutor) fulfilProgram(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	if err := pcommon.ValidateArgsLength(args, 1); err != nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_ARG_COUNT: %w", err)
	}
	if err := p.keeper.FulfilProgram(ctx, args[0].([]byte)); err != nil {
		return nil, err
	}
	return method.Outputs.Pack()
}

func (p PrecompileExecutor) getProgramResult(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	if err := pcommon.ValidateArgsLength(args, 2); err != nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_ARG_COUNT: %w", err)
	}
	result, err := p.keeper.GetProgramResult(ctx, types.Hash32(args[0].([32]byte)), args[1].(uint64))
	if err != nil {
		return nil, err
	}
	return method.Outputs.Pack([32]byte(result.ContentDigest), result.FullLength, result.ResponseLength, result.Response)
}

func (p PrecompileExecutor) getKernelNetworkId(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, error) {
	if err := pcommon.ValidateArgsLength(args, 0); err != nil {
		return nil, fmt.Errorf("LXP_PRECOMPILE_ARG_COUNT: %w", err)
	}
	return method.Outputs.Pack(p.keeper.GetParams(ctx).KernelNetworkId)
}
