package auth

import (
	"bytes"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
)

func TestClaimPolicySubjectsIdentityEvents(t *testing.T) {
	f := ownerHTTPFixture(t)
	f.loginHTTP(t)
	var schema string
	if err := f.pool.QueryRow(f.ctx, "SELECT current_schema()").Scan(&schema); err != nil {
		t.Fatal(err)
	}
	accounts, err := delivery.NewStore(f.pool, schema, delivery.Policy{Subjects: map[string]bool{"users.preferences.updated": true, IdentityRevisionSubject: false}, EventTypes: map[string]bool{"preferences.updated": true}})
	if err != nil {
		t.Fatal(err)
	}
	query := "SELECT jsonb_agg(to_jsonb(o) ORDER BY event_id) FROM " + pgx.Identifier{schema, "delivery_outbox"}.Sanitize() + " o"
	var before, after []byte
	if err := f.pool.QueryRow(f.ctx, query).Scan(&before); err != nil {
		t.Fatal(err)
	}
	leases, err := accounts.Claim(f.ctx, "account_relay", 1000, time.Minute)
	if err != nil || len(leases) != 0 {
		t.Fatalf("account policy claimed identity events: %+v %v", leases, err)
	}
	stats, err := accounts.Stats(f.ctx)
	if err != nil || stats != (delivery.Stats{}) {
		t.Fatalf("account policy counted identity events: %+v %v", stats, err)
	}
	identityStats, err := f.s.events.Store.Stats(f.ctx)
	if err != nil || identityStats.PendingRows != 1 || identityStats.PendingBytes <= 0 || identityStats.QuarantinedRows != 0 {
		t.Fatalf("identity event statistics: %+v %v", identityStats, err)
	}
	var pending bool
	if err := f.pool.QueryRow(f.ctx, "SELECT bool_and(state='pending' AND lease_owner IS NULL AND lease_until IS NULL AND attempts=0 AND fence=0) FROM delivery_outbox").Scan(&pending); err != nil || !pending {
		t.Fatalf("identity events did not remain unleased: %t %v", pending, err)
	}
	if err := f.pool.QueryRow(f.ctx, query).Scan(&after); err != nil || !bytes.Equal(before, after) {
		t.Fatalf("identity events changed under the account policy: %v", err)
	}
}
