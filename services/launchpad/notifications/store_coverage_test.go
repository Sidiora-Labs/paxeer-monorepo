package notifications

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestStoreDeletesOnlyOwnedItemAndRetainsRecentInbox(t *testing.T) {
	ctx := context.Background()
	db, _ := repair958Open(t, t.TempDir())
	repair958Schema(t, db)
	store := Store{DB: db}
	created := time.Now().UTC()
	item, ok, err := store.CommitTrigger(ctx, InboxItem{Identity: Identity{AccountID: "store-account", TriggerEventID: "event", TemplateVersion: "v2"}, Category: "chat", Summary: "New message", CreatedAt: created}, []DeliveryAttempt{})
	if err != nil || !ok {
		t.Fatalf("committed inbox: %+v %v %v", item, ok, err)
	}
	if err := store.DeleteItem(ctx, "another-account", item.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("foreign deletion: %v", err)
	}
	kept, err := store.Retain(ctx, created, 30, 10)
	if err != nil || kept != 0 {
		t.Fatalf("recent inbox retained: %d %v", kept, err)
	}
	if err := store.DeleteItem(ctx, item.Identity.AccountID, item.ID); err != nil {
		t.Fatal(err)
	}
	items, err := store.List(ctx, item.Identity.AccountID, 10)
	if err != nil || len(items) != 0 {
		t.Fatalf("deleted inbox: %+v %v", items, err)
	}
	if _, err := NewStore(nil); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("nil pool admitted: %v", err)
	}
}

