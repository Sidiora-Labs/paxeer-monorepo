package voice

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestRecordingReconcilerKeepsCommittedStartAndStopIntent(t *testing.T) {
	control, ctx := fixture(t)
	control.Provider = localLiveKit(t)
	control.Gates = Gates{true, true, true, true}
	control.RecordingPolicy = RecordingPolicy{Enabled: true, Version: "consent-v2", Purpose: "Community archive", Retention: time.Hour, RestrictedStorageQualified: true}
	room, err := control.Create(ctx, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	live, err := control.Start(ctx, room.ID, room.Revision)
	if err != nil {
		t.Fatal(err)
	}
	recordingID := uuid.NewString()
	if _, err := control.DB.Exec(ctx, `INSERT INTO voice_recordings(id,room_id,state,purpose,policy_version,delete_at) VALUES($1,$2,'starting',$3,$4,$5)`, recordingID, room.ID, control.RecordingPolicy.Purpose, control.RecordingPolicy.Version, time.Now().Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	result, err := control.Reconcile(ctx)
	if !errors.Is(err, ErrUnavailable) || result.Failed != 1 {
		t.Fatalf("recording start reconciliation = %+v: %v", result, err)
	}
	var state string
	if err := control.DB.QueryRow(ctx, `SELECT state FROM voice_recordings WHERE id=$1`, recordingID).Scan(&state); err != nil || state != "starting" {
		t.Fatalf("committed recording start = %q: %v", state, err)
	}
	if _, err := control.DB.Exec(ctx, `UPDATE voice_recordings SET state='stopping' WHERE id=$1`, recordingID); err != nil {
		t.Fatal(err)
	}
	restarted := &Control{DB: control.DB, Provider: control.Provider, Gates: control.Gates, RecordingPolicy: control.RecordingPolicy}
	result, err = restarted.Reconcile(ctx)
	var providerFailure providerError
	if !errors.As(err, &providerFailure) || providerFailure.Code != "internal" || result.Failed != 1 {
		t.Fatalf("recording stop reconciliation = %+v: %v", result, err)
	}
	if err := control.DB.QueryRow(ctx, `SELECT state FROM voice_recordings WHERE id=$1`, recordingID).Scan(&state); err != nil || state != "stopping" {
		t.Fatalf("committed recording stop = %q: %v", state, err)
	}
	if err := control.Provider.End(ctx, live.ProviderSession); err != nil {
		t.Fatal(err)
	}
}

func TestReconcilerRequiresProviderDiscoveryAndObservesCancellation(t *testing.T) {
	control, _ := fixture(t)
	if _, err := control.Reconcile(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("reconciler without provider = %v", err)
	}
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()
	observed := false
	if err := control.RunReconciler(cancelled, func(_ ReconcileResult, outcome error) {
		observed = true
		if !errors.Is(outcome, ErrUnavailable) {
			t.Fatalf("cancelled reconciliation result = %v", outcome)
		}
	}); !errors.Is(err, context.Canceled) || !observed {
		t.Fatalf("reconciler cancellation = %v, observed %t", err, observed)
	}
}
