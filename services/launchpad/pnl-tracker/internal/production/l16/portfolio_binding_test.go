package l16

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
)

func assertPortfolioReplayBindings(t *testing.T, tx pgx.Tx, c *AccountingConsumer, g delivery.GuardedPosition, leg Leg, row portfolioRow) {
	t.Helper()
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		change func(*RebuildSnapshot, *portfolioRow)
		raw    []byte
		digest string
		valid  bool
	}{
		{name: "matching active state", valid: true},
		{name: "older active state", change: func(s *RebuildSnapshot, r *portfolioRow) { s.Height-- }, valid: true},
		{name: "payload hash", raw: []byte(`{}`), digest: strings.Repeat("0", 64)},
		{name: "payload json", raw: []byte(`{`)},
		{name: "inventory invariant", change: func(s *RebuildSnapshot, r *portfolioRow) { s.State.Known = "bad" }},
		{name: "active partition", change: func(s *RebuildSnapshot, r *portfolioRow) { s.Partition += "other" }},
		{name: "current state mismatch", change: func(s *RebuildSnapshot, r *portfolioRow) { s.State.Unknown = "1" }},
		{name: "older unknown state mismatch", change: func(s *RebuildSnapshot, r *portfolioRow) { s.Height--; s.State.Unknown = "1" }},
		{name: "older incomplete state mismatch", change: func(s *RebuildSnapshot, r *portfolioRow) { s.Height--; s.State.IncompleteRealized = true }},
		{name: "fill height", change: func(s *RebuildSnapshot, r *portfolioRow) { r.Fill.Position.Height = "bad" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nested, err := tx.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer inventoryRollback(nested)
			snapshot := RebuildSnapshot{Generation: c.generation, Partition: row.LogicalPartition, Height: int64(leg.Height), BlockHash: row.Fill.Position.BlockHash, State: row.Inventory}
			current := row
			if tc.change != nil {
				tc.change(&snapshot, &current)
			}
			raw, err := json.Marshal(snapshot)
			if err != nil {
				t.Fatal(err)
			}
			if tc.raw != nil {
				raw = tc.raw
			}
			digest := delivery.Hash(raw)
			if tc.digest != "" {
				digest = tc.digest
			}
			if _, err = nested.Exec(ctx, `INSERT INTO inventory_l16_snapshots_v2(generation,partition,snapshot_digest,payload,height,block_hash,release_digest) VALUES($1,$2,$3,$4,$5,$6,$7)`, c.generation, row.LogicalPartition, digest, raw, leg.Height, row.Fill.Position.BlockHash, strings.Repeat("c", 64)); err != nil {
				t.Fatal(err)
			}
			if _, err = nested.Exec(ctx, `INSERT INTO inventory_l16_active_v2(partition,generation,snapshot_digest,height) VALUES($1,$2,$3,$4)`, row.LogicalPartition, c.generation, digest, leg.Height); err != nil {
				t.Fatal(err)
			}
			update, _, _, err := c.commitFullPortfolioTx(ctx, nested, g, leg, []portfolioRow{current})
			if (err == nil) != tc.valid {
				t.Fatal(update, err)
			}
			if tc.valid && (len(update.Changes) != 1 || !update.CompleteBatch || update.Summary.Wallet != leg.Wallet) {
				t.Fatal(update)
			}
		})
	}
	for _, tc := range []struct {
		name   string
		change func(*portfolioRow)
	}{
		{"source ordering", func(r *portfolioRow) { r.Position.SourceOrder.Height = "bad" }},
		{"newest timestamp", func(r *portfolioRow) { r.Fill.BlockTimestampSeconds = "bad" }},
		{"inventory state", func(r *portfolioRow) { r.Inventory.Known = "bad" }},
		{"source binding", func(r *portfolioRow) { r.Fill.Position.LogIndex = "999" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := row
			tc.change(&bad)
			if _, _, _, err := c.commitFullPortfolioTx(ctx, tx, g, leg, []portfolioRow{bad}); err == nil {
				t.Fatal("invalid portfolio accepted")
			}
		})
	}
	if _, _, _, err := c.commitFullPortfolioTx(ctx, tx, g, leg, []portfolioRow{row, row}); err == nil {
		t.Fatal("duplicate portfolio row accepted")
	}
}
