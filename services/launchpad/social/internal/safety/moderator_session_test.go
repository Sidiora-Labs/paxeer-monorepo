package safety

import (
	"context"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/internaltest"
	"github.com/google/uuid"
)

func TestModeratorSessionResolvesCurrentScopes(t *testing.T) {
	s := fixture(t)
	ctx := context.Background()
	ddl := internaltest.CoordinatorMigrationSQL(t, s.DB.Config().ConnConfig.RuntimeParams["search_path"], 380001)
	if _, err := s.DB.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	account, session, caseID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	if _, err := s.DB.Exec(ctx, `INSERT INTO safety_cases(id,target_type,target_id,pool_id,affected_wallet,reporter,category,description) VALUES($1,'message','moderator-session-target','pool-a',$2,$3,'spam','reported content')`, caseID, walletB, walletA); err != nil {
		t.Fatal(err)
	}
	requested := s.now().Add(2 * time.Hour)
	grantExpiry := s.now().Add(time.Hour)
	if _, err := s.DB.Exec(ctx, `INSERT INTO safety_moderator_sessions(session_id,account_id,wallet,role,expires_at,step_up_at) VALUES($1,$2,$3,'room_moderator',$4,$5)`, session, account, walletA, grantExpiry, s.now()); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `INSERT INTO safety_moderator_scopes(session_id,pool_id,case_id) VALUES($1,'pool-a',$2)`, session, caseID); err != nil {
		t.Fatal(err)
	}
	moderator, err := s.ResolvePrincipal(ctx, walletA, account, session, requested)
	if err != nil || moderator.Role != "room_moderator" || !moderator.Expires.Equal(grantExpiry) || len(moderator.Pools) != 1 || moderator.Pools[0] != "pool-a" || len(moderator.Cases) != 1 || moderator.Cases[0] != caseID || !Authorized(moderator, "pool-a", true, s.now()) {
		t.Fatalf("resolved moderator = %+v: %v", moderator, err)
	}
	if _, err := s.DB.Exec(ctx, `DELETE FROM safety_moderator_scopes WHERE session_id=$1`, session); err != nil {
		t.Fatal(err)
	}
	if _, err := s.DB.Exec(ctx, `DELETE FROM safety_moderator_sessions WHERE session_id=$1`, session); err != nil {
		t.Fatal(err)
	}
	participant, err := s.ResolvePrincipal(ctx, walletA, account, session, requested)
	if err != nil || participant.Role != "participant" || len(participant.Pools) != 0 || Authorized(participant, "pool-a", true, s.now()) {
		t.Fatalf("revoked moderator grant = %+v: %v", participant, err)
	}
}
