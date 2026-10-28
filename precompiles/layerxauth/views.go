package layerxauth

import (
	"encoding/binary"
	"fmt"
	"math/big"

	authkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxauth/keeper"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
)

func init() {
	authHandlers["resultByIdempotency"] = resultByIdempotency
	authHandlers["getGrant"] = getGrant
	authHandlers["isLive"] = isLive
	authHandlers["sessionKey"] = sessionKey
	authHandlers["capabilityScope"] = capabilityScope
	authHandlers["checkActivity"] = checkActivity
}

type grantViewInput struct {
	GrantID [32]byte `abi:"grantId"`
}

func resultByIdempotency(p *PrecompileExecutor, ctx sdk.Context, method *abi.Method, _ authkeeper.Actor, args []interface{}, _ *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	var input struct {
		DIDID          [32]byte `abi:"didId"`
		IdempotencyKey [32]byte `abi:"idempotencyKey"`
	}
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	receipt, err := p.keeper.ResultByIdempotency(ctx, input.DIDID, input.IdempotencyKey)
	if err != nil {
		return nil, RevertError(err)
	}
	return method.Outputs.Pack(receipt)
}

func getGrant(p *PrecompileExecutor, ctx sdk.Context, method *abi.Method, _ authkeeper.Actor, args []interface{}, _ *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	var input grantViewInput
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	body, err := p.keeper.GetGrant(ctx, input.GrantID)
	if err != nil {
		return nil, RevertError(err)
	}
	// The keeper's canonical grant has a 114-byte identity prefix, a
	// 180-byte scope, three uint64 values, then its revocation record.
	const revokedOffset = 114 + 180 + 3*8
	if len(body) < revokedOffset+9 || body[revokedOffset] > 1 {
		return nil, fmt.Errorf("invalid keeper grant revocation record")
	}
	return method.Outputs.Pack(body, body[revokedOffset] == 1, binary.BigEndian.Uint64(body[revokedOffset+1:revokedOffset+9]))
}

func isLive(p *PrecompileExecutor, ctx sdk.Context, method *abi.Method, _ authkeeper.Actor, args []interface{}, _ *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	var input struct {
		GrantID     [32]byte `abi:"grantId"`
		Sequence    uint64   `abi:"sequence"`
		TimestampMs uint64   `abi:"timestampMs"`
	}
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	live, reason, err := p.keeper.IsLive(ctx, input.GrantID, input.Sequence, input.TimestampMs)
	if err != nil {
		return nil, RevertError(err)
	}
	return method.Outputs.Pack(live, reason)
}

func sessionKey(p *PrecompileExecutor, ctx sdk.Context, method *abi.Method, _ authkeeper.Actor, args []interface{}, _ *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	var input grantViewInput
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	summary, err := p.keeper.SessionKey(ctx, input.GrantID)
	if err != nil {
		return nil, RevertError(err)
	}
	return method.Outputs.Pack(summary)
}

func capabilityScope(p *PrecompileExecutor, ctx sdk.Context, method *abi.Method, _ authkeeper.Actor, args []interface{}, _ *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	var input grantViewInput
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	scope, err := p.keeper.CapabilityScope(ctx, input.GrantID)
	if err != nil {
		return nil, RevertError(err)
	}
	if len(scope) < 180 || binary.BigEndian.Uint32(scope[12:16]) != 32 || binary.BigEndian.Uint32(scope[144:148]) != 32 {
		return nil, fmt.Errorf("invalid keeper capability scope")
	}
	var assetID, purposeHash [32]byte
	copy(assetID[:], scope[16:48])
	copy(purposeHash[:], scope[148:180])
	return method.Outputs.Pack(
		binary.BigEndian.Uint64(scope[:8]), binary.BigEndian.Uint16(scope[8:10]), binary.BigEndian.Uint16(scope[10:12]), assetID,
		new(big.Int).SetBytes(scope[48:64]), new(big.Int).SetBytes(scope[64:80]), new(big.Int).SetBytes(scope[80:96]),
		binary.BigEndian.Uint64(scope[96:104]), new(big.Int).SetBytes(scope[104:120]), new(big.Int).SetBytes(scope[120:136]),
		binary.BigEndian.Uint64(scope[136:144]), purposeHash,
	)
}

func checkActivity(p *PrecompileExecutor, ctx sdk.Context, method *abi.Method, _ authkeeper.Actor, args []interface{}, _ *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	var input struct {
		GrantID      [32]byte `abi:"grantId"`
		ActivityType uint32   `abi:"activityType"`
		AssetID      [32]byte `abi:"assetId"`
		Amount       *big.Int `abi:"amount"`
	}
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	reason, err := p.keeper.CheckActivity(ctx, input.GrantID, input.ActivityType, input.AssetID, sdk.NewIntFromBigInt(input.Amount))
	if err != nil {
		return nil, RevertError(err)
	}
	return method.Outputs.Pack(reason)
}
