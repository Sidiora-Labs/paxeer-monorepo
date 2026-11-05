package types_test

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"reflect"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxpay/types"
)

func TestDecodeEscrowRecordLayout(t *testing.T) {
	data := make([]byte, 305)
	want := types.EscrowRecord{
		EscrowID: recordTestID(1), Owner: recordTestID(2), EscrowAccount: recordTestID(3),
		Beneficiary: recordTestID(4), Arbiter: recordTestID(5), AssetID: recordTestID(6),
		LockedAmount:   codec.U128{Hi: 0x0102030405060708, Lo: 0x1112131415161718},
		CapturedAmount: codec.U128{Hi: 0x2122232425262728, Lo: 0x3132333435363738},
		State:          2, ExpiryMs: 0x8182838485868788, DisputeWindowMs: 0x9192939495969798,
		TermsHash: recordTestID(7), AgreementReference: recordTestID(8),
	}
	for i, id := range [][32]byte{want.EscrowID, want.Owner, want.EscrowAccount, want.Beneficiary, want.Arbiter, want.AssetID} {
		copy(data[i*32:(i+1)*32], id[:])
	}
	recordTestPut128(data[192:208], 0x0102030405060708, 0x1112131415161718)
	recordTestPut128(data[208:224], 0x2122232425262728, 0x3132333435363738)
	data[224] = 2
	binary.BigEndian.PutUint64(data[225:233], 0x8182838485868788)
	binary.BigEndian.PutUint64(data[233:241], 0x9192939495969798)
	copy(data[241:273], want.TermsHash[:])
	copy(data[273:305], want.AgreementReference[:])
	original := bytes.Clone(data)
	got, err := types.DecodeEscrowRecord(data)
	if err != nil || got != want {
		t.Fatalf("escrow layout: got %+v, err %v; want %+v", got, err, want)
	}
	if !bytes.Equal(data, original) {
		t.Fatal("escrow decoder changed input")
	}
	for n := 0; n < len(data); n++ {
		value, err := types.DecodeEscrowRecord(data[:n])
		recordTestNonCanonical(t, err)
		if value != (types.EscrowRecord{}) {
			t.Fatalf("length %d returned partial escrow", n)
		}
	}
	for _, n := range []int{306, 337, 1024} {
		_, err := types.DecodeEscrowRecord(append(bytes.Clone(data), make([]byte, n-len(data))...))
		recordTestNonCanonical(t, err)
	}
	for state := byte(1); state <= 7; state++ {
		changed := bytes.Clone(data)
		changed[224] = state
		value, err := types.DecodeEscrowRecord(changed)
		if err != nil || value.State != state {
			t.Fatalf("state %d: %+v, %v", state, value, err)
		}
	}
	for _, state := range []byte{0, 8, 255} {
		changed := bytes.Clone(data)
		changed[224] = state
		_, err := types.DecodeEscrowRecord(changed)
		recordTestNonCanonical(t, err)
	}
	changed := bytes.Clone(data)
	clear(changed[:32])
	_, err = types.DecodeEscrowRecord(changed)
	recordTestNonCanonical(t, err)
	clear(data)
	if got != want {
		t.Fatal("escrow record aliases input")
	}

	t.Run("economic_results", func(t *testing.T) {
		data := make([]byte, 276)
		want := types.EscrowResult{
			EscrowID: recordTestID(11), Ordinal: 7, StateAfter: 6,
			CapturedAfter: codec.U128{Hi: 0x0102030405060708, Lo: 0x1112131415161718},
			LockedAfter:   codec.U128{Hi: 0x2122232425262728, Lo: 0x3132333435363738},
			AssetID:       recordTestID(12), From: recordTestID(13), To: recordTestID(14),
			Amount:          codec.U128{Hi: 0x4142434445464748, Lo: 0x5152535455565758},
			SecondaryAmount: codec.U128{Hi: 0x6162636465666768, Lo: 0x7172737475767778},
			TransferSetRoot: recordTestID(15), GlobalSequence: 0x8182838485868788,
			TimestampMs: 0x9192939495969798,
		}
		copy(data[:32], want.EscrowID[:])
		binary.BigEndian.PutUint16(data[32:34], 7)
		data[34] = 6
		recordTestPut128(data[35:51], 0x0102030405060708, 0x1112131415161718)
		recordTestPut128(data[51:67], 0x2122232425262728, 0x3132333435363738)
		copy(data[67:99], want.AssetID[:])
		copy(data[99:131], want.From[:])
		copy(data[131:163], want.To[:])
		recordTestPut128(data[163:179], 0x4142434445464748, 0x5152535455565758)
		recordTestPut128(data[179:195], 0x6162636465666768, 0x7172737475767778)
		copy(data[195:227], want.TransferSetRoot[:])
		binary.BigEndian.PutUint64(data[227:235], 0x8182838485868788)
		binary.BigEndian.PutUint64(data[235:243], 0x9192939495969798)
		data[243] = 2
		digest := recordTestID(16)
		copy(data[244:276], digest[:])
		original := bytes.Clone(data)
		legacy, err := types.DecodeEscrowResult(data[:243])
		if err != nil || legacy != want {
			t.Fatalf("v1 result: got %+v, err %v; want %+v", legacy, err, want)
		}
		want.ContextBound, want.ContextDigest = true, digest
		result, err := types.DecodeEscrowResult(data)
		if err != nil || result != want {
			t.Fatalf("v2 result: got %+v, err %v; want %+v", result, err, want)
		}
		if !bytes.Equal(data, original) {
			t.Fatal("result decoder changed input")
		}
		for n := 0; n < 276; n++ {
			if n == 243 {
				continue
			}
			value, err := types.DecodeEscrowResult(data[:n])
			recordTestNonCanonical(t, err)
			if value != (types.EscrowResult{}) {
				t.Fatalf("length %d returned partial result", n)
			}
		}
		for _, n := range []int{277, 305, 1024} {
			_, err := types.DecodeEscrowResult(append(bytes.Clone(data), make([]byte, n-len(data))...))
			recordTestNonCanonical(t, err)
		}
		for _, mutation := range []func([]byte){
			func(b []byte) { clear(b[:32]) },
			func(b []byte) { binary.BigEndian.PutUint16(b[32:34], 0) },
			func(b []byte) { binary.BigEndian.PutUint16(b[32:34], 8) },
			func(b []byte) { binary.BigEndian.PutUint16(b[32:34], 256) },
			func(b []byte) { b[34] = 0 },
			func(b []byte) { b[34] = 8 },
		} {
			bad := bytes.Clone(data)
			mutation(bad)
			for _, n := range []int{243, 276} {
				_, err := types.DecodeEscrowResult(bad[:n])
				recordTestNonCanonical(t, err)
			}
		}
		for _, version := range []byte{0, 1, 3, 255} {
			bad := bytes.Clone(data)
			bad[243] = version
			_, err := types.DecodeEscrowResult(bad)
			recordTestNonCanonical(t, err)
		}
		bad := bytes.Clone(data)
		clear(bad[244:])
		_, err = types.DecodeEscrowResult(bad)
		recordTestNonCanonical(t, err)
		clear(data)
		if result != want {
			t.Fatal("escrow result aliases input")
		}
	})
}

