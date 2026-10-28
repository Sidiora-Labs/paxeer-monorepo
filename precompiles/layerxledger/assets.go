package layerxledger

import (
	"fmt"
	"math/big"

	ledgerkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/keeper"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
)

const (
	AssetRegistered = "AssetRegistered"
	AssetPaused     = "AssetPaused"
	AssetUnpaused   = "AssetUnpaused"
	Transfer        = "Transfer"
)

func init() {
	ledgerMethods["register"] = (*PrecompileExecutor).register
	ledgerMethods["pause"] = (*PrecompileExecutor).pause
	ledgerMethods["unpause"] = (*PrecompileExecutor).unpause
	ledgerMethods["mint"] = (*PrecompileExecutor).mint
	ledgerMethods["burn"] = (*PrecompileExecutor).burn
	ledgerMethods["getAsset"] = (*PrecompileExecutor).assetView
	ledgerMethods["balanceOf"] = (*PrecompileExecutor).assetView
	ledgerMethods["totalUnits"] = (*PrecompileExecutor).assetView
}

func (p *PrecompileExecutor) register(ctx sdk.Context, method *abi.Method, actor ledgerkeeper.Actor, args []interface{}, evm *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	assetID := args[0].([32]byte)
	result, err := p.keeper.RegisterAsset(ctx, actor, types.RegisterArgs{
		AssetID: assetID, Salt: args[1].([32]byte), Symbol: args[2].(string),
		Name: args[3].(string), Decimals: args[4].(uint8),
		SupplyCap:  sdk.NewIntFromBigInt(args[5].(*big.Int)),
		IssuerKind: args[6].(uint8), CustodyReference: args[7].([]byte),
	}, [32]byte{})
	if err != nil {
		return nil, RevertError(err)
	}
	if err := p.emitAssetEvents(evm, 0x00010001, result); err != nil {
		return nil, err
	}
	return (abi.Arguments{method.Inputs[0]}).Pack(assetID)
}

func (p *PrecompileExecutor) pause(ctx sdk.Context, method *abi.Method, actor ledgerkeeper.Actor, args []interface{}, evm *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	result, err := p.keeper.PauseAsset(ctx, actor, args[0].([32]byte), [32]byte{})
	if err != nil {
		return nil, RevertError(err)
	}
	if err := p.emitAssetEvents(evm, 0x00010002, result); err != nil {
		return nil, err
	}
	return method.Outputs.Pack()
}

func (p *PrecompileExecutor) unpause(ctx sdk.Context, method *abi.Method, actor ledgerkeeper.Actor, args []interface{}, evm *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	result, err := p.keeper.UnpauseAsset(ctx, actor, args[0].([32]byte), [32]byte{})
	if err != nil {
		return nil, RevertError(err)
	}
	if err := p.emitAssetEvents(evm, 0x00010003, result); err != nil {
		return nil, err
	}
	return method.Outputs.Pack()
}

func (p *PrecompileExecutor) mint(ctx sdk.Context, method *abi.Method, actor ledgerkeeper.Actor, args []interface{}, evm *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	result, err := p.keeper.Mint(ctx, actor, args[0].([32]byte), args[1].([32]byte), sdk.NewIntFromBigInt(args[2].(*big.Int)), [32]byte{})
	if err != nil {
		return nil, RevertError(err)
	}
	if err := p.emitAssetEvents(evm, 0x0001000a, result); err != nil {
		return nil, err
	}
	return method.Outputs.Pack()
}

func (p *PrecompileExecutor) burn(ctx sdk.Context, method *abi.Method, actor ledgerkeeper.Actor, args []interface{}, evm *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	result, err := p.keeper.Burn(ctx, actor, args[0].([32]byte), args[1].([32]byte), sdk.NewIntFromBigInt(args[2].(*big.Int)), [32]byte{})
	if err != nil {
		return nil, RevertError(err)
	}
	if err := p.emitAssetEvents(evm, 0x0001000b, result); err != nil {
		return nil, err
	}
	return method.Outputs.Pack()
}

func (p *PrecompileExecutor) emitAssetEvents(evm *vm.EVM, activity uint32, result ledgerkeeper.Result) error {
	events, err := ledgerkeeper.LedgerEvents(activity, result.Receipt, ledgerkeeper.EventArgs{})
	if err != nil {
		return RevertError(err)
	}
	for _, event := range events {
		declaration, ok := p.abi.Events[event.Name]
		if !ok {
			return fmt.Errorf("layerxledger: unknown event %s", event.Name)
		}
		topics := []common.Hash{declaration.ID}
		var data []interface{}
		for _, input := range declaration.Inputs {
			value, ok := event.Fields[input.Name]
			if !ok {
				return fmt.Errorf("layerxledger: missing %s.%s", event.Name, input.Name)
			}
			if input.Indexed {
				indexed, err := abi.MakeTopics([]interface{}{value})
				if err != nil {
					return err
				}
				topics = append(topics, indexed[0][0])
			} else {
				data = append(data, value)
			}
		}
		packed, err := declaration.Inputs.NonIndexed().Pack(data...)
		if err != nil {
			return err
		}
		if err := pcommon.EmitEVMLog(evm, p.address, topics, packed); err != nil {
			return err
		}
	}
	return nil
}

func (p *PrecompileExecutor) assetView(ctx sdk.Context, method *abi.Method, _ ledgerkeeper.Actor, args []interface{}, _ *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	switch method.Name {
	case "getAsset":
		asset, err := p.keeper.GetAsset(ctx, args[0].([32]byte))
		if err != nil {
			return nil, RevertError(err)
		}
		return method.Outputs.Pack(asset.AssetID, asset.Symbol, asset.Name, asset.Decimals,
			asset.SupplyCap.BigInt(), asset.IssuerDID, asset.Salt, asset.IssuerKind,
			asset.CustodyKind, asset.CustodyReference, asset.Paused, asset.TotalUnits.BigInt())
	case "balanceOf":
		balance, err := p.keeper.BalanceOf(ctx, args[0].([32]byte), args[1].([32]byte))
		if err != nil {
			return nil, RevertError(err)
		}
		return method.Outputs.Pack(balance.BigInt())
	case "totalUnits":
		total, err := p.keeper.TotalUnits(ctx, args[0].([32]byte))
		if err != nil {
			return nil, RevertError(err)
		}
		return method.Outputs.Pack(total.BigInt())
	default:
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: layerxledger asset view %s", method.Name)
	}
}
