package safety

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestModerationMethodsRejectInvalidAuthorityAndUnboundedInput(t *testing.T) {
	store := fixture(t)
	member := principal(store, walletB, "participant")
	reviewer := principal(store, walletA, "room_moderator")
	caseID := uuid.NewString()
	for _, tc := range []struct {
		name string
		run  func() error
		want error
	}{
		{"report without actor", func() error {
			_, err := store.Report(context.Background(), Report{TargetType: "message", TargetID: "target", Category: "spam"})
			return err
		}, ErrForbidden},
		{"report unknown target", func() error {
			_, err := store.Report(member, Report{TargetType: "unknown", TargetID: "target", Category: "spam"})
			return err
		}, ErrInvalid},
		{"report blank category", func() error {
			_, err := store.Report(member, Report{TargetType: "message", TargetID: "target"})
			return err
		}, ErrInvalid},
		{"report long description", func() error {
			_, err := store.Report(member, Report{TargetType: "message", TargetID: "target", Category: "spam", Description: strings.Repeat("x", 4001)})
			return err
		}, ErrInvalid},
		{"transition invalid identifier", func() error {
			_, err := store.Transition(reviewer, "invalid", 1, "triaged", "reviewed", "v2")
			return err
		}, ErrInvalid},
		{"transition missing case", func() error { _, err := store.Transition(reviewer, caseID, 1, "triaged", "reviewed", "v2"); return err }, pgx.ErrNoRows},
		{"ban invalid wallet", func() error {
			_, err := store.Ban(reviewer, BanCommand{Wallet: "invalid", Scope: "pool", Resource: "pool-a", Reason: "reviewed", PolicyVersion: "v2"})
			return err
		}, ErrInvalid},
		{"ban expired deadline", func() error {
			past := store.now().Add(-time.Second)
			_, err := store.Ban(reviewer, BanCommand{Wallet: walletB, Scope: "pool", Resource: "pool-a", Reason: "reviewed", PolicyVersion: "v2", ExpiresAt: &past})
			return err
		}, ErrInvalid},
		{"revoke invalid reason", func() error { return store.Revoke(reviewer, caseID, 1, "") }, ErrInvalid},
		{"revoke absent ban", func() error { return store.Revoke(reviewer, caseID, 1, "reviewed") }, pgx.ErrNoRows},
		{"appeal invalid action", func() error { _, err := store.Appeal(member, "invalid", "reviewed"); return err }, ErrInvalid},
		{"appeal absent action", func() error { _, err := store.Appeal(member, caseID, "reviewed"); return err }, pgx.ErrNoRows},
		{"review invalid revision", func() error { return store.ReviewAppeal(reviewer, caseID, 0, "reviewed") }, ErrInvalid},
		{"review absent appeal", func() error { return store.ReviewAppeal(reviewer, caseID, 1, "reviewed") }, pgx.ErrNoRows},
		{"decide invalid outcome", func() error { return store.DecideAppeal(reviewer, caseID, 1, "unknown", "reviewed") }, ErrInvalid},
		{"decide absent appeal", func() error { return store.DecideAppeal(reviewer, caseID, 1, "upheld", "reviewed") }, pgx.ErrNoRows},
		{"visibility invalid status", func() error { _, err := store.SetVisibility(reviewer, caseID, 0, "public", "reviewed"); return err }, ErrInvalid},
		{"visibility absent case", func() error { _, err := store.SetVisibility(reviewer, caseID, 0, "removed", "reviewed"); return err }, pgx.ErrNoRows},
		{"reverse absent appeal", func() error {
			return store.ReverseBanAppeal(reviewer, ReversalCommand{AppealID: caseID, BanID: uuid.NewString(), ExpectedRevision: 1, ExpectedBanRevision: 1, Reason: "reviewed", PolicyVersion: "v2"})
		}, pgx.ErrNoRows},
		{"restore invalid revision", func() error {
			_, err := store.RestoreAppealedVisibility(reviewer, RestoreCommand{CaseID: caseID, Reason: "reviewed", PolicyVersion: "v2"})
			return err
		}, ErrInvalid},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); !errors.Is(err, tc.want) {
				t.Fatalf("moderation decision = %v, want %v", err, tc.want)
			}
		})
	}
	if got := store.Check(context.Background(), "invalid", "global", ""); got != Unavailable {
		t.Fatalf("invalid wallet decision = %s", got)
	}
	if got := store.Check(context.Background(), walletB, "unknown", "resource"); got != Unavailable {
		t.Fatalf("invalid scope decision = %s", got)
	}
	if got := (&Store{}).Check(context.Background(), walletB, "global", ""); got != Unavailable {
		t.Fatalf("unbound policy decision = %s", got)
	}
	if _, err := store.ProjectReplay(context.Background(), ReplayContent{TargetType: "message", TargetID: "replay", VisibilityRevision: -1, Text: "private"}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid replay revision = %v", err)
	}
	if projected, err := store.ProjectReplay(context.Background(), ReplayContent{TargetType: "message", TargetID: "replay", Text: "visible"}); err != nil || projected.Text != "visible" {
		t.Fatalf("unmoderated replay = %+v: %v", projected, err)
	}
	if projected, err := store.ProjectReplay(context.Background(), ReplayContent{TargetType: "message", TargetID: "replay", VisibilityRevision: 1, Text: "private"}); !errors.Is(err, ErrConflict) || projected.Text != "" {
		t.Fatalf("unverified replay = %+v: %v", projected, err)
	}
	current := Visibility{TargetType: "message", TargetID: "replay", Status: "removed", Revision: 1}
	incoming := Visibility{TargetType: "message", TargetID: "replay", Status: "restored", Revision: 2}
	if got := ApplyVisibility(current, incoming); got != incoming {
		t.Fatalf("newer committed visibility = %+v", got)
	}
	incoming.TargetID = "another-message"
	if got := ApplyVisibility(current, incoming); got != current {
		t.Fatalf("unrelated visibility changed replay = %+v", got)
	}
	if projected, err := (&Store{}).ProjectReplay(context.Background(), ReplayContent{TargetType: "message", TargetID: "replay", Text: "private"}); !errors.Is(err, ErrUnavailable) || projected.Text != "" {
		t.Fatalf("replay without moderation authority = %+v: %v", projected, err)
	}
}

