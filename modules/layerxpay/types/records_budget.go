package types

import (
	"bytes"
	"encoding/binary"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
)

const (
	BudgetRecordFixedBytes = 278
	BudgetMaxDelegates     = 16
	BudgetRecordMaxBytes   = BudgetRecordFixedBytes + 32*BudgetMaxDelegates + 32
)

type BudgetRecord struct {
	NativeSource          bool
	SourceAccount         [32]byte
	BudgetID              [32]byte
	Owner                 [32]byte
	BudgetAccount         [32]byte
	AssetID               [32]byte
	PurposeHash           [32]byte
	PerPeriodLimit        codec.U128
	ConfiguredPeriodLimit codec.U128
	CarryCap              codec.U128
	SpentThisPeriod       codec.U128
	Carried               codec.U128
	PeriodLengthMs        uint64
	PeriodStartMs         uint64
	ExpiryMs              uint64
	RevocationSequence    uint64
	RolloverPolicy        uint8
	Closed                bool
	Revoked               bool
	Delegates             [][32]byte
}

func DecodeBudgetRecord(data []byte) (BudgetRecord, error) {
	if len(data) < BudgetRecordFixedBytes || data[0] != 0 || (data[1] != 1 && data[1] != 2) || data[275] > 1 || data[276] > 1 {
		return BudgetRecord{}, errBudgetNonCanonical
	}
	count := int(data[277])
	native := data[1] == 2
	length := BudgetRecordFixedBytes + 32*count
	if native {
		length += 32
	}
	if count > BudgetMaxDelegates || len(data) != length {
		return BudgetRecord{}, errBudgetNonCanonical
	}
	var record BudgetRecord
	record.NativeSource = native
	if native {
		copy(record.SourceAccount[:], data[length-32:])
		if !budgetNonzero(record.SourceAccount) {
			return BudgetRecord{}, errBudgetNonCanonical
		}
	}
	copy(record.BudgetID[:], data[2:34])
	copy(record.Owner[:], data[34:66])
	copy(record.BudgetAccount[:], data[66:98])
	copy(record.AssetID[:], data[98:130])
	copy(record.PurposeHash[:], data[130:162])
	record.PerPeriodLimit = decodeRecordU128(data[162:178])
	record.ConfiguredPeriodLimit = decodeRecordU128(data[178:194])
	record.CarryCap = decodeRecordU128(data[194:210])
	record.SpentThisPeriod = decodeRecordU128(data[210:226])
	record.Carried = decodeRecordU128(data[226:242])
	record.PeriodLengthMs = binary.BigEndian.Uint64(data[242:250])
	record.PeriodStartMs = binary.BigEndian.Uint64(data[250:258])
	record.ExpiryMs = binary.BigEndian.Uint64(data[258:266])
	record.RevocationSequence = binary.BigEndian.Uint64(data[266:274])
	record.RolloverPolicy = data[274]
	record.Closed = data[275] != 0
	record.Revoked = data[276] != 0
	if !budgetNonzero(record.BudgetID) || record.PerPeriodLimit.IsZero() || record.PeriodLengthMs == 0 || record.ExpiryMs <= record.PeriodStartMs || record.RolloverPolicy < 1 || record.RolloverPolicy > 2 || (record.RolloverPolicy == 1 && !record.CarryCap.IsZero()) {
		return BudgetRecord{}, errBudgetNonCanonical
	}
	record.Delegates = make([][32]byte, count)
	for i := range record.Delegates {
		offset := BudgetRecordFixedBytes + 32*i
		if i > 0 && bytes.Compare(data[offset-32:offset], data[offset:offset+32]) >= 0 {
			return BudgetRecord{}, errBudgetNonCanonical
		}
		copy(record.Delegates[i][:], data[offset:offset+32])
	}
	return record, nil
}
