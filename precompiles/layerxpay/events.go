package layerxpay

import (
	"fmt"
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	pcommon "github.com/Sidiora-Labs/Paxeer-X-Network/precompiles/common"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

type payEvent struct {
	name string
	pack func(args []interface{}, record any) ([]interface{}, error)
}

type payBudgetDrainRecord struct {
	Budget  paytypes.BudgetRecord
	Drained codec.U128
}

type payStreamChangeRecord struct {
	Before paytypes.StreamRecord
	After  paytypes.StreamRecord
	Result paytypes.StreamResult
}

type payServiceSequence[T any] struct {
	Record         T
	GlobalSequence uint64
}

var payEvents = map[string]payEvent{
	"EscrowOpened":            payEscrowEvent("EscrowOpened", 1),
	"EscrowCaptured":          payEscrowEvent("EscrowCaptured", 2),
	"EscrowPartiallyCaptured": payEscrowEvent("EscrowPartiallyCaptured", 3),
	"EscrowReleased":          payEscrowEvent("EscrowReleased", 4),
	"EscrowTimedOut":          payEscrowEvent("EscrowTimedOut", 5),
	"EscrowDisputed":          payEscrowEvent("EscrowDisputed", 6),
	"EscrowResolved":          payEscrowEvent("EscrowResolved", 7),
	"BudgetCreated": payRecordEvent("BudgetCreated", 7, func(a []interface{}, r paytypes.BudgetRecord) []interface{} {
		return []interface{}{r.AssetID, a[6]}
	}),
	"BudgetFunded": payArgumentEvent("BudgetFunded", 1),
	"BudgetAmended": payRecordEvent("BudgetAmended", 1, func(_ []interface{}, r paytypes.BudgetRecord) []interface{} {
		return []interface{}{payAmount(r.PerPeriodLimit), r.ExpiryMs}
	}),
	"BudgetDelegateAdded":   payArgumentEvent("BudgetDelegateAdded", 1),
	"BudgetDelegateRemoved": payArgumentEvent("BudgetDelegateRemoved", 1),
	"BudgetSpent":           payArgumentEvent("BudgetSpent", 1, 2),
	"BudgetClosed":          payBudgetDrainEvent("BudgetClosed"),
	"BudgetDefunded": payRecordEvent("BudgetDefunded", 2, func(a []interface{}, r paytypes.BudgetRecord) []interface{} {
		return []interface{}{a[1], payAmount(r.PerPeriodLimit)}
	}),
	"BudgetRevoked": payBudgetDrainEvent("BudgetRevoked"),
	"StreamOpened": payRecordEvent("StreamOpened", 11, func(a []interface{}, r paytypes.StreamRecord) []interface{} {
		return []interface{}{r.Payer, r.StreamAccount, r.Recipient, r.AssetID, r.Mode, a[10]}
	}),
	"StreamToppedUp": payRecordEvent("StreamToppedUp", 2, func(a []interface{}, r payStreamChangeRecord) []interface{} {
		return []interface{}{a[1], r.Before.Underfunded}
	}),
	"StreamMetered": {name: "StreamMetered", pack: packPayStreamMetered},
	"StreamSettled": payRecordEvent("StreamSettled", 1, func(_ []interface{}, r payStreamChangeRecord) []interface{} {
		return []interface{}{payAmount(r.Result.Paid), payAmount(r.After.SettledTotal), r.After.Underfunded}
	}),
	"StreamPaused":  payStreamLifecycleEvent("StreamPaused"),
	"StreamResumed": payStreamLifecycleEvent("StreamResumed"),
	"StreamClosed": payRecordEvent("StreamClosed", 1, func(_ []interface{}, r paytypes.StreamResult) []interface{} {
		return []interface{}{payAmount(r.Paid), payAmount(r.Refunded)}
	}),
	"StreamRateUpdated": payRecordEvent("StreamRateUpdated", 1, func(_ []interface{}, r payStreamChangeRecord) []interface{} {
		return []interface{}{payAmount(r.Before.Rate), payAmount(r.After.Rate), payAmount(r.Result.Paid)}
	}),
	"OfferPublished": payRecordEvent("OfferPublished", 1, func(_ []interface{}, r paytypes.ServiceOffer) []interface{} {
		return []interface{}{r.OfferingAgent, r.DefaultOutcome, r.GlobalSequence}
	}),
	"OfferWithdrawn": payRecordEvent("OfferWithdrawn", 1, func(_ []interface{}, r payServiceSequence[paytypes.ServiceOffer]) []interface{} {
		return []interface{}{r.Record.OfferingAgent, uint8(1), r.GlobalSequence}
	}),
	"AgreementProposed": payRecordEvent("AgreementProposed", 1, func(_ []interface{}, r paytypes.ServiceAgreement) []interface{} {
		return []interface{}{r.Provider, r.State, r.AcceptedSequence}
	}),
	"AgreementAccepted": payRecordEvent("AgreementAccepted", 1, func(_ []interface{}, r paytypes.ServiceAgreement) []interface{} {
		return []interface{}{r.Buyer, r.State, r.AcceptedSequence}
	}),
	"TaskCommitted": payRecordEvent("TaskCommitted", 1, func(_ []interface{}, r paytypes.ServiceCommitment) []interface{} {
		return []interface{}{r.AgreementID, payBoolCode(r.Abandoned), r.GlobalSequence}
	}),
	"CommitAbandoned": payRecordEvent("CommitAbandoned", 1, func(_ []interface{}, r payServiceSequence[paytypes.ServiceCommitment]) []interface{} {
		return []interface{}{r.Record.AgreementID, uint8(1), r.GlobalSequence}
	}),
	"ExecutionAttested": payRecordEvent("ExecutionAttested", 1, func(_ []interface{}, r paytypes.ServiceExecution) []interface{} {
		return []interface{}{r.CommitmentID, uint8(0), r.GlobalSequence}
	}),
	"ProgressReported": payRecordEvent("ProgressReported", 1, func(_ []interface{}, r paytypes.ServiceProgress) []interface{} {
		return []interface{}{r.CommitmentID, payBoolCode(r.ProgressBPS == 10000), r.GlobalSequence}
	}),
	"Delivered": payRecordEvent("Delivered", 1, func(_ []interface{}, r paytypes.ServiceDelivery) []interface{} {
		return []interface{}{r.AgreementID, uint8(len(r.Deliverables)), r.GlobalSequence}
	}),
	"OutcomeAccepted": payRecordEvent("OutcomeAccepted", 1, func(_ []interface{}, r paytypes.ServiceAgreement) []interface{} {
		return []interface{}{r.Provider, r.State, r.OutcomeSequence}
	}),
	"OutcomeRejected": payRecordEvent("OutcomeRejected", 1, func(_ []interface{}, r paytypes.ServiceAgreement) []interface{} {
		return []interface{}{r.Provider, uint8(len(r.ContestedHashes)), r.OutcomeSequence}
	}),
	"DisputeOpened": payRecordEvent("DisputeOpened", 1, func(_ []interface{}, r paytypes.ServiceDispute) []interface{} {
		return []interface{}{r.AgreementID, uint8(len(r.EvidenceHashes)), r.GlobalSequence}
	}),
	"DisputeResolved": payRecordEvent("DisputeResolved", 1, func(_ []interface{}, r paytypes.ServiceDispute) []interface{} {
		return []interface{}{r.AgreementID, uint8(1), r.ResolutionSequence}
	}),
	"DefaultOutcomeApplied": payRecordEvent("DefaultOutcomeApplied", 1, func(_ []interface{}, r paytypes.ServiceAgreement) []interface{} {
		return []interface{}{r.State, r.DefaultApplied, r.OutcomeSequence, r.OutcomeTimestampMs}
	}),
}

func payAmount(v codec.U128) *big.Int {
	b := v.Bytes()
	return new(big.Int).SetBytes(b[:])
}

func payBoolCode(v bool) uint8 {
	if v {
		return 1
	}
	return 0
}

func payRecordEvent[T any](name string, minArgs int, fields func([]interface{}, T) []interface{}) payEvent {
	return payEvent{name: name, pack: func(args []interface{}, record any) ([]interface{}, error) {
		r, ok := record.(T)
		if !ok {
			return nil, fmt.Errorf("%s: invalid post-call record %T", name, record)
		}
		if len(args) < minArgs {
			return nil, fmt.Errorf("%s: need at least %d call arguments", name, minArgs)
		}
		values := fields(args, r)
		if err := validatePayEventValues(name, values); err != nil {
			return nil, err
		}
		return values, nil
	}}
}

func payEscrowEvent(name string, ordinal uint16) payEvent {
	return payRecordEvent(name, 1, func(_ []interface{}, r paytypes.EscrowRecord) []interface{} {
		return []interface{}{ordinal, r.State, payAmount(r.CapturedAmount), payAmount(r.LockedAmount)}
	})
}

func payArgumentEvent(name string, indexes ...int) payEvent {
	return payEvent{name: name, pack: func(args []interface{}, _ any) ([]interface{}, error) {
		values := make([]interface{}, len(indexes))
		for i, index := range indexes {
			if index >= len(args) {
				return nil, fmt.Errorf("%s: missing call argument %d", name, index)
			}
			values[i] = args[index]
		}
		if err := validatePayEventValues(name, values); err != nil {
			return nil, err
		}
		return values, nil
	}}
}

func payBudgetDrainEvent(name string) payEvent {
	return payRecordEvent(name, 1, func(_ []interface{}, r payBudgetDrainRecord) []interface{} {
		return []interface{}{payAmount(r.Drained), r.Budget.RevocationSequence}
	})
}

func payStreamLifecycleEvent(name string) payEvent {
	return payRecordEvent(name, 1, func(_ []interface{}, r paytypes.StreamRecord) []interface{} {
		return []interface{}{r.LastAccrualTimestamp, payAmount(r.AccruedTotal)}
	})
}

func packPayStreamMetered(args []interface{}, record any) ([]interface{}, error) {
	r, ok := record.(payStreamChangeRecord)
	if !ok || len(args) == 0 {
		return nil, fmt.Errorf("StreamMetered: missing call arguments or stream records")
	}
	accrued, ok := r.After.AccruedTotal.CheckedSub(r.Before.AccruedTotal)
	if !ok {
		return nil, fmt.Errorf("StreamMetered: accrued total decreased")
	}
	values := []interface{}{r.After.CumulativeMeter, payAmount(accrued), payAmount(r.After.AccruedTotal)}
	if err := validatePayEventValues("StreamMetered", values); err != nil {
		return nil, err
	}
	return values, nil
}

func validatePayEventValues(name string, values []interface{}) error {
	if gasABIError != nil {
		return gasABIError
	}
	event, ok := gasABI.Events[name]
	if !ok {
		return fmt.Errorf("unknown pay event %q", name)
	}
	if _, err := abi.Arguments(event.Inputs[1:]).Pack(values...); err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return nil
}

func emitPayEvent(evm *vm.EVM, name string, id [32]byte, values []interface{}) error {
	if _, ok := payEvents[name]; !ok {
		return fmt.Errorf("unknown pay event %q", name)
	}
	if err := validatePayEventValues(name, values); err != nil {
		return err
	}
	event := gasABI.Events[name]
	topics := []common.Hash{event.ID, common.Hash(id)}
	dataValues := values
	if event.Inputs[1].Indexed {
		secondary, ok := values[0].([32]byte)
		if !ok {
			return fmt.Errorf("%s: secondary id must be bytes32", name)
		}
		topics = append(topics, common.Hash(secondary))
		dataValues = values[1:]
	}
	data, err := event.Inputs.NonIndexed().Pack(dataValues...)
	if err != nil {
		return fmt.Errorf("%s: %w", name, err)
	}
	return pcommon.EmitEVMLog(evm, common.HexToAddress("0x101B"), topics, data)
}
