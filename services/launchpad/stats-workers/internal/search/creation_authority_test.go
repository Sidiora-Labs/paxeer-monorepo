package search

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func assertCreationAuthority(t *testing.T, pool *pgxpool.Pool, address string, revision int64, public, tombstone bool) {
	t.Helper()
	var gotRevision int64
	var gotPublic, gotTombstone bool
	err := pool.QueryRow(context.Background(), "SELECT visibility_revision,public,tombstone FROM stats_search.authority_v2 WHERE pool_address=$1", address).Scan(&gotRevision, &gotPublic, &gotTombstone)
	if err != nil || gotRevision != revision || gotPublic != public || gotTombstone != tombstone {
		t.Fatalf("authority: revision=%d public=%t tombstone=%t: %v", gotRevision, gotPublic, gotTombstone, err)
	}
}

func assertCreationSearch(t *testing.T, pool *pgxpool.Pool, d IndexedDocument, visible bool) {
	t.Helper()
	for _, query := range []string{d.PoolAddress, d.TokenAddress} {
		page, err := (Store{Pool: pool}).Search(context.Background(), Request{Query: query, Kind: "pool", Limit: 25})
		if err != nil {
			t.Fatal(err)
		}
		if visible {
			if page.Status != "ok" || len(page.Items) != 1 || page.Items[0].PoolAddress == nil || *page.Items[0].PoolAddress != d.PoolAddress || page.Items[0].TokenAddress == nil || *page.Items[0].TokenAddress != d.TokenAddress || page.Items[0].Label != d.Name {
				t.Fatalf("search %s: %+v", query, page)
			}
		} else if page.Status != valueNotIndexed || len(page.Items) != 0 {
			t.Fatalf("search %s: %+v", query, page)
		}
		integrated, err := (Store{Pool: pool}).SearchIntegrated(context.Background(), IntegratedRequest{Request: Request{Query: query, Kind: "pool", Limit: 25}, Sort: "address"})
		if err != nil || len(integrated.Items) != len(page.Items) || integrated.Status != page.Status {
			t.Fatalf("integrated search %s: %+v: %v", query, integrated, err)
		}
	}
}

func TestCreationAuthorityPreservesVisibility(t *testing.T) {
	for _, tc := range []struct {
		name      string
		revision  int64
		public    bool
		tombstone bool
	}{
		{name: "new pool", revision: 1, public: true},
		{name: "hidden pool", revision: 2},
		{name: "deleted pool", revision: 3, tombstone: true},
		{name: "existing public pool", revision: 9, public: true},
		{name: "same revision hidden pool", revision: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pool := integratedDatabase(t)
			d := indexedFixture(401)
			if tc.name != "new pool" {
				applyFixture(t, pool, Change{Version: 2, Kind: "visibility", Revision: tc.revision, PoolAddress: d.PoolAddress, Public: tc.public, Tombstone: tc.tombstone})
			}
			creation := Change{Version: 2, Kind: valueCreation, Revision: 1, PoolAddress: d.PoolAddress, Document: &d}
			if !applyFixture(t, pool, creation) {
				t.Fatal("creation did not write its document")
			}
			assertCreationAuthority(t, pool, d.PoolAddress, tc.revision, tc.public, tc.tombstone)
			assertCreationSearch(t, pool, d, tc.public && !tc.tombstone)
			if applyFixture(t, pool, creation) {
				t.Fatal("duplicate creation changed projection")
			}
			if tc.public {
				applyFixture(t, pool, Change{Version: 2, Kind: "visibility", Revision: tc.revision + 1, PoolAddress: d.PoolAddress})
				if applyFixture(t, pool, creation) {
					t.Fatal("creation changed hidden projection")
				}
				assertCreationAuthority(t, pool, d.PoolAddress, tc.revision+1, false, false)
				assertCreationSearch(t, pool, d, false)
			} else {
				assertCreationAuthority(t, pool, d.PoolAddress, tc.revision, tc.public, tc.tombstone)
				assertCreationSearch(t, pool, d, false)
			}
		})
	}
}

