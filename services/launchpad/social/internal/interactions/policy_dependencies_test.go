package interactions

import (
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/fanout"
	"github.com/google/uuid"
)

func TestInteractionPolicyRefusesUnavailableAuthorityAndPrivacyRelations(t *testing.T) {
	f := connected(t)
	actor, recipient := f.wallets[0], f.wallets[1]
	ctx := f.ctx[actor]
	comment := uuid.NewString()
	mustConnected(t, f.store.CreateComment(ctx, Request{Actor: actor, Pool: f.pool, ID: comment, Body: "policy target", Key: "policy-dependency-comment"}))
	callout := uuid.NewString()
	tx, err := f.db.Begin(ctx)
	mustConnected(t, err)
	mustConnected(t, ApplyCalloutAuthorityTx(ctx, tx, callout, 1, actor, false))
	mustConnected(t, tx.Commit(ctx))
	check := func(request Request) error {
		tx, err := f.db.Begin(ctx)
		if err != nil {
			return err
		}
		defer rollback(tx)
		return f.policy.Check(ctx, tx, request)
	}
	send := func() error {
		tx, err := f.db.Begin(ctx)
		if err != nil {
			return err
		}
		defer rollback(tx)
		return f.policy.SendPolicy(ctx, tx, fanout.SendInput{Actor: actor, Target: recipient, Kind: "dm"})
	}
	for _, tc := range []struct {
		name, table string
		run         func() error
	}{
		{"session authority", "identity_sessions", func() error { return check(Request{Actor: actor, Action: followAction, ID: recipient}) }},
		{"account authority", "account_state", func() error { return check(Request{Actor: actor, Action: followAction, ID: recipient}) }},
		{"ban authority", "safety_bans", func() error { return check(Request{Actor: actor, Action: followAction, ID: recipient}) }},
		{"block authority", "interaction_blocks", func() error { return check(Request{Actor: actor, Action: followAction, ID: recipient}) }},
		{"mention mute", "interaction_mutes", func() error {
			return check(Request{Actor: actor, Action: mentionAction, Parent: recipient, ID: callout, Revision: 1})
		}},
		{"parent authority", "interaction_comments", func() error {
			return check(Request{Actor: actor, Action: commentAction, Pool: f.pool, ID: uuid.NewString(), Parent: comment})
		}},
		{"reaction target", "interaction_comments", func() error { return check(Request{Actor: actor, Action: reactionAction, ID: comment}) }},
		{"pool authority", "pools", func() error { return check(Request{Actor: actor, Action: commentAction, Pool: f.pool}) }},
		{"callout authority", "interaction_callout_authority", func() error {
			return check(Request{Actor: actor, Action: mentionAction, Parent: recipient, ID: callout, Revision: 1})
		}},
		{"direct blocks", "interaction_blocks", send},
		{"direct privacy", "interaction_preferences", send},
		{"block write", "interaction_blocks", func() error { return f.policy.SetBlock(ctx, recipient, false) }},
		{"block removal", "interaction_blocks", func() error { return f.policy.SetBlock(ctx, recipient, true) }},
		{"block preference revision", "interaction_preferences", func() error { return f.policy.SetBlock(ctx, recipient, false) }},
		{"privacy write", "interaction_preferences", func() error { return f.policy.SetPrivacy(ctx, "nobody", 1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withoutInteractionRelation(t, f, tc.table, func() {
				if err := tc.run(); err == nil {
					t.Fatal("unavailable policy relation allowed an action")
				}
			})
		})
	}
	mustConnected(t, f.policy.SetPrivacy(f.ctx[recipient], "followed_only", 1))
	withoutInteractionRelation(t, f, "interaction_follows", func() {
		if err := send(); err == nil {
			t.Fatal("followed-only policy admitted without graph")
		}
	})
	if err := check(Request{Actor: actor, Action: calloutReferenceAction, Body: `{"Mentions":["invalid"]}`}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid mention metadata = %v", err)
	}
	policy := *f.policy
	policy.Pools = nil
	tx, err = f.db.Begin(ctx)
	mustConnected(t, err)
	if err := policy.Check(ctx, tx, Request{Actor: actor, Action: commentAction, Pool: f.pool}); !errors.Is(err, ErrDependency) {
		t.Fatalf("unbound pool authority = %v", err)
	}
	mustConnected(t, tx.Rollback(ctx))
	mustConnected(t, f.policy.SetBlock(ctx, recipient, false))
	var blocks int
	mustConnected(t, f.db.QueryRow(context.Background(), `SELECT count(*) FROM interaction_blocks WHERE owner=$1 AND target=$2`, actor, recipient).Scan(&blocks))
	if blocks != 1 {
		t.Fatalf("recovered block count = %d", blocks)
	}
}

func TestInteractionBlockRollsBackGraphRemovalWithoutPublicationLedger(t *testing.T) {
	f := connected(t)
	actor, peer := f.wallets[0], f.wallets[1]
	ctx := f.ctx[actor]
	mustConnected(t, f.store.Follow(ctx, Request{Actor: actor, ID: peer, Key: "graph-before-block"}))
	for _, table := range []string{"interaction_outbox", "interaction_public_events"} {
		t.Run(table, func(t *testing.T) {
			withoutInteractionRelation(t, f, table, func() {
				if err := f.policy.SetBlock(ctx, peer, false); err == nil {
					t.Fatal("block committed without removal publication")
				}
			})
			var blocks, edges int
			mustConnected(t, f.db.QueryRow(ctx, `SELECT count(*) FROM interaction_blocks WHERE owner=$1 AND target=$2`, actor, peer).Scan(&blocks))
			mustConnected(t, f.db.QueryRow(ctx, `SELECT count(*) FROM interaction_follows WHERE follower=$1 AND followee=$2`, actor, peer).Scan(&edges))
			if blocks != 0 || edges != 1 {
				t.Fatalf("partial block state: blocks=%d follows=%d", blocks, edges)
			}
		})
	}
	mustConnected(t, f.policy.SetBlock(ctx, peer, false))
}
