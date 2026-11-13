package store_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/api/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/core/discovery"
)

func TestDevelopmentDiscoveryReadsPublishedSourcesAndCallouts(t *testing.T) {
	st, db, _ := newStore(t)
	ctx := context.Background()
	const pool = "0x1111111111111111111111111111111111111111"
	if sqlDB := st.DevelopmentDiscoveryDB(); sqlDB == nil {
		t.Fatal("development discovery database unavailable")
	} else if err := sqlDB.PingContext(ctx); err != nil {
		t.Fatal(err)
	}
	if refs, keys, err := st.DevelopmentDiscoverySources(ctx, nil, 2); err != nil || len(refs) != 0 || len(keys) != 0 {
		t.Fatalf("empty source feed: %v, %v, %v", refs, keys, err)
	}
	if _, found, err := st.DevelopmentDiscoverySource(ctx, "event-1"); err != nil || found {
		t.Fatalf("missing source: %v, %v", found, err)
	}
	if callouts, keys, err := st.DevelopmentPoolCallouts(ctx, pool, nil, 2); err != nil || len(callouts) != 0 || len(keys) != 0 {
		t.Fatalf("empty callouts: %v, %v, %v", callouts, keys, err)
	}
	if _, _, err := st.DevelopmentDiscoverySources(ctx, nil, 0); !errors.Is(err, store.ErrDevelopmentScope) {
		t.Fatalf("zero source limit: %v", err)
	}
	if _, _, err := st.DevelopmentPoolCallouts(ctx, "invalid", nil, 2); !errors.Is(err, store.ErrDevelopmentScope) {
		t.Fatalf("invalid callout pool: %v", err)
	}
	if _, found, err := st.DevelopmentDiscoverySource(ctx, ""); !errors.Is(err, store.ErrDevelopmentScope) || found {
		t.Fatalf("empty source id: %v, %v", found, err)
	}
	if _, err := db.Exec(ctx, `CREATE SCHEMA narrative_discovery;
		CREATE TABLE narrative_discovery.sources(id text PRIMARY KEY,event_id text,document jsonb,published_at timestamptz);
		CREATE TABLE narrative_discovery.callouts(id text PRIMARY KEY,document jsonb)`); err != nil {
		t.Fatal(err)
	}
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	ref := discovery.SourceRef{ID: "source-1", EventID: "event-1", Kind: "pool", Pools: []string{pool}, SourceRevision: 1, VisibilityRevision: 1, PublishedAt: at, Public: true, Authorized: true}
	raw, err := json.Marshal(ref)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO narrative_discovery.sources VALUES($1,$2,$3,$4)`, ref.ID, ref.EventID, raw, at); err != nil {
		t.Fatal(err)
	}
	callout := discovery.Callout{ID: "callout-1", PoolAddress: pool, CreatedAt: at, ModerationStatus: "approved", Body: "Opening day", Revision: 1}
	raw, err = json.Marshal(callout)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, `INSERT INTO narrative_discovery.callouts VALUES($1,$2)`, callout.ID, raw); err != nil {
		t.Fatal(err)
	}
	refs, keys, err := st.DevelopmentDiscoverySources(ctx, nil, 2)
	if err != nil || len(refs) != 1 || refs[0].EventID != ref.EventID || len(keys) != 1 || keys[0].ID != ref.EventID {
		t.Fatalf("source feed: %v, %v, %v", refs, keys, err)
	}
	read, found, err := st.DevelopmentDiscoverySource(ctx, ref.ID)
	if err != nil || !found || read.EventID != ref.EventID {
		t.Fatalf("source lookup: %+v, %v, %v", read, found, err)
	}
	callouts, calloutKeys, err := st.DevelopmentPoolCallouts(ctx, pool, nil, 2)
	if err != nil || len(callouts) != 1 || callouts[0].ID != callout.ID || len(calloutKeys) != 1 || calloutKeys[0].ID != callout.ID {
		t.Fatalf("callout feed: %v, %v, %v", callouts, calloutKeys, err)
	}
	refs, _, err = st.DevelopmentDiscoverySources(ctx, &keys[0], 2)
	if err != nil || len(refs) != 0 {
		t.Fatalf("source keyset: %v, %v", refs, err)
	}
	callouts, _, err = st.DevelopmentPoolCallouts(ctx, pool, &calloutKeys[0], 2)
	if err != nil || len(callouts) != 0 {
		t.Fatalf("callout keyset: %v, %v", callouts, err)
	}
	if _, err := db.Exec(ctx, `UPDATE narrative_discovery.sources SET document='{}'::jsonb`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.DevelopmentDiscoverySources(ctx, nil, 2); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("invalid source feed: %v", err)
	}
	if _, found, err := st.DevelopmentDiscoverySource(ctx, ref.ID); found || !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("invalid source lookup: found=%t error=%v", found, err)
	}
	if _, err := db.Exec(ctx, `UPDATE narrative_discovery.callouts SET document=jsonb_set(document,'{ID}','"different"')`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.DevelopmentPoolCallouts(ctx, pool, nil, 2); !errors.Is(err, store.ErrDataIntegrity) {
		t.Fatalf("mismatched callout identity: %v", err)
	}
	if _, err := db.Exec(ctx, `ALTER TABLE narrative_discovery.sources DROP COLUMN event_id`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.DevelopmentDiscoverySources(ctx, nil, 2); err == nil {
		t.Fatal("incomplete source schema accepted")
	}
	if _, err := db.Exec(ctx, `ALTER TABLE narrative_discovery.callouts DROP COLUMN document`); err != nil {
		t.Fatal(err)
	}
	if _, _, err := st.DevelopmentPoolCallouts(ctx, pool, nil, 2); err == nil {
		t.Fatal("incomplete callout schema accepted")
	}

}
