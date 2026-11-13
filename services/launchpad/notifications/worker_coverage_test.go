package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"
)

func TestWorkerPersistsConsentRevocationAndProviderRetry(t *testing.T) {
	ctx := context.Background()
	db, _ := repair958Open(t, t.TempDir())
	repair958Schema(t, db)
	peer := repair958Authorities(t)
	options := repair958Credentials(t)
	credentials, err := NewCredentials(ctx, options, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	authority, err := NewAccountClient(peer.server.URL, peer.client, credentials, "REPAIR958_D11")
	if err != nil {
		t.Fatal(err)
	}
	store := Store{DB: db}
	worker := Worker{Store: store, Authority: authority, Owner: "worker", DeepLinkOrigin: "https://example.test"}
	commit := func(event string) InboxItem {
		t.Helper()
		item, created, e := store.CommitTrigger(ctx, InboxItem{Identity: Identity{AccountID: peer.account.AccountID, TriggerEventID: event, TemplateVersion: "v2"}, Category: categoryWatchlist, Summary: "Watched pool changed", CreatedAt: time.Now().UTC()}, []DeliveryAttempt{{Channel: channelPush, TargetID: peer.target.DeviceInstallationID, TargetRevision: peer.target.Revision, ConsentRevision: peer.target.ConsentRevision}})
		if e != nil || !created {
			t.Fatalf("commit %s: %+v %v %v", event, item, created, e)
		}
		return item
	}
	state := func(item InboxItem) DeliveryAttempt {
		t.Helper()
		var raw []byte
		if e := db.QueryRowContext(ctx, `SELECT deliveries FROM discovery_notifications.outbox WHERE id=$1`, item.ID).Scan(&raw); e != nil {
			t.Fatal(e)
		}
		var deliveries []DeliveryAttempt
		if e := json.Unmarshal(raw, &deliveries); e != nil || len(deliveries) != 1 {
			t.Fatalf("delivery state: %s %v", raw, e)
		}
		return deliveries[0]
	}
	peer.preference.CategoryChannels[categoryWatchlist][channelPush] = false
	consent := commit("consent-off")
	if err := worker.Once(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if got := state(consent); got.State != stateSuppressed || got.Reason == "" {
		t.Fatalf("suppressed consent: %+v", got)
	}
	peer.preference.CategoryChannels[categoryWatchlist][channelPush] = true
	peer.target.Status = "revoked"
	revoked := commit("revoked-target")
	if err := worker.Once(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if got := state(revoked); got.State != stateSuppressed || got.Reason != "target_revoked" {
		t.Fatalf("revoked target: %+v", got)
	}
	peer.target.Status = targetActive
	unavailable := commit("provider-unavailable")
	if err := worker.Once(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if got := state(unavailable); got.State != stateRetry || got.Reason != reasonProviderUnavailable || got.NextAt.Before(time.Now()) {
		t.Fatalf("provider retry: %+v", got)
	}
	peer.preference.Linked = false
	unlinked := commit("unlinked-consent")
	if err := worker.Once(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if got := state(unlinked); got.State != stateRetry || got.Reason != reasonConsentUnavailable {
		t.Fatalf("unlinked consent: %+v", got)
	}
	peer.preference.Linked = true
	minute := time.Now().UTC().Hour()*60 + time.Now().UTC().Minute()
	peer.preference.QuietHours = &QuietHours{StartMinute: (minute + 1438) % 1440, EndMinute: (minute + 2) % 1440}
	quiet := commit("quiet-hours")
	if err := worker.Once(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if got := state(quiet); got.State != stateRetry || got.Reason != "quiet_hours" {
		t.Fatalf("quiet hours: %+v", got)
	}
	peer.preference.QuietHours = &QuietHours{Timezone: "Unknown/Timezone"}
	badTimezone := commit("bad-timezone")
	if err := worker.Once(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if got := state(badTimezone); got.State != stateRetry || got.Reason != reasonConsentUnavailable {
		t.Fatalf("invalid quiet hours timezone: %+v", got)
	}
	peer.preference.QuietHours = nil
	peer.target.Status = "unknown"
	targetUnavailable := commit("target-unavailable")
	if err := worker.Once(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if got := state(targetUnavailable); got.State != stateRetry || got.Reason != "target_unavailable" {
		t.Fatalf("target authority unavailable: %+v", got)
	}
	peer.target.Status = targetActive
	peer.preference.Timezone = "Unknown/Timezone"
	preferenceUnavailable := commit("preference-unavailable")
	if err := worker.Once(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if got := state(preferenceUnavailable); got.State != stateRetry || got.Reason != reasonConsentUnavailable {
		t.Fatalf("preference authority unavailable: %+v", got)
	}
	peer.preference.Timezone = "UTC"
	missingTarget, created, err := store.CommitTrigger(ctx, InboxItem{Identity: Identity{AccountID: peer.account.AccountID, TriggerEventID: "missing-target", TemplateVersion: "v2"}, Category: categoryWatchlist, Summary: "Watched pool changed", CreatedAt: time.Now().UTC()}, []DeliveryAttempt{{Channel: channelPush, TargetID: "foreign-device", TargetRevision: peer.target.Revision, ConsentRevision: peer.target.ConsentRevision}})
	if err != nil || !created {
		t.Fatalf("missing target trigger: %+v %v %v", missingTarget, created, err)
	}
	if err := worker.Once(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if got := state(missingTarget); got.State != stateSuppressed || got.Reason != "target_revoked" {
		t.Fatalf("missing target: %+v", got)
	}
	expiredAt := time.Now().UTC().Add(-time.Minute)
	expired, created, err := store.CommitTrigger(ctx, InboxItem{Identity: Identity{AccountID: peer.account.AccountID, TriggerEventID: "expired-notification", TemplateVersion: "v2"}, Category: "chat", Summary: "Old message", CreatedAt: expiredAt.Add(-time.Minute), ExpiresAt: expiredAt}, []DeliveryAttempt{{Channel: channelPush, TargetID: peer.target.DeviceInstallationID, TargetRevision: peer.target.Revision, ConsentRevision: peer.target.ConsentRevision}})
	if err != nil || !created {
		t.Fatalf("expired trigger: %+v %v %v", expired, created, err)
	}
	if err := worker.Once(ctx, 1); err != nil {
		t.Fatal(err)
	}
	if got := state(expired); got.State != stateSuppressed || got.Reason != "expired" {
		t.Fatalf("expired delivery: %+v", got)
	}
	if err := (Worker{}).Once(ctx, 0); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid worker admitted: %v", err)
	}
}
