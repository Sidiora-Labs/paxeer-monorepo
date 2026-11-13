package interactions

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func withoutInteractionRelation(t *testing.T, f *connectedFixture, table string, run func()) {
	t.Helper()
	name := pgx.Identifier{table}.Sanitize()
	held := pgx.Identifier{table + "_unavailable"}.Sanitize()
	if _, err := f.db.Exec(context.Background(), "ALTER TABLE "+name+" RENAME TO "+held); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := f.db.Exec(context.Background(), "ALTER TABLE "+held+" RENAME TO "+name); err != nil {
			t.Fatal(err)
		}
	}()
	run()
}

func TestInteractionMutationsPreserveStateWhenPersistenceIsUnavailable(t *testing.T) {
	f := connected(t)
	actor, creator, peer := f.wallets[0], f.wallets[1], f.wallets[2]
	ctx := f.ctx[actor]
	comment := uuid.NewString()
	mustConnected(t, f.store.CreateComment(ctx, Request{Actor: actor, ID: comment, Pool: f.pool, Body: "original message", Key: "dependency-comment"}))
	mustConnected(t, f.store.Follow(ctx, Request{Actor: actor, ID: peer, Key: "dependency-follow"}))
	mustConnected(t, f.store.React(f.ctx[creator], Request{Actor: creator, ID: comment, Kind: "like", Key: "dependency-reaction"}))
	callout := uuid.NewString()
	tx, err := f.db.Begin(ctx)
	mustConnected(t, err)
	mustConnected(t, ApplyCalloutAuthorityTx(ctx, tx, callout, 1, actor, false))
	mustConnected(t, tx.Commit(ctx))
	serial := 0
	operations := []struct {
		name   string
		tables []string
		run    func(string) error
	}{
		{"follow", []string{"interaction_mutations", "interaction_follows", "interaction_aggregate_revisions", "interaction_outbox", "interaction_public_events", "interaction_visibility_revision", "delivery_outbox"}, func(key string) error { return f.store.Follow(ctx, Request{Actor: actor, ID: creator, Key: key}) }},
		{"unfollow", []string{"interaction_follows", "interaction_outbox"}, func(key string) error {
			return f.store.Follow(ctx, Request{Actor: actor, ID: peer, Remove: true, Key: key})
		}},
		{"comment", []string{"interaction_comments", "interaction_outbox"}, func(key string) error {
			return f.store.CreateComment(ctx, Request{Actor: actor, ID: uuid.NewString(), Pool: f.pool, Body: "new message", Key: key})
		}},
		{"generated comment", []string{"interaction_mutations"}, func(key string) error {
			return f.store.CreateComment(ctx, Request{Actor: actor, Pool: f.pool, Body: "new message", Key: key})
		}},
		{"edit", []string{"interaction_comments", "interaction_outbox"}, func(key string) error {
			return f.store.EditComment(ctx, Request{Actor: actor, ID: comment, Pool: f.pool, Body: "edited message", Revision: 1, Key: key})
		}},
		{"hide", []string{"interaction_comment_moderation", "interaction_outbox"}, func(key string) error {
			return f.store.HideComment(f.ctx[creator], Request{Actor: creator, ID: comment, Pool: f.pool, Body: "reviewed conduct", Revision: 1, Key: key})
		}},
		{"react", []string{"interaction_reactions", "interaction_outbox"}, func(key string) error {
			return f.store.React(ctx, Request{Actor: actor, ID: comment, Kind: "like", Key: key})
		}},
		{"unreact", []string{"interaction_reactions", "interaction_outbox"}, func(key string) error {
			return f.store.React(f.ctx[creator], Request{Actor: creator, ID: comment, Kind: "like", Remove: true, Key: key})
		}},
		{"reference", []string{"interaction_callout_refs", "interaction_notification_intents", "interaction_outbox"}, func(key string) error {
			return f.store.ReferenceCallout(ctx, Request{Actor: actor, ID: callout, Revision: 1, Key: key}, []string{creator}, "", nil)
		}},
	}
	for _, op := range operations {
		for _, table := range op.tables {
			t.Run(op.name+"/"+table, func(t *testing.T) {
				serial++
				key := fmt.Sprintf("persistence-%d", serial)
				withoutInteractionRelation(t, f, table, func() {
					err := op.run(key)
					var postgres *pgconn.PgError
					if !errors.As(err, &postgres) || postgres.Code != "42P01" {
						t.Fatalf("missing relation error = %v", err)
					}
				})
				var mutations int
				mustConnected(t, f.db.QueryRow(ctx, `SELECT count(*) FROM interaction_mutations WHERE key=$1`, key).Scan(&mutations))
				var body string
				var removed bool
				var revision int64
				mustConnected(t, f.db.QueryRow(ctx, `SELECT body,removed,revision FROM interaction_comments WHERE id=$1`, comment).Scan(&body, &removed, &revision))
				var follows, reactions, references int
				mustConnected(t, f.db.QueryRow(ctx, `SELECT count(*) FROM interaction_follows WHERE follower=$1`, actor).Scan(&follows))
				mustConnected(t, f.db.QueryRow(ctx, `SELECT count(*) FROM interaction_reactions WHERE target=$1`, comment).Scan(&reactions))
				mustConnected(t, f.db.QueryRow(ctx, `SELECT count(*) FROM interaction_callout_refs WHERE id=$1`, callout).Scan(&references))
				if mutations != 0 || body != "original message" || removed || revision != 1 || follows != 1 || reactions != 1 || references != 0 {
					t.Fatalf("mutation committed partial state: mutations=%d body=%q removed=%t revision=%d follows=%d reactions=%d references=%d", mutations, body, removed, revision, follows, reactions, references)
				}
			})
		}
	}
	mustConnected(t, f.store.Follow(ctx, Request{Actor: actor, ID: creator, Key: "restored-follow"}))
}

