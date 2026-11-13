package processor

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
)

func TestInventoryCandidatesRejectMalformedRawAndFillBindings(t *testing.T) {
	ctx := context.Background()
	h := newLifecycleCoverageHarness(t)
	h.inventory.batch(t, ctx, 33963716, 33963716)
	h.inventory.batch(t, ctx, 33963717, 33963733)
	b := store.CanonicalBatch{ChainID: 125, StreamID: realStream, Owner: inventoryTestOwner, Fence: h.Processor.canonicalFence, PreviousHeight: 33963716, Blocks: []store.BlockCompleteness{{Height: 33963733}}}
	for _, tc := range []struct{ name, sql string }{
		{"raw topics", `UPDATE indexer.indexer_raw_logs SET topics='{}'`},
		{"fill encoding", `UPDATE indexer.indexer_accounting_fills SET payload='[]'`},
		{"raw height binding", `UPDATE indexer.indexer_accounting_fills SET payload=jsonb_set(payload,'{position,height}','"1"')`},
		{"degradation store", `ALTER TABLE indexer.indexer_capability_degradation RENAME TO degradation_unavailable`},
		{"baseline store", `ALTER VIEW pnl.inventory_l16_baselines_v1 RENAME TO baseline_unavailable`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := h.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			if _, err = tx.Exec(ctx, tc.sql); err != nil {
				t.Fatal(err)
			}
			if _, err = planInventory(ctx, store.CanonicalTx{Tx: tx}, b, pnlGeneration, pnlInventoryVersion, InventoryBaselineWaitBlocks); err == nil {
				t.Fatal("unbound inventory history admitted")
			}
			if err = tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}
	tx, err := h.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, bound := range []int64{0, -1} {
		if _, err = planInventory(ctx, store.CanonicalTx{Tx: tx}, b, pnlGeneration, pnlInventoryVersion, bound); err == nil {
			t.Fatal("nonpositive inventory wait admitted")
		}
	}
	empty := b
	empty.Blocks = nil
	if _, err = planInventory(ctx, store.CanonicalTx{Tx: tx}, empty, pnlGeneration, pnlInventoryVersion, 1); err == nil {
		t.Fatal("missing canonical blocks admitted")
	}
	early := b
	early.PreviousHeight = 0
	early.Blocks = []store.BlockCompleteness{{Height: 1}}
	plans, err := planInventory(ctx, store.CanonicalTx{Tx: tx}, early, pnlGeneration, pnlInventoryVersion, 300)
	if err != nil || len(plans) != 0 {
		t.Fatalf("early inventory window: %v %v", plans, err)
	}
}

func TestInventoryPartitionRejectsMalformedStoredPositions(t *testing.T) {
	ctx := context.Background()
	h := newInventoryHarness(t, ctx)
	h.batch(t, ctx, 33963716, 33963716)
	h.issueBaseline(t, ctx, realMarketTrader, realMarketToken)
	h.batch(t, ctx, 33963717, 33963733)
	h.batch(t, ctx, 33963734, 33963749)
	fills := h.fills(t, ctx)
	if len(fills) != 2 {
		t.Fatalf("recorded fills %d", len(fills))
	}
	fill := fills[1].fill
	key := "125:" + realMarketTrader + ":" + realMarketToken
	for _, tc := range []struct{ name, sql string }{
		{"position JSON", `UPDATE indexer.delivery_partition_positions SET position='[]' WHERE partition_kind='inventory'`},
		{"position order", `UPDATE indexer.delivery_partition_positions SET position=convert_to(jsonb_set(convert_from(position,'UTF8')::jsonb,'{sourceOrder,height}','"invalid"')::text,'UTF8') WHERE partition_kind='inventory'`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := h.pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			tag, err := tx.Exec(ctx, tc.sql)
			if err != nil || tag.RowsAffected() != 2 {
				t.Fatalf("stored inventory positions %d: %v", tag.RowsAffected(), err)
			}
			ok, err := inventoryPartitionContiguous(ctx, tx, pnlInventoryVersion, key, fill, []string{fills[0].fill.SourceFillID}, nil)
			if err == nil || ok {
				t.Fatalf("unbound stored inventory position %t: %v", ok, err)
			}
			if err = tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSourceFactsRequireEveryAdmittedTransferRawRecord(t *testing.T) {
	ctx := context.Background()
	h := newLifecycleCoverageHarness(t)
	h.inventory.batch(t, ctx, 33963716, 33963716)
	batch, receipts, err := h.Processor.deps.QualifiedBatchSource.FetchQualifiedBatch(ctx, 33963717, 33963733)
	if err != nil {
		t.Fatal(err)
	}
	for _, mode := range []string{"missing raw", "changed raw", "changed topics", "unavailable token facts"} {
		t.Run(mode, func(t *testing.T) {
			body, err := json.Marshal(batch)
			if err != nil {
				t.Fatal(err)
			}
			var b store.CanonicalBatch
			if err = json.Unmarshal(body, &b); err != nil {
				t.Fatal(err)
			}
			tx, err := h.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(ctx)
			switch mode {
			case "missing raw":
				b.Records = nil
			case "changed raw":
				for i := range b.Records {
					b.Records[i].Data = append(b.Records[i].Data, 1)
				}
			case "changed topics":
				for i := range b.Records {
					if len(b.Records[i].Topics) == 3 {
						b.Records[i].Topics[1] = "invalid"
					}
				}
			case "unavailable token facts":
				if _, err = tx.Exec(ctx, `ALTER TABLE indexer.indexer_canonical_creations RENAME TO creations_unavailable`); err != nil {
					t.Fatal(err)
				}
			}
			if _, err = stageQualifiedSourceFacts(ctx, store.CanonicalTx{Tx: tx}, b, receipts, h.Processor.deps.Durable); err == nil {
				t.Fatal("unbound canonical transfer raw admitted")
			}
			if err = tx.Rollback(ctx); err != nil {
				t.Fatal(err)
			}
		})
	}
}
