package streams

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"
)

func TestConnectedOwnerActionsRecordStartEndAndRotationIntents(t *testing.T) {
	f := repair57New(t)
	ctx := context.Background()
	if _, err := f.database.Exec(ctx, f.store.RotationSchemaSQL()); err != nil {
		t.Fatal(err)
	}
	o, _ := f.create("owner-start-end")
	o, err := f.store.BindProvider(ctx, o.ID, "owner_provider")
	if err != nil {
		t.Fatal(err)
	}
	act := func(id, suffix, action string, revision int64) []byte {
		t.Helper()
		path := "/v2/streams/" + id + suffix
		body := []byte(fmt.Sprintf(`{"revision":%q}`, fmt.Sprint(revision)))
		return f.exchange(http.MethodPost, path, body, f.signed(path, action, body, f.key), http.StatusAccepted)
	}
	start := act(o.ID, "/go-live", "stream.start", o.Revision)
	if !bytes.Contains(start, []byte(`"operation"`)) {
		t.Fatalf("start operation = %s", start)
	}
	started, err := f.store.Get(ctx, o.ID)
	if err != nil || started.Intent != "start" || started.Revision != o.Revision+1 {
		t.Fatalf("start intent = %+v, %v", started, err)
	}
	act(o.ID, "/end", "stream.end", started.Revision)
	ending, err := f.store.Get(ctx, o.ID)
	if err != nil || ending.State != stateEnding || ending.Intent != endAction {
		t.Fatalf("end intent = %+v, %v", ending, err)
	}
	rotation, _ := f.create("owner-rotate")
	rotation, err = f.store.BindProvider(ctx, rotation.ID, "rotation_provider")
	if err != nil {
		t.Fatal(err)
	}
	act(rotation.ID, "/rotate-key", "stream.rotate", rotation.Revision)
	requested, err := f.store.GetKeyRotation(ctx, rotation.ID)
	if err != nil || requested.Status != "revoking" {
		t.Fatalf("signed rotation = %+v, %v", requested, err)
	}
	restore, _ := f.create("owner-restore")
	restore, err = f.store.BindProvider(ctx, restore.ID, "restore_provider")
	if err != nil {
		t.Fatal(err)
	}
	restore, err = f.store.Suspend(ctx, restore.ID, SuspensionDecision{OperationID: restore.ID, Revision: restore.Revision, Actor: "policy_reconciler", Reason: "active_restriction", Allowed: true, ExpiresAt: time.Now().Add(time.Minute)})
	if err != nil {
		t.Fatal(err)
	}
	act(restore.ID, "/restore", "stream.restore", restore.Revision)
	restored, err := f.store.GetKeyRotation(ctx, restore.ID)
	if err != nil || restored.Status != "revoking" {
		t.Fatalf("signed restoration = %+v, %v", restored, err)
	}
}
