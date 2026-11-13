package voice

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5"
)

func TestEndingVoiceRoomRevokesEveryConnectedParticipant(t *testing.T) {
	c, ctx := fixture(t)
	provider := localLiveKit(t)
	c.Provider = provider
	c.Gates = Gates{true, true, true, true}
	room, err := c.Create(ctx, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	live, err := c.Start(ctx, room.ID, 1)
	if err != nil {
		t.Fatal(err)
	}
	guest := "0x2222222222222222222222222222222222222222"
	for _, wallet := range []string{host, guest} {
		principal := safety.WithPrincipal(context.Background(), safety.Principal{Wallet: wallet, SessionID: "community-voice", Expires: time.Now().Add(time.Hour)})
		if err := c.Join(principal, room.ID); err != nil {
			t.Fatal(err)
		}
		ticket, err := c.Ticket(principal, room.ID)
		if err != nil {
			t.Fatal(err)
		}
		address := "wss://" + strings.TrimPrefix(provider.Endpoint, "https://") + "/rtc?access_token=" + url.QueryEscape(ticket) + "&auto_subscribe=1&protocol=15&sdk=go&version=1.0.0"
		transport := provider.Client.Transport.(*http.Transport)
		dialer := websocket.Dialer{HandshakeTimeout: 4 * time.Second, TLSClientConfig: transport.TLSClientConfig.Clone()}
		conn, response, err := dialer.DialContext(ctx, address, nil)
		if response != nil && response.Body != nil {
			_ = response.Body.Close()
		}
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = conn.Close() })
		if err := conn.SetReadDeadline(time.Now().Add(4 * time.Second)); err != nil {
			t.Fatal(err)
		}
		if _, payload, err := conn.ReadMessage(); err != nil || len(payload) == 0 {
			t.Fatalf("provider admission = %d bytes: %v", len(payload), err)
		}
	}
	ended, err := c.End(ctx, room.ID, live.Revision)
	if err != nil || ended.State != "ended" {
		t.Fatalf("ended room = %+v: %v", ended, err)
	}
	var active int
	if err := c.DB.QueryRow(ctx, `SELECT count(*) FROM voice_participants WHERE room_id=$1 AND (NOT revoked OR grant_state<>'revoked' OR NOT muted)`, room.ID).Scan(&active); err != nil {
		t.Fatal(err)
	}
	if active != 0 {
		t.Fatalf("active grants after end = %d", active)
	}
	if _, found, err := provider.Find(ctx, room.StartKey); err != nil || found {
		t.Fatalf("provider room remains = %t: %v", found, err)
	}
	replay, err := c.End(ctx, room.ID, ended.Revision)
	if err != nil || replay.Revision != ended.Revision {
		t.Fatalf("replayed end = %+v: %v", replay, err)
	}
}

func TestVoiceActionsRequirePrincipalBeforeChangingDurableState(t *testing.T) {
	c, ctx := fixture(t)
	c.Provider = localLiveKit(t)
	c.Gates = Gates{true, true, true, true}
	c.RecordingPolicy = RecordingPolicy{Enabled: true, Version: "consent-v2", Purpose: "Community archive", Retention: time.Hour, RestrictedStorageQualified: true}
	room, err := c.Create(ctx, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	anonymous := context.Background()
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"end", func() error { _, err := c.End(anonymous, room.ID, 1); return err }},
		{"join", func() error { return c.Join(anonymous, room.ID) }},
		{"speaker request", func() error { return c.RequestSpeaker(anonymous, room.ID, 1) }},
		{"speaker approval", func() error { return c.ApproveSpeaker(anonymous, room.ID, host, 1) }},
		{"mute", func() error { return c.Mute(anonymous, room.ID, host, 1, true) }},
		{"ticket", func() error { _, err := c.Ticket(anonymous, room.ID); return err }},
		{"recording request", func() error { _, err := c.RequestRecording(anonymous, room.ID); return err }},
		{"recording consent", func() error { return c.Consent(anonymous, room.ID, "", 1, true) }},
		{"recording start", func() error { _, err := c.StartConsentedRecording(anonymous, room.ID, 1); return err }},
		{"recording stop", func() error { _, err := c.StopConsentedRecording(anonymous, room.ID, 1); return err }},
		{"leave", func() error { return c.Leave(anonymous, room.ID) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); !errors.Is(err, safety.ErrForbidden) {
				t.Fatalf("anonymous action = %v", err)
			}
		})
	}
	stored, err := c.Read(ctx, room.ID)
	if err != nil || stored.State != "draft" || stored.Revision != 1 {
		t.Fatalf("anonymous durable state = %+v: %v", stored, err)
	}
}

