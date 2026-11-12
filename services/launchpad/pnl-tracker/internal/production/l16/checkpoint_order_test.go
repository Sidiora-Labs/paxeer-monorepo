package l16

import (
	"context"
	"math/big"
	"strings"
	"testing"
	"time"
)

func TestCheckpointPagesPreserveOrderingAndRejectAmbiguousArchives(t *testing.T) {
	ctx := context.Background()
	day := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	legs := make([]TimedLeg, 3)
	for i := range legs {
		l := testLeg(uint32(i + 1))
		l.Height = uint64(100 + i/2)
		l.TransactionIndex = uint32(i % 2)
		if i > 0 {
			l.Predecessor, _ = legs[i-1].Leg.Identity.Key()
		}
		legs[i] = TimedLeg{Leg: l, CommittedAt: day.Add(time.Duration(i) * time.Second)}
	}
	ordered := []TimedLeg{legs[2], legs[1], legs[0]}
	for _, paged := range []bool{false, true} {
		t.Run(map[bool]string{false: "bounded", true: "paged"}[paged], func(t *testing.T) {
			var cp Checkpoint
			var err error
			if paged {
				cp, err = RebuildDayPages(ctx, Checkpoint{}, ordered, day)
			} else {
				cp, err = RebuildDay(ctx, Empty(), ordered, day, true)
			}
			last, _ := legs[2].Leg.Identity.Key()
			if err != nil || cp.State.Known != "30" || cp.LastIdentity != last || cp.LastHeight != 101 {
				t.Fatal(cp, err)
			}
		})
	}
	first, err := RebuildDayPages(ctx, Checkpoint{}, legs[:1], day)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		previous Checkpoint
		input    []TimedLeg
		day      time.Time
	}{
		{"duplicate", Checkpoint{}, []TimedLeg{legs[0], legs[0]}, day},
		{"conflict", Checkpoint{}, []TimedLeg{legs[0], func() TimedLeg { x := legs[0]; x.Leg.Quantity = "11"; return x }()}, day},
		{"day boundary", Checkpoint{}, []TimedLeg{{Leg: legs[0].Leg, CommittedAt: day.AddDate(0, 0, 1)}}, day},
		{"mixed partition", Checkpoint{}, []TimedLeg{legs[0], func() TimedLeg {
			x := legs[1]
			x.Leg.Wallet = "0x" + strings.Repeat("e", 40)
			x.Leg.Recipient = x.Leg.Wallet
			return x
		}()}, day},
		{"frontier", first, []TimedLeg{{Leg: legs[0].Leg, CommittedAt: day.AddDate(0, 0, 1)}}, day.AddDate(0, 0, 1)},
		{"day gap", first, nil, day.AddDate(0, 0, 2)},
		{"implicit seed", Checkpoint{State: first.State}, nil, day},
		{"timestamp", Checkpoint{}, []TimedLeg{{Leg: legs[0].Leg, CommittedAt: time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)}}, time.Date(10000, 1, 1, 0, 0, 0, 0, time.UTC)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cp, err := RebuildDayPages(ctx, tc.previous, tc.input, tc.day)
			if tc.name == "duplicate" {
				if err != nil || cp.State.Known != "10" {
					t.Fatal(cp, err)
				}
			} else if err == nil {
				t.Fatal("ambiguous archive accepted", cp)
			}
		})
	}
	if _, err = ContinueDay(ctx, first, make([]TimedLeg, 10001), day.AddDate(0, 0, 1)); err == nil {
		t.Fatal("oversized day accepted")
	}
	if _, err = RebuildDayPages(ctx, Checkpoint{}, make([]TimedLeg, 1000001), day); err == nil {
		t.Fatal("oversized archive accepted")
	}
	if _, err = RebuildDay(ctx, first.State, nil, day, true); err == nil {
		t.Fatal("unbound nonempty seed accepted")
	}
	empty, err := RebuildDayPages(ctx, Checkpoint{}, nil, day)
	if err != nil {
		t.Fatal(err)
	}
	next, err := RebuildDayPages(ctx, empty, nil, day.AddDate(0, 0, 1))
	if err != nil || next.State != Empty() || next.ParentDigest != empty.InputDigest {
		t.Fatal(next, err)
	}
}

func TestInventoryAmountsPreserveStateOnInvalidSettlement(t *testing.T) {
	state, err := Acquire(Empty(), "10", str("100"))
	if err != nil {
		t.Fatal(err)
	}
	if realized := state.CompleteRealized(); realized == nil || *realized != "0" {
		t.Fatal(realized)
	}
	if next, err := Acquire(state, "1", str("bad")); err == nil || next != state {
		t.Fatal(next, err)
	}
	if next, _, err := Dispose(state, "1", str("bad")); err == nil || next != state {
		t.Fatal(next, err)
	}
	if source, destination, _, err := Transfer(state, Empty(), "11", false, ""); err == nil || source != state || destination != Empty() {
		t.Fatal(source, destination, err)
	}
	if _, err := Unrealized(state, str("bad"), true); err == nil {
		t.Fatal("unbound valuation accepted")
	}
	state.Realized = strings.Repeat("9", 156)
	if _, _, err := Dispose(state, "1", str(new(big.Int).Lsh(big.NewInt(1), 255).String())); err == nil {
		t.Fatal("aggregate overflow accepted")
	}
}
