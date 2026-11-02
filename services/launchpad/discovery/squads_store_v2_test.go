package discovery

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestPostgresSquadRoleChangeAndJoinRequest(t *testing.T) {
	s, id := realStore(t)
	ctx := context.Background()
	owner, moderator, applicant := id+"owner", id+"moderator", id+"applicant"
	q := Squad{ID: id, OwnerAccountID: owner, Name: "Research", Policy: "request", Description: strings.Repeat("a", 280), Visibility: "public", Revision: 1}
	if err := q.Validate(); err != nil {
		t.Fatal("valid squad", err)
	}
	for _, mutate := range []func(*Squad){
		func(q *Squad) { q.Visibility = "" },
		func(q *Squad) { q.Visibility = "unlisted" },
		func(q *Squad) { q.Description += "a" },
		func(q *Squad) { q.Description = strings.Repeat("\u00e9", 141) },
	} {
		invalid := q
		mutate(&invalid)
		if err := invalid.Validate(); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid squad admitted: %+v, %v", invalid, err)
		}
	}
	t.Cleanup(func() {
		for _, table := range []string{"transfers", "invitations", "members"} {
			if _, err := s.DB.Exec(`DELETE FROM narrative_discovery.`+table+` WHERE squad_id=$1`, id); err != nil {
				t.Error(err)
			}
		}
		if _, err := s.DB.Exec(`DELETE FROM narrative_discovery.squads WHERE id=$1`, id); err != nil {
			t.Error(err)
		}
	})
	if err := s.CreateSquad(ctx, owner, q); err != nil {
		t.Fatal(err)
	}
	restarted := Store{s.DB}
	if got, err := restarted.Squad(ctx, id); err != nil || got != q {
		t.Fatal("squad fields did not persist", got, err)
	}
	if _, err := s.Squad(ctx, id+"missing"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatal("missing squad", err)
	}
	readMember := func(account string) Membership {
		t.Helper()
		var b []byte
		var m Membership
		if err := s.DB.QueryRowContext(ctx, `SELECT document FROM narrative_discovery.members WHERE squad_id=$1 AND account_id=$2`, id, account).Scan(&b); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &m); err != nil {
			t.Fatal(err)
		}
		return m
	}
	readInvitation := func(account string) Invitation {
		t.Helper()
		var b []byte
		var inv Invitation
		if err := s.DB.QueryRowContext(ctx, `SELECT document FROM narrative_discovery.invitations WHERE squad_id=$1 AND account_id=$2`, id, account).Scan(&b); err != nil {
			t.Fatal(err)
		}
		if err := json.Unmarshal(b, &inv); err != nil {
			t.Fatal(err)
		}
		return inv
	}
	request := func(account string, expected uint64) Membership {
		t.Helper()
		m, err := s.RequestJoin(ctx, account, id, expected)
		if err != nil || m.Status != "requested" || m.Role != "member" || m.Revision != expected+1 || m.CanPublish() || !m.JoinedAt.IsZero() {
			t.Fatal("request transition", m, err)
		}
		if got := readMember(account); got != m {
			t.Fatal("request not persisted", got, m)
		}
		if inv := readInvitation(account); inv.Status != "requested" || inv.Consumed || inv.SquadID != id || inv.AccountID != account {
			t.Fatal("requested invitation not persisted", inv)
		}
		return m
	}
	approve := func(actor, account string, expected uint64) Membership {
		t.Helper()
		now := time.Now().UTC().Truncate(time.Microsecond)
		m, err := restarted.ApproveRequest(ctx, actor, id, account, expected, now)
		if err != nil || m.Status != "active" || m.Role != "member" || m.Revision != expected+1 || !m.JoinedAt.Equal(now) || !m.CanPublish() {
			t.Fatal("approval transition", m, err)
		}
		if got := readMember(account); got != m {
			t.Fatal("approval not persisted", got, m)
		}
		if inv := readInvitation(account); inv.Status != "approved" || !inv.Consumed {
			t.Fatal("approval did not consume request", inv)
		}
		return m
	}
	if _, err := s.RequestJoin(ctx, moderator, id, 1); !errors.Is(err, ErrConflict) {
		t.Fatal("stale initial request admitted", err)
	}
	request(moderator, 0)
	if _, err := s.RequestJoin(ctx, moderator, id, 1); !errors.Is(err, ErrConflict) {
		t.Fatal("duplicate request admitted", err)
	}
	if _, err := s.ApproveRequest(ctx, moderator, id, moderator, 1, time.Now()); !errors.Is(err, ErrForbidden) {
		t.Fatal("self approval admitted", err)
	}
	if _, err := s.ApproveRequest(ctx, owner, id, moderator, 0, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatal("stale approval admitted", err)
	}
	if m, inv := readMember(moderator), readInvitation(moderator); m.Status != "requested" || m.Revision != 1 || inv.Consumed {
		t.Fatal("rejected approval changed state", m, inv)
	}
	approve(owner, moderator, 1)
	if err := s.SetMemberRole(ctx, moderator, id, moderator, "moderator", 2); !errors.Is(err, ErrForbidden) {
		t.Fatal("non-owner promoted self", err)
	}
	if err := s.SetMemberRole(ctx, owner, id, owner, "member", 1); !errors.Is(err, ErrForbidden) {
		t.Fatal("owner demoted self", err)
	}
	if err := s.SetMemberRole(ctx, owner, id, moderator, "owner", 2); !errors.Is(err, ErrInvalid) {
		t.Fatal("role change bypassed ownership transfer", err)
	}
	if err := s.SetMemberRole(ctx, owner, id, moderator, "moderator", 1); !errors.Is(err, ErrConflict) {
		t.Fatal("stale promotion admitted", err)
	}
	before := readMember(moderator)
	if err := s.SetMemberRole(ctx, owner, id, moderator, "moderator", 2); err != nil {
		t.Fatal(err)
	}
	if m := readMember(moderator); m.Role != "moderator" || m.Revision != 3 || m.Status != "active" || m.JoinedAt != before.JoinedAt {
		t.Fatal("promotion not persisted", m)
	}
	request(applicant, 0)
	if _, err := s.ApproveRequest(ctx, id+"outsider", id, applicant, 1, time.Now()); !errors.Is(err, ErrForbidden) {
		t.Fatal("outsider approved request", err)
	}
	if err := s.SetMemberRole(ctx, moderator, id, applicant, "moderator", 1); !errors.Is(err, ErrForbidden) {
		t.Fatal("moderator changed role", err)
	}
	approve(moderator, applicant, 1)
	if _, err := s.ApproveRequest(ctx, owner, id, applicant, 1, time.Now()); !errors.Is(err, ErrConflict) {
		t.Fatal("stale approval replay admitted", err)
	}
	if _, err := s.ApproveRequest(ctx, owner, id, applicant, 2, time.Now()); !errors.Is(err, ErrForbidden) {
		t.Fatal("consumed approval replay admitted", err)
	}
	if err := s.SetMemberRole(ctx, owner, id, moderator, "member", 2); !errors.Is(err, ErrConflict) {
		t.Fatal("stale demotion admitted", err)
	}
	if err := s.SetMemberRole(ctx, owner, id, moderator, "member", 3); err != nil {
		t.Fatal(err)
	}
	if m := readMember(moderator); m.Role != "member" || m.Revision != 4 {
		t.Fatal("demotion not persisted", m)
	}
	request(id+"next", 0)
	if _, err := s.ApproveRequest(ctx, moderator, id, id+"next", 1, time.Now()); !errors.Is(err, ErrForbidden) {
		t.Fatal("demoted moderator approved request", err)
	}
	if err := s.SetMemberStatus(ctx, owner, id, applicant, "removed", 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RequestJoin(ctx, applicant, id, 2); !errors.Is(err, ErrConflict) {
		t.Fatal("stale rejoin admitted", err)
	}
	request(applicant, 3)
	approve(owner, applicant, 4)
	if err := s.SetMemberStatus(ctx, owner, id, applicant, "banned", 5); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RequestJoin(ctx, applicant, id, 6); !errors.Is(err, ErrForbidden) {
		t.Fatal("banned member requested rejoin", err)
	}
	if err := s.SetMemberRole(ctx, owner, id, applicant, "moderator", 6); !errors.Is(err, ErrForbidden) {
		t.Fatal("banned member promoted", err)
	}
	if _, err := s.ApproveRequest(ctx, owner, id, applicant, 6, time.Now()); !errors.Is(err, ErrForbidden) {
		t.Fatal("banned member approved", err)
	}
	q.Visibility, q.Description, q.Policy, q.Revision = "private", "Private research", "invite", 2
	if err := s.UpdateSquad(ctx, owner, q, 1); err != nil {
		t.Fatal(err)
	}
	if got, err := s.Squad(ctx, id); err != nil || got != q {
		t.Fatal("squad update did not persist", got, err)
	}
	if _, err := s.RequestJoin(ctx, id+"invite-only", id, 0); !errors.Is(err, ErrForbidden) {
		t.Fatal("invite-only join request admitted", err)
	}
	if _, err := s.ApproveRequest(ctx, owner, id, id+"next", 1, time.Now()); !errors.Is(err, ErrForbidden) {
		t.Fatal("approval ignored policy change", err)
	}
	q.Policy, q.Revision = "request", 3
	if err := s.UpdateSquad(ctx, owner, q, 2); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.ApproveRequest(ctx, owner, id, id+"next", 1, time.Now())
			results <- err
		}()
	}
	wg.Wait()
	close(results)
	success, conflict := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrConflict) {
			conflict++
		} else {
			t.Fatal("concurrent approval", err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("approval revision check was not atomic", success, conflict)
	}
	if m := readMember(id + "next"); m.Status != "active" || m.Role != "member" || m.Revision != 2 {
		t.Fatal("concurrent approval persisted incorrect membership", m)
	}
}
