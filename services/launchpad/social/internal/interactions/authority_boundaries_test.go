package interactions

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
)

func TestCommittedCalloutAuthorityAndReceiptVerification(t *testing.T) {
	f := connected(t)
	actor, other := f.wallets[0], f.wallets[1]
	ctx := f.ctx[actor]
	tx, err := f.db.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer rollback(tx)
	for _, tc := range []struct {
		id       string
		revision int64
		actor    string
	}{
		{"", 1, actor},
		{uuid.NewString(), 0, actor},
		{uuid.NewString(), 1, "invalid"},
		{strings.Repeat("x", 161), 1, actor},
	} {
		if err := ApplyCalloutAuthorityTx(ctx, tx, tc.id, tc.revision, tc.actor, false); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid callout authority = %v", err)
		}
	}
	id := uuid.NewString()
	if err := ApplyCalloutAuthorityTx(ctx, tx, id, 1, actor, false); err != nil {
		t.Fatal(err)
	}
	if err := ApplyCalloutAuthorityTx(ctx, tx, id, 1, actor, false); err != nil {
		t.Fatalf("idempotent callout authority = %v", err)
	}
	if err := ApplyCalloutAuthorityTx(ctx, tx, id, 1, other, false); !errors.Is(err, ErrRevision) {
		t.Fatalf("different callout author = %v", err)
	}
	if err := ApplyCalloutAuthorityTx(ctx, tx, id, 1, actor, true); !errors.Is(err, ErrRevision) {
		t.Fatalf("different removal state = %v", err)
	}
	if err := f.policy.CheckCreator(ctx, tx, Request{Actor: actor, Pool: f.pool}); !errors.Is(err, ErrDenied) {
		t.Fatalf("noncreator pool authority = %v", err)
	}
	if err := f.policy.CheckCreator(ctx, tx, Request{Actor: other, Pool: f.pool}); err != nil {
		t.Fatalf("committed pool creator = %v", err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	verifier := CommittedReceiptVerifier{DB: f.db}
	if err := verifier.Verify(ctx, "invalid", 125); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid receipt hash = %v", err)
	}
	if err := verifier.Verify(ctx, "0x"+strings.Repeat("a", 64), 1); !errors.Is(err, ErrInvalid) {
		t.Fatalf("wrong chain receipt = %v", err)
	}
	if err := verifier.Verify(ctx, "0x"+strings.Repeat("a", 64), 125); !errors.Is(err, ErrDependency) {
		t.Fatalf("unbound receipt tables = %v", err)
	}
	if err := (&PGPolicy{}).ValidateSession(context.Background(), actor, uuid.NewString()); !errors.Is(err, ErrDependency) {
		t.Fatalf("unbound session authority = %v", err)
	}
	if err := f.policy.ValidateSession(ctx, actor, "not-a-uuid"); err == nil {
		t.Fatal("invalid session identifier accepted")
	}
}
