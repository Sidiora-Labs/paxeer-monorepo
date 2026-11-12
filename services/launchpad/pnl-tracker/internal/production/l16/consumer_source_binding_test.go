package l16

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
)

func assertConsumerSourceBindings(t *testing.T, tx pgx.Tx, a *BaselineAuthority, c *AccountingConsumer, g delivery.GuardedPosition, leg Leg, row portfolioRow, snapshot string) {
	t.Helper()
	ctx := context.Background()
	for _, tc := range []struct {
		name, sql                         string
		baseline, append, history, source bool
	}{
		{"invalid fill", `UPDATE indexer.indexer_accounting_fills SET payload=jsonb_set(payload,'{side}','"invalid"')`, true, true, true, true},
		{"fill digest", `UPDATE indexer.indexer_accounting_fills SET source_fact_digest=repeat('0',64)`, true, true, true, true},
		{"missing fill", `DELETE FROM indexer.indexer_accounting_fills`, true, false, false, true},
		{"missing publication", `DELETE FROM indexer.indexer_ingest_outbox WHERE event_kind='accounting.inventory.v1'`, true, false, false, false},
		{"publication schema", `UPDATE indexer.indexer_ingest_outbox SET payload=jsonb_set(payload,'{facet}','"unknown"') WHERE payload->>'facet'='inventory'`, true, false, false, false},
		{"publication source digest", `UPDATE indexer.indexer_ingest_outbox SET payload=jsonb_set(payload,'{sourceFactDigest}',to_jsonb(repeat('0',64))) WHERE payload->>'facet'='inventory'`, true, false, false, false},
		{"inventory state", `UPDATE inventory_l16 SET state='{"knownQuantityRaw":"bad"}'`, false, true, false, false},
		{"effect position", `UPDATE inventory_l16_effects SET position='1'`, false, true, false, false},
		{"leg payload", `UPDATE inventory_l16_inbox SET payload='1'`, false, false, true, false},
		{"leg owner", `UPDATE inventory_l16_inbox SET payload=jsonb_set(payload,'{Wallet}',to_jsonb('0x'||repeat('e',40)))`, false, false, true, false},
		{"lease expiry", `UPDATE delivery_partition_leases SET expires_at=clock_timestamp()-interval '1 minute'`, true, false, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nested, err := tx.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer inventoryRollback(nested)
			statement := tc.sql
			if tc.name == "missing publication" {
				statement = `DELETE FROM indexer.indexer_ingest_outbox WHERE payload->>'facet'='inventory'`
			}
			tag, err := nested.Exec(ctx, statement)
			if err != nil || tag.RowsAffected() == 0 {
				t.Fatal(tag, err)
			}
			if tc.baseline {
				if err = a.VerifyFreshSameOwnerTx(ctx, nested, "g1", row.Fill, row.Position); err == nil {
					t.Fatal("unbound baseline accepted")
				}
			}
			if tc.append {
				if err = c.appendPortfolioTx(ctx, nested, g, leg, row.Fill); err == nil {
					t.Fatal("unbound portfolio source accepted")
				}
			}
			if tc.history {
				if err = c.commitPortfolioHistoryTx(ctx, nested, leg.Wallet, snapshot, g.Position.EffectIdentity); err == nil {
					t.Fatal("unbound history source accepted")
				}
			}
			if tc.source {
				digest, err := accounting.FillDigest(row.Fill)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = ReadSourceFillTx(ctx, nested, row.Fill.SourceFillID, digest); err == nil {
					t.Fatal("unbound fill accepted")
				}
			}
		})
	}
	for _, tc := range []struct {
		name string
		raw  []byte
	}{{"invalid json", []byte(`{`)}, {"different value", []byte(`{}`)}} {
		t.Run(tc.name, func(t *testing.T) {
			q := qualifiedBaselineUse{issuer: a, tx: tx, raw: tc.raw, digest: delivery.Hash(tc.raw), value: baselineUse{Generation: a.generation}}
			if err := a.persistFreshUseTx(ctx, tx, q); err == nil {
				t.Fatal("unbound qualified use accepted")
			}
		})
	}
	var proof qualifiedBaselineUse
	if err := a.qualifyFreshTx(ctx, tx, "g1", row.Fill, row.Position, &proof); err != nil || proof.issuer != a || proof.value.Partition != row.LogicalPartition {
		t.Fatal(proof.value, err)
	}
	for _, action := range []string{"lease binding", "digest binding"} {
		t.Run(action, func(t *testing.T) {
			bad := proof
			if action == "lease binding" {
				bad.sourceEpoch++
			} else {
				var value baselineUse
				if err := json.Unmarshal(proof.raw, &value); err != nil {
					t.Fatal(err)
				}
				value.EffectIdentity = "unbound"
				bad.value = value
			}
			if err := a.persistFreshUseTx(ctx, tx, bad); err == nil {
				t.Fatal("unbound qualified use accepted")
			}
		})
	}
}
