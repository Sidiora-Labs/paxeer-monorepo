package safety

import (
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func withoutSafetyRelation(t *testing.T, s *Store, table string, run func()) {
	t.Helper()
	name := pgx.Identifier{table}.Sanitize()
	held := pgx.Identifier{table + "_unavailable"}.Sanitize()
	if _, err := s.DB.Exec(context.Background(), "ALTER TABLE "+name+" RENAME TO "+held); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := s.DB.Exec(context.Background(), "ALTER TABLE "+held+" RENAME TO "+name); err != nil {
			t.Fatal(err)
		}
	}()
	run()
}

func TestModerationCommandsPreserveRevisionsWhenLedgerIsUnavailable(t *testing.T) {
	s := fixture(t)
	if _, err := s.DB.Exec(context.Background(), internaltest.CoordinatorMigrationSQL(t, s.DB.Config().ConnConfig.RuntimeParams["search_path"], 380001)); err != nil {
		t.Fatal(err)
	}
	moderator := principal(s, walletA, "room_moderator")
	reviewer := principal(s, "0x3333333333333333333333333333333333333333", "room_moderator")
	affected := principal(s, walletB, "participant")
	open, err := s.Report(moderator, Report{TargetType: "message", TargetID: "open-review", Category: "abuse", Description: "case evidence"})
	if err != nil {
		t.Fatal(err)
	}
	appealed, err := s.Report(moderator, Report{TargetType: "message", TargetID: "appealed-review", Category: "abuse"})
	if err != nil {
		t.Fatal(err)
	}
	command := BanCommand{Wallet: walletB, Scope: "pool", Resource: "pool-a", Reason: "reviewed conduct", PolicyVersion: "policy-v2"}
	ban, err := s.ActionBan(moderator, appealed.ID, 1, command)
	if err != nil {
		t.Fatal(err)
	}
	var action string
	if err := s.DB.QueryRow(moderator, `SELECT action_id FROM safety_action_bans WHERE ban_id=$1`, ban).Scan(&action); err != nil {
		t.Fatal(err)
	}
	appeal, err := s.Appeal(affected, action, "please review")
	if err != nil {
		t.Fatal(err)
	}
	evidencePrincipal, err := Actor(moderator, s.now())
	if err != nil {
		t.Fatal(err)
	}
	evidencePrincipal.Cases = []string{open.ID}
	evidence := WithPrincipal(moderator, evidencePrincipal)
	reverse := ReversalCommand{AppealID: appeal, BanID: ban, ExpectedRevision: 1, ExpectedBanRevision: 1, Reason: "accepted appeal", PolicyVersion: "policy-v2"}
	for _, tc := range []struct {
		name   string
		tables []string
		run    func() error
	}{
		{"transition", []string{"safety_cases", "safety_actions", "safety_outbox"}, func() error {
			_, err := s.Transition(moderator, open.ID, 1, "triaged", "reviewed", "policy-v2")
			return err
		}},
		{"ban", []string{"safety_bans", "safety_actions", "safety_outbox"}, func() error { _, err := s.Ban(moderator, command); return err }},
		{"linked ban", []string{"safety_cases", "safety_bans", "safety_actions", "safety_action_bans", "safety_outbox"}, func() error { _, err := s.ActionBan(moderator, open.ID, 1, command); return err }},
		{"revoke", []string{"safety_bans", "safety_actions", "safety_outbox"}, func() error { return s.Revoke(moderator, ban, 1, "reviewed reversal") }},
		{"visibility", []string{"safety_cases", "safety_visibility", "safety_actions", "safety_outbox"}, func() error {
			_, err := s.SetVisibility(moderator, open.ID, 0, "removed", "reviewed content")
			return err
		}},
		{"evidence", []string{"safety_cases", "safety_actions"}, func() error { _, err := s.CaseEvidence(evidence, open.ID); return err }},
		{"review appeal", []string{"safety_appeals", "safety_outbox"}, func() error { return s.ReviewAppeal(reviewer, appeal, 1, "independent review") }},
		{"decide appeal", []string{"safety_appeals", "safety_outbox"}, func() error { return s.DecideAppeal(reviewer, appeal, 1, "upheld", "independent review") }},
		{"reverse appeal", []string{"safety_action_bans", "safety_outbox"}, func() error { return s.ReverseBanAppeal(reviewer, reverse) }},
	} {
		for _, table := range tc.tables {
			t.Run(tc.name+"/"+table, func(t *testing.T) {
				withoutSafetyRelation(t, s, table, func() {
					err := tc.run()
					var postgres *pgconn.PgError
					if !errors.As(err, &postgres) || postgres.Code != "42P01" {
						t.Fatalf("moderation dependency = %v", err)
					}
				})
				var openRevision, appealRevision, banRevision int64
				var openState, appealState string
				var revoked bool
				if err := s.DB.QueryRow(moderator, `SELECT state,revision FROM safety_cases WHERE id=$1`, open.ID).Scan(&openState, &openRevision); err != nil {
					t.Fatal(err)
				}
				if err := s.DB.QueryRow(moderator, `SELECT state,revision FROM safety_appeals WHERE id=$1`, appeal).Scan(&appealState, &appealRevision); err != nil {
					t.Fatal(err)
				}
				if err := s.DB.QueryRow(moderator, `SELECT revision,revoked_at IS NOT NULL FROM safety_bans WHERE id=$1`, ban).Scan(&banRevision, &revoked); err != nil {
					t.Fatal(err)
				}
				var visibility int
				if err := s.DB.QueryRow(moderator, `SELECT count(*) FROM safety_visibility WHERE case_id=$1`, open.ID).Scan(&visibility); err != nil {
					t.Fatal(err)
				}
				if openState != "open" || openRevision != 1 || appealState != "submitted" || appealRevision != 1 || banRevision != 1 || revoked || visibility != 0 {
					t.Fatalf("partial moderation state: case=%s/%d appeal=%s/%d ban=%d revoked=%t visibility=%d", openState, openRevision, appealState, appealRevision, banRevision, revoked, visibility)
				}
			})
		}
	}
	if err := s.ReverseBanAppeal(reviewer, reverse); err != nil {
		t.Fatal(err)
	}
	if decision := s.Check(reviewer, walletB, "pool", "pool-a"); decision != Allow {
		t.Fatalf("recovered reversal decision = %s", decision)
	}
}

