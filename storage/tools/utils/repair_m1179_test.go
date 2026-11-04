package utils

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func repairM1179Length(length uint32) []byte {
	var prefix [4]byte
	binary.LittleEndian.PutUint32(prefix[:], length)
	return prefix[:]
}

func repairM1179File(t *testing.T, data []byte) string {
	t.Helper()
	name := filepath.Join(t.TempDir(), "input.kv")
	if err := os.WriteFile(name, data, 0600); err != nil {
		t.Fatal(err)
	}
	return name
}

func repairM1179AssertError(t *testing.T, name, field string, cause error, length, limit uint32) *KVRecordLengthError {
	t.Helper()
	pairs, err := ReadKVEntriesFromFile(name)
	if pairs != nil {
		t.Fatalf("invalid file returned %d partial entries", len(pairs))
	}
	var lengthErr *KVRecordLengthError
	if !errors.As(err, &lengthErr) || !errors.Is(err, cause) {
		t.Fatalf("expected typed length error wrapping %v, got %T: %v", cause, err, err)
	}
	if lengthErr.Length != length || lengthErr.Limit != limit {
		t.Fatalf("length/limit = %d/%d, want %d/%d", lengthErr.Length, lengthErr.Limit, length, limit)
	}
	if !strings.Contains(err.Error(), "read "+field+":") {
		t.Fatalf("missing %s context: %v", field, err)
	}
	return lengthErr
}

func TestRepairM_1179_OversizedLengths(t *testing.T) {
	for _, tc := range []struct {
		name   string
		field  string
		length uint32
		limit  uint32
		prefix []byte
	}{
		{"key_limit_plus_one", "key", MaxKVKeySize + 1, MaxKVKeySize, nil},
		{"key_uint32_max", "key", math.MaxUint32, MaxKVKeySize, nil},
		{"value_limit_plus_one", "value", MaxKVValueSize + 1, MaxKVValueSize, repairM1179Length(0)},
		{"value_uint32_max", "value", math.MaxUint32, MaxKVValueSize, repairM1179Length(0)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			name := repairM1179File(t, append(tc.prefix, repairM1179Length(tc.length)...))
			repairM1179AssertError(t, name, tc.field, ErrKVRecordTooLarge, tc.length, tc.limit)
		})
	}
}

func TestRepairM_1179_OversizedCompletePayloads(t *testing.T) {
	for _, tc := range []struct {
		field string
		limit uint32
	}{
		{"key", MaxKVKeySize},
		{"value", MaxKVValueSize},
	} {
		t.Run(tc.field, func(t *testing.T) {
			var data []byte
			if tc.field == "value" {
				data = repairM1179Length(0)
			}
			data = append(data, repairM1179Length(tc.limit+1)...)
			name := repairM1179File(t, data)
			// A real sparse file supplies the entire oversized payload without
			// allocating it in the test. The supported bound must still win.
			if err := os.Truncate(name, int64(len(data))+int64(tc.limit)+1); err != nil {
				t.Fatal(err)
			}
			repairM1179AssertError(t, name, tc.field, ErrKVRecordTooLarge, tc.limit+1, tc.limit)
		})
	}
}

func TestRepairM_1179_TruncatedFiles(t *testing.T) {
	validEntry := append(repairM1179Length(0), repairM1179Length(0)...)
	for _, tc := range []struct {
		name      string
		data      []byte
		field     string
		length    uint32
		limit     uint32
		remaining int64
	}{
		{"partial_key_header_one", []byte{1}, "key", 0, MaxKVKeySize, -1},
		{"partial_key_header_two", []byte{1, 0}, "key", 0, MaxKVKeySize, -1},
		{"partial_key_header_three", []byte{1, 0, 0}, "key", 0, MaxKVKeySize, -1},
		{"missing_key_payload", repairM1179Length(1), "key", 1, MaxKVKeySize, 0},
		{"short_key_payload", append(repairM1179Length(3), 'k', 'v'), "key", 3, MaxKVKeySize, 2},
		{"missing_value_header", repairM1179Length(0), "value", 0, MaxKVValueSize, 0},
		{"partial_value_header_one", append(repairM1179Length(0), 1), "value", 0, MaxKVValueSize, -1},
		{"partial_value_header_two", append(repairM1179Length(0), 1, 0), "value", 0, MaxKVValueSize, -1},
		{"partial_value_header_three", append(repairM1179Length(0), 1, 0, 0), "value", 0, MaxKVValueSize, -1},
		{"missing_value_payload", append(repairM1179Length(0), repairM1179Length(1)...), "value", 1, MaxKVValueSize, 0},
		{"short_value_payload", append(append(repairM1179Length(0), repairM1179Length(3)...), 'v'), "value", 3, MaxKVValueSize, 1},
		{"partial_header_after_valid_entry", append(validEntry, 1), "key", 0, MaxKVKeySize, -1},
		{"missing_value_after_valid_entry", append(validEntry, repairM1179Length(0)...), "value", 0, MaxKVValueSize, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := repairM1179AssertError(t, repairM1179File(t, tc.data), tc.field, io.ErrUnexpectedEOF, tc.length, tc.limit)
			if err.Remaining != tc.remaining {
				t.Fatalf("remaining = %d, want %d", err.Remaining, tc.remaining)
			}
		})
	}
}

