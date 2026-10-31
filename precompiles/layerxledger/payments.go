package layerxledger

import (
	"crypto/sha256"
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
	PayerGrantIssued  = "PayerGrantIssued"
	PayerGrantRevoked = "PayerGrantRevoked"
	PaymentDrawn      = "PaymentDrawn"
	PaymentExact      = "PaymentExact"
)

func init() {
	ledgerMethods["issueGrant"] = (*PrecompileExecutor).issueGrant
	ledgerMethods["revokeGrant"] = (*PrecompileExecutor).revokeGrant
	ledgerMethods["draw"] = (*PrecompileExecutor).draw
	ledgerMethods["payExact"] = (*PrecompileExecutor).payExact
	ledgerMethods["getPayerGrant"] = (*PrecompileExecutor).getPayerGrant
}

func (p *PrecompileExecutor) paymentLogs(evm *vm.EVM, activity uint32, result ledgerkeeper.Result, args ledgerkeeper.EventArgs) ([]ledgerkeeper.Event, error) {
	events, err := ledgerkeeper.LedgerEvents(activity, result.Receipt, args)
	if err != nil {
		return nil, RevertError(err)
	}
	for _, event := range events {
		definition, found := p.abi.Events[event.Name]
		if !found {
			return nil, RevertError(types.ErrNonCanonical)
		}
		topics := []common.Hash{definition.ID}
		var values []interface{}
		for _, input := range definition.Inputs {
			value, found := event.Fields[input.Name]
			if !found {
				return nil, RevertError(types.ErrNonCanonical)
			}
			if input.Indexed {
				id, ok := value.([32]byte)
				if !ok {
					return nil, RevertError(types.ErrNonCanonical)
				}
				topics = append(topics, common.Hash(id))
			} else {
				values = append(values, value)
			}
		}
		data, err := definition.Inputs.NonIndexed().Pack(values...)
		if err != nil {
			return nil, err
		}
		if err := pcommon.EmitEVMLog(evm, p.address, topics, data); err != nil {
			return nil, err
		}
	}
	return events, nil
}

func (p *PrecompileExecutor) issueGrant(ctx sdk.Context, method *abi.Method, actor ledgerkeeper.Actor, args []interface{}, evm *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	var input struct{ Grant []byte }
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	result, err := p.keeper.IssueGrant(ctx, actor, input.Grant, sha256.Sum256(input.Grant))
	if err != nil {
		return nil, RevertError(err)
	}
	events, err := p.paymentLogs(evm, 0x00010007, result, ledgerkeeper.EventArgs{})
	if err != nil {
		return nil, err
	}
	return p.abi.Methods["revokeGrant"].Inputs[:1].Pack(events[0].Fields["grantId"])
}

func (p *PrecompileExecutor) revokeGrant(ctx sdk.Context, method *abi.Method, actor ledgerkeeper.Actor, args []interface{}, evm *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	var input struct {
		GrantId            [32]byte
		RevocationSequence uint64
	}
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	payload := types.EncodeRevokeGrantPayload(input.GrantId, input.RevocationSequence)
	result, err := p.keeper.RevokeGrant(ctx, actor, input.GrantId, input.RevocationSequence, sha256.Sum256(payload))
	if err != nil {
		return nil, RevertError(err)
	}
	if _, err := p.paymentLogs(evm, 0x00010008, result, ledgerkeeper.EventArgs{}); err != nil {
		return nil, err
	}
	return method.Outputs.Pack()
}

func (p *PrecompileExecutor) draw(ctx sdk.Context, method *abi.Method, actor ledgerkeeper.Actor, args []interface{}, evm *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	var input struct {
		GrantId        [32]byte
		Recipient      [32]byte
		Amount         *big.Int
		IdempotencyKey [32]byte
		Sequence       uint64
		PurposeHash    [32]byte
		ContextHash    [32]byte
	}
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	return nil, RevertError(types.ErrNoPayerGrant)
}

func (p *PrecompileExecutor) payExact(ctx sdk.Context, method *abi.Method, actor ledgerkeeper.Actor, args []interface{}, evm *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	var input struct {
		From             [32]byte
		To               [32]byte
		AssetId          [32]byte
		Amount           *big.Int
		ExpiresAt        uint64 `abi:"sequence"`
		PaymentReference [32]byte
	}
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	result, err := p.keeper.PayExact(ctx, actor, ledgerkeeper.PayExactArgs{
		From: input.From, To: input.To, Asset: input.AssetId,
		Amount: sdk.NewIntFromBigInt(input.Amount), ExpiresAt: input.ExpiresAt,
		PaymentReference: input.PaymentReference,
	})
	if err != nil {
		return nil, RevertError(err)
	}
	if _, err := p.paymentLogs(evm, 0x00010005, result, ledgerkeeper.EventArgs{
		PayExact: true, PaymentReference: input.PaymentReference,
	}); err != nil {
		return nil, err
	}
	return method.Outputs.Pack()
}

func (p *PrecompileExecutor) getPayerGrant(ctx sdk.Context, method *abi.Method, _ ledgerkeeper.Actor, args []interface{}, _ *vm.EVM, _ *tracing.Hooks) ([]byte, error) {
	var input struct{ GrantId [32]byte }
	if err := method.Inputs.Copy(&input, args); err != nil {
		return nil, err
	}
	return nil, RevertError(types.ErrNoPayerGrant)
}
