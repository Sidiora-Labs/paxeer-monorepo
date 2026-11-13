package voice

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestVoiceActionsReportUnavailableCommittedRelations(t *testing.T) {
	setup := func(t *testing.T) (*Control, context.Context, Room, Recording) {
		t.Helper()
		c, ctx := fixture(t)
		c.Provider = localLiveKit(t)
		c.Gates = Gates{true, true, true, true}
		c.RecordingPolicy = RecordingPolicy{Enabled: true, Version: "consent-v2", Purpose: "Community session archive", Retention: time.Hour, RestrictedStorageQualified: true}
		room, err := c.Create(ctx, "known-pool")
		if err != nil {
			t.Fatal(err)
		}
		room, err = c.Start(ctx, room.ID, room.Revision)
		if err != nil {
			t.Fatal(err)
		}
		if err = c.Join(ctx, room.ID); err != nil {
			t.Fatal(err)
		}
		recording, err := c.RequestRecording(ctx, room.ID)
		if err != nil {
			t.Fatal(err)
		}
		return c, ctx, room, recording
	}
	t.Run("participants", func(t *testing.T) {
		c, ctx, room, recording := setup(t)
		if _, err := c.DB.Exec(ctx, `DROP TABLE voice_participants CASCADE`); err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			name string
			run  func() error
		}{
			{"join", func() error { return c.Join(ctx, room.ID) }},
			{"speaker request", func() error { return c.RequestSpeaker(ctx, room.ID, 1) }},
			{"speaker approval", func() error { return c.ApproveSpeaker(ctx, room.ID, host, 1) }},
			{"mute", func() error { return c.Mute(ctx, room.ID, host, 1, true) }},
			{"ticket", func() error { _, err := c.Ticket(ctx, room.ID); return err }},
			{"recording read", func() error { _, err := c.Recording(ctx, room.ID); return err }},
			{"consent", func() error { return c.Consent(ctx, room.ID, recording.ID, recording.Revision, true) }},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if err := tc.run(); err == nil {
					t.Fatal("unavailable participant relation accepted an action")
				}
			})
		}
	})
	t.Run("recordings", func(t *testing.T) {
		c, ctx, room, recording := setup(t)
		if _, err := c.DB.Exec(ctx, `DROP TABLE voice_recordings CASCADE`); err != nil {
			t.Fatal(err)
		}
		for _, tc := range []struct {
			name string
			run  func() error
		}{
			{"recording read", func() error { _, err := c.Recording(ctx, room.ID); return err }},
			{"recording request", func() error { _, err := c.RequestRecording(ctx, room.ID); return err }},
			{"consent", func() error { return c.Consent(ctx, room.ID, recording.ID, recording.Revision, true) }},
			{"recording start", func() error { _, err := c.StartConsentedRecording(ctx, room.ID, recording.Revision); return err }},
			{"recording stop", func() error { _, err := c.StopConsentedRecording(ctx, room.ID, recording.Revision); return err }},
			{"reconcile", func() error { _, err := c.Reconcile(ctx); return err }},
			{"new participant", func() error {
				return c.JoinWithConsent(ctx, room.ID, &JoinConsent{RecordingID: uuid.NewString(), PolicyVersion: "consent-v2", Purpose: c.RecordingPolicy.Purpose, Agree: true})
			}},
		} {
			t.Run(tc.name, func(t *testing.T) {
				if err := tc.run(); err == nil {
					t.Fatal("unavailable recording relation accepted an action")
				}
			})
		}
	})
}