func TestStoreRejectsInvalidClaimsAndCursorInputs(t *testing.T) {
	ctx := context.Background()
	db, _ := repair958Open(t, t.TempDir())
	repair958Schema(t, db)
	store := Store{DB: db}
	valid := InboxItem{Identity: Identity{AccountID: "scope-account", TriggerEventID: "event", TemplateVersion: "v2"}, Category: "chat", Summary: "New message", CreatedAt: time.Now().UTC()}
	for _, item := range []InboxItem{
		{Identity: Identity{AccountID: "", TriggerEventID: "event", TemplateVersion: "v2"}, Summary: "New message", CreatedAt: valid.CreatedAt},
		{Identity: valid.Identity, Summary: "", CreatedAt: valid.CreatedAt},
		{Identity: valid.Identity, Summary: "New message"},
	} {
		if _, _, err := store.CommitTrigger(ctx, item, nil); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid trigger admitted: %+v %v", item, err)
		}
	}
	if _, _, err := store.CommitTrigger(ctx, valid, []DeliveryAttempt{{Channel: channelPush, TargetID: "device"}, {Channel: channelPush, TargetID: "device"}}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("duplicate target admitted: %v", err)
	}
	if _, err := store.Claim(ctx, "", time.Now(), 1); !errors.Is(err, ErrInvalid) {
		t.Fatalf("anonymous claim: %v", err)
	}
	if _, err := store.Claim(ctx, "worker", time.Now(), 101); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unbounded claim: %v", err)
	}
	if err := store.Complete(ctx, "worker", "invalid", DeliveryAttempt{State: stateAccepted}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid completion: %v", err)
	}
	if err := store.Renew(ctx, "worker", "invalid"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid renewal: %v", err)
	}
	if err := store.Release(ctx, "worker", "invalid"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid release: %v", err)
	}
	if err := store.BeginSend(ctx, "worker", "invalid", DeliveryAttempt{}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid send claim: %v", err)
	}
	if _, err := store.List(ctx, "", 10); !errors.Is(err, ErrInvalid) {
		t.Fatalf("anonymous list: %v", err)
	}
	if _, err := store.ListPage(ctx, "scope-account", false, "invalid", 10); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid page cursor: %v", err)
	}
	if _, err := store.Retain(ctx, time.Now(), 91, 10); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unbounded retention: %v", err)
	}
	if err := store.DeleteAccount(ctx, ""); !errors.Is(err, ErrInvalid) {
		t.Fatalf("empty deletion scope: %v", err)
	}
	item, created, err := store.CommitTrigger(ctx, valid, []DeliveryAttempt{{Channel: channelPush, TargetID: "device"}})
	if err != nil || !created {
		t.Fatalf("committed scoped item: %+v %v %v", item, created, err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE discovery_notifications.outbox SET deliveries='{}'::jsonb WHERE id=$1`, item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Claim(ctx, "worker", time.Now(), 1); err == nil {
		t.Fatal("malformed delivery document accepted")
	}
	if _, err := db.ExecContext(ctx, `UPDATE discovery_notifications.inbox SET document='[]'::jsonb WHERE id=$1`, item.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := store.List(ctx, item.Identity.AccountID, 10); err == nil {
		t.Fatal("malformed inbox document accepted")
	}
	if _, err := store.ListPage(ctx, item.Identity.AccountID, false, "", 10); err == nil {
		t.Fatal("malformed page document accepted")
	}
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	other := valid
	other.Identity.TriggerEventID = "canceled-event"
	for _, operation := range []struct {
		name string
		call func() error
	}{
		{"commit", func() error { _, _, e := store.CommitTrigger(canceled, other, []DeliveryAttempt{}); return e }},
		{"claim", func() error { _, e := store.Claim(canceled, "worker", time.Now(), 1); return e }},
		{"delete item", func() error { return store.DeleteItem(canceled, item.Identity.AccountID, item.ID) }},
		{"list", func() error { _, e := store.List(canceled, item.Identity.AccountID, 10); return e }},
		{"page", func() error { _, e := store.ListPage(canceled, item.Identity.AccountID, false, "", 10); return e }},
		{"retain", func() error { _, e := store.Retain(canceled, time.Now(), 30, 10); return e }},
		{"delete account", func() error { return store.DeleteAccount(canceled, item.Identity.AccountID) }},
		{"renew", func() error { return store.Renew(canceled, "worker", item.ID+":1") }},
		{"release", func() error { return store.Release(canceled, "worker", item.ID+":1") }},
		{"complete", func() error {
			return store.Complete(canceled, "worker", item.ID+":1", DeliveryAttempt{State: stateRejected})
		}},
		{"begin send", func() error {
			return store.BeginSend(canceled, "worker", item.ID+":1", DeliveryAttempt{NotificationID: item.ID, Attempt: 1, TargetRevision: 1, ConsentRevision: 1})
		}},
	} {
		if err := operation.call(); !errors.Is(err, context.Canceled) {
			t.Fatalf("%s continued after cancellation: %v", operation.name, err)
		}
	}
}

func TestTriggerTransactionRollsBackWhenOutboxCannotCommit(t *testing.T) {
	ctx := context.Background()
	db, _ := repair958Open(t, t.TempDir())
	repair958Schema(t, db)
	if _, err := db.ExecContext(ctx, `DROP TABLE discovery_notifications.outbox`); err != nil {
		t.Fatal(err)
	}
	store := Store{DB: db}
	item := InboxItem{Identity: Identity{AccountID: "atomic-account", TriggerEventID: "event", TemplateVersion: "v2"}, Category: "chat", Summary: "New message", CreatedAt: time.Now().UTC()}
	if _, created, err := store.CommitTrigger(ctx, item, []DeliveryAttempt{}); err == nil || created {
		t.Fatalf("inbox committed without outbox: %v %v", created, err)
	}
	items, err := store.List(ctx, item.Identity.AccountID, 10)
	if err != nil || len(items) != 0 {
		t.Fatalf("partial inbox effect: %+v %v", items, err)
	}
	if _, err := store.Claim(ctx, "worker", time.Now(), 1); err == nil {
		t.Fatal("missing outbox claimed")
	}
	if _, err := store.ListPage(ctx, item.Identity.AccountID, false, "", 10); err == nil {
		t.Fatal("page read concealed missing outbox")
	}
}

func TestSendClaimRejectsStaleTargetRevisionAndRetainsFence(t *testing.T) {
	ctx := context.Background()
	db, _ := repair958Open(t, t.TempDir())
	repair958Schema(t, db)
	store := Store{DB: db}
	item, created, err := store.CommitTrigger(ctx, InboxItem{Identity: Identity{AccountID: "send-account", TriggerEventID: "send-event", TemplateVersion: "v2"}, Category: categoryWatchlist, Summary: "Threshold crossed", CreatedAt: time.Now().UTC()}, []DeliveryAttempt{{Channel: channelPush, TargetID: "device", TargetRevision: 7, ConsentRevision: 3}})
	if err != nil || !created {
		t.Fatalf("send trigger: %+v %v %v", item, created, err)
	}
	claims, err := store.Claim(ctx, "owner", time.Now(), 1)
	if err != nil || len(claims) != 1 {
		t.Fatalf("send claim: %+v %v", claims, err)
	}
	claim := claims[0]
	if err := store.Renew(ctx, "another-owner", claim.ID); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("foreign lease renewed: %v", err)
	}
	if err := store.Renew(ctx, "owner", item.ID+":bad"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unparseable fence renewed: %v", err)
	}
	attempt := claim.Deliveries[0]
	attempt.Attempt = 1
	attempt.NextAt = time.Now().Add(time.Minute)
	attempt.TargetRevision = 8
	if err := store.BeginSend(ctx, "owner", item.ID+":bad", attempt); !errors.Is(err, ErrInvalid) {
		t.Fatalf("unparseable send fence: %v", err)
	}
	if err := store.BeginSend(ctx, "owner", claim.ID, attempt); !errors.Is(err, ErrInvalid) {
		t.Fatalf("stale target revision sent: %v", err)
	}
	attempt.TargetRevision = 7
	if err := store.BeginSend(ctx, "owner", claim.ID, attempt); err != nil {
		t.Fatal(err)
	}
	attempt.State = stateAccepted
	if err := store.Complete(ctx, "owner", claim.ID, attempt); !errors.Is(err, ErrInvalid) {
		t.Fatalf("acceptance without receipt: %v", err)
	}
	attempt.ProviderReceipt = "receipt-1"
	if err := store.Complete(ctx, "owner", claim.ID, attempt); err != nil {
		t.Fatal(err)
	}
	if err := store.Release(ctx, "owner", claim.ID); err != nil {
		t.Fatal(err)
	}
}