func TestVoiceReportTargetsResolveCommittedParticipantScope(t *testing.T) {
	store := fixture(t)
	resolver := VoiceTargets{DB: store.DB, Next: fixtureTargets{}}
	if pool, affected, err := resolver.Resolve(context.Background(), "message", "reported-message"); err != nil || pool != "pool-a" || affected != walletB {
		t.Fatalf("nonvoice target = %q %q: %v", pool, affected, err)
	}
	if _, _, err := (VoiceTargets{}).Resolve(context.Background(), "message", "reported-message"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing target authority = %v", err)
	}
	if _, _, err := resolver.Resolve(context.Background(), "voice_participant", "invalid"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid participant target = %v", err)
	}
	room := uuid.NewString()
	if _, err := store.DB.Exec(context.Background(), `INSERT INTO voice_rooms(id,pool_id,host_wallet,start_key)VALUES($1,'pool-a',$2,$3)`, room, walletA, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	for _, wallet := range []string{walletA, walletB} {
		if _, err := store.DB.Exec(context.Background(), `INSERT INTO voice_participants(room_id,wallet)VALUES($1,$2)`, room, wallet); err != nil {
			t.Fatal(err)
		}
		pool, affected, err := resolver.Resolve(context.Background(), "voice_participant", room+":"+wallet)
		wantPool := "pool-a"
		if wallet == walletA {
			wantPool = ""
		}
		if err != nil || pool != wantPool || affected != wallet {
			t.Fatalf("participant scope = %q %q: %v", pool, affected, err)
		}
	}
}
