package layerxledger

import (
	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
)

func init() {
	ledgerMethods["getAsset"] = (*PrecompileExecutor).getAsset
	ledgerMethods["balanceOf"] = (*PrecompileExecutor).balanceOf
	ledgerMethods["totalUnits"] = (*PrecompileExecutor).totalUnits
	ledgerMethods["accountId"] = (*PrecompileExecutor).accountId
	ledgerMethods["accountOwner"] = (*PrecompileExecutor).accountOwner
	ledgerMethods["getPayerGrant"] = getPayerGrant
	ledgerMethods["nextSequence"] = (*PrecompileExecutor).nextSequence
}

type accountAssetInputs struct {
	AccountID [32]byte `abi:"accountId"`
	AssetID   [32]byte `abi:"assetId"`
}

func (p *PrecompileExecutor) getAsset(ctx sdk.Context, method *abi.Method, _ ledgerkeeper.Actor, args []interface{}, _ *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	var id [32]byte
	if err := method.Inputs.Copy(&id, args); err != nil {
		return nil, err
	}
	asset, err := p.keeper.GetAsset(ctx, id)
	if err != nil {
		return nil, RevertError(err)
	}
	return method.Outputs.Pack(asset.AssetID, asset.Symbol, asset.Name, asset.Decimals,
		asset.SupplyCap.BigInt(), asset.IssuerDID, asset.Salt, asset.IssuerKind,
		asset.CustodyKind, asset.CustodyReference, asset.Paused, asset.TotalUnits.BigInt())
}

func (p *PrecompileExecutor) balanceOf(ctx sdk.Context, method *abi.Method, _ ledgerkeeper.Actor, args []interface{}, _ *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	var input accountAssetInputs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	balance, err := p.keeper.BalanceOf(ctx, input.AccountID, input.AssetID)
	if err != nil {
		return nil, RevertError(err)
	}
	return method.Outputs.Pack(balance.BigInt())
}

func (p *PrecompileExecutor) totalUnits(ctx sdk.Context, method *abi.Method, _ ledgerkeeper.Actor, args []interface{}, _ *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	var id [32]byte
	if err := method.Inputs.Copy(&id, args); err != nil {
		return nil, err
	}
	total, err := p.keeper.TotalUnits(ctx, id)
	if err != nil {
		return nil, RevertError(err)
	}
	return method.Outputs.Pack(total.BigInt())
}

func (p *PrecompileExecutor) accountId(_ sdk.Context, method *abi.Method, _ ledgerkeeper.Actor, args []interface{}, _ *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	var input struct {
		Owner   [32]byte `abi:"owner"`
		AssetID [32]byte `abi:"assetId"`
	}
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	_, id, err := lxphost.AssetAccountID(input.Owner, input.AssetID)
	if err != nil {
		return nil, RevertError(err)
	}
	return method.Outputs.Pack(id)
}

func (p *PrecompileExecutor) accountOwner(ctx sdk.Context, method *abi.Method, _ ledgerkeeper.Actor, args []interface{}, _ *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	var id [32]byte
	if err := method.Inputs.Copy(&id, args); err != nil {
		return nil, err
	}
	owner, err := p.keeper.AccountOwner(ctx, id)
	if err != nil {
		return nil, RevertError(err)
	}
	return method.Outputs.Pack(owner)
}

func getPayerGrant(p *PrecompileExecutor, ctx sdk.Context, method *abi.Method, actor ledgerkeeper.Actor, args []interface{}, evm *vm.EVM, hooks *tracing.Hooks) ([]byte, error) {
	return p.getPayerGrant(ctx, method, actor, args, evm, hooks)
}

func (p *PrecompileExecutor) nextSequence(ctx sdk.Context, method *abi.Method, _ ledgerkeeper.Actor, args []interface{}, _ *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	var input accountAssetInputs
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	sequence, err := p.keeper.NextSequence(ctx, input.AccountID, input.AssetID)
	if err != nil {
		return nil, RevertError(err)
	}
	return method.Outputs.Pack(sequence)
}
