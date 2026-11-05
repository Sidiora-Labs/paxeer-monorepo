package delivery

import (
	"errors"
	"github.com/jackc/pgx/v5"
	"strconv"
	"testing"
)

func TestShadowExactSetAndDuplicateRejection(t *testing.T) {
	a := ShadowRecord{"a", Hash([]byte("a"))}
	b := ShadowRecord{"b", Hash([]byte("b"))}
	d, err := CompareShadow([]ShadowRecord{a, b}, []ShadowRecord{b, a})
	if err != nil || !archiveHash.MatchString(d.Digest) {
		t.Fatal(d, err)
	}
	for _, records := range [][]ShadowRecord{{a}, {a, b, b}, {a, {"b", Hash([]byte("different"))}}, {a, b, {"c", Hash([]byte("extra"))}}} {
		if _, err = CompareShadow([]ShadowRecord{a, b}, records); !errors.Is(err, ErrIntegrity) {
			t.Fatal("shadow mismatch accepted", records, err)
		}
	}
}

// Exercise the same persisted comparison against PostgreSQL when the private
// integration fixture is explicitly admitted. No external fixture is in the
// TestRepairM_962 gate.
func TestShadowPostgresDurableObservations(t *testing.T) {
	for _, fault := range []string{"exact", "missing", "extra", "changed", "wrong_range"} {
		t.Run(fault, func(t *testing.T) {
			ctx, pool, archive, engine, manifest, digest := repair41Fixture(t)
			if _, err := pool.Exec(ctx, "DELETE FROM "+archive.store.table("delivery_shadow_comparisons")); err != nil {
				t.Fatal(err)
			}
			var eventID string
			if err := pool.QueryRow(ctx, "SELECT event_id FROM "+archive.store.table("delivery_shadow_observations")+" WHERE path='durable' ORDER BY event_id LIMIT 1").Scan(&eventID); err != nil {
				t.Fatal(err)
			}
			var err error
			switch fault {
			case "missing":
				_, err = pool.Exec(ctx, "DELETE FROM "+archive.store.table("delivery_shadow_observations")+" WHERE path='durable' AND event_id=$1", eventID)
			case "extra":
				e := fixtureEnvelope("extra-shadow-record")
				e.SourceOrder.Height = strconv.FormatInt(manifest.FromHeight, 10)
				err = archive.store.mutate(ctx, func(tx pgx.Tx) error {
					return archive.store.RecordShadowTx(ctx, tx, digest, "durable", fixtureSubject, e)
				})
			case "changed":
				_, err = pool.Exec(ctx, "UPDATE "+archive.store.table("delivery_shadow_observations")+" SET payload_hash=$2 WHERE path='durable' AND event_id=$1", eventID, Hash([]byte("changed")))
			case "wrong_range":
				_, err = pool.Exec(ctx, "UPDATE "+archive.store.table("delivery_shadow_observations")+" SET source_order=jsonb_set(source_order,'{height}',to_jsonb($2::text)) WHERE path='durable' AND event_id=$1", eventID, strconv.FormatInt(manifest.ToHeight+1, 10))
			}
			if err != nil {
				t.Fatal(err)
			}
			_, err = archive.store.CompareShadowGeneration(ctx, digest, []string{fixtureSubject}, engine.consumer, engine.generation)
			var proofs int
			if scan := pool.QueryRow(ctx, "SELECT count(*) FROM "+archive.store.table("delivery_shadow_comparisons")).Scan(&proofs); scan != nil {
				t.Fatal(scan)
			}
			if fault == "exact" {
				if err != nil || proofs != 1 {
					t.Fatal("exact observations not persisted", proofs, err)
				}
			} else if !errors.Is(err, ErrIntegrity) || proofs != 0 {
				t.Fatal("mismatch persisted promotion evidence", fault, proofs, err)
			}
		})
	}
}
