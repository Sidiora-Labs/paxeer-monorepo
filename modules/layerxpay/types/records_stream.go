package types

import (
	"bytes"
	"encoding/binary"
	"math/big"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
)

const (
	StreamRecordBytes = 541
	StreamResultBytes = 99
)

type StreamRecord struct {
	StreamID             [32]byte
	Payer                [32]byte
	StreamAccount        [32]byte
	Recipient            [32]byte
	AssetID              [32]byte
	Mode                 uint8
	Rate                 codec.U128
	RateUnit             uint64
	StartTimestamp       uint64
	LastAccrualTimestamp uint64
	EndTimestamp         uint64
	TotalCap             codec.U128
	AccruedTotal         codec.U128
	SettledTotal         codec.U128
	RemainderCarry       codec.U128
	CumulativeMeter      uint64
	MeterAuthorities     [][32]byte
	Underfunded          bool
	Paused               bool
	Closed               bool
}

func DecodeStreamRecord(b []byte) (StreamRecord, error) {
	if len(b) != StreamRecordBytes {
		return StreamRecord{}, errStreamNonCanonical
	}
	count := int(b[281])
	if count > StreamMaxMeterAuthorities || b[538] > 1 || b[539] > 1 || b[540] > 1 {
		return StreamRecord{}, errStreamNonCanonical
	}
	var r StreamRecord
	copy(r.StreamID[:], b[0:32])
	copy(r.Payer[:], b[32:64])
	copy(r.StreamAccount[:], b[64:96])
	copy(r.Recipient[:], b[96:128])
	copy(r.AssetID[:], b[128:160])
	r.Mode = b[160]
	r.Rate = streamRecordU128(b[161:177])
	r.RateUnit = binary.BigEndian.Uint64(b[177:185])
	r.StartTimestamp = binary.BigEndian.Uint64(b[185:193])
	r.LastAccrualTimestamp = binary.BigEndian.Uint64(b[193:201])
	r.EndTimestamp = binary.BigEndian.Uint64(b[201:209])
	r.TotalCap = streamRecordU128(b[209:225])
	r.AccruedTotal = streamRecordU128(b[225:241])
	r.SettledTotal = streamRecordU128(b[241:257])
	r.RemainderCarry = streamRecordU128(b[257:273])
	r.CumulativeMeter = binary.BigEndian.Uint64(b[273:281])
	r.MeterAuthorities = make([][32]byte, count)
	for i := 0; i < StreamMaxMeterAuthorities; i++ {
		var authority [32]byte
		copy(authority[:], b[282+32*i:314+32*i])
		if i < count {
			if authority == ([32]byte{}) || i > 0 && bytes.Compare(r.MeterAuthorities[i-1][:], authority[:]) >= 0 {
				return StreamRecord{}, errStreamNonCanonical
			}
			r.MeterAuthorities[i] = authority
		} else if authority != ([32]byte{}) {
			return StreamRecord{}, errStreamNonCanonical
		}
	}
	r.Underfunded, r.Paused, r.Closed = b[538] == 1, b[539] == 1, b[540] == 1
	if !budgetNonzero(r.StreamID, r.Payer, r.StreamAccount, r.Recipient, r.AssetID) ||
		r.StreamAccount == r.Payer || r.StreamAccount == r.Recipient ||
		!(r.Mode == StreamModeTime && count == 0 || r.Mode == StreamModeMetered && count != 0) ||
		r.Rate.IsZero() || r.RateUnit == 0 || r.StartTimestamp == 0 ||
		r.LastAccrualTimestamp < r.StartTimestamp ||
		(r.EndTimestamp != 0 && r.EndTimestamp <= r.StartTimestamp) ||
		r.TotalCap.IsZero() || streamRecordU128Greater(r.AccruedTotal, r.TotalCap) ||
		streamRecordU128Greater(r.SettledTotal, r.AccruedTotal) ||
		r.RemainderCarry.Hi != 0 || r.RemainderCarry.Lo >= r.RateUnit {
		return StreamRecord{}, errStreamNonCanonical
	}
	return r, nil
}

type StreamResult struct {
	StreamID        [32]byte
	TransferSetRoot [32]byte
	Paid            codec.U128
	Refunded        codec.U128
	Ordinal         uint16
	LegCount        uint8
}

func DecodeStreamResult(b []byte) (StreamResult, error) {
	if len(b) != StreamResultBytes {
		return StreamResult{}, errStreamNonCanonical
	}
	ordinal := binary.BigEndian.Uint16(b[64:66])
	if ordinal == 0 || ordinal > 8 || b[66] > 2 {
		return StreamResult{}, errStreamNonCanonical
	}
	var r StreamResult
	copy(r.TransferSetRoot[:], b[0:32])
	r.Paid = streamRecordU128(b[32:48])
	r.Refunded = streamRecordU128(b[48:64])
	r.Ordinal, r.LegCount = ordinal, b[66]
	copy(r.StreamID[:], b[67:99])
	return r, nil
}

type StreamTuple struct {
	Payer            [32]byte
	StreamAccount    [32]byte
	Recipient        [32]byte
	AssetId          [32]byte
	Mode             uint8
	Rate             *big.Int
	RateUnit         uint64
	StartMs          uint64
	LastAccrualMs    uint64
	EndMs            uint64
	TotalCap         *big.Int
	AccruedTotal     *big.Int
	SettledTotal     *big.Int
	RemainderCarry   *big.Int
	CumulativeMeter  uint64
	MeterAuthorities [][32]byte
	Underfunded      bool
	Paused           bool
	Closed           bool
}

func (r StreamRecord) ABITuple() StreamTuple {
	return StreamTuple{
		Payer: r.Payer, StreamAccount: r.StreamAccount, Recipient: r.Recipient, AssetId: r.AssetID,
		Mode: r.Mode, Rate: streamRecordBigInt(r.Rate), RateUnit: r.RateUnit,
		StartMs: r.StartTimestamp, LastAccrualMs: r.LastAccrualTimestamp, EndMs: r.EndTimestamp,
		TotalCap: streamRecordBigInt(r.TotalCap), AccruedTotal: streamRecordBigInt(r.AccruedTotal),
		SettledTotal: streamRecordBigInt(r.SettledTotal), RemainderCarry: streamRecordBigInt(r.RemainderCarry),
		CumulativeMeter: r.CumulativeMeter, MeterAuthorities: append([][32]byte(nil), r.MeterAuthorities...),
		Underfunded: r.Underfunded, Paused: r.Paused, Closed: r.Closed,
	}
}

type StreamResultTuple struct {
	StreamId        [32]byte
	TransferSetRoot [32]byte
	Paid            *big.Int
	Refunded        *big.Int
	Ordinal         uint16
	LegCount        uint8
}

func (r StreamResult) ABITuple() StreamResultTuple {
	return StreamResultTuple{
		StreamId: r.StreamID, TransferSetRoot: r.TransferSetRoot,
		Paid: streamRecordBigInt(r.Paid), Refunded: streamRecordBigInt(r.Refunded),
		Ordinal: r.Ordinal, LegCount: r.LegCount,
	}
}

func streamRecordU128(b []byte) codec.U128 {
	return codec.U128{Hi: binary.BigEndian.Uint64(b[:8]), Lo: binary.BigEndian.Uint64(b[8:])}
}

func streamRecordU128Greater(a, b codec.U128) bool {
	return a.Hi > b.Hi || a.Hi == b.Hi && a.Lo > b.Lo
}

func streamRecordBigInt(v codec.U128) *big.Int {
	b := v.Bytes()
	return new(big.Int).SetBytes(b[:])
}
