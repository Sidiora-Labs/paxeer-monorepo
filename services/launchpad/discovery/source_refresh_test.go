package discovery

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"
)

func TestSourceRenewalBatchesAndComparesWholeDocument(t *testing.T) {
	store, _, _ := proofDiscoveryStore(t)
	ctx := context.Background()
	if _, err := store.DB.ExecContext(ctx, `CREATE TABLE source_renewal_statements(rows_changed bigint NOT NULL);
CREATE FUNCTION record_source_renewal() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN INSERT INTO source_renewal_statements SELECT count(*) FROM renewed; RETURN NULL; END $$;
CREATE TRIGGER source_renewal AFTER UPDATE ON narrative_discovery.sources REFERENCING NEW TABLE AS renewed FOR EACH STATEMENT EXECUTE FUNCTION record_source_renewal()`); err != nil {
		t.Fatal(err)
	}
	old := time.Now().UTC().Add(-2 * time.Minute).Truncate(time.Microsecond)
	documents := make([]json.RawMessage, 0, 501)
	for index := 0; index < 501; index++ {
		ref := SourceRef{ID: fmt.Sprintf("renewal-%03d", index), EventID: fmt.Sprintf("publication-%03d", index), Kind: "pool", SourceRevision: 1, VisibilityRevision: 1, PublishedAt: old, Public: true, Authorized: true}
		if err := store.Apply(ctx, "publications", ref.EventID, ref, old); err != nil {
			t.Fatal(err)
		}
		var raw json.RawMessage
		if err := store.DB.QueryRowContext(ctx, `SELECT document FROM narrative_discovery.sources WHERE id=$1`, ref.ID).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		documents = append(documents, raw)
	}
	if _, err := store.DB.ExecContext(ctx, `UPDATE narrative_discovery.sources SET document=document || '{"AdditionalVisibilityRevision":2}'::jsonb WHERE id='renewal-000'; DELETE FROM source_renewal_statements`); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	if err := store.RenewSources(ctx, documents, now); err != nil {
		t.Fatal(err)
	}
	var statements, changed, renewed, marks int
	if err := store.DB.QueryRowContext(ctx, `SELECT count(*),sum(rows_changed) FROM source_renewal_statements`).Scan(&statements, &changed); err != nil {
		t.Fatal(err)
	}
	if statements != 2 || changed != 500 {
		t.Fatalf("renewal statements=%d changed=%d", statements, changed)
	}
	if err := store.DB.QueryRowContext(ctx, `SELECT count(*) FROM narrative_discovery.sources WHERE observed_at=$1`, now).Scan(&renewed); err != nil {
		t.Fatal(err)
	}
	if err := store.DB.QueryRowContext(ctx, `SELECT count(*) FROM narrative_discovery.consumer_events`).Scan(&marks); err != nil {
		t.Fatal(err)
	}
	var unchanged time.Time
	if err := store.DB.QueryRowContext(ctx, `SELECT observed_at FROM narrative_discovery.sources WHERE id='renewal-000'`).Scan(&unchanged); err != nil {
		t.Fatal(err)
	}
	if renewed != 500 || marks != 501 || !unchanged.Equal(old) {
		t.Fatalf("renewed=%d marks=%d changed document observed=%v", renewed, marks, unchanged)
	}
}
