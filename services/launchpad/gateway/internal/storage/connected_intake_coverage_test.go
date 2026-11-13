package storage

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

func connectedProductionView(t *testing.T, a *Authority, migration, prefix, original, replacement string) {
	t.Helper()
	raw, err := os.ReadFile("../../../../../migrations/" + migration)
	if err != nil {
		t.Fatal(err)
	}
	for _, statement := range strings.Split(string(raw), ";") {
		statement = strings.TrimSpace(statement)
		if !strings.HasPrefix(statement, prefix) {
			continue
		}
		statement = strings.ReplaceAll(statement, original, replacement)
		if _, err := a.Pool.Exec(context.Background(), statement); err != nil {
			t.Fatal(err)
		}
		return
	}
	t.Fatalf("production view %q absent from %s", prefix, migration)
}

func TestConnectedRestrictionEventsEvictAndReplay(t *testing.T) {
	base, ctx := connectedAuthority(t)
	hash, key := seedConnectedAsset(t, base, ctx)
	a, err := NewConnectedAuthority(base.Pool, base.Schema, base.MetadataSchema, base.SafetySchema)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Pool.Exec(ctx, a.RestrictionReceiptSchemaDDL()); err != nil {
		t.Fatal(err)
	}
	connectedProductionView(t, a, "390001.sql", "CREATE VIEW metadata.assets_v2 ", "metadata.", pgx.Identifier{base.MetadataSchema}.Sanitize()+".")
	connectedProductionView(t, a, "380001.sql", "CREATE VIEW chat.safety_outbox_v2 ", "chat.", pgx.Identifier{base.SafetySchema}.Sanitize()+".")
	pool := "0x" + strings.Repeat("d", 40)
	insert := func(revision int64, status string) string {
		t.Helper()
		id := uuid.NewString()
		payload, err := json.Marshal(RestrictionVisibility{TargetType: "token", TargetID: pool, Status: status, Revision: revision})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = a.Pool.Exec(ctx, "INSERT INTO "+base.safety("safety_outbox")+"(id,kind,entity_id,revision,payload) VALUES($1,'visibility_changed',$2,$3,$4)", id, pool, revision, payload); err != nil {
			t.Fatal(err)
		}
		return id
	}
	first := insert(2, "removed")
	evicted := []string{}
	failed := false
	evict := func(_ context.Context, objectKey string) error {
		if !failed {
			failed = true
			return errors.New("object eviction unavailable")
		}
		evicted = append(evicted, objectKey)
		return nil
	}
	if count, err := a.IntakeRestrictions(ctx, evict); count != 0 || err == nil {
		t.Fatalf("unacknowledged eviction: count=%d err=%v", count, err)
	}
	var receipts int
	if err := a.Pool.QueryRow(ctx, "SELECT count(*) FROM "+a.table("restriction_receipts")+" WHERE event_id=$1", first).Scan(&receipts); err != nil || receipts != 0 {
		t.Fatalf("eviction failure receipt: count=%d err=%v", receipts, err)
	}
	if count, err := a.IntakeRestrictions(ctx, evict); count != 1 || err != nil {
		t.Fatalf("removed visibility intake: count=%d err=%v", count, err)
	}
	if len(evicted) != 1 || evicted[0] != key {
		t.Fatalf("evicted keys = %v, want %q", evicted, key)
	}
	var state string
	if err := a.Pool.QueryRow(ctx, "SELECT state FROM "+a.table("restrictions")+" WHERE hash=$1", hash).Scan(&state); err != nil || state != blockedState {
		t.Fatalf("removed asset state=%q err=%v", state, err)
	}
	if count, err := a.IntakeRestrictions(ctx, evict); count != 0 || err != nil || len(evicted) != 1 {
		t.Fatalf("replayed event: count=%d err=%v evicted=%v", count, err, evicted)
	}
	insert(3, "restored")
	if count, err := a.IntakeRestrictions(ctx, evict); count != 1 || err != nil {
		t.Fatalf("restored visibility intake: count=%d err=%v", count, err)
	}
	if err := a.Pool.QueryRow(ctx, "SELECT state FROM "+a.table("restrictions")+" WHERE hash=$1", hash).Scan(&state); err != nil || state != approvedState {
		t.Fatalf("restored asset state=%q err=%v", state, err)
	}
	badID := uuid.NewString()
	if _, err := a.Pool.Exec(ctx, "INSERT INTO "+base.safety("safety_outbox")+"(id,kind,entity_id,revision,payload) VALUES($1,'visibility_changed',$2,4,$3)", badID, pool, []byte(`{"targetType":"token","targetId":"`+pool+`","status":"removed","revision":3}`)); err != nil {
		t.Fatal(err)
	}
	if count, err := a.IntakeRestrictions(ctx, evict); count != 0 || !errors.Is(err, ErrConflict) {
		t.Fatalf("mismatched revision: count=%d err=%v", count, err)
	}
	if err := a.Pool.QueryRow(ctx, "SELECT count(*) FROM "+a.table("restriction_receipts")+" WHERE event_id=$1", badID).Scan(&receipts); err != nil || receipts != 0 {
		t.Fatalf("invalid event receipt: count=%d err=%v", receipts, err)
	}
	if _, err := a.Pool.Exec(ctx, "DELETE FROM "+base.safety("safety_outbox")+" WHERE id=$1", badID); err != nil {
		t.Fatal(err)
	}
	assetID := "asset"
	assetEvent := uuid.NewString()
	assetPayload, err := json.Marshal(RestrictionVisibility{TargetType: "asset", TargetID: assetID, Status: "removed", Revision: 4})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Pool.Exec(ctx, "INSERT INTO "+base.safety("safety_outbox")+"(id,kind,entity_id,revision,payload) VALUES($1,'visibility_changed',$2,4,$3)", assetEvent, assetID, assetPayload); err != nil {
		t.Fatal(err)
	}
	if count, err := a.IntakeRestrictions(ctx, evict); count != 1 || err != nil || len(evicted) != 3 {
		t.Fatalf("asset visibility intake: count=%d evicted=%v err=%v", count, evicted, err)
	}
	staleEvent := uuid.NewString()
	stalePayload, err := json.Marshal(RestrictionVisibility{TargetType: "asset", TargetID: assetID, Status: "restored", Revision: 2})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Pool.Exec(ctx, "INSERT INTO "+base.safety("safety_outbox")+"(id,kind,entity_id,revision,payload) VALUES($1,'visibility_changed',$2,2,$3)", staleEvent, assetID, stalePayload); err != nil {
		t.Fatal(err)
	}
	if count, err := a.IntakeRestrictions(ctx, evict); count != 1 || err != nil || len(evicted) != 4 {
		t.Fatalf("stale visibility receipt: count=%d evicted=%v err=%v", count, evicted, err)
	}
	if err := a.Pool.QueryRow(ctx, "SELECT state FROM "+a.table("restrictions")+" WHERE hash=$1", hash).Scan(&state); err != nil || state != blockedState {
		t.Fatalf("stale visibility changed restriction: state=%q err=%v", state, err)
	}
}

