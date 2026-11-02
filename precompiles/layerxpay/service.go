package layerxpay

import (
	"fmt"

	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
)

func (p *Executor) serviceHandlers(ctx sdk.Context, method *abi.Method, caller common.Address, args []interface{}, key [32]byte, evm *vm.EVM) ([]byte, error) {
	idArg := func(i int) [32]byte { return args[i].([32]byte) }
	var id [32]byte
	if method.Name != "serviceToolExecAttest" {
		id = idArg(0)
	}
	var event string
	var receipt lxphost.Receipt
	var err error
	switch method.Name {
	case "serviceOfferPublish":
		receipt, err = p.keeper.ServiceOfferPublish(ctx, caller, id, idArg(1), payU128(args[2]), idArg(3), idArg(4), args[5].(uint64), args[6].(uint64), args[7].(uint64), args[8].(uint8), args[9].(uint64))
		event = "OfferPublished"
	case "serviceOfferWithdraw":
		receipt, err = p.keeper.ServiceOfferWithdraw(ctx, caller, id)
		event = "OfferWithdrawn"
	case "serviceAgreementPropose":
		receipt, err = p.keeper.ServiceAgreementPropose(ctx, caller, id, idArg(1), idArg(2), idArg(3), key)
		event = "AgreementProposed"
	case "serviceAgreementAccept":
		receipt, err = p.keeper.ServiceAgreementAccept(ctx, caller, id, key)
		event = "AgreementAccepted"
	case "serviceCommitTask":
		receipt, err = p.keeper.ServiceCommitTask(ctx, caller, id, idArg(1), idArg(2), idArg(3), args[4].(uint64), args[5].(uint64), key)
		event = "TaskCommitted"
	case "serviceCommitAbandon":
		receipt, err = p.keeper.ServiceCommitAbandon(ctx, caller, id, args[1].(uint16), key)
		event = "CommitAbandoned"
	case "serviceToolExecAttest":
		execution := args[0].([]byte)
		if len(execution) != paytypes.ServiceExecutionBytes {
			return nil, fmt.Errorf("LXP_NON_CANONICAL: service execution must be %d bytes", paytypes.ServiceExecutionBytes)
		}
		id = [32]byte(execution[:32])
		activityID := [32]byte(execution[32:64])
		receipt, err = p.keeper.ServiceToolExecAttest(ctx, caller, execution, activityID)
		event = "ExecutionAttested"
	case "serviceProgressReport":
		receipt, err = p.keeper.ServiceProgressReport(ctx, caller, id, idArg(1), idArg(2), idArg(3), args[4].(uint32), key)
		event = "ProgressReported"
	case "serviceDeliver":
		tuples := *abi.ConvertType(args[2], new([]serviceDeliverableTuple)).(*[]serviceDeliverableTuple)
		items := make([]paytypes.ServiceDeliverable, len(tuples))
		for i, tuple := range tuples {
			items[i] = paytypes.ServiceDeliverable{Hash: tuple.Hash, ArtifactSize: tuple.ArtifactSizeBytes, AvailabilityReference: tuple.AvailabilityReference}
		}
		receipt, err = p.keeper.ServiceDeliver(ctx, caller, id, idArg(1), items, key)
		event = "Delivered"
	case "serviceAccept":
		agreement, found := p.keeper.GetAgreement(ctx, id)
		if found && agreement.State == 3 && uint64(ctx.BlockTime().UnixMilli()) > agreement.AcceptanceWindowEndMs {
			receipt, err = p.keeper.ServiceApplyDefaultOutcomes(ctx)
		} else {
			receipt, err = p.keeper.ServiceAccept(ctx, caller, id, key)
		}
		event = "OutcomeAccepted"
	case "serviceReject":
		agreement, found := p.keeper.GetAgreement(ctx, id)
		if found && agreement.State == 3 && uint64(ctx.BlockTime().UnixMilli()) > agreement.AcceptanceWindowEndMs {
			receipt, err = p.keeper.ServiceApplyDefaultOutcomes(ctx)
		} else {
			receipt, err = p.keeper.ServiceReject(ctx, caller, id, args[1].(uint16), args[2].([][32]byte), key)
		}
		event = "OutcomeRejected"
	case "serviceDisputeOpen":
		receipt, err = p.keeper.ServiceDisputeOpen(ctx, caller, id, idArg(1), args[2].([][32]byte))
		event = "DisputeOpened"
	case "serviceDisputeResolve":
		receipt, err = p.keeper.ServiceDisputeResolve(ctx, caller, id, args[1].(uint16), args[2].(uint32), idArg(3))
		event = "DisputeResolved"
	default:
		return nil, fmt.Errorf("LXP_PRECOMPILE_UNKNOWN_METHOD: %s", method.Name)
	}
	if err != nil {
		return nil, err
	}
	var record any
	var found bool
	switch method.Name {
	case "serviceOfferPublish", "serviceOfferWithdraw":
		var offer paytypes.ServiceOffer
		offer, found = p.keeper.GetOffer(ctx, id)
		record = offer
		if method.Name == "serviceOfferWithdraw" {
			record = payServiceSequence[paytypes.ServiceOffer]{Record: offer, GlobalSequence: receipt.GlobalSequence}
		}
	case "serviceAgreementPropose", "serviceAgreementAccept", "serviceAccept", "serviceReject":
		var agreement paytypes.ServiceAgreement
		agreement, found = p.keeper.GetAgreement(ctx, id)
		record = agreement
		if (method.Name == "serviceAccept" || method.Name == "serviceReject") && agreement.DefaultApplied {
			event = "DefaultOutcomeApplied"
		}
	case "serviceCommitTask", "serviceCommitAbandon":
		var commitment paytypes.ServiceCommitment
		commitment, found = p.keeper.GetCommitment(ctx, id)
		record = commitment
		if method.Name == "serviceCommitAbandon" {
			record = payServiceSequence[paytypes.ServiceCommitment]{Record: commitment, GlobalSequence: receipt.GlobalSequence}
		}
	case "serviceToolExecAttest":
		record, found = p.keeper.GetExecution(ctx, id)
	case "serviceProgressReport":
		record, found = p.keeper.GetProgress(ctx, id)
	case "serviceDeliver":
		record, found = p.keeper.GetDelivery(ctx, id)
	case "serviceDisputeOpen", "serviceDisputeResolve":
		record, found = p.keeper.GetDispute(ctx, id)
	}
	if !found {
		return nil, fmt.Errorf("LXP_FATAL_INVARIANT: service record missing after %s", method.Name)
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
