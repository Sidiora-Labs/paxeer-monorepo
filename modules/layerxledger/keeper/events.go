package keeper

import (
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxledger/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/lxphost"
)

type Event struct {
	Name   string
	Fields map[string]any
}

type EventArgs struct {
	GrantID          [32]byte
	IdempotencyKey   [32]byte
	ContextHash      [32]byte
	PayExact         bool
	PaymentReference [32]byte
}

func LedgerEvents(activityType uint32, receipt lxphost.Receipt, args EventArgs) ([]Event, error) {
	var name, field string
	var code uint16
	switch activityType {
	case 0x00010001:
		name, field, code = "AssetRegistered", "assetId", 1
	case 0x00010002:
		name, field, code = "AssetPaused", "assetId", 2
	case 0x00010003:
		name, field, code = "AssetUnpaused", "assetId", 3
	case 0x00010004:
		name, field, code = "AccountOpened", "accountId", 4
	case 0x00010005, 0x00010006, 0x00010009, 0x0001000a, 0x0001000b:
	case 0x00010007:
		name, field, code = "PayerGrantIssued", "grantId", 7
	case 0x00010008:
		name, field, code = "PayerGrantRevoked", "grantId", 8
	default:
		return nil, types.ErrNonCanonical
	}
	if args.PayExact && activityType != 0x00010005 {
		return nil, types.ErrNonCanonical
	}
	if err := FromKernel(receipt.ResultCode); err != nil {
		return nil, err
	}
	if name != "" {
		var id [32]byte
		found := false
		for _, event := range receipt.Events {
			if event.Code != code {
				continue
			}
			if found || len(event.Body) != len(id) {
				return nil, types.ErrNonCanonical
			}
			copy(id[:], event.Body)
			found = true
		}
		if !found {
			return nil, types.ErrNonCanonical
		}
		return []Event{{Name: name, Fields: map[string]any{field: id}}}, nil
	}
	if len(receipt.Transfers) == 0 ||
		((activityType == 0x00010006 || args.PayExact) && len(receipt.Transfers) != 1) {
		return nil, types.ErrNonCanonical
	}
	events := make([]Event, 0, len(receipt.Transfers)+1)
	for _, leg := range receipt.Transfers {
		if leg.Amount.IsNil() || !leg.Amount.IsPositive() || leg.Amount.BigInt().BitLen() > 128 {
			return nil, types.ErrNonCanonical
		}
		events = append(events, Event{Name: "Transfer", Fields: map[string]any{
			"from": leg.From, "to": leg.To, "assetId": leg.Asset,
			"amount": leg.Amount.BigInt(), "kind": uint8(activityType & 0xffff),
		}})
	}
	if activityType == 0x00010006 {
		leg := receipt.Transfers[0]
		events = append(events, Event{Name: "PaymentDrawn", Fields: map[string]any{
			"grantId": args.GrantID, "from": leg.From, "to": leg.To,
			"assetId": leg.Asset, "amount": leg.Amount.BigInt(),
			"idempotencyKey": args.IdempotencyKey, "contextHash": args.ContextHash,
		}})
	}
	if args.PayExact {
		leg := receipt.Transfers[0]
		events = append(events, Event{Name: "PaymentExact", Fields: map[string]any{
			"paymentReference": args.PaymentReference, "from": leg.From, "to": leg.To,
			"assetId": leg.Asset, "amount": leg.Amount.BigInt(),
		}})
	}
	return events, nil
}
