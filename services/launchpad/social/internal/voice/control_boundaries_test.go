package voice

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
	"github.com/gorilla/websocket"
)

func TestVoiceControlAdmitsAndRevokesCommittedParticipants(t *testing.T) {
	c, hostContext := fixture(t)
	c.Provider = localLiveKit(t)
	c.Gates = Gates{true, true, true, true}
	principal := func(wallet string) context.Context {
		return safety.WithPrincipal(context.Background(), safety.Principal{Wallet: wallet, SessionID: "voice-control-" + wallet, Expires: time.Now().Add(time.Hour)})
	}
	guest := "0x2222222222222222222222222222222222222222"
	guestContext := principal(guest)
	if _, err := c.Create(context.Background(), "known-pool"); !errors.Is(err, safety.ErrForbidden) {
		t.Fatalf("anonymous room creation = %v", err)
	}
	if _, err := c.Create(hostContext, strings.Repeat("p", 257)); !errors.Is(err, safety.ErrInvalid) {
		t.Fatalf("unbounded pool = %v", err)
	}
	if _, err := c.Create(hostContext, "missing-pool"); !errors.Is(err, safety.ErrInvalid) {
		t.Fatalf("unknown pool = %v", err)
	}
	room, err := c.Create(hostContext, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Start(guestContext, room.ID, room.Revision); !errors.Is(err, safety.ErrForbidden) {
		t.Fatalf("guest started room = %v", err)
	}
	if _, err := c.Start(hostContext, room.ID, room.Revision+1); !errors.Is(err, safety.ErrConflict) {
		t.Fatalf("stale start = %v", err)
	}
	if err := c.Join(hostContext, room.ID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("draft admission = %v", err)
	}
	live, err := c.Start(hostContext, room.ID, room.Revision)
	if err != nil || live.State != liveState {
		t.Fatalf("live room = %+v: %v", live, err)
	}
	if _, err := c.Start(hostContext, room.ID, room.Revision); !errors.Is(err, safety.ErrConflict) {
		t.Fatalf("second start = %v", err)
	}
	if err := c.ApproveSpeaker(guestContext, room.ID, host, 1); !errors.Is(err, safety.ErrForbidden) {
		t.Fatalf("guest approved speaker = %v", err)
	}
	if err := c.Join(guestContext, room.ID); err != nil {
		t.Fatal(err)
	}
	if err := c.Join(guestContext, room.ID); err != nil {
		t.Fatalf("idempotent admission = %v", err)
	}
	token, err := c.Ticket(guestContext, room.ID)
	if err != nil || strings.Count(token, ".") != 2 {
		t.Fatalf("listener ticket = %q: %v", token, err)
	}
	provider := c.Provider.(*LiveKit)
	address := "wss://" + strings.TrimPrefix(provider.Endpoint, "https://") + "/rtc?access_token=" + url.QueryEscape(token) + "&auto_subscribe=1&protocol=15&sdk=go&version=1.0.0"
	transport := provider.Client.Transport.(*http.Transport)
	dialer := websocket.Dialer{HandshakeTimeout: 4 * time.Second, TLSClientConfig: transport.TLSClientConfig.Clone()}
	connection, response, err := dialer.DialContext(guestContext, address, nil)
	if response != nil && response.Body != nil {
		_ = response.Body.Close()
	}
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = connection.Close() }()
	if err := connection.SetReadDeadline(time.Now().Add(4 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, payload, err := connection.ReadMessage(); err != nil || len(payload) == 0 {
		t.Fatalf("provider admission = %d bytes: %v", len(payload), err)
	}
	if err := c.RequestSpeaker(guestContext, room.ID, 1); err != nil {
		t.Fatalf("speaker request = %v", err)
	}
	if err := c.ApproveSpeaker(hostContext, room.ID, guest, 1); !errors.Is(err, safety.ErrConflict) {
		t.Fatalf("stale speaker revision = %v", err)
	}
	if err := c.ApproveSpeaker(hostContext, room.ID, guest, 2); err != nil {
		t.Fatalf("speaker grant = %v", err)
	}
	var role, grant string
	var revision int64
	if err := c.DB.QueryRow(hostContext, `SELECT role,grant_state,revision FROM voice_participants WHERE room_id=$1 AND wallet=$2`, room.ID, guest).Scan(&role, &grant, &revision); err != nil || role != "speaker" || grant != "active" {
		t.Fatalf("speaker state = %s %s %d: %v", role, grant, revision, err)
	}
	if err := c.Mute(guestContext, room.ID, guest, revision, false); !errors.Is(err, safety.ErrForbidden) {
		t.Fatalf("guest muted speaker = %v", err)
	}
	if err := c.Mute(hostContext, room.ID, guest, revision+1, true); !errors.Is(err, safety.ErrConflict) {
		t.Fatalf("stale ejection = %v", err)
	}
	if err := c.Mute(hostContext, room.ID, guest, revision, true); err != nil {
		t.Fatalf("speaker ejection = %v", err)
	}
	if err := c.Join(guestContext, room.ID); !errors.Is(err, safety.ErrForbidden) {
		t.Fatalf("ejected participant rejoined = %v", err)
	}
	if _, err := c.Ticket(guestContext, room.ID); !errors.Is(err, safety.ErrForbidden) {
		t.Fatalf("ejected participant received ticket = %v", err)
	}
	if _, err := c.End(guestContext, room.ID, live.Revision); !errors.Is(err, safety.ErrForbidden) {
		t.Fatalf("guest ended room = %v", err)
	}
	if _, err := c.End(hostContext, room.ID, live.Revision+1); !errors.Is(err, safety.ErrConflict) {
		t.Fatalf("stale end = %v", err)
	}
	ended, err := c.End(hostContext, room.ID, live.Revision)
	if err != nil || ended.State != "ended" {
		t.Fatalf("ended room = %+v: %v", ended, err)
	}
	if again, err := c.End(hostContext, room.ID, ended.Revision); err != nil || again.Revision != ended.Revision {
		t.Fatalf("idempotent end = %+v: %v", again, err)
	}
}

func TestVoiceControlEnforcesParticipantCapacity(t *testing.T) {
	c, ctx := fixture(t)
	c.Provider = localLiveKit(t)
	c.Gates = Gates{true, true, true, true}
	room, err := c.Create(ctx, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Start(ctx, room.ID, room.Revision); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 50; i++ {
		wallet := fmt.Sprintf("0x%040x", i+1000)
		if _, err := c.DB.Exec(ctx, `INSERT INTO voice_participants(room_id,wallet)VALUES($1,$2)`, room.ID, wallet); err != nil {
			t.Fatal(err)
		}
	}
	guest := safety.WithPrincipal(context.Background(), safety.Principal{Wallet: "0x3333333333333333333333333333333333333333", SessionID: "voice-capacity", Expires: time.Now().Add(time.Hour)})
	if err := c.Join(guest, room.ID); !errors.Is(err, safety.ErrConflict) {
		t.Fatalf("over capacity admission = %v", err)
	}
	if _, err := c.DB.Exec(ctx, `DELETE FROM voice_participants WHERE room_id=$1 AND wallet=$2`, room.ID, fmt.Sprintf("0x%040x", 1001)); err != nil {
		t.Fatal(err)
	}
	if err := c.Join(guest, room.ID); err != nil {
		t.Fatalf("admission after capacity frees = %v", err)
	}
}

func TestSpeakerGrantFailureLeavesRevokedProviderAuthority(t *testing.T) {
	control, hostContext := fixture(t)
	provider := localLiveKit(t)
	control.Provider = provider
	control.Gates = Gates{true, true, true, true}
	room, err := control.Create(hostContext, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	live, err := control.Start(hostContext, room.ID, room.Revision)
	if err != nil {
		t.Fatal(err)
	}
	guest := "0x2222222222222222222222222222222222222222"
	guestContext := safety.WithPrincipal(context.Background(), safety.Principal{Wallet: guest, SessionID: "speaker-grant", Expires: time.Now().Add(time.Hour)})
	if err := control.Join(guestContext, room.ID); err != nil {
		t.Fatal(err)
	}
	if err := provider.End(hostContext, live.ProviderSession); err != nil {
		t.Fatal(err)
	}
	if err := control.ApproveSpeaker(hostContext, room.ID, guest, 1); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("grant after provider room ends = %v", err)
	}
	var role, grant string
	var muted bool
	if err := control.DB.QueryRow(hostContext, `SELECT role,grant_state,muted FROM voice_participants WHERE room_id=$1 AND wallet=$2`, room.ID, guest).Scan(&role, &grant, &muted); err != nil || role != "listener" || grant != "revoked" || !muted {
		t.Fatalf("failed provider grant state = %s %s muted %t: %v", role, grant, muted, err)
	}
}
