package interactions

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestErasureRetainsHeldAccountThenCompletesCommittedCleanup(t *testing.T) {
	f := connected(t)
	actor, peer := f.wallets[0], f.wallets[1]
	ctx := f.ctx[actor]
	if err := f.store.Follow(ctx, Request{Actor: actor, ID: peer, Key: "held-erasure-follow"}); err != nil {
		t.Fatal(err)
	}
	commentID := uuid.NewString()
	if err := f.store.CreateComment(ctx, Request{Actor: actor, Pool: f.pool, ID: commentID, Body: "community message", Key: "held-erasure-comment"}); err != nil {
		t.Fatal(err)
	}
	caseID := uuid.NewString()
	hold := time.Now().UTC().Add(time.Hour)
	tx, err := f.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.store.BeginErasureTx(ctx, tx, ErasureCommand{CaseID: caseID, Wallet: actor, PolicyVersion: "retention-v2", HoldUntil: &hold}); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var receipt ErasureReceipt
	for i := 0; i < 20; i++ {
		receipt, err = f.store.EraseBatch(ctx, caseID)
		if err != nil {
			t.Fatal(err)
		}
		if receipt.State == "retained" {
			break
		}
	}
	if receipt.State != "retained" || receipt.CompletedAt != nil || receipt.RowsProcessed < 2 {
		t.Fatalf("held erasure receipt = %+v", receipt)
	}
	var follows int
	var body string
	var removed bool
	if err := f.db.QueryRow(ctx, `SELECT count(*) FROM interaction_follows WHERE follower=$1 AND followee=$2`, actor, peer).Scan(&follows); err != nil {
		t.Fatal(err)
	}
	if err := f.db.QueryRow(ctx, `SELECT body,removed FROM interaction_comments WHERE id=$1`, commentID).Scan(&body, &removed); err != nil {
		t.Fatal(err)
	}
	if follows != 0 || body != "" || !removed {
		t.Fatalf("committed erasure cleanup = follows %d, body %q, removed %t", follows, body, removed)
	}
	if _, err := f.db.Exec(ctx, `UPDATE interaction_erasure_jobs SET hold_until=$2 WHERE deletion_case_id=$1`, caseID, time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 20; i++ {
		receipt, err = f.store.EraseBatch(context.Background(), caseID)
		if err != nil {
			t.Fatal(err)
		}
		if receipt.State == "completed" {
			break
		}
	}
	if receipt.State != "completed" || receipt.CompletedAt == nil || receipt.RowsProcessed < 2 {
		t.Fatalf("completed erasure receipt = %+v", receipt)
	}
}
