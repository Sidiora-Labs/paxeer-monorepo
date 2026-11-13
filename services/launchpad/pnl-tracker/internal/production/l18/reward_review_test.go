package l18

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRewardReviewCorrectionAndWalletLiabilities(t *testing.T) {
	l := storage(t)
	ctx := context.Background()
	if err := l.Fund(ctx, funding()); err != nil {
		t.Fatal(err)
	}
	if out, err := l.Accrue(ctx, award("reviewed-award", actorB)); err != nil || out.PayableRaw != "1000000" {
		t.Fatal(out, err)
	}
	reviewer := Actor{Wallet: actorA, Roles: map[string]bool{"reward-review": true, "reward-correction-propose": true, "reward-correction-approve": true}}
	request := uuid.NewString()
	for i := 0; i < 2; i++ {
		if err := l.ReviewHold(ctx, reviewer, request, "reviewed-award", "beneficiary review"); err != nil {
			t.Fatal(err)
		}
	}
	if got := query(t, l, `SELECT status||':'||appeal_status FROM l18_awards WHERE identity='reviewed-award'`); got != "pending_review:open" {
		t.Fatal(got)
	}
	if err := l.ReviewHold(ctx, reviewer, request, "reviewed-award", "different reason"); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err := l.ReviewHold(ctx, reviewer, uuid.NewString(), "absent", "beneficiary review"); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	correction := uuid.NewString()
	for i := 0; i < 2; i++ {
		if err := l.Correction(ctx, reviewer, correction, "reviewed-award", "review accounting evidence", false); err != nil {
			t.Fatal(err)
		}
	}
	if err := l.Correction(ctx, reviewer, correction, "reviewed-award", "approve evidence", true); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	approver := Actor{Wallet: actorC, Roles: map[string]bool{"reward-correction-approve": true}}
	for i := 0; i < 2; i++ {
		if err := l.Correction(ctx, approver, correction, "reviewed-award", "approve evidence", true); err != nil {
			t.Fatal(err)
		}
	}
	if got := query(t, l, `SELECT proposer||':'||approver||':'||state FROM l18_corrections WHERE identity=$1`, correction); got != actorA+":"+actorC+":approved" {
		t.Fatal(got)
	}
	if err := l.Correction(ctx, approver, correction, "reviewed-award", "changed reason", true); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
	if err := l.Correction(ctx, approver, "invalid", "reviewed-award", "reason", true); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err := l.Correction(ctx, approver, uuid.NewString(), "reviewed-award", "reason", true); err == nil {
		t.Fatal("missing proposal approved")
	}
	items, err := l.Liabilities(ctx, Actor{Wallet: actorB}, "", 1)
	if err != nil || len(items) != 1 || items[0].Status != "pending_review" || items[0].PayableRaw != "1000000" || items[0].PaidRaw != "0" {
		t.Fatal(items, err)
	}
	if next, err := l.Liabilities(ctx, Actor{Wallet: actorB}, items[0].Identity, 1); err != nil || len(next) != 0 {
		t.Fatal(next, err)
	}
	tx, err := l.begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `UPDATE l18_awards SET status='paid' WHERE identity='reviewed-award'`); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	items, err = l.Liabilities(ctx, Actor{Wallet: actorB}, "", 10)
	if err != nil || len(items) != 1 || items[0].PayableRaw != "0" || items[0].PaidRaw != "1000000" {
		t.Fatal(items, err)
	}
	if err = l.ReviewHold(ctx, reviewer, uuid.NewString(), "reviewed-award", "review"); !errors.Is(err, ErrConflict) {
		t.Fatal(err)
	}
}

