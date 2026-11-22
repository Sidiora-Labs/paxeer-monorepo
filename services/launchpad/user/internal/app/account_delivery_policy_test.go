package app

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestClaimPolicySubjectsAccountEvents(t *testing.T) {
	f := newNotificationEligibilityFixture(t)
	ctx := context.Background()
	identity, err := delivery.NewStore(f.store.DB, "public", delivery.Policy{Subjects: map[string]bool{"identity.account.v1": true, f.runtime.Subjects["preferences.updated"]: false}, EventTypes: map[string]bool{"identity.account.revision": true}})
	if err != nil {
		t.Fatal(err)
	}
	const query = "SELECT jsonb_agg(to_jsonb(o) ORDER BY event_id) FROM public.delivery_outbox o"
	var before, after []byte
	if err := f.store.DB.QueryRow(ctx, query).Scan(&before); err != nil {
		t.Fatal(err)
	}
	leases, err := identity.Claim(ctx, "identity_relay", 1000, time.Minute)
	if err != nil || len(leases) != 0 {
		t.Fatalf("identity policy claimed account events: %+v %v", leases, err)
	}
	stats, err := identity.Stats(ctx)
	if err != nil || stats != (delivery.Stats{}) {
		t.Fatalf("identity policy counted account events: %+v %v", stats, err)
	}
	var count int64
	var unleased bool
	if err := f.store.DB.QueryRow(ctx, "SELECT count(*),bool_and(state='pending' AND lease_owner IS NULL AND lease_until IS NULL AND attempts=0 AND fence=0) FROM public.delivery_outbox").Scan(&count, &unleased); err != nil || count == 0 || !unleased {
		t.Fatalf("account events did not remain unleased: %d %t %v", count, unleased, err)
	}
	accountStats, err := f.runtime.Delivery.Stats(ctx)
	if err != nil || accountStats.PendingRows != count || accountStats.PendingBytes <= 0 || accountStats.QuarantinedRows != 0 {
		t.Fatalf("account event statistics: %+v %v", accountStats, err)
	}
	if err := f.store.DB.QueryRow(ctx, query).Scan(&after); err != nil || !bytes.Equal(before, after) {
		t.Fatalf("account events changed under the identity policy: %v", err)
	}
}
