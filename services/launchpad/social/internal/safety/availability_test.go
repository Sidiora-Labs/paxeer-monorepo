package safety

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestModerationStoreRequiresCommittedDatabaseForDecisions(t *testing.T) {
	store := &Store{}
	ctx := WithPrincipal(context.Background(), Principal{Wallet: walletA, SessionID: "moderation-availability", Role: "global_administrator"})
	id := uuid.NewString()
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"report", func() error {
			_, err := store.Report(ctx, Report{TargetType: "message", TargetID: "message", Category: "spam"})
			return err
		}},
		{"transition", func() error { _, err := store.Transition(ctx, id, 1, "triaged", "reviewed", "v2"); return err }},
		{"ban", func() error {
			_, err := store.Ban(ctx, BanCommand{Wallet: walletB, Scope: "global", Reason: "reviewed", PolicyVersion: "v2"})
			return err
		}},
		{"revoke", func() error { return store.Revoke(ctx, id, 1, "reviewed") }},
		{"case evidence", func() error { _, err := store.CaseEvidence(ctx, id); return err }},
		{"appeal", func() error { _, err := store.Appeal(ctx, id, "reviewed"); return err }},
		{"decide appeal", func() error { return store.DecideAppeal(ctx, id, 1, "upheld", "reviewed") }},
		{"reverse appeal", func() error {
			return store.ReverseBanAppeal(ctx, ReversalCommand{AppealID: id, BanID: uuid.NewString(), ExpectedRevision: 1, ExpectedBanRevision: 1, Reason: "reviewed", PolicyVersion: "v2"})
		}},
		{"action ban", func() error {
			_, err := store.ActionBan(ctx, id, 1, BanCommand{Wallet: walletB, Scope: "global", Reason: "reviewed", PolicyVersion: "v2"})
			return err
		}},
		{"restore visibility", func() error {
			_, err := store.RestoreAppealedVisibility(ctx, RestoreCommand{CaseID: id, ExpectedRevision: 1, Reason: "reviewed", PolicyVersion: "v2"})
			return err
		}},
		{"set visibility", func() error { _, err := store.SetVisibility(ctx, id, 1, "removed", "reviewed"); return err }},
		{"read visibility", func() error { _, err := store.Visibility(ctx, "message", "message"); return err }},
		{"list cases", func() error { _, err := store.ListCases(ctx, "", "", 10); return err }},
		{"list bans", func() error { _, err := store.ListBans(ctx, "", 10); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("unbound moderation decision = %v", err)
			}
		})
	}
	if got := store.CheckTx(ctx, nil, walletB, "global", ""); got != Unavailable {
		t.Fatalf("unbound transactional policy = %s", got)
	}
}

func TestModerationListsApplyDocumentedPageBounds(t *testing.T) {
	store := fixture(t)
	ctx := principal(store, walletA, "global_administrator")
	for _, limit := range []int{0, 101} {
		cases, err := store.ListCases(ctx, "", "", limit)
		if err != nil || len(cases) != 0 {
			t.Fatalf("scoped case page at limit %d = %+v: %v", limit, cases, err)
		}
		bans, err := store.ListBans(ctx, "", limit)
		if err != nil || len(bans) != 0 {
			t.Fatalf("scoped ban page at limit %d = %+v: %v", limit, bans, err)
		}
	}
	if _, err := store.ListCases(ctx, "", string(make([]byte, 37)), 10); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unbounded case cursor = %v", err)
	}
	if _, err := store.ListBans(ctx, string(make([]byte, 37)), 10); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unbounded ban cursor = %v", err)
	}
	if _, _, err := (VoiceTargets{DB: store.DB}).Resolve(ctx, "voice_participant", uuid.NewString()+":"+walletB); !errors.Is(err, pgx.ErrNoRows) {
		t.Fatalf("unjoined voice participant = %v", err)
	}
}

func TestModerationDecisionsRejectClosedCommittedStore(t *testing.T) {
	store := fixture(t)
	ctx := principal(store, walletA, "global_administrator")
	id := uuid.NewString()
	store.DB.Close()
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"report", func() error {
			_, err := store.Report(ctx, Report{TargetType: "message", TargetID: "message", Category: "spam"})
			return err
		}},
		{"transition", func() error { _, err := store.Transition(ctx, id, 1, "triaged", "reviewed", "v2"); return err }},
		{"ban", func() error {
			_, err := store.Ban(ctx, BanCommand{Wallet: walletB, Scope: "global", Reason: "reviewed", PolicyVersion: "v2"})
			return err
		}},
		{"revoke", func() error { return store.Revoke(ctx, id, 1, "reviewed") }},
		{"case evidence", func() error { _, err := store.CaseEvidence(ctx, id); return err }},
		{"appeal", func() error { _, err := store.Appeal(ctx, id, "reviewed"); return err }},
		{"decide appeal", func() error { return store.DecideAppeal(ctx, id, 1, "upheld", "reviewed") }},
		{"reverse appeal", func() error {
			return store.ReverseBanAppeal(ctx, ReversalCommand{AppealID: id, BanID: uuid.NewString(), ExpectedRevision: 1, ExpectedBanRevision: 1, Reason: "reviewed", PolicyVersion: "v2"})
		}},
		{"action ban", func() error {
			_, err := store.ActionBan(ctx, id, 1, BanCommand{Wallet: walletB, Scope: "global", Reason: "reviewed", PolicyVersion: "v2"})
			return err
		}},
		{"restore visibility", func() error {
			_, err := store.RestoreAppealedVisibility(ctx, RestoreCommand{CaseID: id, ExpectedRevision: 1, Reason: "reviewed", PolicyVersion: "v2"})
			return err
		}},
		{"set visibility", func() error { _, err := store.SetVisibility(ctx, id, 1, "removed", "reviewed"); return err }},
		{"read visibility", func() error { _, err := store.Visibility(ctx, "message", "message"); return err }},
		{"list cases", func() error { _, err := store.ListCases(ctx, "", "", 10); return err }},
		{"list bans", func() error { _, err := store.ListBans(ctx, "", 10); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err == nil {
				t.Fatal("closed moderation store accepted a decision")
			}
		})
	}
}
