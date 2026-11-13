package checked

import (
	"math"
	"testing"
)

func TestConvertBoundaries(t *testing.T) {
	for _, tc := range []struct {
		name      string
		value     int64
		want      int8
		wantError bool
	}{
		{"below lower bound", math.MinInt8 - 1, 0, true},
		{"lower bound", math.MinInt8, math.MinInt8, false},
		{"upper bound", math.MaxInt8, math.MaxInt8, false},
		{"above upper bound", math.MaxInt8 + 1, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Convert[int8](tc.value)
			if (err != nil) != tc.wantError || got != tc.want {
				t.Fatalf("Convert[int8](%d) = %d, %v; want %d, error %v", tc.value, got, err, tc.want, tc.wantError)
			}
		})
	}
	for _, tc := range []struct {
		name      string
		value     int64
		want      uint8
		wantError bool
	}{
		{"below unsigned lower bound", -1, 0, true},
		{"unsigned lower bound", 0, 0, false},
		{"unsigned upper bound", math.MaxUint8, math.MaxUint8, false},
		{"above unsigned upper bound", math.MaxUint8 + 1, 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Convert[uint8](tc.value)
			if (err != nil) != tc.wantError || got != tc.want {
				t.Fatalf("Convert[uint8](%d) = %d, %v; want %d, error %v", tc.value, got, err, tc.want, tc.wantError)
			}
		})
	}
}

func TestConvertIntegerRepresentations(t *testing.T) {
	checkSignedConversion[int](t, math.MinInt, math.MaxInt)
	checkSignedConversion[int16](t, math.MinInt16, math.MaxInt16)
	checkSignedConversion[int32](t, math.MinInt32, math.MaxInt32)
	checkSignedConversion[int64](t, math.MinInt64, math.MaxInt64)
	checkUnsignedConversion[uint](t, math.MaxUint)
	checkUnsignedConversion[uint16](t, math.MaxUint16)
	checkUnsignedConversion[uint32](t, math.MaxUint32)
	checkUnsignedConversion[uint64](t, math.MaxUint64)
	checkUnsignedConversion[uintptr](t, math.MaxUint)
	if got, err := Convert[int64](int8(-1)); err != nil || got != -1 {
		t.Fatalf("signed widening = %d, %v", got, err)
	}
	if got, err := Convert[uint64](uint8(math.MaxUint8)); err != nil || got != math.MaxUint8 {
		t.Fatalf("unsigned widening = %d, %v", got, err)
	}
}

func checkSignedConversion[T Integer](t *testing.T, lower, upper int64) {
	t.Helper()
	for _, value := range []int64{lower, upper, 0} {
		got, err := Convert[T](value)
		if err != nil || int64(got) != value {
			t.Fatalf("Convert[%T](%d) = %d, %v", got, value, got, err)
		}
	}
	if got, err := Convert[T](uint64(upper) + 1); err == nil || got != 0 {
		t.Fatalf("unsigned value above signed upper bound = %d, %v", got, err)
	}
	if lower > math.MinInt64 {
		if got, err := Convert[T](lower - 1); err == nil || got != 0 {
			t.Fatalf("value below signed lower bound = %d, %v", got, err)
		}
	}
}

func checkUnsignedConversion[T Integer](t *testing.T, upper uint64) {
	t.Helper()
	for _, value := range []uint64{0, upper} {
		got, err := Convert[T](value)
		if err != nil || uint64(got) != value {
			t.Fatalf("Convert[%T](%d) = %d, %v", got, value, got, err)
		}
	}
	if got, err := Convert[T](int64(-1)); err == nil || got != 0 {
		t.Fatalf("value below unsigned lower bound = %d, %v", got, err)
	}
	if upper < math.MaxUint64 {
		if got, err := Convert[T](upper + 1); err == nil || got != 0 {
			t.Fatalf("value above unsigned upper bound = %d, %v", got, err)
		}
	}
}
