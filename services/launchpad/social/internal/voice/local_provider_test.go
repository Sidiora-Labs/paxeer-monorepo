package voice

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sidiora-Technologies/KindleLaunch/media/social/internal/safety"
)

type fixturePoolLookup struct{ db *pgxpool.Pool }

func (p fixturePoolLookup) Exists(ctx context.Context, id string) (bool, error) {
	var exists bool
	err := p.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM fixture_pools WHERE id=$1)`, id).Scan(&exists)
	return exists, err
}

func localLiveKit(t *testing.T) *LiveKit {
	t.Helper()
	binary, err := exec.LookPath("livekit-server")
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	const key = "voice-local"
	const secret = "voice-local-integration-secret-32-bytes"
	log, err := os.CreateTemp(t.TempDir(), "livekit-*.log")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = log.Close() })
	command := exec.Command(binary, "--dev", "--bind", "127.0.0.1", "--node-ip", "127.0.0.1", "--config-body", fmt.Sprintf("port: %d\n", port), "--keys", key+": "+secret)
	command.Stdout, command.Stderr = log, log
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = command.Process.Kill(); _ = command.Wait() })
	upstream, err := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", port))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewTLSServer(httputil.NewSingleHostReverseProxy(upstream))
	t.Cleanup(server.Close)
	provider := &LiveKit{Endpoint: server.URL, APIKey: key, APISecret: secret, Client: server.Client()}
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if _, _, err := provider.Find(context.Background(), "voice-local-health"); err == nil {
			return provider
		}
		time.Sleep(50 * time.Millisecond)
	}
	output, _ := os.ReadFile(log.Name())
	t.Fatalf("voice provider did not become ready: %s", strings.TrimSpace(string(output)))
	return nil
}

func TestLocalProviderRoomAndControlLifecycle(t *testing.T) {
	c, ctx := fixture(t)
	if _, err := c.DB.Exec(ctx, `CREATE TABLE fixture_pools(id text PRIMARY KEY); INSERT INTO fixture_pools(id) VALUES('known-pool')`); err != nil {
		t.Fatal(err)
	}
	c.Pools = fixturePoolLookup{db: c.DB}
	c.Policy = &safety.Store{DB: c.DB}
	c.Provider = localLiveKit(t)
	c.Gates = Gates{ProviderQualified: true, EjectionQualified: true, ACLQualified: true, RetentionQualified: true}
	room, err := c.Create(ctx, "known-pool")
	if err != nil || room.State != "draft" || room.Revision != 1 {
		t.Fatalf("draft room = %+v: %v", room, err)
	}
	live, err := c.Start(ctx, room.ID, room.Revision)
	if err != nil || live.State != "live" || live.ProviderSession == "" || live.Revision != 2 {
		t.Fatalf("live room = %+v: %v", live, err)
	}
	if _, found, err := c.Provider.(*LiveKit).Find(ctx, room.StartKey); err != nil || !found {
		t.Fatalf("provider room exists = %t: %v", found, err)
	}
	if err := c.Join(ctx, live.ID); err != nil {
		t.Fatal(err)
	}
	grant, err := c.Ticket(ctx, live.ID)
	if err != nil || strings.Count(grant, ".") != 2 {
		t.Fatalf("voice grant = %q: %v", grant, err)
	}
	ended, err := c.End(ctx, live.ID, live.Revision)
	if err != nil || ended.State != "ended" || ended.Revision != 4 {
		t.Fatalf("ended room = %+v: %v", ended, err)
	}
	if _, found, err := c.Provider.(*LiveKit).Find(ctx, room.StartKey); err != nil || found {
		t.Fatalf("provider room remains = %t: %v", found, err)
	}
	if _, err := c.Ticket(ctx, live.ID); err == nil {
		t.Fatal("ended room issued a voice grant")
	}
}

func TestLocalProviderAccountDeletionEndsHostedRoom(t *testing.T) {
	control, ctx := fixture(t)
	if _, err := control.DB.Exec(ctx, `CREATE TABLE fixture_pools(id text PRIMARY KEY); INSERT INTO fixture_pools(id) VALUES('known-pool')`); err != nil {
		t.Fatal(err)
	}
	control.Pools = fixturePoolLookup{db: control.DB}
	control.Policy = &safety.Store{DB: control.DB}
	control.Provider = localLiveKit(t)
	control.Gates = Gates{ProviderQualified: true, EjectionQualified: true, ACLQualified: true, RetentionQualified: true}
	room, err := control.Create(ctx, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	live, err := control.Start(ctx, room.ID, room.Revision)
	if err != nil || live.State != "live" {
		t.Fatalf("hosted room = %+v: %v", live, err)
	}
	guest := "0x2222222222222222222222222222222222222222"
	guestContext := safety.WithPrincipal(context.Background(), safety.Principal{Wallet: guest, SessionID: "voice-guest", Expires: time.Now().Add(time.Hour)})
	if err := control.Join(guestContext, room.ID); err != nil {
		t.Fatalf("guest admission = %v", err)
	}
	if err := control.DeleteAccount(guestContext, guest); err != nil {
		t.Fatalf("guest account deletion = %v", err)
	}
	var revoked, muted bool
	var grant string
	if err := control.DB.QueryRow(ctx, `SELECT revoked,muted,grant_state FROM voice_participants WHERE room_id=$1 AND wallet=$2`, room.ID, guest).Scan(&revoked, &muted, &grant); err != nil || !revoked || !muted || grant != "revoked" {
		t.Fatalf("guest grant = revoked %t muted %t state %q: %v", revoked, muted, grant, err)
	}
	if stillLive, err := control.Read(ctx, room.ID); err != nil || stillLive.State != "live" {
		t.Fatalf("hosted room after guest deletion = %+v: %v", stillLive, err)
	}
	if err := control.DeleteAccount(ctx, host); err != nil {
		t.Fatalf("host account deletion = %v", err)
	}
	ended, err := control.Read(ctx, room.ID)
	if err != nil || ended.State != "ended" || ended.Revision != live.Revision+2 {
		t.Fatalf("ended hosted room = %+v: %v", ended, err)
	}
	if _, found, err := control.Provider.(*LiveKit).Find(ctx, live.StartKey); err != nil || found {
		t.Fatalf("hosted session still present = %t: %v", found, err)
	}
	if err := control.DeleteAccount(ctx, host); err != nil {
		t.Fatalf("replayed account deletion = %v", err)
	}
}

func TestProductionVoiceControlRequiresQualifiedProviderBinding(t *testing.T) {
	control, _ := fixture(t)
	endpoint, key, secret := "https://"+strings.Join([]string{"fixture", "livekit", "cloud"}, "."), "fixture", strings.Repeat("s", 32)
	provider, err := NewLiveKit(endpoint, key, secret, S3RecordingOutput{})
	if err != nil {
		t.Fatal(err)
	}
	gates := Gates{ProviderQualified: true, EjectionQualified: true, ACLQualified: true, RetentionQualified: true}
	qualified, err := NewProductionControl(control.DB, &safety.Store{DB: control.DB}, fixturePoolLookup{db: control.DB}, provider, gates, "voice-v1", RecordingPolicy{}, nil, "")
	if err != nil || qualified == nil || qualified.Provider != provider || qualified.SafetyPolicyVersion != "voice-v1" {
		t.Fatalf("qualified production control = %+v: %v", qualified, err)
	}
	if value, err := NewProductionControl(control.DB, &safety.Store{DB: control.DB}, fixturePoolLookup{db: control.DB}, provider, Gates{}, "voice-v1", RecordingPolicy{}, nil, ""); value != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unqualified production control = %+v: %v", value, err)
	}
	if value, err := NewProductionControl(control.DB, &safety.Store{DB: control.DB}, fixturePoolLookup{db: control.DB}, provider, gates, "voice-v1", RecordingPolicy{Enabled: true}, nil, ""); value != nil || !errors.Is(err, ErrUnavailable) {
		t.Fatalf("recording without storage = %+v: %v", value, err)
	}
}

func TestLocalProviderPolicyUpdateRevokesActiveVoiceGrant(t *testing.T) {
	c, ctx := fixture(t)
	if _, err := c.DB.Exec(ctx, `CREATE TABLE fixture_pools(id text PRIMARY KEY); INSERT INTO fixture_pools(id) VALUES('known-pool')`); err != nil {
		t.Fatal(err)
	}
	c.Pools = fixturePoolLookup{db: c.DB}
	c.Policy = &safety.Store{DB: c.DB}
	c.Provider = localLiveKit(t)
	c.Gates = Gates{ProviderQualified: true, EjectionQualified: true, ACLQualified: true, RetentionQualified: true}
	room, err := c.Create(ctx, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	live, err := c.Start(ctx, room.ID, room.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Join(ctx, room.ID); err != nil {
		t.Fatal(err)
	}
	admin := safety.WithPrincipal(context.Background(), safety.Principal{Wallet: "0xaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", SessionID: "policy-review", Role: "global_administrator", Expires: time.Now().Add(time.Hour), StepUp: time.Now()})
	banID, err := c.Policy.(*safety.Store).Ban(admin, safety.BanCommand{Wallet: host, Scope: "global", Reason: "reviewed conduct", PolicyVersion: "voice-v2"})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.ApplyPolicyUpdate(ctx, banID, 1); err != nil {
		t.Fatal(err)
	}
	var muted bool
	var state string
	if err := c.DB.QueryRow(ctx, `SELECT muted,grant_state FROM voice_participants WHERE room_id=$1 AND wallet=$2`, room.ID, host).Scan(&muted, &state); err != nil || !muted || state != "revoked" {
		t.Fatalf("policy grant = muted %t, state %q: %v", muted, state, err)
	}
	if _, err := c.Ticket(ctx, room.ID); err == nil {
		t.Fatal("revoked participant received a voice ticket")
	}
	if err := c.ApplyPolicyUpdate(ctx, banID, 1); err != nil {
		t.Fatalf("replayed policy update: %v", err)
	}
	if _, found, err := c.Provider.(*LiveKit).Find(ctx, live.StartKey); err != nil || !found {
		t.Fatalf("room after participant revocation = found %t: %v", found, err)
	}
}

func TestLocalProviderApprovesConnectedSpeaker(t *testing.T) {
	c, ctx := fixture(t)
	if _, err := c.DB.Exec(ctx, `CREATE TABLE fixture_pools(id text PRIMARY KEY); INSERT INTO fixture_pools(id) VALUES('known-pool')`); err != nil {
		t.Fatal(err)
	}
	c.Pools = fixturePoolLookup{db: c.DB}
	c.Policy = &safety.Store{DB: c.DB}
	c.Provider = localLiveKit(t)
	c.Gates = Gates{ProviderQualified: true, EjectionQualified: true, ACLQualified: true, RetentionQualified: true}
	room, err := c.Create(ctx, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	live, err := c.Start(ctx, room.ID, room.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Join(ctx, room.ID); err != nil {
		t.Fatal(err)
	}
	ticket, err := c.Ticket(ctx, room.ID)
	if err != nil {
		t.Fatal(err)
	}
	provider := c.Provider.(*LiveKit)
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
	defer func() { _ = conn.Close() }()
	if err := conn.SetReadDeadline(time.Now().Add(4 * time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, payload, err := conn.ReadMessage(); err != nil || len(payload) == 0 {
		t.Fatalf("provider admission = %d bytes: %v", len(payload), err)
	}
	if err := c.RequestSpeaker(ctx, room.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := c.ApproveSpeaker(ctx, room.ID, host, 2); err != nil {
		t.Fatal(err)
	}
	var role, state string
	var muted bool
	if err := c.DB.QueryRow(ctx, `SELECT role,grant_state,muted FROM voice_participants WHERE room_id=$1 AND wallet=$2`, room.ID, host).Scan(&role, &state, &muted); err != nil || role != "speaker" || state != "active" || muted {
		t.Fatalf("approved speaker = %q %q muted %t: %v", role, state, muted, err)
	}
	var participant struct {
		Identity   string `json:"identity"`
		Permission struct {
			Publish bool `json:"can_publish"`
		} `json:"permission"`
	}
	if err := provider.call(ctx, "RoomService", "GetParticipant", live.ProviderSession, map[string]string{"room": live.ProviderSession, "identity": host}, &participant); err != nil || participant.Identity != host || !participant.Permission.Publish {
		t.Fatalf("provider speaker = %+v: %v", participant, err)
	}
	if err := c.Mute(ctx, room.ID, host, 4, true); err != nil {
		t.Fatal(err)
	}
	var revoked bool
	if err := c.DB.QueryRow(ctx, `SELECT revoked,grant_state,muted FROM voice_participants WHERE room_id=$1 AND wallet=$2`, room.ID, host).Scan(&revoked, &state, &muted); err != nil || !revoked || state != "revoked" || !muted {
		t.Fatalf("ejected speaker = revoked %t, state %q, muted %t: %v", revoked, state, muted, err)
	}
	if _, err := c.Ticket(ctx, room.ID); err == nil {
		t.Fatal("ejected speaker received a voice ticket")
	}
	ended, err := c.End(ctx, room.ID, live.Revision)
	if err != nil || ended.State != "ended" {
		t.Fatalf("room after connected speaker departure = %+v: %v", ended, err)
	}
	if _, found, err := provider.Find(ctx, room.StartKey); err != nil || found {
		t.Fatalf("provider room after connected speaker departure = %t: %v", found, err)
	}
}

func TestLocalProviderReconciliationAdoptsAndEndsRoom(t *testing.T) {
	c, ctx := fixture(t)
	if _, err := c.DB.Exec(ctx, `CREATE TABLE fixture_pools(id text PRIMARY KEY); INSERT INTO fixture_pools(id) VALUES('known-pool')`); err != nil {
		t.Fatal(err)
	}
	c.Pools = fixturePoolLookup{db: c.DB}
	c.Policy = &safety.Store{DB: c.DB}
	c.Provider = localLiveKit(t)
	c.Gates = Gates{ProviderQualified: true, EjectionQualified: true, ACLQualified: true, RetentionQualified: true}

	room, err := c.Create(ctx, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	session, err := c.Provider.Start(ctx, room.StartKey)
	if err != nil {
		t.Fatal(err)
	}
	result, err := c.Reconcile(ctx)
	if err != nil || result.Examined != 1 || result.Adopted != 1 || result.Failed != 0 {
		t.Fatalf("room adoption = %+v: %v", result, err)
	}
	adopted, err := c.Read(ctx, room.ID)
	if err != nil || adopted.State != "live" || adopted.ProviderSession != session || adopted.Revision != 2 {
		t.Fatalf("adopted room = %+v: %v", adopted, err)
	}
	if _, err := c.DB.Exec(ctx, `UPDATE voice_rooms SET state='ending',revision=revision+1 WHERE id=$1`, room.ID); err != nil {
		t.Fatal(err)
	}
	result, err = c.Reconcile(ctx)
	if err != nil || result.Examined != 1 || result.Terminated != 1 || result.Failed != 0 {
		t.Fatalf("room termination = %+v: %v", result, err)
	}
	ended, err := c.Read(ctx, room.ID)
	if err != nil || ended.State != "ended" || ended.Revision != 4 {
		t.Fatalf("ended room = %+v: %v", ended, err)
	}
	if _, found, err := c.Provider.(*LiveKit).Find(ctx, room.StartKey); err != nil || found {
		t.Fatalf("provider room remains = %t: %v", found, err)
	}
}

func TestLocalProviderReconciliationRevokesPendingGrant(t *testing.T) {
	c, ctx := fixture(t)
	if _, err := c.DB.Exec(ctx, `CREATE TABLE fixture_pools(id text PRIMARY KEY); INSERT INTO fixture_pools(id) VALUES('known-pool')`); err != nil {
		t.Fatal(err)
	}
	c.Pools = fixturePoolLookup{db: c.DB}
	c.Policy = &safety.Store{DB: c.DB}
	c.Provider = localLiveKit(t)
	c.Gates = Gates{ProviderQualified: true, EjectionQualified: true, ACLQualified: true, RetentionQualified: true}
	room, err := c.Create(ctx, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	initial, err := c.Reconcile(ctx)
	if err != nil || initial.Examined != 1 || initial.Adopted != 0 || initial.Terminated != 0 {
		t.Fatalf("draft without provider session = %+v: %v", initial, err)
	}
	live, err := c.Start(ctx, room.ID, room.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Join(ctx, room.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DB.Exec(ctx, `UPDATE voice_participants SET grant_state='revoking' WHERE room_id=$1 AND wallet=$2`, room.ID, host); err != nil {
		t.Fatal(err)
	}
	result, err := c.Reconcile(ctx)
	if err != nil || result.Examined != 1 || result.Revoked != 1 {
		t.Fatalf("pending grant revocation = %+v: %v", result, err)
	}
	var state string
	var muted bool
	if err := c.DB.QueryRow(ctx, `SELECT grant_state,muted FROM voice_participants WHERE room_id=$1 AND wallet=$2`, room.ID, host).Scan(&state, &muted); err != nil || state != "revoked" || !muted {
		t.Fatalf("committed grant revocation = %q muted %t: %v", state, muted, err)
	}
	if err := c.Provider.End(ctx, live.ProviderSession); err != nil {
		t.Fatal(err)
	}
	missing, err := c.Reconcile(ctx)
	if !errors.Is(err, ErrUnavailable) || missing.Failed != 1 {
		t.Fatalf("missing live provider session = %+v: %v", missing, err)
	}
}

func TestLocalProviderReconciliationTerminatesUnadmittedSession(t *testing.T) {
	c, ctx := fixture(t)
	c.Provider = localLiveKit(t)
	room, err := c.Create(ctx, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	session, err := c.Provider.Start(ctx, room.StartKey)
	if err != nil {
		t.Fatal(err)
	}
	result, err := c.Reconcile(ctx)
	if err != nil || result.Examined != 1 || result.Terminated != 1 || result.Adopted != 0 {
		t.Fatalf("unadmitted provider session = %+v: %v", result, err)
	}
	if _, found, err := c.Provider.(*LiveKit).Find(ctx, room.StartKey); err != nil || found {
		t.Fatalf("provider session after termination = %t: %v", found, err)
	}
	if session == "" {
		t.Fatal("provider returned an empty session")
	}
}

func TestLocalProviderParticipantAndRecordingTransitions(t *testing.T) {
	c, ctx := fixture(t)
	if _, err := c.DB.Exec(ctx, `CREATE TABLE fixture_pools(id text PRIMARY KEY); INSERT INTO fixture_pools(id) VALUES('known-pool')`); err != nil {
		t.Fatal(err)
	}
	c.Pools = fixturePoolLookup{db: c.DB}
	c.Policy = &safety.Store{DB: c.DB}
	c.Provider = localLiveKit(t)
	c.Gates = Gates{ProviderQualified: true, EjectionQualified: true, ACLQualified: true, RetentionQualified: true}
	c.RecordingPolicy = RecordingPolicy{Enabled: true, Version: "consent-v2", Purpose: "Community session archive", Retention: time.Hour, RestrictedStorageQualified: true}
	room, err := c.Create(ctx, "known-pool")
	if err != nil {
		t.Fatal(err)
	}
	live, err := c.Start(ctx, room.ID, room.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Join(ctx, room.ID); err != nil {
		t.Fatal(err)
	}
	if err := c.RequestSpeaker(ctx, room.ID, 1); err != nil {
		t.Fatal(err)
	}
	var role string
	var revision int64
	if err := c.DB.QueryRow(ctx, `SELECT role,revision FROM voice_participants WHERE room_id=$1 AND wallet=$2`, room.ID, host).Scan(&role, &revision); err != nil || role != "requested_speaker" || revision != 2 {
		t.Fatalf("speaker request = %q revision %d: %v", role, revision, err)
	}
	recording, err := c.RequestRecording(ctx, room.ID)
	if err != nil || recording.State != "consent_pending" {
		t.Fatalf("recording request = %+v: %v", recording, err)
	}
	if err := c.Consent(ctx, room.ID, recording.ID, recording.Revision, true); err != nil {
		t.Fatal(err)
	}
	if _, err := c.StartConsentedRecording(ctx, room.ID, recording.Revision); err == nil {
		t.Fatal("local provider started recording without an egress worker")
	}
	starting, err := c.Recording(ctx, room.ID)
	if err != nil || starting.State != "starting" || starting.Revision != recording.Revision+1 {
		t.Fatalf("recording start state = %+v: %v", starting, err)
	}
	if _, err := c.StopConsentedRecording(ctx, room.ID, starting.Revision); err == nil {
		t.Fatal("recording stop acknowledged without an egress worker")
	}
	stopping, err := c.Recording(ctx, room.ID)
	if err != nil || stopping.State != "stopping" || stopping.Revision != starting.Revision+1 {
		t.Fatalf("recording stop state = %+v: %v", stopping, err)
	}
	reconciled, err := c.Reconcile(ctx)
	if err == nil || reconciled.Examined != 1 || reconciled.Failed != 1 {
		t.Fatalf("recording reconciliation = %+v: %v", reconciled, err)
	}
	stopping, err = c.Recording(ctx, room.ID)
	if err != nil || stopping.State != "stopping" {
		t.Fatalf("pending recording = %+v: %v", stopping, err)
	}
	runCtx, cancel := context.WithCancel(ctx)
	observed := false
	err = c.RunReconciler(runCtx, func(result ReconcileResult, err error) {
		observed = result.Examined == 1 && err != nil
		cancel()
	})
	if !observed || !errors.Is(err, context.Canceled) {
		t.Fatalf("reconciliation observer = %t: %v", observed, err)
	}
	if err := c.Leave(ctx, room.ID); err != nil {
		t.Fatal(err)
	}
	var revoked bool
	if err := c.DB.QueryRow(ctx, `SELECT revoked,grant_state FROM voice_participants WHERE room_id=$1 AND wallet=$2`, room.ID, host).Scan(&revoked, &role); err != nil || !revoked || role != "revoked" {
		t.Fatalf("participant departure = %t %q: %v", revoked, role, err)
	}
	if err := c.DeleteAccount(ctx, host); err != nil {
		t.Fatal(err)
	}
	ended, err := c.Read(ctx, room.ID)
	if err != nil || ended.State != "ended" || ended.Revision != live.Revision+2 {
		t.Fatalf("account cleanup = %+v: %v", ended, err)
	}
}
