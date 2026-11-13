package search

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5/pgxpool"
)

func applySearchChangeError(t *testing.T, pool *pgxpool.Pool, change Change) error {
	t.Helper()
	ctx := context.Background()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	_, err = applyChangeTx(ctx, tx, change)
	return err
}

func TestProjectionRejectsChangedSourceIdentityAndVisibility(t *testing.T) {
	pool := integratedDatabase(t)
	base := seedIndexed(t, pool, 110)
	ctx := context.Background()
	mutations := []struct {
		name     string
		edit     func(*IndexedDocument)
		kind     string
		revision int64
	}{
		{"token", func(d *IndexedDocument) { d.TokenAddress = fixtureAddress(212) }, "metadata", 2},
		{"creator", func(d *IndexedDocument) { d.Creator = fixtureAddress(213) }, "metadata", 2},
		{"creation event", func(d *IndexedDocument) { d.CreationEventID = "other" }, "metadata", 2},
		{"source block", func(d *IndexedDocument) { d.SourceBlockHash = "0x" + strings.Repeat("2", 64) }, "metadata", 2},
		{"visibility revision", func(d *IndexedDocument) { d.VisibilityRevision = 0 }, "metadata", 2},
	}
	for _, tc := range mutations {
		t.Run(tc.name, func(t *testing.T) {
			d := base
			d.MetadataRevision = tc.revision
			d.Name, d.NameKey = "Changed", "changed"
			tc.edit(&d)
			err := applySearchChangeError(t, pool, Change{Version: 2, Kind: tc.kind, Revision: tc.revision, PoolAddress: d.PoolAddress, Document: &d})
			if !errors.Is(err, delivery.ErrIntegrity) {
				t.Fatalf("source identity: %v", err)
			}
		})
	}
	missing := indexedFixture(111)
	missing.MetadataRevision = 2
	if err := applySearchChangeError(t, pool, Change{Version: 2, Kind: "metadata", Revision: 2, PoolAddress: missing.PoolAddress, Document: &missing}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing creation: %v", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE stats_search.source_documents_v2 SET payload='[]'::jsonb WHERE pool_address=$1`, base.PoolAddress); err != nil {
		t.Fatal(err)
	}
	changed := base
	changed.MetadataRevision = 2
	changed.Name, changed.NameKey = "Changed", "changed"
	if err := applySearchChangeError(t, pool, Change{Version: 2, Kind: "metadata", Revision: 2, PoolAddress: changed.PoolAddress, Document: &changed}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("malformed durable source: %v", err)
	}
}

func TestProjectionRequiresActiveGenerationAndKeepsPrivateProfileDeleted(t *testing.T) {
	pool := integratedDatabase(t)
	ctx := context.Background()
	d := indexedFixture(112)
	if _, err := pool.Exec(ctx, `UPDATE stats_search.generations_v2 SET state='retained'`); err != nil {
		t.Fatal(err)
	}
	if err := applySearchChangeError(t, pool, Change{Version: 2, Kind: "creation", Revision: 1, PoolAddress: d.PoolAddress, Document: &d}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("missing generation: %v", err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO stats_search.generations_v2(generation,state,source_watermark,source_observed_at) VALUES('g2','active',1,clock_timestamp())`); err != nil {
		t.Fatal(err)
	}
	wallet := fixtureAddress(501)
	if !applyFixture(t, pool, Change{Version: 2, Kind: "profile_deletion", Revision: 3, ProfileWallet: wallet}) {
		t.Fatal("profile deletion had no durable effect")
	}
	d.ProfileWallet, d.ProfileLabel = wallet, "Private label"
	if !applyFixture(t, pool, Change{Version: 2, Kind: "creation", Revision: 1, PoolAddress: d.PoolAddress, Document: &d}) {
		t.Fatal("creation had no durable effect")
	}
	var raw []byte
	if err := pool.QueryRow(ctx, `SELECT payload FROM stats_search.source_documents_v2 WHERE pool_address=$1`, d.PoolAddress).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var saved IndexedDocument
	if err := json.Unmarshal(raw, &saved); err != nil || saved.ProfileWallet != "" || saved.ProfileLabel != "" {
		t.Fatalf("private profile persisted: %+v %v", saved, err)
	}
}

func TestSearchStrictJSONRejectsAmbiguityAndInvalidChangeKinds(t *testing.T) {
	for _, raw := range []string{`{"version":2,"Version":2}`, `{"nested":{"same":1,"same":2}}`, `{"version":2} {}`, `[]`, `{"unknown":1}`} {
		var change Change
		if err := strictJSON([]byte(raw), &change); err == nil {
			t.Fatalf("ambiguous document accepted: %s", raw)
		}
	}
	d := indexedFixture(113)
	for _, change := range []Change{
		{Version: 1, Kind: "creation", Revision: 1, PoolAddress: d.PoolAddress, Document: &d},
		{Version: 2, Kind: "metrics", Revision: 0, PoolAddress: d.PoolAddress, Document: &d},
		{Version: 2, Kind: "unknown", Revision: 1, PoolAddress: d.PoolAddress, Document: &d},
		{Version: 2, Kind: "visibility", Revision: 1, PoolAddress: "other"},
		{Version: 2, Kind: "profile_deletion", Revision: 1, ProfileWallet: "other"},
	} {
		if err := change.Validate(); !errors.Is(err, ErrInvalid) {
			t.Fatalf("invalid change: %+v %v", change, err)
		}
	}
	if err := (Change{Version: 2, Kind: "visibility", Revision: 1, PoolAddress: d.PoolAddress}).Validate(); err != nil {
		t.Fatal(err)
	}
}