func TestConnectedApprovedBatchImportsLineage(t *testing.T) {
	base, ctx := connectedAuthority(t)
	hash, _ := seedConnectedAsset(t, base, ctx)
	a, err := NewConnectedAuthority(base.Pool, base.Schema, base.MetadataSchema, base.SafetySchema)
	if err != nil {
		t.Fatal(err)
	}
	connectedProductionView(t, a, "390001.sql", "CREATE TABLE \"metadata\".\"asset_details_v2\"", "\"metadata\"", pgx.Identifier{base.MetadataSchema}.Sanitize())
	connectedProductionView(t, a, "390001.sql", "CREATE VIEW metadata.assets_v2 ", "metadata.", pgx.Identifier{base.MetadataSchema}.Sanitize()+".")
	connectedProductionView(t, a, "390001.sql", "CREATE VIEW metadata.asset_details_v2_v2 ", "metadata.", pgx.Identifier{base.MetadataSchema}.Sanitize()+".")
	assetID := uuid.NewString()
	if _, err := a.Pool.Exec(ctx, "UPDATE "+base.metadata("assets")+" SET id=$1 WHERE id='asset'", assetID); err != nil {
		t.Fatal(err)
	}
	variants := map[string]string{}
	for _, size := range []string{"256", "512", "1024"} {
		variants[size] = "approved/" + hash + "/" + size + "/" + strings.Repeat("c", 64) + ".png"
	}
	encoded, err := json.Marshal(variants)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = a.Pool.Exec(ctx, "INSERT INTO "+pgx.Identifier{base.MetadataSchema, "asset_details_v2"}.Sanitize()+"(asset_id,mime,width,height,variants) VALUES($1,'image/png',256,256,$2)", assetID, encoded); err != nil {
		t.Fatal(err)
	}
	if count, err := a.ImportApprovedBatch(ctx); count != 1 || err != nil {
		t.Fatalf("approved batch: count=%d err=%v", count, err)
	}
	if count, err := a.ImportApprovedBatch(ctx); count != 0 || err != nil {
		t.Fatalf("replayed approved batch: count=%d err=%v", count, err)
	}
	var originals, variantsCount int
	if err := a.Pool.QueryRow(ctx, "SELECT count(*) FILTER (WHERE kind='original'), count(*) FILTER (WHERE kind='variant') FROM "+a.table("lineage")+" WHERE hash=$1", hash).Scan(&originals, &variantsCount); err != nil || originals != 1 || variantsCount != 4 {
		t.Fatalf("imported lineage: originals=%d variants=%d err=%v", originals, variantsCount, err)
	}
}