func TestRepairM_1179_TruncationBeforeAllocation(t *testing.T) {
	// The declared value fits the supported limit, but only one byte remains.
	data := append(repairM1179Length(0), repairM1179Length(MaxKVValueSize)...)
	name := repairM1179File(t, append(data, 'v'))
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	for i := 0; i < 4; i++ {
		err := repairM1179AssertError(t, name, "value", io.ErrUnexpectedEOF, MaxKVValueSize, MaxKVValueSize)
		if err.Remaining != 1 {
			t.Fatalf("remaining = %d, want 1", err.Remaining)
		}
	}
	runtime.ReadMemStats(&after)
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated >= uint64(MaxKVValueSize) {
		t.Fatalf("truncated files allocated %d bytes; a payload-sized buffer must not be allocated", allocated)
	}

	f, err := os.Open(name)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if _, err := f.Seek(4, io.SeekStart); err != nil {
		t.Fatal(err)
	}
	payload, err := readByteSlice(f, MaxKVValueSize)
	if payload != nil || !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("truncated payload = %v, error = %v", payload, err)
	}
	if offset, err := f.Seek(0, io.SeekCurrent); err != nil || offset != 8 {
		t.Fatalf("payload consumed before validation: offset=%d, error=%v", offset, err)
	}
}

func TestRepairM_1179_BoundaryEntries(t *testing.T) {
	key := bytes.Repeat([]byte{0xa5}, int(MaxKVKeySize))
	value := bytes.Repeat([]byte{0x5a}, int(MaxKVValueSize))
	want := []KeyValuePair{
		{Key: []byte{}, Value: []byte{}},
		{Key: key, Value: value},
		{Key: []byte("trailing-key"), Value: []byte("trailing-value")},
	}
	f, err := CreateFile(t.TempDir(), "boundary.kv")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = f.Close() })
	for _, pair := range want {
		for _, data := range [][]byte{pair.Key, pair.Value} {
			if err := binary.Write(f, binary.LittleEndian, uint32(len(data))); err != nil {
				t.Fatal(err)
			}
			if _, err := f.Write(data); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	got, err := ReadKVEntriesFromFile(f.Name())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("entry count = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if !bytes.Equal(got[i].Key, want[i].Key) || !bytes.Equal(got[i].Value, want[i].Value) {
			t.Fatalf("entry %d did not round trip", i)
		}
	}
	got, err = ReadKVEntriesFromFile(repairM1179File(t, nil))
	if err != nil || len(got) != 0 {
		t.Fatalf("empty file = %v, %v", got, err)
	}
}

func TestRepairM_1179_StreamLengths(t *testing.T) {
	// Readers without a known file size still enforce the limit and report
	// truncated payloads with the same error type.
	for _, tc := range []struct {
		name  string
		data  []byte
		cause error
	}{
		{"oversized", repairM1179Length(math.MaxUint32), ErrKVRecordTooLarge},
		{"missing", repairM1179Length(1), io.ErrUnexpectedEOF},
		{"short", append(repairM1179Length(2), 'v'), io.ErrUnexpectedEOF},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := readByteSlice(bytes.NewBuffer(tc.data), MaxKVValueSize)
			var lengthErr *KVRecordLengthError
			if data != nil || !errors.As(err, &lengthErr) || !errors.Is(err, tc.cause) {
				t.Fatalf("payload = %v, error = %T: %v", data, err, err)
			}
		})
	}
}

func TestRepairM_1179_LoadConsumer(t *testing.T) {
	dir := t.TempDir()
	for _, key := range []byte{'a', 'b'} {
		data := append(repairM1179Length(1), key)
		data = append(data, repairM1179Length(1)...)
		data = append(data, key+1)
		if err := os.WriteFile(filepath.Join(dir, string(key)+".kv"), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	pairs, err := LoadAndShuffleKV(dir, 2)
	if err != nil || len(pairs) != 2 {
		t.Fatalf("load returned %d entries, error=%v", len(pairs), err)
	}
	seen := make(map[string]string)
	for _, pair := range pairs {
		seen[string(pair.Key)] = string(pair.Value)
	}
	if len(seen) != 2 || seen["a"] != "b" || seen["b"] != "c" {
		t.Fatalf("loaded entries = %v", seen)
	}
}
