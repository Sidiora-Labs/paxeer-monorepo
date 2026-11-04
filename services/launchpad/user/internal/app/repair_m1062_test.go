package app

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
	_ "time/tzdata"

	n "github.com/Sidiora-Technologies/KindleLaunch/core/notifications"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/internal/account"
	"github.com/Sidiora-Technologies/KindleLaunch/media/user/migrations"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/secrets"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/testpki"
	pglite "github.com/elliots/go-pglite"
	"github.com/google/uuid"
)

const repairM1062Token = "repair-m1062-loopback-service-token-32-characters"

type repairM1062Fixture struct {
	store     account.Store
	principal account.Principal
	client    *n.AccountClient
	db        *sql.DB
	device    uuid.UUID
}

func repairM1062Must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}

// The SQL peers run the committed schemas in-process, without an installed
// PostgreSQL server, subprocess, external configuration or native artifact.
func repairM1062Engine(t *testing.T) *pglite.PGlite {
	t.Helper()
	engine, err := pglite.New(pglite.Config{DataDir: t.TempDir()})
	repairM1062Must(t, err)
	t.Cleanup(func() { repairM1062Must(t, engine.Close()) })
	return engine
}

func repairM1062New(t *testing.T) *repairM1062Fixture {
	t.Helper()
	ctx := context.Background()
	engine := repairM1062Engine(t)
	pool, err := engine.Pool(ctx)
	repairM1062Must(t, err)
	t.Cleanup(pool.Close)
	accountSchema, err := migrations.FS.ReadFile("00002_account_store.sql")
	repairM1062Must(t, err)
	_, err = pool.Exec(ctx, strings.Split(string(accountSchema), "-- +goose Down")[0])
	repairM1062Must(t, err)
	identitySchema, err := os.ReadFile("../../../gateway/internal/devidentity/migrations/00001_identity.sql")
	repairM1062Must(t, err)
	_, err = pool.Exec(ctx, strings.Split(string(identitySchema), "-- +goose Down")[0])
	repairM1062Must(t, err)

	f := &repairM1062Fixture{store: account.Store{DB: pool}, principal: account.Principal{AccountID: uuid.New()}, device: uuid.New()}
	wallet := "0x1062106210621062106210621062106210621062"
	repairM1062Must(t, f.store.Create(ctx, f.principal.AccountID, wallet))
	_, err = pool.Exec(ctx, `INSERT INTO identity_accounts VALUES($1,1,clock_timestamp())`, f.principal.AccountID)
	repairM1062Must(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO identity_wallet_links VALUES($1,$2,125,$3,'private','verified',clock_timestamp())`, uuid.New(), f.principal.AccountID, wallet)
	repairM1062Must(t, err)
	session := uuid.New()
	_, err = pool.Exec(ctx, `INSERT INTO identity_sessions(session_id,account_id,wallet,auth_epoch,access_hash,csrf_hash,issued_at,access_expires_at,refresh_expires_at,reauthenticated_at,last_seen_at,device_label)
		VALUES($1,$2,$3,1,$4,$5,clock_timestamp(),clock_timestamp()+interval '1 hour',clock_timestamp()+interval '1 day',clock_timestamp(),clock_timestamp(),'repair-m1062')`, session, f.principal.AccountID, wallet, []byte(uuid.NewString()), []byte(uuid.NewString()))
	repairM1062Must(t, err)
	runtime := &account.Runtime{EndpointKey: bytes.Repeat([]byte{62}, 32)}
	sealed, err := account.Seal(runtime.EndpointKey, []byte("repair-m1062-device-token"), []byte(f.principal.AccountID.String()))
	repairM1062Must(t, err)
	fingerprint := sha256.Sum256([]byte("repair-m1062-device-token"))
	_, err = pool.Exec(ctx, `INSERT INTO account_device_installation VALUES($1,$2,$3,'fcm','android',clock_timestamp(),false)`, f.device, f.principal.AccountID, session)
	repairM1062Must(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO account_endpoint VALUES($1,$2,$3,'android',0,clock_timestamp(),false,$4,'fcm',$5)`, fingerprint[:], f.principal.AccountID, sealed, f.device, session)
	repairM1062Must(t, err)

	// A separate engine avoids sharing the embedded peer's single session
	// between the worker's outbox and the concurrent HTTP authority request.
	f.db = repairM1062Engine(t).DB()
	_, err = f.db.Exec(n.Schema)
	repairM1062Must(t, err)
	pki := testpki.New(t)
	serverLeaf, clientLeaf := pki.Leaf("user", "user.test"), pki.Leaf("notifications", "notifications.test")
	server := httptest.NewUnstartedServer(accountNotificationAuthority(f.store, runtime, repairM1062Token, clientLeaf.SHA256))
	server.TLS = &tls.Config{MinVersion: tls.VersionTLS12, Certificates: []tls.Certificate{serverLeaf.Pair}, ClientAuth: tls.RequireAndVerifyClientCert, ClientCAs: pki.Roots()}
	server.StartTLS()
	t.Cleanup(server.Close)
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{TLSClientConfig: &tls.Config{MinVersion: tls.VersionTLS12, RootCAs: pki.Roots(), Certificates: []tls.Certificate{clientLeaf.Pair}}}}
	t.Cleanup(client.CloseIdleConnections)
	// Only a test-generated loopback credential is installed; the gate needs
	// no preconfigured environment variables or credentials.
	t.Setenv("REPAIR_M1062_LOOPBACK_TOKEN", repairM1062Token)
	credentials, err := n.NewCredentials(ctx, secrets.Options{Provider: secrets.ProviderEnv}, "REPAIR_M1062_LOOPBACK_TOKEN")
	repairM1062Must(t, err)
	f.client, err = n.NewAccountClient(server.URL, client, credentials, "REPAIR_M1062_LOOPBACK_TOKEN")
	repairM1062Must(t, err)
	return f
}

