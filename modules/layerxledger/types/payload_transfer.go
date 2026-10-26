package types

import (
	"encoding/binary"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

const (
	receiveWireTag             = 0x5201
	receiveFieldCount          = 10
	receiveHeadLength          = 2 + 2 + 32 + 32 + 32 + 16 + 32 + 8 + 32 + 32
	issueGrantPayloadLength    = 346
	grantRecurringOffset       = 32*4 + 16*2
	grantReferenceOffset       = grantRecurringOffset + 1 + 8 + 8 + 32
	transferAuthorizationOwner = 1
)

type SendArgs struct {
	From            [32]byte
	To              [32]byte
	Asset           [32]byte
	Amount          sdk.Int
	Sequence        uint64
	IdempotencyKey  [32]byte
	ExpiresAt       uint64
	ContextHash     [32]byte
	PublicKey       [32]byte
	NetworkID       uint32
	ProtocolVersion uint16
}

type ReceiveArgs struct {
	From             [32]byte
	To               [32]byte
	Asset            [32]byte
	Amount           sdk.Int
	GrantID          [32]byte
	ReceiverSequence uint64
	IdempotencyKey   [32]byte
	ContextHash      [32]byte
	PublicKey        [32]byte
	NetworkID        uint32
	ProtocolVersion  uint16
	PayerGrant       []byte
}

type PayExactArgs struct {
	From             [32]byte
	To               [32]byte
	Asset            [32]byte
	Amount           sdk.Int
	ExpiresAt        uint64
	PaymentReference [32]byte
}

type DrawArgs struct {
	From             [32]byte
	Asset            [32]byte
	Amount           sdk.Int
	GrantID          [32]byte
	ReceiverSequence uint64
	IdempotencyKey   [32]byte
	ContextHash      [32]byte
}

func transferAmount(amount sdk.Int) ([16]byte, error) {
	var encoded [16]byte
	if amount.IsNil() || amount.IsNegative() || amount.BigInt().BitLen() > 128 {
		return encoded, ErrNonCanonical
	}
	if amount.IsZero() {
		return encoded, ErrZeroAmount
	}
	amount.BigInt().FillBytes(encoded[:])
	return encoded, nil
}

func encodeTransferAuthorization(out []byte, controller, publicKey, contextHash [32]byte, networkID uint32, protocolVersion uint16) {
	out[0] = transferAuthorizationOwner
	copy(out[1:33], controller[:])
	copy(out[33:65], publicKey[:])
	copy(out[129:161], contextHash[:])
	binary.BigEndian.PutUint32(out[161:165], networkID)
	binary.BigEndian.PutUint16(out[165:167], protocolVersion)
}

func EncodeSendPayload(args SendArgs) ([]byte, error) {
	amount, err := transferAmount(args.Amount)
	if err != nil {
		return nil, err
	}
	payload := make([]byte, sendHeadLength+sendAuthorizationLength)
	binary.BigEndian.PutUint16(payload[0:2], sendWireTag)
	binary.BigEndian.PutUint16(payload[2:4], sendFieldCount)
	copy(payload[4:36], args.From[:])
	copy(payload[36:68], args.To[:])
	copy(payload[68:100], args.Asset[:])
	copy(payload[100:116], amount[:])
	binary.BigEndian.PutUint64(payload[116:124], args.Sequence)
	copy(payload[124:156], args.IdempotencyKey[:])
	binary.BigEndian.PutUint64(payload[156:164], args.ExpiresAt)
	copy(payload[164:196], args.ContextHash[:])
	encodeTransferAuthorization(payload[sendHeadLength:], args.From, args.PublicKey, args.ContextHash, args.NetworkID, args.ProtocolVersion)
	return payload, nil
}

func EncodeReceivePayload(args ReceiveArgs) ([]byte, error) {
	amount, err := transferAmount(args.Amount)
	if err != nil {
		return nil, err
	}
	if err := ValidateIssueGrantPayload(args.PayerGrant); err != nil {
		return nil, err
	}
	payload := make([]byte, receiveHeadLength+sendAuthorizationLength+issueGrantPayloadLength)
	binary.BigEndian.PutUint16(payload[0:2], receiveWireTag)
	binary.BigEndian.PutUint16(payload[2:4], receiveFieldCount)
	copy(payload[4:36], args.From[:])
	copy(payload[36:68], args.To[:])
	copy(payload[68:100], args.Asset[:])
	copy(payload[100:116], amount[:])
	copy(payload[116:148], args.GrantID[:])
	binary.BigEndian.PutUint64(payload[148:156], args.ReceiverSequence)
	copy(payload[156:188], args.IdempotencyKey[:])
	copy(payload[188:220], args.ContextHash[:])
	encodeTransferAuthorization(payload[receiveHeadLength:], args.To, args.PublicKey, args.ContextHash, args.NetworkID, args.ProtocolVersion)
	copy(payload[receiveHeadLength+sendAuthorizationLength:], args.PayerGrant)
	return payload, nil
}

func ValidateIssueGrantPayload(payload []byte) error {
	if len(payload) != issueGrantPayloadLength || payload[grantRecurringOffset] > 1 || payload[grantReferenceOffset] > 1 {
		return ErrNonCanonical
	}
	return nil
}