func TestAppealSubmissionAndRestorationRequireDurableDependencies(t *testing.T) {
	s := fixture(t)
	if _, err := s.DB.Exec(context.Background(), internaltest.CoordinatorMigrationSQL(t, s.DB.Config().ConnConfig.RuntimeParams["search_path"], 380001)); err != nil {
		t.Fatal(err)
	}
	moderator := principal(s, walletA, "room_moderator")
	reviewer := principal(s, "0x3333333333333333333333333333333333333333", "room_moderator")
	affected := principal(s, walletB, "participant")
	reported, err := s.Report(moderator, Report{TargetType: "message", TargetID: "restored-content", Category: "abuse"})
	if err != nil {
		t.Fatal(err)
	}
	ban, err := s.ActionBan(moderator, reported.ID, 1, BanCommand{Wallet: walletB, Scope: "pool", Resource: "pool-a", Reason: "reviewed conduct", PolicyVersion: "policy-v2"})
	if err != nil {
		t.Fatal(err)
	}
	var action string
	if err := s.DB.QueryRow(moderator, `SELECT action_id FROM safety_action_bans WHERE ban_id=$1`, ban).Scan(&action); err != nil {
		t.Fatal(err)
	}
	for _, table := range []string{"safety_appeals", "safety_outbox"} {
		t.Run("submission/"+table, func(t *testing.T) {
			withoutSafetyRelation(t, s, table, func() {
				if _, err := s.Appeal(affected, action, "please review"); err == nil {
					t.Fatal("appeal committed without durable dependency")
				}
			})
		})
	}
	appeal, err := s.Appeal(affected, action, "please review")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ReverseBanAppeal(reviewer, ReversalCommand{AppealID: appeal, BanID: ban, ExpectedRevision: 1, ExpectedBanRevision: 1, Reason: "accepted appeal", PolicyVersion: "policy-v2"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetVisibility(moderator, reported.ID, 0, "removed", "reviewed content"); err != nil {
		t.Fatal(err)
	}
	restore := RestoreCommand{CaseID: reported.ID, ExpectedRevision: 1, Reason: "accepted appeal", PolicyVersion: "policy-v2"}
	for _, table := range []string{"safety_bans", "safety_visibility", "safety_outbox"} {
		t.Run("restore/"+table, func(t *testing.T) {
			withoutSafetyRelation(t, s, table, func() {
				_, err := s.RestoreAppealedVisibility(reviewer, restore)
				if err == nil {
					t.Fatal("restoration committed without durable dependency")
				}
				if table == "safety_bans" && !errors.Is(err, ErrUnavailable) {
					t.Fatalf("ban authority dependency = %v", err)
				}
			})
			visibility, err := s.Visibility(moderator, "message", "restored-content")
			if err != nil || visibility.Status != "removed" || visibility.Revision != 1 {
				t.Fatalf("partial restoration = %+v: %v", visibility, err)
			}
		})
	}
	restored, err := s.RestoreAppealedVisibility(reviewer, restore)
	if err != nil || restored.Status != "restored" || restored.Revision != 2 {
		t.Fatalf("recovered restoration = %+v: %v", restored, err)
	}
}