func (f *repairM1062Fixture) save(t *testing.T, display, zone, start, end string) n.DeliveryPreference {
	t.Helper()
	ctx := context.Background()
	before, err := f.store.ReadPreferences(ctx, f.principal, f.principal.AccountID)
	repairM1062Must(t, err)
	yes, no := true, false
	patch := account.ExtendedPreferencesPatch{
		PreferencesPatch: account.PreferencesPatch{
			ExpectedRevision: before.Revision, Timezone: &display,
			Channels:   &account.InputChannels{Push: &yes, InApp: &yes, Email: &no},
			QuietHours: &account.QuietHours{Timezone: zone, Start: start, End: end},
		},
		Consents: []account.ConsentPatch{{Category: "chat", Channel: "push", Enabled: true}},
	}
	repairM1062Must(t, f.store.PatchExtendedPreferences(ctx, f.principal, patch))
	saved, err := f.store.ReadPreferences(ctx, f.principal, f.principal.AccountID)
	repairM1062Must(t, err)
	if saved.Timezone != display || saved.QuietHours != *patch.QuietHours {
		t.Fatalf("preferences did not retain independent zones: %+v", saved)
	}
	revision, err := strconv.ParseUint(saved.Revision, 10, 64)
	repairM1062Must(t, err)
	_, err = f.store.DB.Exec(ctx, `UPDATE account_endpoint SET consent_revision=$2 WHERE account_id=$1`, f.principal.AccountID, revision)
	repairM1062Must(t, err)
	p, err := f.client.CurrentDeliveryPreference(ctx, f.principal.AccountID.String())
	repairM1062Must(t, err)
	startTime, err := time.Parse("15:04", start)
	repairM1062Must(t, err)
	endTime, err := time.Parse("15:04", end)
	repairM1062Must(t, err)
	if p.AccountID != f.principal.AccountID.String() || p.Revision != revision || !p.Linked || p.Timezone != display || p.QuietHours == nil || p.QuietHours.Timezone != zone || p.QuietHours.StartMinute != startTime.Hour()*60+startTime.Minute() || p.QuietHours.EndMinute != endTime.Hour()*60+endTime.Minute() || !p.CategoryChannels["chat"]["push"] {
		t.Fatalf("saved quiet-hours settings changed across producer/HTTP/consumer: %+v", p)
	}
	return p
}

func (f *repairM1062Fixture) enqueue(t *testing.T, at time.Time) n.InboxItem {
	t.Helper()
	consumer := n.RecipientConsumer{Store: n.Store{DB: f.db}, Authority: f.client}
	item, inserted, err := consumer.EnqueueChat(context.Background(), uuid.NewString(), f.principal.AccountID.String(), "repair-m1062-chat", at.Add(-time.Minute))
	repairM1062Must(t, err)
	if !inserted || item.ID == "" {
		t.Fatal("real recipient consumer did not enqueue notification")
	}
	return item
}

