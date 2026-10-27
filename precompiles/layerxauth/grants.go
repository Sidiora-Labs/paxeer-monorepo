package layerxauth

import (
	"crypto/sha256"

	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
)

const (
	SessionGranted    = "SessionGranted"
	GrantBody         = "GrantBody"
	CapabilityGranted = "CapabilityGranted"
)

func init() {
	authHandlers["grantSession"] = (*PrecompileExecutor).grantSession
	authHandlers["grantSessionSuccessor"] = (*PrecompileExecutor).grantSessionSuccessor
	authHandlers["grantCapability"] = (*PrecompileExecutor).grantCapability
}

func (p *PrecompileExecutor) grantArguments(method *abi.Method, actor authkeeper.Actor, args []interface{}) ([]interface{}, [32]byte, error) {
	if err := pcommon.ValidateArgsLength(args, len(method.Inputs)); err != nil {
		return nil, [32]byte{}, err
	}
	packed, err := method.Inputs.Pack(args...)
	if err != nil {
		return nil, [32]byte{}, err
	}
	values, err := method.Inputs.Unpack(packed)
	if err != nil {
		return nil, [32]byte{}, err
	}
	preimage := append([]byte("layerxauth/grants/v1"), actor.DIDKey[:]...)
	preimage = append(preimage, method.ID...)
	key := sha256.Sum256(append(preimage, packed...))
	return values, key, nil
}

func (p *PrecompileExecutor) grantSession(ctx sdk.Context, method *abi.Method, actor authkeeper.Actor, args []interface{}, evm *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	values, key, err := p.grantArguments(method, actor, args)
	if err != nil {
		return nil, err
	}
	result, err := p.keeper.GrantSession(ctx, actor, values[0].([]byte), values[1].(uint64), values[2].([32]byte), key)
	if err != nil {
		return nil, RevertError(err)
	}
	return p.grantResult(method, result, SessionGranted, evm)
}

func (p *PrecompileExecutor) grantSessionSuccessor(ctx sdk.Context, method *abi.Method, actor authkeeper.Actor, args []interface{}, evm *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	values, key, err := p.grantArguments(method, actor, args)
	if err != nil {
		return nil, err
	}
	result, err := p.keeper.GrantSessionSuccessor(ctx, actor, values[0].([]byte), values[1].(uint64), values[2].([32]byte), values[3].([32]byte), values[4].([32]byte), key)
	if err != nil {
		return nil, RevertError(err)
	}
	return p.grantResult(method, result, SessionGranted, evm)
}

func (p *PrecompileExecutor) grantCapability(ctx sdk.Context, method *abi.Method, actor authkeeper.Actor, args []interface{}, evm *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	values, key, err := p.grantArguments(method, actor, args)
	if err != nil {
		return nil, err
	}
	result, err := p.keeper.GrantCapability(ctx, actor, values[0].([]byte), key)
	if err != nil {
		return nil, RevertError(err)
	}
	return p.grantResult(method, result, CapabilityGranted, evm)
}

func (p *PrecompileExecutor) grantResult(method *abi.Method, result authkeeper.Result, granted string, evm *vm.EVM) ([]byte, error) {
	events, err := authkeeper.AuthEvents(result.Receipt)
	if err != nil {
		return nil, RevertError(err)
	}
	var id [32]byte
	found := false
	for _, event := range events {
		if event.Name == granted {
			if found {
				return nil, types.ErrNonCanonical.Wrap("multiple grant events in receipt")
			}
			id, found = event.GrantID, true
		}
	}
	if !found {
		return nil, types.ErrNonCanonical.Wrap("missing grant event in receipt")
	}
	for _, event := range events {
		switch event.Name {
		case SessionGranted:
			err = p.log(evm, event.Name, []common.Hash{event.GrantID}, event.Body)
		case GrantBody:
			err = p.log(evm, event.Name, []common.Hash{event.GrantID}, event.Sub, event.Chunk, event.Body)
		case CapabilityGranted:
			err = p.log(evm, event.Name, []common.Hash{event.GrantID})
		case "IdentityStateUpdated":
			err = p.log(evm, event.Name, []common.Hash{event.DIDID}, event.Body)
		default:
			return nil, types.ErrNonCanonical.Wrap("unexpected grant receipt event")
		}
		if err != nil {
			return nil, err
		}
	}
	return method.Outputs.Pack(id)
}
