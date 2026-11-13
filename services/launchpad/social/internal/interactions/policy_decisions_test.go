package interactions

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func TestCommittedInteractionPolicyChecksGraphAndPoolDecisions(t *testing.T) {
	f := connected(t)
	actor, target := f.wallets[0], f.wallets[1]
	ctx := f.ctx[actor]
	check := func(request Request, want error) {
		t.Helper()
		tx, err := f.db.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer rollback(tx)
		if err = f.policy.Check(ctx, tx, request); !errors.Is(err, want) {
			t.Fatalf("policy for %s = %v, want %v", request.Action, err, want)
		}
	}
	follow := Request{Actor: actor, ID: target, Action: followAction}
	check(follow, nil)
	if _, err := f.db.Exec(ctx, `INSERT INTO interaction_blocks(owner,target) VALUES($1,$2)`, target, actor); err != nil {
		t.Fatal(err)
	}
	check(follow, ErrDenied)
	if _, err := f.db.Exec(ctx, `DELETE FROM interaction_blocks WHERE owner=$1 AND target=$2`, target, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(ctx, `INSERT INTO interaction_mutes(owner,target) VALUES($1,$2)`, target, actor); err != nil {
		t.Fatal(err)
	}
	check(Request{Actor: actor, Parent: target, Action: mentionAction}, ErrDenied)
	if _, err := f.db.Exec(ctx, `DELETE FROM interaction_mutes WHERE owner=$1 AND target=$2`, target, actor); err != nil {
		t.Fatal(err)
	}
	check(Request{Actor: actor, Pool: f.pool, Parent: uuid.NewString(), Action: commentAction}, ErrInvalid)
	check(Request{Actor: actor, Action: calloutReferenceAction, Body: `not json`}, ErrInvalid)
	check(Request{Actor: actor, Pool: f.pool, Action: commentAction}, nil)
	if _, err := f.db.Exec(ctx, `INSERT INTO safety_bans(id,wallet,scope,resource,actor) VALUES($1,$2,'pool',$3,$4)`, uuid.NewString(), actor, f.pool, target); err != nil {
		t.Fatal(err)
	}
	check(Request{Actor: actor, Pool: f.pool, Action: commentAction}, ErrDenied)
	if _, err := f.db.Exec(ctx, `DELETE FROM safety_bans WHERE wallet=$1`, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(ctx, `INSERT INTO safety_bans(id,wallet,scope,resource,actor) VALUES($1,$2,'global','',$3)`, uuid.NewString(), actor, target); err != nil {
		t.Fatal(err)
	}
	check(follow, ErrDenied)
	if _, err := f.db.Exec(ctx, `DELETE FROM safety_bans WHERE wallet=$1`, actor); err != nil {
		t.Fatal(err)
	}
	check(Request{Actor: actor, ID: uuid.NewString(), Revision: 1, Action: mentionAction}, pgx.ErrNoRows)
}
