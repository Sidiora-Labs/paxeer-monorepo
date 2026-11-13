package safety

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestModerationCommandsRequireSessionBeforeReadingPrivateState(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	id := uuid.NewString()
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"transition", func() error { _, err := s.Transition(ctx, id, 1, "triaged", "reviewed", "policy-v2"); return err }},
		{"revoke", func() error { return s.Revoke(ctx, id, 1, "reviewed") }},
		{"evidence", func() error { _, err := s.CaseEvidence(ctx, id); return err }},
		{"appeal", func() error { _, err := s.Appeal(ctx, id, "reviewed"); return err }},
		{"private appeal", func() error { _, err := s.PrivateAppeal(ctx, id); return err }},
		{"review", func() error { return s.ReviewAppeal(ctx, id, 1, "reviewed") }},
		{"decide", func() error { return s.DecideAppeal(ctx, id, 1, "upheld", "reviewed") }},
		{"linked ban", func() error { _, err := s.ActionBan(ctx, id, 1, BanCommand{}); return err }},
		{"reverse", func() error { return s.ReverseBanAppeal(ctx, ReversalCommand{}) }},
		{"restore", func() error { _, err := s.RestoreAppealedVisibility(ctx, RestoreCommand{}); return err }},
		{"visibility", func() error { _, err := s.SetVisibility(ctx, id, 0, "removed", "reviewed"); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); !errors.Is(err, ErrForbidden) {
				t.Fatalf("anonymous moderation command = %v", err)
			}
		})
	}
	if _, err := (&Store{}).PrivateAppeal(ctx, id); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("private appeal without storage = %v", err)
	}
	if err := (&Store{}).ReviewAppeal(ctx, id, 1, "reviewed"); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("appeal review without storage = %v", err)
	}
	for _, tc := range []struct {
		err  error
		want int
	}{
		{ErrConflict, 409},
		{pgx.ErrNoRows, 404},
	} {
		if status := HTTPStatus(tc.err); status != tc.want {
			t.Fatalf("moderation status = %d, want %d", status, tc.want)
		}
	}
}