func TestRepairM_1062_DifferingZonesAndDST(t *testing.T) {
	f := repairM1062New(t)
	for _, tc := range []struct {
		name, display, zone, start, end, stamp string
		quiet                                  bool
	}{
		{"local_daytime_quiet", "UTC", "America/Los_Angeles", "09:00", "10:00", "2026-01-15T17:30:00Z", true},
		{"display_daytime_quiet_only", "UTC", "America/Los_Angeles", "09:00", "10:00", "2026-01-15T09:30:00Z", false},
		{"overnight_before_start", "UTC", "America/New_York", "22:00", "07:00", "2026-01-16T02:59:00Z", false},
		{"overnight_at_start", "UTC", "America/New_York", "22:00", "07:00", "2026-01-16T03:00:00Z", true},
		{"overnight_after_midnight", "UTC", "America/New_York", "22:00", "07:00", "2026-01-16T10:30:00Z", true},
		{"overnight_at_end", "UTC", "America/New_York", "22:00", "07:00", "2026-01-16T12:00:00Z", false},
		{"spring_before_jump", "UTC", "Europe/Berlin", "01:30", "03:30", "2026-03-29T00:45:00Z", true},
		{"spring_after_jump", "UTC", "Europe/Berlin", "01:30", "03:30", "2026-03-29T01:15:00Z", true},
		{"spring_end_boundary", "UTC", "Europe/Berlin", "01:30", "03:30", "2026-03-29T01:30:00Z", false},
		{"fall_first_repeated_hour", "UTC", "Europe/Berlin", "02:00", "03:00", "2026-10-25T00:30:00Z", true},
		{"fall_second_repeated_hour", "UTC", "Europe/Berlin", "02:00", "03:00", "2026-10-25T01:30:00Z", true},
		{"fall_after_repeated_hour", "UTC", "Europe/Berlin", "02:00", "03:00", "2026-10-25T02:00:00Z", false},
		{"equal_boundaries", "Pacific/Auckland", "Europe/Berlin", "02:00", "02:00", "2026-10-25T01:30:00Z", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			p := f.save(t, tc.display, tc.zone, tc.start, tc.end)
			at, err := time.Parse(time.RFC3339, tc.stamp)
			repairM1062Must(t, err)
			item := f.enqueue(t, at)
			decision, err := n.Decide(p, item, "push", at)
			repairM1062Must(t, err)
			reason := "consented"
			if tc.quiet {
				reason = "quiet_hours"
			}
			if decision.Send == tc.quiet || decision.Reason != reason || decision.ConsentRevision != p.Revision {
				t.Fatalf("%s: delivery ignored saved zone: %+v", tc.stamp, decision)
			}
		})
	}
}

func TestRepairM_1062_WorkerPersistsQuietHours(t *testing.T) {
	f := repairM1062New(t)
	zone := "Etc/GMT+12"
	loc, err := time.LoadLocation(zone)
	repairM1062Must(t, err)
	now := time.Now()
	// This two-hour window is twelve hours away from the display-zone clock,
	// with an hour of margin so the proof is independent of minute boundaries.
	start := now.In(loc).Add(-time.Hour).Format("15:04")
	end := now.In(loc).Add(time.Hour).Format("15:04")
	p := f.save(t, "UTC", zone, start, end)
	item := f.enqueue(t, now)
	worker := n.Worker{Store: n.Store{DB: f.db}, Authority: f.client, Owner: "repair-m1062", DeepLinkOrigin: "https://launchpad.test"}
	// No adapter is installed: reaching provider selection instead of enforcing
	// quiet hours produces provider_unavailable and fails the assertion below.
	repairM1062Must(t, worker.Once(context.Background(), 1))
	var raw []byte
	var terminal bool
	err = f.db.QueryRow(`SELECT deliveries,terminal FROM discovery_notifications.outbox WHERE id=$1`, item.ID).Scan(&raw, &terminal)
	repairM1062Must(t, err)
	var deliveries []n.DeliveryAttempt
	repairM1062Must(t, json.Unmarshal(raw, &deliveries))
	if len(deliveries) != 1 {
		t.Fatalf("real target enumeration did not queue one delivery: %s", raw)
	}
	d := deliveries[0]
	if terminal || d.NotificationID != item.ID || d.TargetID != f.device.String() || d.Attempt != 1 || d.State != "retry" || d.Reason != "quiet_hours" || d.ConsentRevision != p.Revision || d.ProviderReceipt != "" || !d.NextAt.After(now) {
		t.Fatalf("worker did not durably defer quiet-hours delivery: %+v (terminal=%v)", d, terminal)
	}
}

func TestRepairM_1062_ExplicitInvalidZoneAndLegacyWire(t *testing.T) {
	f := repairM1062New(t)
	p := f.save(t, "Europe/Berlin", "UTC", "02:00", "03:00")
	at := time.Date(2026, 10, 25, 0, 30, 0, 0, time.UTC)
	item := f.enqueue(t, at)
	p.QuietHours.Timezone = "Invalid/QuietTimezone"
	d, err := n.Decide(p, item, "push", at)
	if !errors.Is(err, n.ErrInvalid) || d.Send {
		t.Fatalf("explicit invalid quiet zone inherited display zone: %+v, %v", d, err)
	}
	// The old wire omitted the new field. Preserve that documented contract and
	// the existing notification model tests while current producers send a zone.
	p.QuietHours.Timezone = ""
	d, err = n.Decide(p, item, "push", at)
	repairM1062Must(t, err)
	if d.Send || d.Reason != "quiet_hours" {
		t.Fatalf("legacy preference no longer uses its display zone: %+v", d)
	}
	p.QuietHours.Timezone = "UTC"
	d, err = n.Decide(p, item, "push", at)
	repairM1062Must(t, err)
	if !d.Send || d.Reason != "consented" {
		t.Fatalf("explicit UTC quiet zone lost to display zone: %+v", d)
	}
	p.QuietHours.StartMinute = -1
	d, err = n.Decide(p, item, "push", at)
	if !errors.Is(err, n.ErrInvalid) || d.Send {
		t.Fatalf("quiet-minute validation weakened: %+v, %v", d, err)
	}
}
