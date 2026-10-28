package layerxpay

import (
	"bytes"
	"fmt"
	"math/big"

	paykeeper "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/keeper"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

type escrowTuple struct {
	Owner              [32]byte
	EscrowAccount      [32]byte
	Beneficiary        [32]byte
	Arbiter            [32]byte
	AssetId            [32]byte
	LockedAmount       *big.Int
	CapturedAmount     *big.Int
	State              uint8
	ExpiryMs           uint64
	DisputeWindowMs    uint64
	TermsHash          [32]byte
	AgreementReference [32]byte
}

func (p *Executor) escrowHandlers(ctx sdk.Context, method *abi.Method, caller common.Address, args []interface{}, key [32]byte, evm *vm.EVM) ([]byte, error) {
	id := args[0].([32]byte)
	switch method.Name {
	case "getEscrow":
		r, found := p.keeper.GetEscrow(ctx, id)
		if !found {
			return nil, fmt.Errorf("LXP_UNKNOWN_FIELD: escrow not found")
		}
		return method.Outputs.Pack(escrowTuple{
			Owner: r.Owner, EscrowAccount: r.EscrowAccount, Beneficiary: r.Beneficiary,
			Arbiter: r.Arbiter, AssetId: r.AssetID, LockedAmount: payAmount(r.LockedAmount),
			CapturedAmount: payAmount(r.CapturedAmount), State: r.State, ExpiryMs: r.ExpiryMs,
			DisputeWindowMs: r.DisputeWindowMs, TermsHash: r.TermsHash, AgreementReference: r.AgreementReference,
		})
	case "escrowRemaining":
		remaining, found := p.keeper.EscrowRemaining(ctx, id)
		if !found {
			return nil, fmt.Errorf("LXP_UNKNOWN_FIELD: escrow not found")
		}
		return method.Outputs.Pack(payAmount(remaining))
	case "getEscrowResult":
		if _, found := p.keeper.GetEscrowResult(ctx, id); !found {
			return nil, fmt.Errorf("LXP_UNKNOWN_FIELD: escrow result not found")
		}
		storageKey := paytypes.KernelKVKey(paytypes.EscrowModuleID, append([]byte("result:"), id[:]...))
		for _, pair := range p.keeper.ExportGenesis(ctx).KernelRecords {
			if bytes.Equal(pair.Key, storageKey) {
				return method.Outputs.Pack(pair.Value)
			}
		}
		return nil, fmt.Errorf("LXP_FATAL_INVARIANT: escrow result disappeared")
	}
	var event string
	var err error
	switch method.Name {
	case "escrowOpen":
		_, err = p.keeper.EscrowOpen(ctx, caller, paykeeper.EscrowOpenArgs{
			EscrowID: id, Owner: args[1].([32]byte), EscrowAccount: args[2].([32]byte),
			Beneficiary: args[3].([32]byte), Arbiter: args[4].([32]byte), AssetID: args[5].([32]byte),
			Amount: payU128(args[6]), ExpiryMs: args[7].(uint64), DisputeWindowMs: args[8].(uint64),
			TermsHash: args[9].([32]byte), AgreementReference: args[10].([32]byte),
		})
		event = "EscrowOpened"
	case "escrowCapture":
		_, err = p.keeper.EscrowCapture(ctx, caller, id, payU128(args[1]), args[2].([32]byte))
		event = "EscrowCaptured"
	case "escrowPartialCapture":
		_, err = p.keeper.EscrowPartialCapture(ctx, caller, id, payU128(args[1]), args[2].([32]byte))
		event = "EscrowPartiallyCaptured"
	case "escrowRelease":
		_, err = p.keeper.EscrowRelease(ctx, caller, id, args[1].([32]byte))
		event = "EscrowReleased"
	case "escrowTimeout":
		_, err = p.keeper.EscrowTimeout(ctx, caller, id, args[1].([32]byte))
		event = "EscrowTimedOut"
	case "escrowDisputeOpen":
		_, err = p.keeper.EscrowDisputeOpen(ctx, caller, id, key)
		event = "EscrowDisputed"
	case "escrowDisputeResolve":
		_, err = p.keeper.EscrowDisputeResolve(ctx, caller, id, args[1].(uint32), args[2].([32]byte))
		event = "EscrowResolved"
	default:
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: %s", method.Name)
	}
	if err != nil {
		return nil, err
	}
	record, found := p.keeper.GetEscrow(ctx, id)
	if !found {
		return nil, fmt.Errorf("LXP_FATAL_INVARIANT: escrow missing after %s", method.Name)
	}
	values, err := payEvents[event].pack(args, record)
	if err != nil {
		return nil, err
	}
	if err := emitPayEvent(evm, event, id, values); err != nil {
		return nil, err
	}
	return method.Outputs.Pack()
}