func TestCreationAuthorityAndDocumentRollbackTogether(t *testing.T) {
	pool := integratedDatabase(t)
	ctx := context.Background()
	d := indexedFixture(402)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer rollbackSearch(tx)
	changed, err := applyChangeTx(ctx, tx, Change{Version: 2, Kind: valueCreation, Revision: 1, PoolAddress: d.PoolAddress, Document: &d})
	if err != nil || !changed {
		t.Fatal(changed, err)
	}
	var documents, authority int
	if err = tx.QueryRow(ctx, "SELECT (SELECT count(*) FROM stats_search.documents_v2),(SELECT count(*) FROM stats_search.authority_v2)").Scan(&documents, &authority); err != nil || documents != 1 || authority != 1 {
		t.Fatal(documents, authority, err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var sources int
	if err = pool.QueryRow(ctx, "SELECT (SELECT count(*) FROM stats_search.documents_v2),(SELECT count(*) FROM stats_search.authority_v2),(SELECT count(*) FROM stats_search.source_documents_v2)").Scan(&documents, &authority, &sources); err != nil || documents != 0 || authority != 0 || sources != 0 {
		t.Fatal(documents, authority, sources, err)
	}
}

func TestBackfillAuthorityUsesActiveGenerationAndPreservesRows(t *testing.T) {
	pool := integratedDatabase(t)
	ctx := context.Background()
	seed := func(number int) IndexedDocument {
		d := indexedFixture(number)
		d.TokenAddress = fixtureAddress(number + 1000)
		applyFixture(t, pool, Change{Version: 2, Kind: valueCreation, Revision: 1, PoolAddress: d.PoolAddress, Document: &d})
		return d
	}
	missing := seed(410)
	missing.VisibilityRevision = 7
	missing.MetadataRevision = 2
	applyFixture(t, pool, Change{Version: 2, Kind: "metadata", Revision: 2, PoolAddress: missing.PoolAddress, Document: &missing})
	hidden := seed(411)
	deleted := seed(412)
	public := seed(413)
	inactive := seed(414)
	applyFixture(t, pool, Change{Version: 2, Kind: "visibility", Revision: 8, PoolAddress: hidden.PoolAddress})
	applyFixture(t, pool, Change{Version: 2, Kind: "visibility", Revision: 9, PoolAddress: deleted.PoolAddress, Tombstone: true})
	applyFixture(t, pool, Change{Version: 2, Kind: "visibility", Revision: 10, PoolAddress: public.PoolAddress, Public: true})
	if _, err := pool.Exec(ctx, "INSERT INTO stats_search.generations_v2(generation,state,source_watermark,source_observed_at) VALUES('retained','retained',1,clock_timestamp())"); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "UPDATE stats_search.documents_v2 SET generation='retained' WHERE pool_address=$1", inactive.PoolAddress); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, "DELETE FROM stats_search.authority_v2 WHERE pool_address IN ($1,$2)", missing.PoolAddress, inactive.PoolAddress); err != nil {
		t.Fatal(err)
	}
	assertCreationSearch(t, pool, missing, false)
	store := Store{Pool: pool}
	rows, err := store.BackfillAuthority(ctx)
	if err != nil || rows != 1 {
		t.Fatal(rows, err)
	}
	assertCreationAuthority(t, pool, missing.PoolAddress, 7, true, false)
	assertCreationAuthority(t, pool, hidden.PoolAddress, 8, false, false)
	assertCreationAuthority(t, pool, deleted.PoolAddress, 9, false, true)
	assertCreationAuthority(t, pool, public.PoolAddress, 10, true, false)
	assertCreationSearch(t, pool, hidden, false)
	assertCreationSearch(t, pool, deleted, false)
	var inactiveRows int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM stats_search.authority_v2 WHERE pool_address=$1", inactive.PoolAddress).Scan(&inactiveRows); err != nil || inactiveRows != 0 {
		t.Fatal(inactiveRows, err)
	}
	page, err := store.Search(ctx, Request{Query: missing.PoolAddress, Kind: "pool", Limit: 25})
	if err != nil || len(page.Items) != 1 || page.Items[0].Label != missing.Name {
		t.Fatal(page, err)
	}
	rows, err = store.BackfillAuthority(ctx)
	if err != nil || rows != 0 {
		t.Fatal("restart changed authority", rows, err)
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err = store.BackfillAuthority(cancelled); err == nil {
		t.Fatal("cancelled backfill succeeded")
	}
}
