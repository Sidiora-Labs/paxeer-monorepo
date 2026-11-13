package l18

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestRewardLedgerValidatesFundingAndAwardsBeforeReservation(t *testing.T) {
	l := storage(t)
	ctx := context.Background()
	for _, change := range []func(*Funding){
		func(f *Funding) { f.Program = "!" }, func(f *Funding) { f.Version = "!" }, func(f *Funding) { f.Adapter = "!" }, func(f *Funding) { f.Proof = "!" }, func(f *Funding) { f.Day = "2026-02-30" },
		func(f *Funding) { f.Asset = "invalid" }, func(f *Funding) { f.Treasury = "invalid" }, func(f *Funding) { f.FundedRaw = "-1" }, func(f *Funding) { f.DailyCapRaw = "01" }, func(f *Funding) { f.BeneficiaryCapRaw = "bad" },
	} {
		f := funding()
		change(&f)
		if err := l.Fund(ctx, f); !errors.Is(err, ErrInvalid) {
			t.Fatal(f, err)
		}
	}
	for _, change := range []func(*Award){
		func(a *Award) { a.Identity = "!" }, func(a *Award) { a.Program = "!" }, func(a *Award) { a.Version = "!" }, func(a *Award) { a.Day = "invalid" }, func(a *Award) { a.Provenance = "!" }, func(a *Award) { a.Kind = "unknown" },
		func(a *Award) { a.Asset = "invalid" }, func(a *Award) { a.Beneficiary = "invalid" }, func(a *Award) { a.EligibleRaw = "-1" },
	} {
		a := award("invalid-award", actorB)
		change(&a)
		if _, err := l.Accrue(ctx, a); !errors.Is(err, ErrInvalid) {
			t.Fatal(a, err)
		}
	}
	if got := query(t, l, `SELECT count(*)::text FROM l18_funding`); got != "0" {
		t.Fatal(got)
	}
	if got := query(t, l, `SELECT count(*)::text FROM l18_awards`); got != "0" {
		t.Fatal(got)
	}
	if _, err := l.Liability(ctx, "invalid id"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	for _, tc := range []struct{ request, id, reason string }{{"!", "award", "review"}, {"request", "!", "review"}, {"request", "award", ""}, {"request", "award", strings.Repeat("x", 513)}} {
		if err := l.Hold(ctx, tc.request, tc.id, tc.reason); !errors.Is(err, ErrInvalid) {
			t.Fatal(tc, err)
		}
	}
	if _, err := l.RecordCreatorClaim(ctx, nil, "", "", ""); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	var absent *Ledger
	if err := absent.RequireEpochFence(); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if absent.MatchesAuthority(nil) || l.MatchesAuthority(nil) {
		t.Fatal("missing authority matched")
	}
	if err := l.RequireEpochFence(); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if _, err := NewLedger(nil, "pnl", "service", nil); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if _, err := NewRuntimeLedger(l.pool, l.schema, l.service, l.authority, nil); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	guard := func(context.Context, pgx.Tx) error { return nil }
	if _, err := NewRuntimeLedger(nil, l.schema, l.service, l.authority, guard); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
}

func TestRewardReservationRespectsDailyAndBeneficiaryLimits(t *testing.T) {
	for _, tc := range []struct{ name, daily, cap, want string }{{"daily limit", "30", "100", "30"}, {"beneficiary limit", "100", "40", "40"}} {
		t.Run(tc.name, func(t *testing.T) {
			l := storage(t)
			ctx := context.Background()
			f := funding()
			f.FundedRaw = "1000"
			f.DailyCapRaw = tc.daily
			f.BeneficiaryCapRaw = tc.cap
			if err := l.Fund(ctx, f); err != nil {
				t.Fatal(err)
			}
			a := award("bounded", actorB)
			a.EligibleRaw = "100"
			got, err := l.Accrue(ctx, a)
			if err != nil || got.PayableRaw != tc.want || got.Status != "funded" {
				t.Fatal(got, err)
			}
			if reserved := query(t, l, `SELECT reserved::text FROM l18_funding`); reserved != tc.want {
				t.Fatal(reserved)
			}
			for i := 0; i < 2; i++ {
				if err = l.Hold(ctx, "review-bounded", a.Identity, "reward review"); err != nil {
					t.Fatal(err)
				}
			}
			if err = l.Hold(ctx, "review-missing", "missing", "reward review"); !errors.Is(err, ErrConflict) {
				t.Fatal(err)
			}
			got, err = l.Liability(ctx, a.Identity)
			if err != nil || got.Status != "pending_review" || got.HoldReason != "reward review" || got.PayableRaw != tc.want {
				t.Fatal(got, err)
			}
		})
	}
}
