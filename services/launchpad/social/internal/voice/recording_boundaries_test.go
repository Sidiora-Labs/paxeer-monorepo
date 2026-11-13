package voice

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/google/uuid"
)

func TestRecordingRequiresCurrentParticipantConsentAndHostAuthority(t *testing.T) {
	c, hostContext := fixture(t)
	c.Provider = localLiveKit(t)
	c.Gates = Gates{true, true, true, true}
	c.RecordingPolicy = RecordingPolicy{Enabled: true, Version: "consent-v2", Purpose: "Community session archive", Retention: time.Hour, RestrictedStorageQualified: true}
	guestWallet := "0x2222222222222222222222222222222222222222"
	guestContext := safety.WithPrincipal(context.Background(), safety.Principal{Wallet: guestWallet, SessionID: "recording-guest", Expires: time.Now().Add(time.Hour)})
	strangerContext := safety.WithPrincipal(context.Background(), safety.Principal{Wallet: "0x3333333333333333333333333333333333333333", SessionID: "recording-stranger", Expires: time.Now().Add(time.Hour)})
	room, err := c.Create(hostContext, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.RequestRecording(hostContext, room.ID); !errors.Is(err, safety.ErrConflict) {
		t.Fatalf("draft recording request = %v", err)
	}
	live, err := c.Start(hostContext, room.ID, room.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.RequestRecording(guestContext, room.ID); !errors.Is(err, safety.ErrForbidden) {
		t.Fatalf("guest recording request = %v", err)
	}
	if err := c.Join(hostContext, room.ID); err != nil {
		t.Fatal(err)
	}
	if err := c.Join(guestContext, room.ID); err != nil {
		t.Fatal(err)
	}
	requested, err := c.RequestRecording(hostContext, room.ID)
	if err != nil || requested.State != "consent_pending" {
		t.Fatalf("recording request = %+v: %v", requested, err)
	}
	duplicate, err := c.RequestRecording(hostContext, room.ID)
	if err != nil || duplicate.ID != requested.ID {
		t.Fatalf("idempotent recording request = %+v: %v", duplicate, err)
	}
	if _, err := c.Recording(strangerContext, room.ID); !errors.Is(err, safety.ErrForbidden) {
		t.Fatalf("unadmitted recording read = %v", err)
	}
	if err := c.Consent(strangerContext, room.ID, requested.ID, requested.Revision, true); !errors.Is(err, safety.ErrForbidden) {
		t.Fatalf("unadmitted consent = %v", err)
	}
	if err := c.Consent(guestContext, room.ID, requested.ID, requested.Revision+1, true); !errors.Is(err, safety.ErrConflict) {
		t.Fatalf("stale consent = %v", err)
	}
	if _, err := c.StartConsentedRecording(guestContext, room.ID, requested.Revision); !errors.Is(err, safety.ErrForbidden) {
		t.Fatalf("guest recording start = %v", err)
	}
	if err := c.Consent(hostContext, room.ID, requested.ID, requested.Revision, true); err != nil {
		t.Fatal(err)
	}
	if _, err := c.StartConsentedRecording(hostContext, room.ID, requested.Revision); !errors.Is(err, safety.ErrForbidden) {
		t.Fatalf("missing guest consent = %v", err)
	}
	if err := c.Consent(guestContext, room.ID, requested.ID, requested.Revision, true); err != nil {
		t.Fatal(err)
	}
	if _, err := c.StartConsentedRecording(hostContext, room.ID, requested.Revision); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("recording without restricted provider output = %v", err)
	}
	starting, err := c.Recording(hostContext, room.ID)
	if err != nil || starting.State != startingState || starting.Revision != requested.Revision+1 {
		t.Fatalf("durable recording start = %+v: %v", starting, err)
	}
	if _, err := c.StartConsentedRecording(hostContext, room.ID, starting.Revision); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("replayed recording start without output = %v", err)
	}
	if err := c.JoinWithConsent(strangerContext, room.ID, &JoinConsent{RecordingID: requested.ID, PolicyVersion: "consent-v2", Purpose: c.RecordingPolicy.Purpose, Agree: false}); !errors.Is(err, safety.ErrForbidden) {
		t.Fatalf("declined admission = %v", err)
	}
	if err := c.JoinWithConsent(strangerContext, room.ID, &JoinConsent{RecordingID: uuid.NewString(), PolicyVersion: "consent-v2", Purpose: c.RecordingPolicy.Purpose, Agree: true}); !errors.Is(err, safety.ErrConflict) {
		t.Fatalf("different recording consent = %v", err)
	}
	if _, err := c.StopConsentedRecording(guestContext, room.ID, requested.Revision); !errors.Is(err, safety.ErrForbidden) {
		t.Fatalf("guest recording stop = %v", err)
	}
	if _, err := c.StopConsentedRecording(hostContext, room.ID, requested.Revision); !errors.Is(err, safety.ErrConflict) {
		t.Fatalf("stale recording stop = %v", err)
	}
	if _, err := c.StopConsentedRecording(hostContext, room.ID, starting.Revision); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("stop before provider recording exists = %v", err)
	}
	stopping, err := c.Recording(hostContext, room.ID)
	if err != nil || stopping.State != "stopping" || stopping.Revision != starting.Revision+1 {
		t.Fatalf("durable recording stop = %+v: %v", stopping, err)
	}
	if _, err := c.End(hostContext, room.ID, live.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DB.Exec(hostContext, `UPDATE voice_recordings SET state='starting' WHERE id=$1`, requested.ID); err != nil {
		t.Fatal(err)
	}
	actor, err := safety.Actor(hostContext, c.now())
	if err != nil {
		t.Fatal(err)
	}
	if err := c.recordingTransition(hostContext, requested.ID, room.ID, stopping.Revision, startingState, recordingState, "provider-recording", actor); !errors.Is(err, safety.ErrConflict) {
		t.Fatalf("provider start after room end = %v", err)
	}
	if err := c.DB.QueryRow(hostContext, `SELECT state FROM voice_recordings WHERE id=$1`, requested.ID).Scan(&stopping.State); err != nil || stopping.State != "stopping" {
		t.Fatalf("interrupted recording state = %q: %v", stopping.State, err)
	}
	if err := c.recordingTransition(hostContext, requested.ID, room.ID, stopping.Revision, "stopping", "stopped", "provider-recording", actor); !errors.Is(err, safety.ErrConflict) {
		t.Fatalf("stale recording transition = %v", err)
	}
	if err := c.Consent(guestContext, room.ID, requested.ID, requested.Revision, true); !errors.Is(err, safety.ErrConflict) {
		t.Fatalf("ended room consent = %v", err)
	}
}
