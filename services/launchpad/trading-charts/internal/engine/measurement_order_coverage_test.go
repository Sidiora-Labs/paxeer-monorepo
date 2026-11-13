package engine

import (
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
)

func TestMeasurementReferenceOrdersTradesAndPrices(t *testing.T) {
	reference, err := NewMeasurementReference(100)
	if err != nil {
		t.Fatal(err)
	}
	actor := "0x1111111111111111111111111111111111111111"
	first := seriesSource(2, 1700000040, actor)
	high := seriesSource(3, 1700000040, "")
	high.USDLAmountRaw, high.PaidInputRaw = "2000000", "2000000"
	low := seriesSource(1, 1700000040, "")
	low.USDLAmountRaw, low.PaidInputRaw, low.Side, low.FeeDisposition = "500000", low.TokenAmountRaw, "sell", "retained_token_reserve"
	earlier := seriesSource(0, 1700000040, "")
	for _, fill := range []accounting.FillV1{first, high, low, earlier} {
		if err := reference.Add(fill); err != nil {
			t.Fatalf("add %s: %v", fill.SourceFillID, err)
		}
	}
	var minute *MeasurementBar
	for _, bar := range reference.Bars() {
		if bar.Interval == "1m" && bar.Pool == first.PoolAddress {
			copy := bar
			minute = &copy
		}
	}
	if minute == nil || minute.Trades != 4 || minute.First != earlier.Position || minute.Last != high.Position || minute.BuyVolumeUsdlRaw != "4000002" || minute.SellVolumeUsdlRaw != "500000" || minute.UnknownActors != 3 || minute.UniqueTraders != 1 {
		t.Fatalf("ordered measurement: %+v", minute)
	}
	if minute.High.Numerator == minute.Low.Numerator || minute.Open == minute.Close {
		t.Fatalf("price ordering: %+v", minute)
	}
	if _, err := MeasurementDigest(reference.Bars()); err != nil {
		t.Fatalf("ordered digest: %v", err)
	}
	if reference.Bars() == nil || len(reference.Bars()) != len(ExactIntervals) {
		t.Fatalf("reference interval count: %d", len(reference.Bars()))
	}
}

func TestMeasurementReferenceSortsPoolsAndStarts(t *testing.T) {
	reference, err := NewMeasurementReference(30)
	if err != nil {
		t.Fatal(err)
	}
	later := seriesSource(2, 1700000220, "")
	earlier := seriesSource(1, 1700000040, "")
	other := seriesSource(3, 1700000040, "")
	other.PoolAddress = "0x" + strings.Repeat("1", 40)
	for _, fill := range []accounting.FillV1{later, earlier, other} {
		if err := reference.Add(fill); err != nil {
			t.Fatal(err)
		}
	}
	bars := reference.Bars()
	if len(bars) <= len(ExactIntervals) || bars[0].Pool != other.PoolAddress {
		t.Fatalf("ordered multi-pool bars: %+v", bars)
	}
	for i := 1; i < len(bars); i++ {
		if bars[i-1].Pool > bars[i].Pool || bars[i-1].Pool == bars[i].Pool && bars[i-1].Interval == bars[i].Interval && bars[i-1].Start > bars[i].Start {
			t.Fatalf("unordered bars at %d: %+v %+v", i, bars[i-1], bars[i])
		}
	}
	firstDigest, err := MeasurementDigest(bars)
	if err != nil {
		t.Fatal(err)
	}
	reversed := append([]MeasurementBar(nil), bars...)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	secondDigest, err := MeasurementDigest(reversed)
	if err != nil || firstDigest != secondDigest {
		t.Fatalf("order-independent measurement digest: %s %s %v", firstDigest, secondDigest, err)
	}
}

func TestMeasurementReferenceBoundsAndCounter(t *testing.T) {
	if ref, err := NewMeasurementReference(0); ref != nil || err == nil {
		t.Fatalf("zero bar budget: %+v, %v", ref, err)
	}
	var absent *MeasurementReference
	if absent.Bars() != nil || !errors.Is(absent.Add(seriesSource(0, 60, "")), ErrInvalidFill) {
		t.Fatal("absent reference was admitted")
	}
	reference, err := NewMeasurementReference(7)
	if err != nil {
		t.Fatal(err)
	}
	source := seriesSource(0, 1700000040, "")
	if err := reference.Add(source); err != nil {
		t.Fatal(err)
	}
	key := source.PoolAddress + ":1m:1700000040"
	reference.bars[key].bar.Trades = ^uint64(0)
	if err := reference.Add(source); !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("overflowing trade count: %v", err)
	}
}
