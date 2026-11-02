package types

import (
	"encoding/binary"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
)

const (
	EscrowRecordBytes   = 305
	EscrowResultBytes   = 243
	EscrowResultV2Bytes = 276
)

type EscrowRecord struct {
	EscrowID           [32]byte
	Owner              [32]byte
	EscrowAccount      [32]byte
	Beneficiary        [32]byte
	Arbiter            [32]byte
	AssetID            [32]byte
	LockedAmount       codec.U128
	CapturedAmount     codec.U128
	State              uint8
	ExpiryMs           uint64
	DisputeWindowMs    uint64
	TermsHash          [32]byte
	AgreementReference [32]byte
}

func DecodeEscrowRecord(data []byte) (EscrowRecord, error) {
	if len(data) != EscrowRecordBytes {
		return EscrowRecord{}, errBudgetNonCanonical
	}
	var record EscrowRecord
	copy(record.EscrowID[:], data[:32])
	if !budgetNonzero(record.EscrowID) || data[224] < 1 || data[224] > 7 {
		return EscrowRecord{}, errBudgetNonCanonical
	}
	copy(record.Owner[:], data[32:64])
	copy(record.EscrowAccount[:], data[64:96])
	copy(record.Beneficiary[:], data[96:128])
	copy(record.Arbiter[:], data[128:160])
	copy(record.AssetID[:], data[160:192])
	record.LockedAmount = decodeRecordU128(data[192:208])
	record.CapturedAmount = decodeRecordU128(data[208:224])
	record.State = data[224]
	record.ExpiryMs = binary.BigEndian.Uint64(data[225:233])
	record.DisputeWindowMs = binary.BigEndian.Uint64(data[233:241])
	copy(record.TermsHash[:], data[241:273])
	copy(record.AgreementReference[:], data[273:305])
	return record, nil
}

type EscrowResult struct {
	EscrowID        [32]byte
	Ordinal         uint16
	StateAfter      uint8
	CapturedAfter   codec.U128
	LockedAfter     codec.U128
	AssetID         [32]byte
	From            [32]byte
	To              [32]byte
	Amount          codec.U128
	SecondaryAmount codec.U128
	TransferSetRoot [32]byte
	GlobalSequence  uint64
	TimestampMs     uint64
	ContextBound    bool
	ContextDigest   [32]byte
}

func DecodeEscrowResult(data []byte) (EscrowResult, error) {
	if len(data) != EscrowResultBytes && len(data) != EscrowResultV2Bytes {
		return EscrowResult{}, errBudgetNonCanonical
	}
	var result EscrowResult
	copy(result.EscrowID[:], data[:32])
	result.Ordinal = binary.BigEndian.Uint16(data[32:34])
	result.StateAfter = data[34]
	if !budgetNonzero(result.EscrowID) || result.Ordinal < 1 || result.Ordinal > 7 || result.StateAfter < 1 || result.StateAfter > 7 {
		return EscrowResult{}, errBudgetNonCanonical
	}
	if len(data) == EscrowResultV2Bytes {
		copy(result.ContextDigest[:], data[244:276])
		if data[243] != 2 || !budgetNonzero(result.ContextDigest) {
			return EscrowResult{}, errBudgetNonCanonical
		}
		result.ContextBound = true
	}
	result.CapturedAfter = decodeRecordU128(data[35:51])
	result.LockedAfter = decodeRecordU128(data[51:67])
	copy(result.AssetID[:], data[67:99])
	copy(result.From[:], data[99:131])
	copy(result.To[:], data[131:163])
	result.Amount = decodeRecordU128(data[163:179])
	result.SecondaryAmount = decodeRecordU128(data[179:195])
	copy(result.TransferSetRoot[:], data[195:227])
	result.GlobalSequence = binary.BigEndian.Uint64(data[227:235])
	result.TimestampMs = binary.BigEndian.Uint64(data[235:243])
	return result, nil
}

func decodeRecordU128(data []byte) codec.U128 {
	return codec.U128{Hi: binary.BigEndian.Uint64(data[:8]), Lo: binary.BigEndian.Uint64(data[8:16])}
}
