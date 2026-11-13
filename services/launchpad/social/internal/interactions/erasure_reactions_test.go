package interactions

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestErasureRemovesCommittedReactionsAndKeepsCaseIdentity(t *testing.T) {
	f := connected(t)
	actor := f.wallets[0]
	ctx := f.ctx[actor]
	commentID := uuid.NewString()
	if err := f.store.CreateComment(ctx, Request{Actor: actor, Pool: f.pool, ID: commentID, Body: "reaction target", Key: "erasure-reaction-comment"}); err != nil {
		t.Fatal(err)
	}
	if err := f.store.React(ctx, Request{Actor: actor, ID: commentID, Kind: "like", Key: "erasure-reaction"}); err != nil {
		t.Fatal(err)
	}
	caseID := uuid.NewString()
	command := ErasureCommand{CaseID: caseID, Wallet: actor, PolicyVersion: "retention-v2"}
	tx, err := f.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = f.store.BeginErasureTx(ctx, tx, command); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	conflict, err := f.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer rollback(conflict)
	if err = f.store.BeginErasureTx(ctx, conflict, ErasureCommand{CaseID: caseID, Wallet: actor, PolicyVersion: "different-retention"}); !errors.Is(err, ErrRevision) {
		t.Fatalf("changed erasure command = %v", err)
	}
	if err = conflict.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	removed := false
	for i := 0; i < 16; i++ {
		receipt, batchErr := f.store.EraseBatch(ctx, caseID)
		if batchErr != nil {
			t.Fatal(batchErr)
		}
		var count int
		if err := f.db.QueryRow(ctx, `SELECT count(*) FROM interaction_reactions WHERE actor=$1 AND target=$2`, actor, commentID).Scan(&count); err != nil {
			t.Fatal(err)
		}
		if count == 0 {
			removed = true
		}
		if receipt.State == "completed" {
			break
		}
	}
	if !removed {
		t.Fatal("erasure retained the committed reaction")
	}
	for i := 0; i < 16; i++ {
		receipt, err := f.store.EraseBatch(ctx, caseID)
		if err != nil {
			t.Fatal(err)
		}
		if receipt.State == "completed" {
			break
		}
	}
	completed, err := f.store.EraseBatch(ctx, caseID)
	if err != nil || completed.State != "completed" || completed.CompletedAt == nil {
		t.Fatalf("idempotent erasure receipt = %+v: %v", completed, err)
	}
	stored, err := f.store.ErasureReceipt(ctx, caseID)
	if err != nil || stored.State != "completed" || stored.CaseID != caseID {
		t.Fatalf("stored erasure receipt = %+v: %v", stored, err)
	}
}

func TestErasureRejectsInvalidCasesAndUnboundStorage(t *testing.T) {
	f := connected(t)
	ctx := f.ctx[f.wallets[0]]
	for _, command := range []ErasureCommand{
		{CaseID: "invalid", Wallet: f.wallets[0], PolicyVersion: "v2"},
		{CaseID: uuid.NewString(), Wallet: "invalid", PolicyVersion: "v2"},
		{CaseID: uuid.NewString(), Wallet: f.wallets[0]},
	} {
		if err := command.validate(); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid erasure command = %v", err)
		}
	}
	if _, err := f.store.EraseBatch(ctx, "invalid"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid erasure case = %v", err)
	}
	if _, err := f.store.EraseBatch(ctx, uuid.NewString()); err == nil {
		t.Fatal("unknown erasure case accepted")
	}
	if _, err := f.store.ErasureReceipt(ctx, uuid.NewString()); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unknown receipt = %v", err)
	}
	if _, err := (Store{}).EraseBatch(context.Background(), uuid.NewString()); !errors.Is(err, ErrDependency) {
		t.Fatalf("unbound erasure batch = %v", err)
	}
	if _, err := (Store{}).ErasureReceipt(context.Background(), uuid.NewString()); !errors.Is(err, ErrDependency) {
		t.Fatalf("unbound erasure receipt = %v", err)
	}
}
