package processor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/jackc/pgx/v5"
)

func TestInventoryQualificationRejectsChangedCanonicalHistory(t *testing.T) {
	ctx := context.Background()
	h := newInventoryHarness(t, ctx)
	h.batch(t, ctx, 33963716, 33963716)
	h.issueBaseline(t, ctx, realMarketTrader, realMarketToken)
	h.batch(t, ctx, 33963717, 33963733)
	h.batch(t, ctx, 33963734, 33963749)
	fills := h.fills(t, ctx)
	if len(fills) != 2 {
		t.Fatalf("source fills %d", len(fills))
	}
	b := store.CanonicalBatch{ChainID: 125, StreamID: realStream, Owner: inventoryTestOwner, Fence: h.proc.readyFence.Load()}
	original := fills[1].fill
	for _, tc := range []struct {
		name     string
		sql      string
		edit     func(*accounting.FillV1)
		expected error
	}{
		{name: "payer", edit: func(f *accounting.FillV1) { f.Attribution.Payer = nil }, expected: errInventoryIneligible},
		{name: "recipient", edit: func(f *accounting.FillV1) { f.Attribution.Recipient = nil }, expected: errInventoryIneligible},
		{name: "trader", edit: func(f *accounting.FillV1) { v := realMarketPool; f.Attribution.Trader = &v }, expected: errInventoryIneligible},
		{name: "height", edit: func(f *accounting.FillV1) { f.Position.Height = "0" }, expected: store.ErrIntegrity},
		{name: "movement digest", sql: `UPDATE pnl.inventory_l16_baselines SET movement_digest='invalid'`, expected: store.ErrIntegrity},
		{name: "missing baseline", sql: `DELETE FROM pnl.inventory_l16_baselines`, expected: errInventoryIneligible},
		{name: "incomplete range", sql: `DELETE FROM indexer.indexer_block_completeness WHERE height=33963720`, expected: errInventoryIneligible},
		{name: "routed fill", sql: `UPDATE indexer.indexer_accounting_fills SET payload=jsonb_set(payload,'{attribution,transactionSender}',to_jsonb('0x0000000000000000000000000000000000000001'::text))`, expected: errInventoryIneligible},
		{name: "missing movement", sql: `DELETE FROM indexer.indexer_token_transfers WHERE height>33963716`},
		{name: "different movement", sql: `UPDATE indexer.indexer_token_transfers SET payload=jsonb_set(payload,'{amountRaw}',to_jsonb('1'::text)) WHERE height>33963716`, expected: errInventoryIneligible},
		{name: "fill schema", sql: `ALTER TABLE indexer.indexer_accounting_fills RENAME TO accounting_fills_unavailable`},
		{name: "movement schema", sql: `ALTER TABLE indexer.indexer_token_transfers RENAME TO token_transfers_unavailable`},
		{name: "creation schema", sql: `ALTER TABLE indexer.indexer_canonical_creations RENAME TO creations_unavailable`},
		{name: "malformed fill", sql: `UPDATE indexer.indexer_accounting_fills SET payload=jsonb_set(payload,'{version}','"unsupported"')`},
		{name: "source byte bound", sql: `UPDATE indexer.indexer_accounting_fills SET payload=payload || jsonb_build_object('padding',repeat('x',33554433))`, expected: errInventoryIneligible},
		{name: "movement byte bound", sql: `UPDATE indexer.indexer_token_transfers SET payload=payload || jsonb_build_object('padding',repeat('x',33554433)) WHERE height>33963716`, expected: errInventoryIneligible},
		{name: "malformed movement", sql: `UPDATE indexer.indexer_token_transfers SET payload='[]'::jsonb WHERE height>33963716`},
		{name: "unbound transfer proof", sql: `UPDATE indexer.indexer_accounting_fills SET payload=jsonb_set(payload,'{attribution,evidence}',(SELECT jsonb_agg(CASE WHEN e->>'kind' IN ('input-transfer','output-transfer') THEN jsonb_set(e,'{logIndex}','null'::jsonb) ELSE e END) FROM jsonb_array_elements(payload->'attribution'->'evidence') e))`, expected: store.ErrIntegrity},
		{name: "transfer receipt proof", sql: `UPDATE indexer.indexer_accounting_fills SET payload=jsonb_set(payload,'{attribution,evidence}',(SELECT jsonb_agg(CASE WHEN e->>'kind' IN ('input-transfer','output-transfer') THEN jsonb_set(e,'{artifactSHA256}',to_jsonb(repeat('a',64))) ELSE e END) FROM jsonb_array_elements(payload->'attribution'->'evidence') e))`, expected: store.ErrIntegrity},
		{name: "missing transfer proof", sql: `UPDATE indexer.indexer_accounting_fills SET payload=jsonb_set(payload,'{attribution,evidence}',(SELECT jsonb_agg(CASE WHEN e->>'kind' IN ('input-transfer','output-transfer') THEN jsonb_set(e,'{kind}','"transaction"') ELSE e END) FROM jsonb_array_elements(payload->'attribution'->'evidence') e))`, expected: errInventoryIneligible},
		{name: "projection identity", edit: func(f *accounting.FillV1) { f.SourceFillID = "invalid" }},
		{name: "baseline schema", sql: `ALTER VIEW pnl.inventory_l16_baselines_v1 RENAME TO baseline_unavailable`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := h.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if tc.sql != "" {
				if _, err = tx.Exec(ctx, tc.sql); err != nil {
					t.Fatal(err)
				}
			}
			raw, err := json.Marshal(original)
			if err != nil {
				t.Fatal(err)
			}
			var fill accounting.FillV1
			if err = json.Unmarshal(raw, &fill); err != nil {
				t.Fatal(err)
			}
			if tc.edit != nil {
				tc.edit(&fill)
			}
			_, _, err = qualifiedInventoryEnrichment(ctx, store.CanonicalTx{Tx: tx}, b, fill, pnlGeneration, pnlInventoryVersion)
			if err == nil || (tc.expected != nil && !errors.Is(err, tc.expected)) {
				t.Fatalf("changed history accepted: %v", err)
			}
		})
	}
	tx, err := h.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	enrichment, prior, err := qualifiedInventoryEnrichment(ctx, store.CanonicalTx{Tx: tx}, b, original, pnlGeneration, pnlInventoryVersion)
	if err != nil || enrichment.SourceFillID != original.SourceFillID || len(prior) != 1 || prior[0] != fills[0].fill.SourceFillID {
		t.Fatalf("qualified inventory %+v %v %v", enrichment, prior, err)
	}
	key := "125:" + realMarketTrader + ":" + realMarketToken
	if ok, err := inventoryPartitionContiguous(ctx, tx, pnlInventoryVersion, key, original, prior, nil); err != nil || !ok {
		t.Fatalf("contiguous inventory %t %v", ok, err)
	}
	if ok, err := inventoryPartitionContiguous(ctx, tx, pnlInventoryVersion, key, original, prior, map[string]bool{"unbound": true}); !errors.Is(err, store.ErrIntegrity) || ok {
		t.Fatalf("unbound partition %t %v", ok, err)
	}
	if _, err = tx.Exec(ctx, `ALTER TABLE indexer.delivery_partition_positions RENAME TO partition_positions_unavailable`); err != nil {
		t.Fatal(err)
	}
	if _, err = inventoryPartitionContiguous(ctx, tx, pnlInventoryVersion, key, original, prior, nil); err == nil {
		t.Fatal("unavailable partition accepted")
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if _, _, err = qualifiedInventoryEnrichment(ctx, store.CanonicalTx{Tx: tx}, b, original, pnlGeneration, pnlInventoryVersion); !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatalf("closed transaction accepted: %v", err)
	}
}
