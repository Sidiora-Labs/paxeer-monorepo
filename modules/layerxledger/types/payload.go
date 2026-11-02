package types

import (
	"encoding/binary"

	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
)

type RegisterArgs struct {
	AssetID          [32]byte
	Salt             [32]byte
	Symbol           string
	Name             string
	Decimals         uint8
	SupplyCap        sdk.Int
	IssuerKind       uint8
	CustodyReference []byte
}

func EncodeRegisterPayload(args RegisterArgs) ([]byte, error) {
	if len(args.Symbol) == 0 || len(args.Symbol) > AssetSymbolMax ||
		len(args.Name) == 0 || len(args.Name) > AssetNameMax ||
		len(args.CustodyReference) > AssetCustodyReferenceMax || args.Decimals > AssetDecimalsMax {
		return nil, ErrNonCanonical
	}
	capBytes, err := assetPayloadU128(args.SupplyCap)
	if err != nil {
		return nil, err
	}
	payload := make([]byte, 66, 87+len(args.Symbol)+len(args.Name)+len(args.CustodyReference))
	binary.BigEndian.PutUint16(payload[:2], 1)
	copy(payload[2:34], args.AssetID[:])
	copy(payload[34:66], args.Salt[:])
	payload = append(payload, byte(len(args.Symbol)))
	payload = append(payload, args.Symbol...)
	payload = append(payload, byte(len(args.Name)))
	payload = append(payload, args.Name...)
	payload = append(payload, args.Decimals)
	payload = append(payload, capBytes[:]...)
	payload = append(payload, args.IssuerKind, byte(len(args.CustodyReference)))
	payload = append(payload, args.CustodyReference...)
	if err := ValidateRegisterPayload(payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func EncodePausePayload(assetID [32]byte) []byte {
	payload := make([]byte, PausePayloadLength)
	binary.BigEndian.PutUint16(payload[:2], 1)
	copy(payload[2:], assetID[:])
	return payload
}

func EncodeOpenAccountPayload(assetID [32]byte) []byte {
	return EncodePausePayload(assetID)
}

func EncodeSupplyPayload(assetID, accountID [32]byte, amount sdk.Int) ([]byte, error) {
	amountBytes, err := assetPayloadU128(amount)
	if err != nil {
		return nil, err
	}
	if amount.IsZero() {
		return nil, ErrZeroAmount
	}
	payload := make([]byte, 82)
	binary.BigEndian.PutUint16(payload[:2], 1)
	copy(payload[2:34], assetID[:])
	copy(payload[34:66], accountID[:])
	copy(payload[66:82], amountBytes[:])
	return payload, nil
}

func EncodeRevokeGrantPayload(grantID [32]byte, revocationSequence uint64) []byte {
	payload := make([]byte, 42)
	binary.BigEndian.PutUint16(payload[:2], 1)
	copy(payload[2:34], grantID[:])
	binary.BigEndian.PutUint64(payload[34:42], revocationSequence)
	return payload
}

func assetPayloadU128(value sdk.Int) ([16]byte, error) {
	var encoded [16]byte
	if value.IsNil() || value.IsNegative() {
		return encoded, ErrNonCanonical
	}
	integer := value.BigInt()
	if integer.BitLen() > 128 {
		return encoded, ErrNonCanonical
	}
	integer.FillBytes(encoded[:])
	return encoded, nil
}
