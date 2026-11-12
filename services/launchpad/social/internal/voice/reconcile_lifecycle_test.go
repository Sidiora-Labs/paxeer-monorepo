package voice

import (
	"context"
	"testing"
	"time"
)

func TestReconcilerAdoptsAndTerminatesCommittedRoomLifecycle(t *testing.T) {
	control, ctx := fixture(t)
	control.Provider = localLiveKit(t)
	control.Gates = Gates{true, true, true, true}
	adopt, err := control.Create(ctx, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	session, err := control.Provider.Start(ctx, adopt.StartKey)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := control.DB.Exec(ctx, `UPDATE voice_rooms SET updated_at=$2 WHERE id=$1`, adopt.ID, time.Now().Add(-6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	result, err := control.Reconcile(ctx)
	if err != nil || result.Adopted != 1 || result.Orphans != 1 {
		t.Fatalf("adopted room = %+v: %v", result, err)
	}
	stored, err := control.Read(ctx, adopt.ID)
	if err != nil || stored.State != liveState || stored.ProviderSession != session {
		t.Fatalf("committed adoption = %+v: %v", stored, err)
	}
	if _, err := control.DB.Exec(ctx, `UPDATE voice_rooms SET state='ending',updated_at=$2 WHERE id=$1`, adopt.ID, time.Now().Add(-6*time.Minute)); err != nil {
		t.Fatal(err)
	}
	result, err = control.Reconcile(ctx)
	if err != nil || result.Terminated != 1 || result.Orphans != 1 {
		t.Fatalf("ended room = %+v: %v", result, err)
	}
	stored, err = control.Read(ctx, adopt.ID)
	if err != nil || stored.State != "ended" {
		t.Fatalf("committed end = %+v: %v", stored, err)
	}
	if _, found, err := control.Provider.(SessionFinder).Find(context.Background(), adopt.StartKey); err != nil || found {
		t.Fatalf("provider session after end = %t: %v", found, err)
	}
}

func TestReconcilerTerminatesProviderRoomWithoutQualifiedAdmission(t *testing.T) {
	control, ctx := fixture(t)
	control.Provider = localLiveKit(t)
	room, err := control.Create(ctx, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := control.Provider.Start(ctx, room.StartKey); err != nil {
		t.Fatal(err)
	}
	result, err := control.Reconcile(ctx)
	if err != nil || result.Terminated != 1 || result.Adopted != 0 {
		t.Fatalf("unqualified room = %+v: %v", result, err)
	}
	stored, err := control.Read(ctx, room.ID)
	if err != nil || stored.State != draftState {
		t.Fatalf("committed room intent = %+v: %v", stored, err)
	}
	if _, found, err := control.Provider.(SessionFinder).Find(ctx, room.StartKey); err != nil || found {
		t.Fatalf("unqualified provider session = %t: %v", found, err)
	}
}
