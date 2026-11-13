package voice

import (
	"context"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/google/uuid"
)

func TestVoiceActionsRequireBoundSessionAndStorage(t *testing.T) {
	_, ctx := fixture(t)
	control := &Control{}
	room := uuid.NewString()
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"read", func() error { _, err := control.Read(ctx, room); return err }},
		{"create", func() error { _, err := control.Create(ctx, "known-pool"); return err }},
		{"start", func() error { _, err := control.Start(ctx, room, 1); return err }},
		{"end", func() error { _, err := control.End(ctx, room, 1); return err }},
		{"join", func() error { return control.Join(ctx, room) }},
		{"speaker request", func() error { return control.RequestSpeaker(ctx, room, 1) }},
		{"speaker approval", func() error { return control.ApproveSpeaker(ctx, room, host, 1) }},
		{"mute", func() error { return control.Mute(ctx, room, host, 1, true) }},
		{"ticket", func() error { _, err := control.Ticket(ctx, room); return err }},
		{"recording", func() error { _, err := control.Recording(ctx, room); return err }},
		{"recording request", func() error { _, err := control.RequestRecording(ctx, room); return err }},
		{"recording object", func() error { _, err := control.OpenRecording(ctx, room); return err }},
		{"reconcile", func() error { _, err := control.Reconcile(ctx); return err }},
		{"account cleanup", func() error { return control.DeleteAccount(ctx, host) }},
		{"policy update", func() error { return control.ApplyPolicyUpdate(ctx, uuid.NewString(), 1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("unbound voice action = %v", err)
			}
		})
	}
	if _, err := control.DeleteExpiredRecordings(context.Background()); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unbound recording retention = %v", err)
	}
	if _, err := control.StartConsentedRecording(ctx, room, 1); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unbound recording start = %v", err)
	}
	if _, err := control.StopConsentedRecording(ctx, room, 1); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unbound recording stop = %v", err)
	}
	if err := control.Consent(ctx, room, uuid.NewString(), 1, true); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unbound recording consent = %v", err)
	}
	if decision := (&safety.Store{}).Check(ctx, host, "voice", room); decision != safety.Unavailable {
		t.Fatalf("unbound voice policy = %s", decision)
	}
}

func TestVoiceActionsRejectClosedCommittedStore(t *testing.T) {
	control, ctx := fixture(t)
	room := uuid.NewString()
	control.DB.Close()
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"read", func() error { _, err := control.Read(ctx, room); return err }},
		{"create", func() error { _, err := control.Create(ctx, "known-pool"); return err }},
		{"start", func() error { _, err := control.Start(ctx, room, 1); return err }},
		{"end", func() error { _, err := control.End(ctx, room, 1); return err }},
		{"join", func() error { return control.Join(ctx, room) }},
		{"speaker request", func() error { return control.RequestSpeaker(ctx, room, 1) }},
		{"speaker approval", func() error { return control.ApproveSpeaker(ctx, room, host, 1) }},
		{"mute", func() error { return control.Mute(ctx, room, host, 1, true) }},
		{"ticket", func() error { _, err := control.Ticket(ctx, room); return err }},
		{"recording", func() error { _, err := control.Recording(ctx, room); return err }},
		{"recording request", func() error { _, err := control.RequestRecording(ctx, room); return err }},
		{"recording object", func() error { _, err := control.OpenRecording(ctx, room); return err }},
		{"account cleanup", func() error { return control.DeleteAccount(ctx, host) }},
		{"policy update", func() error { return control.ApplyPolicyUpdate(ctx, uuid.NewString(), 1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); err == nil {
				t.Fatal("closed voice store accepted an action")
			}
		})
	}
}
