package layerxauth

import (
	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
)

func init() {
	authHandlers["recordLifecycle"] = (*PrecompileExecutor).recordLifecycle
}

func (p *PrecompileExecutor) recordLifecycle(ctx sdk.Context, method *abi.Method, actor authkeeper.Actor, args []interface{}, _ *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	if err := pcommon.ValidateArgsLength(args, 2); err != nil {
		return nil, err
	}
	var input struct {
		Payload        []byte
		IdempotencyKey [32]byte
	}
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	result, err := p.keeper.RecordLifecycle(ctx, actor, input.Payload, input.IdempotencyKey)
	if err != nil {
		return nil, RevertError(err)
	}
	return method.Outputs.Pack(result.GlobalSequence)
}