func TestDecodeBudgetRecordLayout(t *testing.T) {
	for _, native := range []bool{false, true} {
		for count := 0; count <= 16; count++ {
			t.Run(fmt.Sprintf("native_%t/delegates_%d", native, count), func(t *testing.T) {
				data, want := recordTestBudget(native, count)
				original := bytes.Clone(data)
				got, err := types.DecodeBudgetRecord(data)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("budget layout: got %+v, err %v; want %+v", got, err, want)
				}
				if !bytes.Equal(data, original) {
					t.Fatal("budget decoder changed input")
				}
				for n := 0; n < len(data); n++ {
					value, err := types.DecodeBudgetRecord(data[:n])
					recordTestNonCanonical(t, err)
					if !reflect.DeepEqual(value, types.BudgetRecord{}) {
						t.Fatalf("length %d returned partial budget", n)
					}
				}
				for _, extra := range []int{1, 32, 64} {
					_, err := types.DecodeBudgetRecord(append(bytes.Clone(data), make([]byte, extra)...))
					recordTestNonCanonical(t, err)
				}
				clear(data)
				if !reflect.DeepEqual(got, want) {
					t.Fatal("budget record aliases input")
				}
			})
		}
	}
	for name, mutation := range map[string]func([]byte){
		"header":                 func(b []byte) { b[0] = 1 },
		"version_zero":           func(b []byte) { b[1] = 0 },
		"version_unknown":        func(b []byte) { b[1] = 3 },
		"closed":                 func(b []byte) { b[275] = 2 },
		"revoked":                func(b []byte) { b[276] = 2 },
		"delegate_count":         func(b []byte) { b[277] = 17 },
		"delegate_missing":       func(b []byte) { b[277] = 3 },
		"delegate_extra":         func(b []byte) { b[277] = 1 },
		"delegate_duplicate":     func(b []byte) { copy(b[310:342], b[278:310]) },
		"delegate_unsorted":      func(b []byte) { b[310] = 0 },
		"id":                     func(b []byte) { clear(b[2:34]) },
		"limit":                  func(b []byte) { clear(b[162:178]) },
		"period_length":          func(b []byte) { clear(b[242:250]) },
		"expiry_zero":            func(b []byte) { clear(b[258:266]) },
		"expiry_equal_start":     func(b []byte) { copy(b[258:266], b[250:258]) },
		"rollover_zero":          func(b []byte) { b[274] = 0 },
		"rollover_unknown":       func(b []byte) { b[274] = 3 },
		"rollover_none_with_cap": func(b []byte) { b[274] = 1 },
	} {
		t.Run(name, func(t *testing.T) {
			bad, _ := recordTestBudget(false, 2)
			mutation(bad)
			_, err := types.DecodeBudgetRecord(bad)
			recordTestNonCanonical(t, err)
		})
	}
	t.Run("native_source_required", func(t *testing.T) {
		bad, _ := recordTestBudget(true, 2)
		clear(bad[len(bad)-32:])
		_, err := types.DecodeBudgetRecord(bad)
		recordTestNonCanonical(t, err)
	})
	t.Run("maximum_delegate_count", func(t *testing.T) {
		bad, _ := recordTestBudget(false, 16)
		bad[277] = 17
		bad = append(bad, make([]byte, 32)...)
		_, err := types.DecodeBudgetRecord(bad)
		recordTestNonCanonical(t, err)
	})
	t.Run("no_rollover", func(t *testing.T) {
		data, want := recordTestBudget(false, 0)
		data[274], data[275], data[276] = 1, 0, 0
		clear(data[194:210])
		want.RolloverPolicy, want.Closed, want.Revoked, want.CarryCap = 1, false, false, codec.U128{}
		got, err := types.DecodeBudgetRecord(data)
		if err != nil || !reflect.DeepEqual(got, want) {
			t.Fatalf("no rollover: got %+v, err %v; want %+v", got, err, want)
		}
	})
}