func TestInteractionReadsReturnUnavailableRelationErrors(t *testing.T) {
	f := connected(t)
	actor := f.wallets[0]
	ctx := f.ctx[actor]
	for _, tc := range []struct {
		name, table string
		run         func() error
	}{
		{"follow overlay", "interaction_erased_wallets", func() error { _, err := f.store.Follows(ctx, actor, true, 10, ""); return err }},
		{"follow rows", "interaction_follows", func() error { _, err := f.store.Follows(ctx, actor, true, 10, ""); return err }},
		{"stats overlay", "interaction_erased_wallets", func() error { _, _, err := f.store.FollowStats(ctx, actor, "", nil); return err }},
		{"stats revision", "interaction_visibility_revision", func() error { _, _, err := f.store.FollowStats(ctx, actor, "", nil); return err }},
		{"stats rows", "interaction_follows", func() error { _, _, err := f.store.FollowStats(ctx, actor, "", nil); return err }},
		{"replay", "interaction_public_events", func() error { _, err := f.store.PruneReplay(ctx, 10); return err }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withoutInteractionRelation(t, f, tc.table, func() {
				var postgres *pgconn.PgError
				err := tc.run()
				if !errors.As(err, &postgres) || postgres.Code != "42P01" {
					t.Fatalf("read dependency = %v", err)
				}
			})
		})
	}
}

func TestErasureKeepsCheckpointWhenStageRelationsAreUnavailable(t *testing.T) {
	f := connected(t)
	actor := f.wallets[0]
	ctx := f.ctx[actor]
	caseID := uuid.NewString()
	tx, err := f.db.Begin(ctx)
	mustConnected(t, err)
	mustConnected(t, f.store.BeginErasureTx(ctx, tx, ErasureCommand{CaseID: caseID, Wallet: actor, PolicyVersion: "retention-v2"}))
	mustConnected(t, tx.Commit(ctx))
	for stage, table := range []string{"interaction_follows", "interaction_comments", "interaction_reactions", "interaction_notification_intents", "interaction_callout_refs", "interaction_mutations", "interaction_blocks", "interaction_mutes", "interaction_feed_projection", "interaction_preferences"} {
		t.Run(table, func(t *testing.T) {
			_, err := f.db.Exec(ctx, `UPDATE interaction_erasure_jobs SET stage=$2 WHERE deletion_case_id=$1`, caseID, stage)
			mustConnected(t, err)
			withoutInteractionRelation(t, f, table, func() {
				_, err := f.store.EraseBatch(ctx, caseID)
				var postgres *pgconn.PgError
				if !errors.As(err, &postgres) || postgres.Code != "42P01" {
					t.Fatalf("erasure dependency = %v", err)
				}
			})
			var stored int
			var count int64
			var state string
			mustConnected(t, f.db.QueryRow(ctx, `SELECT stage,rows_processed,state FROM interaction_erasure_jobs WHERE deletion_case_id=$1`, caseID).Scan(&stored, &count, &state))
			if stored != stage || count != 0 || state != "running" {
				t.Fatalf("erasure checkpoint = %d, %d, %s", stored, count, state)
			}
		})
	}
	f.db.Close()
	if _, err := f.store.EraseBatch(ctx, caseID); err == nil {
		t.Fatal("closed database accepted erasure")
	}
}

func TestInteractionTransactionHelpersRejectClosedTransactions(t *testing.T) {
	f := connected(t)
	actor := f.wallets[0]
	ctx := f.ctx[actor]
	tx, err := f.db.Begin(ctx)
	mustConnected(t, err)
	mustConnected(t, tx.Rollback(ctx))
	request := Request{Actor: actor, ID: f.wallets[1], Action: followAction}
	checks := []struct {
		name string
		run  func() error
	}{
		{"authority", func() error { return f.policy.Check(ctx, tx, request) }},
		{"active", func() error { return f.policy.active(ctx, tx, actor) }},
		{"creator", func() error { return f.policy.CheckCreator(ctx, tx, Request{Actor: actor, Pool: f.pool}) }},
		{"policy lock", func() error { return policyLocks(ctx, tx, actor) }},
		{"change", func() error { return recordChange(ctx, tx, request) }},
		{"callout authority", func() error { return ApplyCalloutAuthorityTx(ctx, tx, uuid.NewString(), 1, actor, false) }},
		{"erasure", func() error {
			return f.store.BeginErasureTx(ctx, tx, ErasureCommand{CaseID: uuid.NewString(), Wallet: actor, PolicyVersion: "retention-v2"})
		}},
		{"snapshot", func() error { _, err := publicSnapshot(ctx, tx, request); return err }},
		{"publication", func() error { return f.policy.Publisher.Record(ctx, tx, request) }},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); !errors.Is(err, pgx.ErrTxClosed) {
				t.Fatalf("closed transaction = %v", err)
			}
		})
	}
}
