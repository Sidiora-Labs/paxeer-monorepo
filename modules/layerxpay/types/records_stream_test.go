package types

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"reflect"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/ethereum/go-ethereum/accounts/abi"
)

func streamRecordFixture(count int) ([]byte, StreamRecord) {
	want := StreamRecord{
		StreamID: streamID(0x10), Payer: streamID(0x20), StreamAccount: streamID(0x30),
		Recipient: streamID(0x40), AssetID: streamID(0x50), Mode: StreamModeMetered,
		Rate:     codec.U128{Hi: 0x0102030405060708, Lo: 0x1112131415161718},
		RateUnit: 0x2122232425262728, StartTimestamp: 0x3132333435363738,
		LastAccrualTimestamp: 0x4142434445464748, EndTimestamp: 0x5152535455565758,
		TotalCap:       codec.U128{Hi: 0xf1f2f3f4f5f6f7f8, Lo: 0x8182838485868788},
		AccruedTotal:   codec.U128{Hi: 0xa1a2a3a4a5a6a7a8, Lo: 0x9192939495969798},
		SettledTotal:   codec.U128{Hi: 0x7172737475767778, Lo: 0xb1b2b3b4b5b6b7b8},
		RemainderCarry: codec.U128{Lo: 0x0102030405060708}, CumulativeMeter: 0xc1c2c3c4c5c6c7c8,
		MeterAuthorities: make([][32]byte, count), Underfunded: true, Paused: true, Closed: true,
	}
	if count == 0 {
		want.Mode = StreamModeTime
	}
	b := make([]byte, 541)
	for i, id := range [][32]byte{want.StreamID, want.Payer, want.StreamAccount, want.Recipient, want.AssetID} {
		copy(b[32*i:32*(i+1)], id[:])
	}
	b[160] = want.Mode
	for _, field := range []struct {
		offset int
		value  codec.U128
	}{{161, want.Rate}, {209, want.TotalCap}, {225, want.AccruedTotal}, {241, want.SettledTotal}, {257, want.RemainderCarry}} {
		binary.BigEndian.PutUint64(b[field.offset:field.offset+8], field.value.Hi)
		binary.BigEndian.PutUint64(b[field.offset+8:field.offset+16], field.value.Lo)
	}
	for _, field := range []struct {
		offset int
		value  uint64
	}{{177, want.RateUnit}, {185, want.StartTimestamp}, {193, want.LastAccrualTimestamp}, {201, want.EndTimestamp}, {273, want.CumulativeMeter}} {
		binary.BigEndian.PutUint64(b[field.offset:field.offset+8], field.value)
	}
	b[281] = byte(count)
	for i := range want.MeterAuthorities {
		want.MeterAuthorities[i] = streamID(byte(i + 1))
		copy(b[282+32*i:314+32*i], want.MeterAuthorities[i][:])
	}
	b[538], b[539], b[540] = 1, 1, 1
	return b, want
}

