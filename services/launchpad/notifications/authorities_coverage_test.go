package notifications

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCanonicalPoolAuthorityRejectsUnboundCanonicalRows(t *testing.T) {
	ctx := context.Background()
	db, _ := repair958Open(t, t.TempDir())
	if _, err := db.ExecContext(ctx, `CREATE SCHEMA indexer; CREATE TABLE indexer.indexer_canonical_creations_v1(pool text,token text,creator text,height bigint,block_hash text,transaction_hash text,log_index bigint,block_timestamp bigint)`); err != nil {
		t.Fatal(err)
	}
	pool := "0x3333333333333333333333333333333333333333"
	canonical := CanonicalPools{DB: db}
	if found, err := (CanonicalPools{}).CanonicalPool(ctx, pool); !errors.Is(err, ErrUnavailable) || found {
		t.Fatalf("missing canonical database: %v %v", found, err)
	}
	if _, err := canonical.CanonicalPool(ctx, "not-a-pool"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid pool address: %v", err)
	}
	if _, err := canonical.ResolvePublic(ctx, resourcePool, "invalid-pool"); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid public pool: %v", err)
	}
	if _, err := canonical.ResolvePublic(ctx, "callout", pool); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("unowned source kind: %v", err)
	}
	if found, err := canonical.CanonicalPool(ctx, pool); err != nil || found {
		t.Fatalf("absent canonical pool: %v %v", found, err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO indexer.indexer_canonical_creations_v1 VALUES($1,$2,$3,0,$4,$5,0,$6)`, pool, pool, pool, "0x"+strings.Repeat("a", 64), "0x"+strings.Repeat("b", 64), time.Now().Unix()); err != nil {
		t.Fatal(err)
	}
	if _, err := canonical.ResolvePublic(ctx, resourcePool, pool); !errors.Is(err, ErrInvalid) {
		t.Fatalf("zero canonical height: %v", err)
	}
	if _, err := canonical.PublicSources(ctx); !errors.Is(err, ErrInvalid) {
		t.Fatalf("invalid public source: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE indexer.indexer_canonical_creations_v1 SET height=1 WHERE pool=$1`, pool); err != nil {
		t.Fatal(err)
	}
	if found, err := canonical.CanonicalPool(ctx, pool); err != nil || !found {
		t.Fatalf("canonical pool: %v %v", found, err)
	}
	sources, err := canonical.PublicSources(ctx)
	if err != nil || len(sources) != 1 || sources[0].ID != pool || !sources[0].Public || sources[0].EventID != "0x"+strings.Repeat("b", 64)+":0" {
		t.Fatalf("canonical sources: %+v %v", sources, err)
	}
	if _, _, err := NewRuntimeAuthorities(ctx, nil, RuntimeConfig{}, pool); !errors.Is(err, ErrInvalid) {
		t.Fatalf("missing authority database: %v", err)
	}
	if _, _, err := NewRuntimeAuthorities(ctx, db, RuntimeConfig{}, pool); err == nil {
		t.Fatal("missing interval inventory admitted")
	}
}
