package interactions

import (
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/google/uuid"
)

func TestCommittedChatPolicyAppliesRecipientPreferencesAndBans(t *testing.T) {
	f := connected(t)
	actor, recipient := f.wallets[0], f.wallets[1]
	ctx := f.ctx[actor]
	check := func(in fanout.SendInput, expected error) {
		t.Helper()
		tx, err := f.db.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		defer rollback(tx)
		err = f.policy.SendPolicy(ctx, tx, in)
		if !errors.Is(err, expected) {
			t.Fatalf("chat policy for %s = %v, want %v", in.Kind, err, expected)
		}
	}
	dm := fanout.SendInput{Actor: actor, Kind: "dm", Target: recipient}
	check(dm, nil)
	check(fanout.SendInput{Actor: recipient, Kind: "dm", Target: actor}, ErrDenied)
	check(fanout.SendInput{Actor: actor, Kind: "unknown", Target: recipient}, ErrInvalid)
	if _, err := f.db.Exec(context.Background(), `UPDATE interaction_preferences SET inbox='nobody' WHERE wallet=$1`, recipient); err != nil {
		t.Fatal(err)
	}
	check(dm, ErrDenied)
	if _, err := f.db.Exec(context.Background(), `UPDATE interaction_preferences SET inbox='followed_only' WHERE wallet=$1`, recipient); err != nil {
		t.Fatal(err)
	}
	check(dm, ErrDenied)
	if _, err := f.db.Exec(context.Background(), `INSERT INTO interaction_follows(follower,followee) VALUES($1,$2)`, recipient, actor); err != nil {
		t.Fatal(err)
	}
	check(dm, nil)
	if _, err := f.db.Exec(context.Background(), `INSERT INTO interaction_blocks(owner,target) VALUES($1,$2)`, recipient, actor); err != nil {
		t.Fatal(err)
	}
	check(dm, ErrDenied)
	if _, err := f.db.Exec(context.Background(), `DELETE FROM interaction_blocks WHERE owner=$1 AND target=$2`, recipient, actor); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(context.Background(), `INSERT INTO safety_bans(id,wallet,scope,resource,actor) VALUES($1,$2,'global','',$3)`, uuid.NewString(), actor, recipient); err != nil {
		t.Fatal(err)
	}
	check(dm, ErrDenied)
	if _, err := f.db.Exec(context.Background(), `DELETE FROM safety_bans WHERE wallet=$1`, actor); err != nil {
		t.Fatal(err)
	}
	check(dm, nil)
	if _, err := f.db.Exec(context.Background(), `DELETE FROM interaction_preferences WHERE wallet=$1`, recipient); err != nil {
		t.Fatal(err)
	}
	check(dm, ErrDenied)
	if _, err := f.db.Exec(context.Background(), `INSERT INTO interaction_preferences(wallet,inbox) VALUES($1,'everyone')`, recipient); err != nil {
		t.Fatal(err)
	}
	check(dm, nil)
	if _, err := f.db.Exec(context.Background(), `INSERT INTO interaction_erased_wallets(wallet,deletion_case_id,policy_version) VALUES($1,$2,'retention-v1')`, recipient, uuid.NewString()); err != nil {
		t.Fatal(err)
	}
	check(dm, ErrDenied)
	if _, err := f.db.Exec(context.Background(), `DELETE FROM interaction_erased_wallets WHERE wallet=$1`, recipient); err != nil {
		t.Fatal(err)
	}
	if _, err := f.db.Exec(context.Background(), `UPDATE account_state SET deleted=true WHERE wallet=$1`, recipient); err != nil {
		t.Fatal(err)
	}
	check(dm, ErrDenied)
	if _, err := f.db.Exec(context.Background(), `UPDATE account_state SET deleted=false WHERE wallet=$1`, recipient); err != nil {
		t.Fatal(err)
	}
	check(dm, nil)
}

func TestCommittedPolicyReadsDirectSessionAndPoolAuthority(t *testing.T) {
	f := connected(t)
	actor, creator := f.wallets[0], f.wallets[1]
	ctx := f.ctx[actor]
	policy := *f.policy
	policy.SessionGuard = nil
	policy.AccountGuard = nil
	policy.PoolGuard = nil
	tx, err := f.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer rollback(tx)
	if err := policy.trusted(ctx, tx, actor); err != nil {
		t.Fatalf("committed direct session authority = %v", err)
	}
	if err := policy.active(ctx, tx, actor); err != nil {
		t.Fatalf("committed direct account authority = %v", err)
	}
	if err := policy.CheckCreator(ctx, tx, Request{Actor: creator, Pool: f.pool}); err != nil {
		t.Fatalf("committed direct creator authority = %v", err)
	}
	if err := policy.CheckCreator(ctx, tx, Request{Actor: actor, Pool: f.pool}); !errors.Is(err, ErrDenied) {
		t.Fatalf("noncreator direct authority = %v", err)
	}
	policy.Sessions = nil
	if err := policy.trusted(ctx, tx, actor); !errors.Is(err, ErrDependency) {
		t.Fatalf("unbound session relation = %v", err)
	}
	policy.Accounts = nil
	if err := policy.active(ctx, tx, actor); !errors.Is(err, ErrDependency) {
		t.Fatalf("unbound account relation = %v", err)
	}
	policy.Pools = nil
	if err := policy.CheckCreator(ctx, tx, Request{Actor: creator, Pool: f.pool}); !errors.Is(err, ErrDependency) {
		t.Fatalf("unbound pool relation = %v", err)
	}
}
