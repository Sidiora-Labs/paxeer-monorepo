package safety

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestModerationBoundariesPreserveCommittedState(t *testing.T) {
	store := fixture(t)
	moderator := principal(store, walletA, "room_moderator")
	participant := principal(store, walletB, "participant")
	created, err := store.Report(participant, Report{TargetType: "message", TargetID: "boundary-message", Category: "spam", Description: "reported content"})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name string
		run  func() error
		want error
	}{
		{"anonymous case listing", func() error { _, err := store.ListCases(context.Background(), "", "", 10); return err }, ErrForbidden},
		{"participant case listing", func() error { _, err := store.ListCases(participant, "", "", 10); return err }, ErrForbidden},
		{"long case cursor", func() error { _, err := store.ListCases(moderator, "", strings.Repeat("x", 37), 10); return err }, ErrInvalid},
		{"long case state", func() error { _, err := store.ListCases(moderator, strings.Repeat("x", 17), "", 10); return err }, ErrInvalid},
		{"participant ban listing", func() error { _, err := store.ListBans(participant, "", 10); return err }, ErrForbidden},
		{"long ban cursor", func() error { _, err := store.ListBans(moderator, strings.Repeat("x", 37), 10); return err }, ErrInvalid},
		{"invalid visibility state", func() error {
			_, err := store.SetVisibility(moderator, created.ID, 0, "public", "reviewed")
			return err
		}, ErrInvalid},
		{"restore without removal", func() error {
			_, err := store.SetVisibility(moderator, created.ID, 0, "restored", "reviewed")
			return err
		}, ErrConflict},
		{"negative visibility revision", func() error {
			_, err := store.SetVisibility(moderator, created.ID, -1, "removed", "reviewed")
			return err
		}, ErrInvalid},
		{"unreviewed restoration", func() error {
			_, err := store.RestoreAppealedVisibility(moderator, RestoreCommand{CaseID: created.ID, ExpectedRevision: 1, Reason: "reviewed", PolicyVersion: "v2"})
			return err
		}, ErrConflict},
		{"invalid restoration revision", func() error {
			_, err := store.RestoreAppealedVisibility(moderator, RestoreCommand{CaseID: created.ID, Reason: "reviewed", PolicyVersion: "v2"})
			return err
		}, ErrInvalid},
		{"invalid reversal appeal", func() error {
			return store.ReverseBanAppeal(moderator, ReversalCommand{AppealID: "invalid", BanID: uuid.NewString(), ExpectedRevision: 1, ExpectedBanRevision: 1, Reason: "reviewed", PolicyVersion: "v2"})
		}, ErrInvalid},
		{"invalid reversal revision", func() error {
			return store.ReverseBanAppeal(moderator, ReversalCommand{AppealID: uuid.NewString(), BanID: uuid.NewString(), Reason: "reviewed", PolicyVersion: "v2"})
		}, ErrInvalid},
		{"invalid ban target", func() error {
			_, err := store.ActionBan(moderator, created.ID, 1, BanCommand{Wallet: "invalid", Scope: "pool", Resource: "pool-a", Reason: "reviewed", PolicyVersion: "v2"})
			return err
		}, ErrInvalid},
		{"stale case ban", func() error {
			_, err := store.ActionBan(moderator, created.ID, 2, BanCommand{Wallet: walletB, Scope: "pool", Resource: "pool-a", Reason: "reviewed", PolicyVersion: "v2"})
			return err
		}, ErrConflict},
		{"invalid revocation", func() error { return store.Revoke(moderator, uuid.NewString(), 0, "reviewed") }, ErrInvalid},
		{"expired moderator", func() error {
			_, err := store.ListCases(WithPrincipal(context.Background(), Principal{Wallet: walletA, SessionID: "expired", Role: "room_moderator", Expires: store.now().Add(-time.Second)}), "", "", 10)
			return err
		}, ErrForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); !errors.Is(err, tc.want) {
				t.Fatalf("moderation decision = %v, want %v", err, tc.want)
			}
		})
	}
	var caseRevision int64
	if err := store.DB.QueryRow(context.Background(), `SELECT revision FROM safety_cases WHERE id=$1`, created.ID).Scan(&caseRevision); err != nil || caseRevision != 1 {
		t.Fatalf("case revision after refused changes = %d: %v", caseRevision, err)
	}
}
