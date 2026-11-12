package streams

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
)

func TestLifecycleTransitionsRequireCurrentRevisionAndDurableIntent(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	o, intent := f.create("lifecycle-revision")
	decision := func(o Operation, action string) ActionDecision {
		return ActionDecision{Wallet: f.wallet, OperationID: o.ID, Action: action, Revision: o.Revision, Allowed: true, ExpiresAt: time.Now().Add(time.Minute)}
	}
	if _, err := f.store.Transition(ctx, o.ID, f.wallet, "start", o.Revision, decision(o, "start")); !errors.Is(err, ErrConflict) {
		t.Fatalf("start before provider binding = %v", err)
	}
	o, err := f.store.BindProvider(ctx, o.ID, "transition_provider")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.store.Transition(ctx, o.ID, f.wallet, "other", o.Revision, decision(o, "other")); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unsupported intent = %v", err)
	}
	if _, err := f.store.Transition(ctx, o.ID, f.wallet, "start", o.Revision+1, decision(o, "start")); !errors.Is(err, ErrDenied) {
		t.Fatalf("unbound revision = %v", err)
	}
	started, err := f.store.Transition(ctx, o.ID, f.wallet, "start", o.Revision, decision(o, "start"))
	if err != nil || started.Intent != "start" || started.State != stateReady {
		t.Fatalf("start intent = %+v, %v", started, err)
	}
	if _, err := f.store.Transition(ctx, o.ID, f.wallet, "start", o.Revision, decision(o, "start")); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale start = %v", err)
	}
	ended, err := f.store.Transition(ctx, o.ID, f.wallet, endAction, started.Revision, decision(started, endAction))
	if err != nil || ended.State != stateEnding {
		t.Fatalf("end intent = %+v, %v", ended, err)
	}
	rows, err := f.store.List(ctx, intent.Pool, true)
	if err != nil || len(rows) != 0 {
		t.Fatalf("live list after end intent = %+v, %v", rows, err)
	}
	rows, err = f.store.List(ctx, intent.Pool, false)
	if err != nil || len(rows) != 1 || rows[0].ID != o.ID {
		t.Fatalf("pool list after end intent = %+v, %v", rows, err)
	}
	if _, err := f.store.List(ctx, "invalid", false); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unbound pool list = %v", err)
	}
}

func TestSuspensionDecisionBindsOwnerRevisionAndExpiry(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	o, _ := f.create("suspension-decision")
	decision := SuspensionDecision{OperationID: o.ID, Revision: o.Revision, Actor: "policy_reconciler", Reason: "active_restriction", Allowed: true, ExpiresAt: time.Now().Add(time.Minute)}
	for _, changed := range []SuspensionDecision{
		{OperationID: o.ID, Revision: o.Revision, Actor: "policy_reconciler", Reason: "active_restriction", ExpiresAt: decision.ExpiresAt},
		{OperationID: "different", Revision: o.Revision, Actor: decision.Actor, Reason: decision.Reason, Allowed: true, ExpiresAt: decision.ExpiresAt},
		{OperationID: o.ID, Revision: o.Revision, Actor: "invalid actor", Reason: decision.Reason, Allowed: true, ExpiresAt: decision.ExpiresAt},
		{OperationID: o.ID, Revision: o.Revision, Actor: decision.Actor, Reason: decision.Reason, Allowed: true, ExpiresAt: time.Now().Add(-time.Second)},
	} {
		if _, err := f.store.Suspend(ctx, o.ID, changed); !errors.Is(err, ErrDenied) {
			t.Fatalf("unbound suspension = %+v, %v", changed, err)
		}
	}
	stale := decision
	stale.Revision++
	if _, err := f.store.Suspend(ctx, o.ID, stale); !errors.Is(err, ErrConflict) {
		t.Fatalf("stale suspension = %v", err)
	}
	suspended, err := f.store.Suspend(ctx, o.ID, decision)
	if err != nil || suspended.State != stateSuspended || suspended.Intent != endAction {
		t.Fatalf("durable suspension = %+v, %v", suspended, err)
	}
	if _, err := f.store.Suspend(ctx, o.ID, decision); !errors.Is(err, ErrConflict) {
		t.Fatalf("replayed suspension = %v", err)
	}
}

func TestOwnerEditAndRotationRequireCurrentStoredState(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	if _, err := f.database.Exec(ctx, f.store.RotationSchemaSQL()); err != nil {
		t.Fatal(err)
	}
	o, _ := f.create("owner-edit-state")
	tx, err := f.database.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer rollbackBounded(tx)
	for _, tc := range []struct {
		id, wallet, title string
		revision          int64
		want              error
	}{
		{o.ID, f.wallet, "", o.Revision, ErrInvalid},
		{"11111111-1111-4111-8111-111111111111", f.wallet, "Updated", o.Revision, pgx.ErrNoRows},
		{o.ID, f.wallet, "Updated", o.Revision + 1, ErrConflict},
		{o.ID, "0x1111111111111111111111111111111111111111", "Updated", o.Revision, ErrDenied},
	} {
		_, err := f.store.EditTx(ctx, tx, tc.id, tc.wallet, tc.title, tc.revision)
		if !errors.Is(err, tc.want) {
			t.Fatalf("unbound edit = %v, want %v", err, tc.want)
		}
	}
	if _, err := f.store.RequestRotationTx(ctx, tx, o.ID, f.wallet, o.Revision); !errors.Is(err, ErrConflict) {
		t.Fatalf("rotation before provider readiness = %v", err)
	}
	changed, err := f.store.EditTx(ctx, tx, o.ID, f.wallet, "Updated", o.Revision)
	if err != nil || changed.Title != "Updated" || changed.Revision != o.Revision+1 {
		t.Fatalf("owner edit = %+v, %v", changed, err)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	stored, err := f.store.Get(ctx, o.ID)
	if err != nil || stored.Title != "Updated" {
		t.Fatalf("stored title = %+v, %v", stored, err)
	}
}
