package layerxauth

import (
	"crypto/sha256"
	"fmt"

	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
)

const GrantRevoked = "GrantRevoked"
const IdentityStateUpdated = "IdentityStateUpdated"

func init() {
	authHandlers["revokeGrant"] = revokeGrant
}

func revokeGrant(p *PrecompileExecutor, ctx sdk.Context, method *abi.Method, actor authkeeper.Actor, args []interface{}, evm *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	if err := pcommon.ValidateArgsLength(args, 3); err != nil {
		return nil, err
	}
	var input struct {
		GrantId  [32]byte
		Reason   uint8
		Sequence uint64
	}
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	payload := types.EncodeRevokePayload(input.GrantId, input.Reason, input.Sequence)
	key := append([]byte("layerxauth/revokeGrant"), actor.Authority.Actor[:]...)
	idempotencyKey := sha256.Sum256(append(key, payload...))
	result, err := p.keeper.RevokeGrant(ctx, actor, input.GrantId, input.Reason, input.Sequence, idempotencyKey)
	if err != nil {
		return nil, RevertError(err)
	}
	events, err := authkeeper.AuthEvents(result.Receipt)
	if err != nil {
		return nil, RevertError(err)
	}
	for _, event := range events {
		var topic common.Hash
		switch event.Name {
		case GrantRevoked:
			topic = event.GrantID
		case IdentityStateUpdated:
			topic = event.DIDID
		default:
			return nil, types.ErrNonCanonical.Wrap("unexpected revoke event")
		}
		if err := p.log(evm, event.Name, []common.Hash{topic}, event.Body); err != nil {
			return nil, err
		}
	}
	return method.Outputs.Pack()
}

func (p *PrecompileExecutor) log(evm *vm.EVM, name string, topics []common.Hash, data ...interface{}) error {
	event, ok := p.abi.Events[name]
	if !ok {
		return fmt.Errorf("layerxauth: unknown event %s", name)
	}
	packed, err := event.Inputs.NonIndexed().Pack(data...)
	if err != nil {
		return err
	}
	return pcommon.EmitEVMLog(evm, p.address, append([]common.Hash{event.ID}, topics...), packed)
}