func streamViewOutputs(t *testing.T, method string) abi.Arguments {
	t.Helper()
	b, err := os.ReadFile("../../../precompiles/layerxpay/abi.json")
	if err != nil {
		t.Fatal(err)
	}
	contract, err := abi.JSON(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	view, ok := contract.Methods[method]
	if !ok {
		t.Fatalf("missing ABI method %s", method)
	}
	return view.Outputs
}

func streamABIWord(value []byte) []byte {
	word := make([]byte, 32)
	copy(word[32-len(value):], value)
	return word
}

func TestDecodeStreamRecordLayout(t *testing.T) {
	if StreamRecordBytes != 541 || StreamMaxMeterAuthorities != 8 {
		t.Fatal("stream record constants differ from lx_stream.h")
	}
	outputs := streamViewOutputs(t, "getStream")
	for count := 0; count <= StreamMaxMeterAuthorities; count++ {
		t.Run(fmt.Sprintf("authorities_%d", count), func(t *testing.T) {
			b, want := streamRecordFixture(count)
			got, err := DecodeStreamRecord(b)
			if err != nil || !reflect.DeepEqual(got, want) {
				t.Fatalf("decoded record = %+v, %v; want %+v", got, err, want)
			}
			tuple := got.ABITuple()
			packed, err := outputs.Pack(tuple)
			if err != nil {
				t.Fatalf("pack Solidity Stream tuple: %v", err)
			}
			encoded := streamABIWord([]byte{32})
			for _, field := range []struct{ start, end int }{
				{32, 64}, {64, 96}, {96, 128}, {128, 160}, {160, 161}, {161, 177},
				{177, 185}, {185, 193}, {193, 201}, {201, 209}, {209, 225}, {225, 241},
				{241, 257}, {257, 273}, {273, 281},
			} {
				encoded = append(encoded, streamABIWord(b[field.start:field.end])...)
			}
			encoded = append(encoded, streamABIWord([]byte{0x02, 0x60})...)
			for _, flag := range b[538:541] {
				encoded = append(encoded, streamABIWord([]byte{flag})...)
			}
			encoded = append(encoded, streamABIWord([]byte{byte(count)})...)
			encoded = append(encoded, b[282:282+32*count]...)
			if !bytes.Equal(packed, encoded) {
				t.Fatalf("Solidity Stream encoding differs:\ngot  %x\nwant %x", packed, encoded)
			}
			if count > 0 {
				b[282] ^= 0xff
				if got.MeterAuthorities[0] != want.MeterAuthorities[0] {
					t.Fatal("record aliases input authorities")
				}
				tuple.MeterAuthorities[0][0] ^= 0xff
				if got.MeterAuthorities[0] != want.MeterAuthorities[0] {
					t.Fatal("ABI tuple aliases record authorities")
				}
			}
		})
	}
	b, _ := streamRecordFixture(2)
	for flags := byte(0); flags < 8; flags++ {
		b[538], b[539], b[540] = flags&1, (flags>>1)&1, (flags>>2)&1
		got, err := DecodeStreamRecord(b)
		if err != nil || got.Underfunded != (flags&1 != 0) || got.Paused != (flags&2 != 0) || got.Closed != (flags&4 != 0) {
			t.Fatalf("flags %d: record %+v, error %v", flags, got, err)
		}
	}
	assertRejected := func(t *testing.T, input []byte) {
		t.Helper()
		got, err := DecodeStreamRecord(input)
		if !errors.Is(err, errStreamNonCanonical) || err.Error() != "LXP_NON_CANONICAL" || !reflect.DeepEqual(got, StreamRecord{}) {
			t.Fatalf("malformed record of length %d returned %+v, %v", len(input), got, err)
		}
	}
	assertRejected(t, nil)
	for length := 0; length < 541; length++ {
		assertRejected(t, b[:length])
	}
	assertRejected(t, append(append([]byte(nil), b...), 0))
	assertRejected(t, append(append([]byte(nil), b...), b...))
	for _, tc := range []struct {
		name string
		edit func([]byte)
	}{
		{"count_over_max", func(b []byte) { b[281] = 9 }},
		{"count_255", func(b []byte) { b[281] = 255 }},
		{"mode_zero", func(b []byte) { b[160] = 0 }},
		{"mode_unknown", func(b []byte) { b[160] = 3 }},
		{"time_with_authorities", func(b []byte) { b[160] = StreamModeTime }},
		{"metered_without_authorities", func(b []byte) { clear(b[281:538]) }},
		{"authority_zero", func(b []byte) { clear(b[282:314]) }},
		{"authority_duplicate", func(b []byte) { copy(b[314:346], b[282:314]) }},
		{"authority_unsorted", func(b []byte) { b[282] = 3 }},
		{"unused_authority_nonzero", func(b []byte) { b[537] = 1 }},
		{"stream_account_is_payer", func(b []byte) { copy(b[64:96], b[32:64]) }},
		{"stream_account_is_recipient", func(b []byte) { copy(b[64:96], b[96:128]) }},
		{"rate_zero", func(b []byte) { clear(b[161:177]) }},
		{"rate_unit_zero", func(b []byte) { clear(b[177:185]) }},
		{"start_zero", func(b []byte) { clear(b[185:193]) }},
		{"last_before_start", func(b []byte) { clear(b[193:201]) }},
		{"end_equals_start", func(b []byte) { copy(b[201:209], b[185:193]) }},
		{"cap_zero", func(b []byte) { clear(b[209:225]) }},
		{"accrued_exceeds_cap", func(b []byte) { b[225] = 0xff }},
		{"settled_exceeds_accrued", func(b []byte) { b[241] = 0xff }},
		{"remainder_high_word", func(b []byte) { b[257] = 1 }},
		{"remainder_equals_unit", func(b []byte) { copy(b[265:273], b[177:185]) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			input, _ := streamRecordFixture(2)
			tc.edit(input)
			assertRejected(t, input)
		})
	}
	for _, offset := range []int{0, 32, 64, 96, 128} {
		input, _ := streamRecordFixture(2)
		clear(input[offset : offset+32])
		assertRejected(t, input)
	}
	for _, offset := range []int{538, 539, 540} {
		for _, value := range []byte{2, 255} {
			input, _ := streamRecordFixture(2)
			input[offset] = value
			assertRejected(t, input)
		}
	}
	boundary, _ := streamRecordFixture(0)
	clear(boundary[201:209])
	copy(boundary[193:201], boundary[185:193])
	copy(boundary[225:241], boundary[209:225])
	copy(boundary[241:257], boundary[225:241])
	clear(boundary[257:273])
	if _, err := DecodeStreamRecord(boundary); err != nil {
		t.Fatalf("valid open-ended stream at cap with zero remainder: %v", err)
	}
}

func TestDecodeStreamResultLayout(t *testing.T) {
	if StreamResultBytes != 99 {
		t.Fatal("stream result size differs from lx_stream.h")
	}
	want := StreamResult{
		StreamID: streamID(0x20), TransferSetRoot: streamID(0x80),
		Paid:     codec.U128{Hi: 0xffffffffffffffff, Lo: 0xffffffffffffffff},
		Refunded: codec.U128{Hi: 0x8182838485868788, Lo: 0x9192939495969798},
		Ordinal:  8, LegCount: 2,
	}
	b := make([]byte, 99)
	copy(b[0:32], want.TransferSetRoot[:])
	binary.BigEndian.PutUint64(b[32:40], want.Paid.Hi)
	binary.BigEndian.PutUint64(b[40:48], want.Paid.Lo)
	binary.BigEndian.PutUint64(b[48:56], want.Refunded.Hi)
	binary.BigEndian.PutUint64(b[56:64], want.Refunded.Lo)
	binary.BigEndian.PutUint16(b[64:66], want.Ordinal)
	b[66] = want.LegCount
	copy(b[67:99], want.StreamID[:])
	got, err := DecodeStreamResult(b)
	if err != nil || got != want {
		t.Fatalf("decoded result = %+v, %v; want %+v", got, err, want)
	}
	packed, err := streamViewOutputs(t, "getStreamResult").Pack(got.ABITuple())
	if err != nil {
		t.Fatalf("pack Solidity StreamResult tuple: %v", err)
	}
	var encoded []byte
	for _, field := range []struct{ start, end int }{{67, 99}, {0, 32}, {32, 48}, {48, 64}, {64, 66}, {66, 67}} {
		encoded = append(encoded, streamABIWord(b[field.start:field.end])...)
	}
	if !bytes.Equal(packed, encoded) {
		t.Fatalf("Solidity StreamResult encoding differs:\ngot  %x\nwant %x", packed, encoded)
	}
	assertRejected := func(input []byte) {
		t.Helper()
		got, err := DecodeStreamResult(input)
		if !errors.Is(err, errStreamNonCanonical) || err.Error() != "LXP_NON_CANONICAL" || got != (StreamResult{}) {
			t.Fatalf("malformed result of length %d returned %+v, %v", len(input), got, err)
		}
	}
	assertRejected(nil)
	for length := 0; length < 99; length++ {
		assertRejected(b[:length])
	}
	assertRejected(append(append([]byte(nil), b...), 0))
	assertRejected(append(append([]byte(nil), b...), b...))
	for _, ordinal := range []uint16{0, 9, 256, 65535} {
		input := append([]byte(nil), b...)
		binary.BigEndian.PutUint16(input[64:66], ordinal)
		assertRejected(input)
	}
	for _, legs := range []byte{3, 255} {
		input := append([]byte(nil), b...)
		input[66] = legs
		assertRejected(input)
	}
	for ordinal := uint16(1); ordinal <= 8; ordinal++ {
		for legs := byte(0); legs <= 2; legs++ {
			input := make([]byte, 99)
			binary.BigEndian.PutUint16(input[64:66], ordinal)
			input[66] = legs
			got, err := DecodeStreamResult(input)
			if err != nil || got != (StreamResult{Ordinal: ordinal, LegCount: legs}) {
				t.Fatalf("valid zero-amount result ordinal %d, legs %d: %+v, %v", ordinal, legs, got, err)
			}
		}
	}
	clear(b)
	if got != want {
		t.Fatal("result aliases input bytes")
	}
}