func TestVoiceProviderRejectsInvalidRequestEncodingAndCancelledTransport(t *testing.T) {
	provider := localLiveKit(t)
	ctx := context.Background()
	if _, err := provider.token(host, map[string]any{"invalid": make(chan int)}, time.Minute); err == nil {
		t.Fatal("unsupported token claim encoded")
	}
	if err := provider.call(ctx, "RoomService", "ListRooms", "", make(chan int), nil); err == nil {
		t.Fatal("unsupported provider request encoded")
	}
	invalid := *provider
	invalid.Endpoint = "%"
	if err := invalid.call(ctx, "RoomService", "ListRooms", "", map[string]any{}, nil); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("invalid endpoint = %v", err)
	}
	invalid = *provider
	invalid.APIKey = ""
	if err := invalid.call(ctx, "RoomService", "ListRooms", "", map[string]any{}, nil); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unbound provider key = %v", err)
	}
	if err := (*LiveKit)(nil).call(ctx, "RoomService", "ListRooms", "", map[string]any{}, nil); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unbound provider = %v", err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"start", func() error { _, err := provider.Start(cancelled, "cancelled-room"); return err }},
		{"end", func() error { return provider.End(cancelled, "cancelled-room") }},
		{"record", func() error {
			copy := *provider
			copy.RecordingOutput = S3RecordingOutput{Bucket: "community-recordings", Region: "test", AccessKey: "test", Secret: "test"}
			_, err := copy.Record(cancelled, "cancelled-room", "recording")
			return err
		}},
		{"stop recording", func() error { return provider.StopRecording(cancelled, "cancelled-room", "recording") }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("cancelled provider operation = %v", err)
			}
		})
	}
}

func TestReconciliationRoomTransactionsRequireActiveDatabaseTransaction(t *testing.T) {
	c, ctx := fixture(t)
	room, err := c.Create(ctx, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := c.DB.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	closed := postgresReconciliationRoom{Tx: tx, id: room.ID}
	for _, tc := range []struct {
		name string
		run  func() error
	}{
		{"room", func() error { _, _, err := closed.room(ctx); return err }},
		{"adopt", func() error { return closed.adopt(ctx, "", time.Now()) }},
		{"end", func() error { return closed.end(ctx, time.Now()) }},
		{"grants", func() error { _, err := closed.pendingGrants(ctx); return err }},
		{"revoke", func() error { return closed.revokeGrant(ctx, reconciliationGrant{wallet: host, revision: 1}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := tc.run(); !errors.Is(err, pgx.ErrTxClosed) {
				t.Fatalf("closed room transaction = %v", err)
			}
		})
	}
	c.DB.Close()
	if _, err := (postgresReconciliationStore{c}).beginRoom(ctx, room.ID); err == nil {
		t.Fatal("closed store began room transaction")
	}
	if err := c.recordingTransition(ctx, "", room.ID, 1, "starting", "recording", "", safety.Principal{}); err == nil {
		t.Fatal("closed store changed recording")
	}
	if err := c.Consent(ctx, room.ID, "", 1, true); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unbound recording policy = %v", err)
	}
}
