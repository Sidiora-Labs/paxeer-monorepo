package voice

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func withoutVoiceRelation(t *testing.T, c *Control, table string, run func()) {
	t.Helper()
	name := pgx.Identifier{table}.Sanitize()
	held := pgx.Identifier{table + "_unavailable"}.Sanitize()
	if _, err := c.DB.Exec(context.Background(), "ALTER TABLE "+name+" RENAME TO "+held); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := c.DB.Exec(context.Background(), "ALTER TABLE "+held+" RENAME TO "+name); err != nil {
			t.Fatal(err)
		}
	}()
	run()
}

func TestVoiceConsentAndEjectionRollBackWhenLedgerIsUnavailable(t *testing.T) {
	c, ctx := fixture(t)
	c.Provider = localLiveKit(t)
	c.Gates = Gates{true, true, true, true}
	c.RecordingPolicy = RecordingPolicy{Enabled: true, Version: "consent-v2", Purpose: "Community archive", Retention: time.Hour, RestrictedStorageQualified: true}
	room, err := c.Create(ctx, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	room, err = c.Start(ctx, room.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Join(ctx, room.ID); err != nil {
		t.Fatal(err)
	}
	withoutVoiceRelation(t, c, "voice_recording_events", func() {
		if _, err := c.RequestRecording(ctx, room.ID); err == nil {
			t.Fatal("recording request committed without audit ledger")
		}
	})
	var requests int
	if err := c.DB.QueryRow(ctx, `SELECT count(*) FROM voice_recordings WHERE room_id=$1`, room.ID).Scan(&requests); err != nil {
		t.Fatal(err)
	}
	if requests != 0 {
		t.Fatalf("unaudited recording requests = %d", requests)
	}
	recording, err := c.RequestRecording(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	guest := safety.WithPrincipal(ctx, safety.Principal{Wallet: "0x2222222222222222222222222222222222222222", SessionID: "consenting-guest", Expires: time.Now().Add(time.Hour)})
	for _, tc := range []struct {
		name, table string
		run         func() error
	}{
		{"consent write", "voice_consents", func() error { return c.Consent(ctx, room.ID, recording.ID, 1, true) }},
		{"consent audit", "voice_recording_events", func() error { return c.Consent(ctx, room.ID, recording.ID, 1, true) }},
		{"joined consent", "voice_consents", func() error {
			return c.JoinWithConsent(guest, room.ID, &JoinConsent{RecordingID: recording.ID, PolicyVersion: recording.PolicyVersion, Purpose: recording.Purpose, Agree: true})
		}},
		{"start consent", "voice_consents", func() error { _, err := c.StartConsentedRecording(ctx, room.ID, 1); return err }},
		{"ejection ledger", "voice_ejections", func() error { return c.Mute(ctx, room.ID, host, 1, true) }},
		{"account cleanup", "voice_participants", func() error { return c.DeleteAccount(ctx, host) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			withoutVoiceRelation(t, c, tc.table, func() {
				err := tc.run()
				var postgres *pgconn.PgError
				if !errors.As(err, &postgres) || postgres.Code != "42P01" {
					t.Fatalf("voice ledger dependency = %v", err)
				}
			})
		})
	}
	var consents int
	var revision int64
	var muted, revoked bool
	if err := c.DB.QueryRow(ctx, `SELECT count(*) FROM voice_consents WHERE recording_id=$1`, recording.ID).Scan(&consents); err != nil {
		t.Fatal(err)
	}
	if err := c.DB.QueryRow(ctx, `SELECT revision,muted,revoked FROM voice_participants WHERE room_id=$1 AND wallet=$2`, room.ID, host).Scan(&revision, &muted, &revoked); err != nil {
		t.Fatal(err)
	}
	if consents != 0 || revision != 1 || muted || revoked {
		t.Fatalf("partial consent or ejection state: consents=%d revision=%d muted=%t revoked=%t", consents, revision, muted, revoked)
	}
	if err := c.Consent(ctx, room.ID, recording.ID, 1, true); err != nil {
		t.Fatal(err)
	}
	withoutVoiceRelation(t, c, "voice_recording_events", func() {
		if _, err := c.StartConsentedRecording(ctx, room.ID, 1); err == nil {
			t.Fatal("recording start committed without audit ledger")
		}
	})
	stored, err := c.Recording(ctx, room.ID)
	if err != nil || stored.State != "consent_pending" || stored.Revision != 1 {
		t.Fatalf("unaudited recording state = %+v: %v", stored, err)
	}
	c.DB.Close()
	if err := c.Consent(ctx, room.ID, recording.ID, 1, true); err == nil {
		t.Fatal("closed store accepted consent")
	}
	if _, err := c.Reconcile(ctx); err == nil {
		t.Fatal("closed store reconciled rooms")
	}
}

func TestVoiceProviderNormalizesNestedResponseNames(t *testing.T) {
	input := map[string]any{"roomComposite": map[string]any{"fileOutputs": []any{map[string]any{"egressId": "recording", "filePath": "restricted/voice/recording.ogg"}}}}
	normalizeProviderJSON(input)
	room, ok := input["room_composite"].(map[string]any)
	if !ok {
		t.Fatalf("normalized room response = %#v", input)
	}
	files, ok := room["file_outputs"].([]any)
	if !ok || len(files) != 1 {
		t.Fatalf("normalized file response = %#v", room)
	}
	item := files[0].(map[string]any)
	if item["egress_id"] != "recording" || item["file_path"] != "restricted/voice/recording.ogg" {
		t.Fatalf("normalized recording response = %#v", item)
	}
	if _, ok := input["roomComposite"]; ok {
		t.Fatal("response retained duplicate camel-case room key")
	}
}