func recordTestBudget(native bool, count int) ([]byte, types.BudgetRecord) {
	length := 278 + count*32
	if native {
		length += 32
	}
	data := make([]byte, length)
	want := types.BudgetRecord{
		NativeSource: native, BudgetID: recordTestID(1), Owner: recordTestID(2),
		BudgetAccount: recordTestID(3), AssetID: recordTestID(4), PurposeHash: recordTestID(5),
		PerPeriodLimit:        codec.U128{Hi: 0x0102030405060708, Lo: 0x1112131415161718},
		ConfiguredPeriodLimit: codec.U128{Hi: 0x2122232425262728, Lo: 0x3132333435363738},
		CarryCap:              codec.U128{Hi: 0x4142434445464748, Lo: 0x5152535455565758},
		SpentThisPeriod:       codec.U128{Hi: 0x6162636465666768, Lo: 0x7172737475767778},
		Carried:               codec.U128{Hi: 0xffffffffffffffff, Lo: 0xffffffffffffffff},
		PeriodLengthMs:        0x8182838485868788, PeriodStartMs: 0x9192939495969798,
		ExpiryMs: 0xa1a2a3a4a5a6a7a8, RevocationSequence: 0xb1b2b3b4b5b6b7b8,
		RolloverPolicy: 2, Closed: true, Revoked: true, Delegates: make([][32]byte, count),
	}
	data[1] = 1
	if native {
		data[1] = 2
		want.SourceAccount = recordTestID(6)
		copy(data[length-32:], want.SourceAccount[:])
	}
	for i, id := range [][32]byte{want.BudgetID, want.Owner, want.BudgetAccount, want.AssetID, want.PurposeHash} {
		copy(data[2+i*32:34+i*32], id[:])
	}
	recordTestPut128(data[162:178], 0x0102030405060708, 0x1112131415161718)
	recordTestPut128(data[178:194], 0x2122232425262728, 0x3132333435363738)
	recordTestPut128(data[194:210], 0x4142434445464748, 0x5152535455565758)
	recordTestPut128(data[210:226], 0x6162636465666768, 0x7172737475767778)
	recordTestPut128(data[226:242], 0xffffffffffffffff, 0xffffffffffffffff)
	binary.BigEndian.PutUint64(data[242:250], 0x8182838485868788)
	binary.BigEndian.PutUint64(data[250:258], 0x9192939495969798)
	binary.BigEndian.PutUint64(data[258:266], 0xa1a2a3a4a5a6a7a8)
	binary.BigEndian.PutUint64(data[266:274], 0xb1b2b3b4b5b6b7b8)
	data[274], data[275], data[276], data[277] = 2, 1, 1, byte(count)
	for i := range want.Delegates {
		want.Delegates[i] = recordTestID(byte(10 + i))
		copy(data[278+32*i:310+32*i], want.Delegates[i][:])
	}
	return data, want
}

func recordTestID(seed byte) [32]byte {
	var id [32]byte
	for i := range id {
		id[i] = seed + byte(i)
	}
	return id
}

func recordTestPut128(data []byte, hi, lo uint64) {
	binary.BigEndian.PutUint64(data[:8], hi)
	binary.BigEndian.PutUint64(data[8:16], lo)
}

func recordTestNonCanonical(t *testing.T, err error) {
	t.Helper()
	if err == nil || err.Error() != "LXP_NON_CANONICAL" {
		t.Fatalf("got error %v, want LXP_NON_CANONICAL", err)
	}
}
