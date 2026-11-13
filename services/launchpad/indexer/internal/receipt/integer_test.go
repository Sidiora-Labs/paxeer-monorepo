package receipt

import (
	"context"
	"errors"
	"fmt"
	"math"
	"reflect"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestReceiptRangeRefusals(t *testing.T) {
	ctx := context.Background()
	a := new(RPCQualification)
	for _, tc := range []struct {
		name     string
		from, to uint64
		want     string
	}{
		{"first height above signed bound", math.MaxInt64 + 1, math.MaxInt64 + 1, "integer 9223372036854775808 is outside the range of int64"},
		{"first height unsigned bound", math.MaxUint64, math.MaxUint64, "integer 18446744073709551615 is outside the range of int64"},
		{"last height above signed bound", math.MaxInt64, math.MaxInt64 + 1, ErrBinding.Error()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			batch, receipts, err := a.FetchQualifiedBatch(ctx, tc.from, tc.to)
			if err == nil || err.Error() != tc.want || receipts != nil || len(batch.Blocks) != 0 || len(batch.Records) != 0 {
				t.Fatalf("FetchQualifiedBatch(%d, %d) = %+v, %v, %v; want %q", tc.from, tc.to, batch, receipts, err, tc.want)
			}
		})
	}
	for _, height := range []int64{math.MinInt64, -1} {
		batch := store.CanonicalBatch{ChainID: 125, Blocks: []store.BlockCompleteness{{Height: height}}}
		if got, err := a.QualifiedBatch(ctx, batch); got != nil || !errors.Is(err, ErrBinding) {
			t.Fatalf("QualifiedBatch height %d = %v, %v", height, got, err)
		}
	}
}

func TestFillLogIndexRefusals(t *testing.T) {
	ctx := context.Background()
	input := repair53InputsFromProducer(t)
	authority, closeClient := repair53Open(t, ctx, input)
	defer closeClient()
	if err := authority.Rehydrate(ctx, input.Markets); err != nil {
		t.Fatal(err)
	}
	authority = authority.current()
	bundle, err := authority.Bundle(ctx, input.NativeSwap)
	if err != nil {
		t.Fatal(err)
	}
	sender, err := coordinates(bundle)
	if err != nil {
		t.Fatal(err)
	}
	for _, log := range bundle.Receipt.Logs {
		envelope, err := authority.Registry().Decode(*log)
		if err != nil {
			t.Fatal(err)
		}
		if !qualifiedSwapFamily(envelope, log) {
			continue
		}
		for _, index := range []uint{math.MaxUint32 + 1, math.MaxUint} {
			t.Run(fmt.Sprintf("log index %d", index), func(t *testing.T) {
				changed := *log
				changed.Index = index
				got, err := makeFill(ctx, authority, bundle, envelope, &changed, sender, [32]byte{}, [32]byte{})
				want := fmt.Sprintf("integer %d is outside the range of uint32", index)
				if err == nil || err.Error() != want || !reflect.DeepEqual(got, accounting.FillV1{}) {
					t.Fatalf("makeFill index %d = %+v, %v; want %q", index, got, err, want)
				}
			})
		}
		return
	}
	t.Fatal("recorded swap absent")
}
