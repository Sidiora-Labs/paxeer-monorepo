package layerxpay

import (
	"fmt"
	"reflect"

	paytypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

type serviceDeliverableTuple struct {
	Hash                  [32]byte
	ArtifactSizeBytes     uint64
	AvailabilityReference [32]byte
}

type serviceDeliveryTuple struct {
	AgreementID    [32]byte
	Provider       [32]byte
	Deliverables   []serviceDeliverableTuple
	GlobalSequence uint64
}

func serviceDeliveryABI(record paytypes.ServiceDelivery) serviceDeliveryTuple {
	items := make([]serviceDeliverableTuple, len(record.Deliverables))
	for i, item := range record.Deliverables {
		items[i] = serviceDeliverableTuple{item.Hash, item.ArtifactSize, item.AvailabilityReference}
	}
	return serviceDeliveryTuple{record.AgreementID, record.Provider, items, record.GlobalSequence}
}

func (p *Executor) serviceViewHandlers(ctx sdk.Context, method *abi.Method, args []interface{}) ([]byte, bool, error) {
	var record interface{}
	var found bool
	switch method.Name {
	case "getOffer":
		record, found = p.keeper.GetOffer(ctx, args[0].([32]byte))
	case "getAgreement":
		record, found = p.keeper.GetAgreement(ctx, args[0].([32]byte))
	case "getCommitment":
		record, found = p.keeper.GetCommitment(ctx, args[0].([32]byte))
	case "getExecution":
		execution, ok := p.keeper.GetExecution(ctx, args[0].([32]byte))
		if !ok {
			return nil, true, fmt.Errorf("LXP_UNKNOWN_FIELD: execution not found")
		}
		payload := make([]byte, int(execution.CanonicalPayloadLength)+len(execution.Signature))
		copy(payload, execution.CanonicalPayload[:execution.CanonicalPayloadLength])
		copy(payload[execution.CanonicalPayloadLength:], execution.Signature[:])
		output, err := method.Outputs.Pack(payload)
		return output, true, err
	case "getDelivery":
		delivery, ok := p.keeper.GetDelivery(ctx, args[0].([32]byte))
		record, found = serviceDeliveryABI(delivery), ok
	case "latestDelivery":
		delivery, ok := p.keeper.LatestDelivery(ctx, args[0].([32]byte))
		record, found = serviceDeliveryABI(delivery), ok
	case "getProgress":
		record, found = p.keeper.GetProgress(ctx, args[0].([32]byte))
	case "getDispute":
		record, found = p.keeper.GetDispute(ctx, args[0].([32]byte))
	case "progressHighWater":
		value, _ := p.keeper.ProgressHighWater(ctx, args[0].([32]byte))
		output, err := method.Outputs.Pack(value)
		return output, true, err
	default:
		return nil, false, nil
	}
	if !found {
		return nil, true, fmt.Errorf("LXP_UNKNOWN_FIELD: %s record not found", method.Name)
	}
	tuple, err := payTuple(method.Outputs[0].Type, reflect.ValueOf(record))
	if err != nil {
		return nil, true, err
	}
	output, err := method.Outputs.Pack(tuple.Interface())
	return output, true, err
}