func TestReferralDashboardTracksConsentBindingsAndAnalytics(t *testing.T) {
	l := storage(t)
	ctx := context.Background()
	now := time.Now().UTC()
	a, err := NewAttribution(l, []byte(strings.Repeat("k", 32)), func() time.Time { return now }, "campaign", "v1")
	if err != nil {
		t.Fatal(err)
	}
	empty, err := a.Dashboard(ctx, actorA)
	if err != nil || empty.Code != nil || empty.Binding != nil || empty.EligibleRaw != "0" || empty.Clicks != 0 {
		t.Fatal(empty, err)
	}
	if _, err = a.Dashboard(ctx, "invalid"); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	codeRaw, err := a.IssueCode(ctx, actorA, uuid.NewString())
	if err != nil {
		t.Fatal(err)
	}
	var code ReferralCode
	if err = json.Unmarshal(codeRaw, &code); err != nil || code.Code == "" {
		t.Fatal(code, err)
	}
	// The shared referral table is also read by legacy campaign analytics.
	if _, err = l.pool.Exec(ctx, `INSERT INTO pnl.referrals(short_code,sharer_address,card_id,created_at) VALUES($1,$2,'portfolio-card',$3)`, code.Code, actorA, now.Unix()); err != nil {
		t.Fatal(err)
	}
	clickRaw, err := a.ClickCode(ctx, actorB, uuid.NewString(), code.Code, true)
	if err != nil {
		t.Fatal(err)
	}
	var click ReferralClick
	if err = json.Unmarshal(clickRaw, &click); err != nil {
		t.Fatal(err)
	}
	binding, err := a.BindIssued(ctx, actorB, uuid.NewString(), true, []string{click.Token})
	if err != nil || binding.Referrer != actorA {
		t.Fatal(binding, err)
	}
	read, err := a.Binding(ctx, actorB)
	if err != nil || read != binding {
		t.Fatal(read, err)
	}
	if _, err = a.Binding(ctx, actorC); err == nil {
		t.Fatal("missing binding returned")
	}
	if _, err = a.Binding(ctx, "invalid"); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	request := uuid.NewString()
	for i := 0; i < 2; i++ {
		if err = l.RecordAnalytics(ctx, Actor{Wallet: actorB}, request, "campaign", code.Code, "view", now); err != nil {
			t.Fatal(err)
		}
	}
	if err = l.RecordAnalytics(ctx, Actor{Wallet: actorB}, uuid.NewString(), "campaign", "unknown", "click", now); !errors.Is(err, ErrInvalid) {
		t.Fatal(err)
	}
	if err = l.RecordAnalytics(ctx, Actor{Wallet: "invalid"}, uuid.NewString(), "campaign", code.Code, "view", now); !errors.Is(err, ErrUnauthorized) {
		t.Fatal(err)
	}
	dashboard, err := a.Dashboard(ctx, actorA)
	if err != nil || dashboard.Code == nil || dashboard.Code.Code != code.Code || dashboard.Clicks != 1 || dashboard.BoundAccounts != 1 || dashboard.Asset != USDL {
		t.Fatal(dashboard, err)
	}
	dashboard, err = a.Dashboard(ctx, actorB)
	if err != nil || dashboard.Binding == nil || *dashboard.Binding != binding || !dashboard.Consent.Consent {
		t.Fatal(dashboard, err)
	}
	if _, err = a.SetConsent(ctx, actorB, uuid.NewString(), false); err != nil {
		t.Fatal(err)
	}
	dashboard, err = a.Dashboard(ctx, actorB)
	if err != nil || dashboard.Consent.Consent {
		t.Fatal(dashboard, err)
	}
	if _, err = l.pool.Exec(ctx, `INSERT INTO `+l.schema+`.l18_analytics(identity,campaign,actor,day,short_code,kind) SELECT gen_random_uuid(),'campaign',$1,$2::date,$3,'view' FROM generate_series(1,999)`, actorB, now.Format("2006-01-02"), code.Code); err != nil {
		t.Fatal(err)
	}
	if err = l.RecordAnalytics(ctx, Actor{Wallet: actorB}, uuid.NewString(), "campaign", code.Code, "view", now); !errors.Is(err, ErrUnavailable) {
		t.Fatal(err)
	}
	if got := query(t, l, `SELECT count(*)::text FROM l18_analytics`); got != "1001" {
		t.Fatal(got)
	}
}
