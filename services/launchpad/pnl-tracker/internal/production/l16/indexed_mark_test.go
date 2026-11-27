package l16

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/db/production/coordinatortest"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type indexedMarkFixture struct {
	ctx            context.Context
	admin, runtime *pgxpool.Pool
	service        *Service
	fills          []accounting.FillV1
	publications   []accounting.PublicationV1
}

func newIndexedMarkFixture(t *testing.T) indexedMarkFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)
	db := coordinatortest.Bootstrap(t, realMarketEnv(t, "PNL_TEST_POSTGRES_DSN"), "pnl_runtime")
	admin, err := pgxpool.New(ctx, db.AdminDSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(admin.Close)
	runtime, err := pgxpool.New(ctx, db.Runtimes["pnl_runtime"].DSN)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(runtime.Close)
	f := indexedMarkFixture{ctx: ctx, admin: admin, runtime: runtime}
	raw, err := os.ReadFile(filepath.Join("testdata", "indexer_real_market", "traded.json"))
	if err != nil {
		t.Fatal(err)
	}
	var tables map[string]json.RawMessage
	if err = json.Unmarshal(raw, &tables); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]json.RawMessage
	if err = json.Unmarshal(tables["indexer.indexer_accounting_fills"], &rows); err != nil {
		t.Fatal(err)
	}
	for _, row := range rows {
		var fill accounting.FillV1
		if err = json.Unmarshal(row["payload"], &fill); err != nil {
			t.Fatal(err)
		}
		canonical, e := json.Marshal(fill)
		if e != nil {
			t.Fatal(e)
		}
		var recordedDigest string
		if e = json.Unmarshal(row["source_fact_digest"], &recordedDigest); e != nil {
			t.Fatal(e)
		}
		if delivery.Hash(bytes.Replace(canonical, []byte(`"feeBPS":`), []byte(`"feeBps":`), 1)) != recordedDigest {
			t.Fatal("recorded fill does not match its source digest")
		}
		fill, e = accounting.DecodeFillV1(canonical)
		if e != nil {
			t.Fatal(e)
		}
		digest, e := accounting.FillDigest(fill)
		if e != nil {
			t.Fatal(e)
		}
		row["payload"] = canonical
		row["source_fact_digest"], e = json.Marshal(digest)
		if e != nil {
			t.Fatal(e)
		}
		f.fills = append(f.fills, fill)
	}
	tables["indexer.indexer_accounting_fills"], err = json.Marshal(rows)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"indexer.indexer_raw_logs", "indexer.indexer_accounting_fills", "indexer.pools", "indexer.pool_state_snapshots"} {
		if err = insertRecordedTable(ctx, admin, name, tables[name]); err != nil {
			t.Fatal(name, err)
		}
	}
	var outbox []struct {
		Kind    string          `json:"event_kind"`
		Payload json.RawMessage `json:"payload"`
	}
	if err = json.Unmarshal(tables["indexer.indexer_ingest_outbox"], &outbox); err != nil {
		t.Fatal(err)
	}
	for _, row := range outbox {
		if row.Kind == accounting.InventorySubject {
			var publication accounting.PublicationV1
			if err = json.Unmarshal(row.Payload, &publication); err != nil {
				t.Fatal(err)
			}
			f.publications = append(f.publications, publication)
		}
	}
	sort.Slice(f.fills, func(i, j int) bool {
		a, e := accounting.CompareSourcePosition(f.fills[i].Position, f.fills[j].Position)
		if e != nil {
			t.Fatal(e)
		}
		return a < 0
	})
	if len(f.fills) != 2 || len(f.publications) != 2 {
		t.Fatal(len(f.fills), len(f.publications))
	}
	guard, err := queue.NewEpochGuard("recovery", coordinatortest.Epoch, coordinatortest.Region)
	if err != nil {
		t.Fatal(err)
	}
	policy := delivery.Policy{Subjects: map[string]bool{RebuildSubject: true, portfolioSnapshotV2: true}, EventTypes: map[string]bool{RebuildSubject: true, portfolioSnapshotV2: true}}
	out, err := delivery.NewStoreWithEpochGuard(runtime, "pnl", policy, guard.Assert)
	if err != nil {
		t.Fatal(err)
	}
	a := &BaselineAuthority{pool: runtime, schema: "pnl", consumerID: "pnl", generation: "g1", owner: "pnl-1", projectionVersion: "inventory-v1", epochGuard: guard.Assert}
	f.service = &Service{authority: a, release: strings.Repeat("c", 64), outbox: out}
	return f
}

func (f indexedMarkFixture) transaction(t *testing.T) pgx.Tx {
	t.Helper()
	tx, err := f.runtime.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { inventoryRollback(tx) })
	if _, err = tx.Exec(f.ctx, "SET LOCAL search_path TO pnl,pg_catalog"); err != nil {
		t.Fatal(err)
	}
	return tx
}

func indexedFillHeight(t *testing.T, fill accounting.FillV1) int64 {
	t.Helper()
	height, err := strconv.ParseInt(fill.Position.Height, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return height
}

func (f indexedMarkFixture) expected(t *testing.T, fill accounting.FillV1) *l17.Price {
	t.Helper()
	var virtual, real, reserve string
	if err := f.admin.QueryRow(f.ctx, `SELECT virtual_reserve,real_reserve,token_reserve FROM indexer.pool_state_snapshots_v1 WHERE block_number=$1 AND log_index=$2`, fill.Position.Height, fill.Position.LogIndex).Scan(&virtual, &real, &reserve); err != nil {
		t.Fatal(err)
	}
	v, err := Uint256(virtual)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Uint256(real)
	if err != nil {
		t.Fatal(err)
	}
	q, err := Uint256(reserve)
	if err != nil {
		t.Fatal(err)
	}
	seconds, err := strconv.ParseInt(fill.BlockTimestampSeconds, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	p := fill.Position
	mark, err := SpotMark(fill.Token.Address, int(fill.Token.Decimals), new(big.Int).Add(v, r), q, fill.SourceFillID, l17.ChainPosition{ChainID: p.ChainID, Height: p.Height, TransactionIndex: p.TransactionIndex, LogIndex: p.LogIndex, TransactionHash: p.TransactionHash, BlockHash: p.BlockHash}, time.Unix(seconds, 0), fill.PoolAddress)
	if err != nil {
		t.Fatal(err)
	}
	return mark
}

func assertIndexedMark(t *testing.T, got, want *l17.Price, err error) {
	t.Helper()
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("mark = %+v, want %+v, error = %v", got, want, err)
	}
}

func TestIndexedMarkReadsTheRowOfTheFill(t *testing.T) {
	parallelDatabaseTest(t)
	f := newIndexedMarkFixture(t)
	fill := f.fills[0]
	mark, err := indexedMarkPriceTx(f.ctx, f.transaction(t), fill.Token.Address, indexedFillHeight(t, fill))
	assertIndexedMark(t, mark, f.expected(t, fill), err)
	fill.Token.Decimals = 18
	raw, err := json.Marshal(fill)
	if err != nil {
		t.Fatal(err)
	}
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.admin.Exec(f.ctx, `UPDATE indexer.indexer_accounting_fills SET payload=$1,source_fact_digest=$2 WHERE source_fill_id=$3`, raw, digest, fill.SourceFillID); err != nil {
		t.Fatal(err)
	}
	mark, err = indexedMarkPriceTx(f.ctx, f.transaction(t), fill.Token.Address, indexedFillHeight(t, fill))
	assertIndexedMark(t, mark, f.expected(t, fill), err)
}

func TestIndexedMarkWithoutARow(t *testing.T) {
	parallelDatabaseTest(t)
	f := newIndexedMarkFixture(t)
	fill := f.fills[0]
	if _, err := f.admin.Exec(f.ctx, `DELETE FROM indexer.pool_state_snapshots WHERE block_number=$1`, fill.Position.Height); err != nil {
		t.Fatal(err)
	}
	tx := f.transaction(t)
	for _, token := range []string{fill.Token.Address, "0x" + strings.Repeat("e", 40)} {
		mark, err := indexedMarkPriceTx(f.ctx, tx, token, indexedFillHeight(t, fill))
		assertIndexedMark(t, mark, nil, err)
	}
}

func TestIndexedMarkWithALaterRowInTheBlock(t *testing.T) {
	parallelDatabaseTest(t)
	f := newIndexedMarkFixture(t)
	fill := f.fills[0]
	want := f.expected(t, fill)
	height := indexedFillHeight(t, fill)
	if _, err := f.admin.Exec(f.ctx, `INSERT INTO indexer.pool_state_snapshots SELECT id||'-later',pool_id,virtual_reserve,real_reserve,token_reserve,price,block_number,block_timestamp,tx_hash,log_index+1 FROM indexer.pool_state_snapshots WHERE block_number=$1`, height); err != nil {
		t.Fatal(err)
	}
	mark, err := indexedMarkPriceTx(f.ctx, f.transaction(t), fill.Token.Address, height)
	assertIndexedMark(t, mark, nil, err)
	if _, err = f.admin.Exec(f.ctx, `UPDATE indexer.pool_state_snapshots SET block_number=block_number+1 WHERE id LIKE '%-later'`); err != nil {
		t.Fatal(err)
	}
	mark, err = indexedMarkPriceTx(f.ctx, f.transaction(t), fill.Token.Address, height+1)
	assertIndexedMark(t, mark, want, err)
}

func TestIndexedMarkTakesTheNewestFillAtOrBelowTheHeight(t *testing.T) {
	parallelDatabaseTest(t)
	f := newIndexedMarkFixture(t)
	for _, tc := range []struct {
		height int64
		fill   accounting.FillV1
	}{
		{indexedFillHeight(t, f.fills[0]) - 1, accounting.FillV1{}},
		{indexedFillHeight(t, f.fills[0]) + 1, f.fills[0]},
		{indexedFillHeight(t, f.fills[1]), f.fills[1]},
	} {
		var want *l17.Price
		if tc.fill.SourceFillID != "" {
			want = f.expected(t, tc.fill)
		}
		mark, err := indexedMarkPriceTx(f.ctx, f.transaction(t), f.fills[0].Token.Address, tc.height)
		assertIndexedMark(t, mark, want, err)
	}
}

func (f indexedMarkFixture) buy(t *testing.T) (accounting.PartitionPosition, Leg) {
	t.Helper()
	fill := f.fills[0]
	for _, p := range f.publications {
		if p.SourceFillID != fill.SourceFillID {
			continue
		}
		e := *p.Enrichment
		var err error
		e.SourceFactDigest, err = accounting.FillDigest(fill)
		if err != nil {
			t.Fatal(err)
		}
		raw, err := json.Marshal(e)
		if err != nil {
			t.Fatal(err)
		}
		position := p.Positions[0]
		position.PayloadDigest = accounting.Digest(delivery.Hash(raw))
		current, err := accounting.NewEnrichmentPublication(e, []accounting.PartitionPosition{position})
		if err != nil {
			t.Fatal(err)
		}
		leg, err := InventoryLeg(fill, current)
		if err != nil {
			t.Fatal(err)
		}
		return position, leg
	}
	t.Fatal("recorded buy publication is absent")
	return accounting.PartitionPosition{}, Leg{}
}

func (f indexedMarkFixture) committedPosition(t *testing.T, snapshot string) l17.Position {
	t.Helper()
	var raw []byte
	if err := f.runtime.QueryRow(f.ctx, `SELECT payload FROM pnl.portfolio_snapshot_rows WHERE snapshot_id=$1`, snapshot).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var position l17.Position
	if err := json.Unmarshal(raw, &position); err != nil {
		t.Fatal(err)
	}
	return position
}

func assertIndexedPosition(t *testing.T, position l17.Position, want *l17.Price) {
	t.Helper()
	assertIndexedMark(t, position.MarkPrice, want, nil)
	quality := statusUnpriced
	if want != nil {
		quality = "priced"
	}
	if position.PricingQuality != quality {
		t.Fatal(position)
	}
	if (position.UnrealizedPnlRaw == nil) != (want == nil) {
		t.Fatal(position)
	}
}

func TestIndexedMarkTradePathCommitsAPrice(t *testing.T) {
	parallelDatabaseTest(t)
	for _, priced := range []bool{true, false} {
		t.Run(strconv.FormatBool(priced), func(t *testing.T) {
			f := newIndexedMarkFixture(t)
			var want *l17.Price
			if priced {
				want = f.expected(t, f.fills[0])
			} else if _, err := f.admin.Exec(f.ctx, `DELETE FROM indexer.pool_state_snapshots`); err != nil {
				t.Fatal(err)
			}
			position, leg := f.buy(t)
			consumer := portfolioReplayConsumer(t, f.service, f.runtime)
			tx := f.transaction(t)
			guarded := delivery.GuardedPosition{Position: position, Lease: delivery.PartitionLease{Key: delivery.ConsumerKey{ConsumerID: "pnl", ProjectionGeneration: "g1", PartitionKind: "inventory", PartitionKey: position.PartitionKey}, Epoch: 1}}
			state, inserted, err := ApplyOwnedLegTx(f.ctx, tx, guarded, leg, "g1")
			if err != nil || !inserted {
				t.Fatal(state, inserted, err)
			}
			row := portfolioRow{LogicalPartition: position.PartitionKey, Revision: 1, Inventory: state, Position: position, Fill: f.fills[0]}
			update, watermark, occurred, err := consumer.commitFullPortfolioTx(f.ctx, tx, guarded, leg, []portfolioRow{row})
			if err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(f.ctx); err != nil {
				t.Fatal(err)
			}
			stored := f.committedPosition(t, update.SnapshotId)
			assertIndexedPosition(t, stored, want)
			if stored.SourceWatermark.Height != f.fills[0].Position.Height || stored.KnownQuantityRaw != state.Known {
				t.Fatal(stored)
			}
			if len(update.Changes) != 1 || !update.CompleteBatch || update.Changes[0].Position == nil || update.Changes[0].Position.PricingQuality != statusUnpriced || update.Changes[0].Position.MarkPrice != nil || update.Changes[0].Position.UnrealizedPnlRaw != nil {
				t.Fatal(update)
			}
			streamPosition, err := ProjectPosition(leg.Wallet, leg.Token, state, stored.SourceWatermark, nil, statusUnpriced)
			if err != nil {
				t.Fatal(err)
			}
			raw, err := json.Marshal(update.Changes[0].Position)
			if err != nil {
				t.Fatal(err)
			}
			var actualPosition l17.Position
			if err = json.Unmarshal(raw, &actualPosition); err != nil || !reflect.DeepEqual(actualPosition, streamPosition) {
				t.Fatal(actualPosition, streamPosition, err)
			}
			var storedSummary l17.Summary
			if err = f.runtime.QueryRow(f.ctx, `SELECT summary FROM pnl.portfolio_snapshots WHERE id=$1`, update.SnapshotId).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			if err = json.Unmarshal(raw, &storedSummary); err != nil {
				t.Fatal(err)
			}
			wantStoredSummary, err := l17.Aggregate(leg.Wallet, update.SnapshotId, storedSummary.Watermark, []l17.Position{stored})
			if err != nil || !reflect.DeepEqual(storedSummary, wantStoredSummary) {
				t.Fatal(storedSummary, wantStoredSummary, err)
			}
			wantStreamSummary, err := l17.Aggregate(leg.Wallet, update.SnapshotId, storedSummary.Watermark, []l17.Position{streamPosition})
			if err != nil {
				t.Fatal(err)
			}
			raw, err = json.Marshal(update.Summary)
			if err != nil {
				t.Fatal(err)
			}
			var streamSummary l17.Summary
			if err = json.Unmarshal(raw, &streamSummary); err != nil || !reflect.DeepEqual(streamSummary, wantStreamSummary) {
				t.Fatal(streamSummary, wantStreamSummary, err)
			}
			if string(update.SummaryRevision) != storedSummary.Watermark.ProjectionRevision || update.Changes[0].PositionId != stored.PositionID || string(update.Changes[0].Revision) != stored.SourceWatermark.ProjectionRevision || string(watermark.Height) != storedSummary.Watermark.Height || string(watermark.ProjectionRevision) != storedSummary.Watermark.ProjectionRevision || watermark.BlockHash != storedSummary.Watermark.BlockHash || occurred != storedSummary.Watermark.ObservedAt {
				t.Fatal(update, watermark, occurred, storedSummary)
			}
		})
	}
}

func TestIndexedMarkRebuildPathCommitsAPrice(t *testing.T) {
	parallelDatabaseTest(t)
	for _, priced := range []bool{true, false} {
		t.Run(strconv.FormatBool(priced), func(t *testing.T) {
			f := newIndexedMarkFixture(t)
			fill := f.fills[0]
			var want *l17.Price
			if priced {
				want = f.expected(t, fill)
			} else if _, err := f.admin.Exec(f.ctx, `DELETE FROM indexer.pool_state_snapshots`); err != nil {
				t.Fatal(err)
			}
			position, leg := f.buy(t)
			state, err := Apply(Empty(), leg)
			if err != nil {
				t.Fatal(err)
			}
			seconds, err := strconv.ParseInt(fill.BlockTimestampSeconds, 10, 64)
			if err != nil {
				t.Fatal(err)
			}
			at := time.Unix(seconds, 0).UTC()
			decimals := int(fill.Token.Decimals)
			digest, err := accounting.FillDigest(fill)
			if err != nil {
				t.Fatal(err)
			}
			record := EconomicRecord{TimedLeg: TimedLeg{Leg: leg, CommittedAt: at}, BlockHash: fill.Position.BlockHash, TokenDecimals: &decimals, SourceFillID: fill.SourceFillID, SourceDigest: string(digest), State: state, Payer: leg.Payer, Recipient: leg.Recipient, FeeAsset: &fill.Settlement, FeeRaw: fill.FeeRaw, GasPAXRaw: "0"}
			records := []EconomicRecord{record}
			raw, err := json.Marshal(records)
			if err != nil {
				t.Fatal(err)
			}
			last, err := leg.Identity.Key()
			if err != nil {
				t.Fatal(err)
			}
			cp, err := RebuildDay(f.ctx, Empty(), []TimedLeg{record.TimedLeg}, at, true)
			if err != nil {
				t.Fatal(err)
			}
			replay := RebuildSnapshot{Generation: "g1", Partition: position.PartitionKey, Algorithm: Algorithm, ReleaseDigest: f.service.release, Height: indexedFillHeight(t, fill), BlockHash: fill.Position.BlockHash, ObservedThrough: at, State: state, LastIdentity: last, InputDigest: delivery.Hash(raw), Checkpoints: []Checkpoint{cp}}
			tx := f.transaction(t)
			if err = f.service.persistReplayTx(f.ctx, tx, replay, records); err != nil {
				t.Fatal(err)
			}
			if err = tx.Commit(f.ctx); err != nil {
				t.Fatal(err)
			}
			summary, err := f.service.PublishRebuiltWallet(f.ctx, leg.Wallet, replay.Height, l17.Repository{Pool: f.runtime})
			if err != nil {
				t.Fatal(err)
			}
			stored := f.committedPosition(t, summary.SnapshotID)
			assertIndexedPosition(t, stored, want)
			if stored.KnownQuantityRaw != state.Known || stored.SourceWatermark.Height != fill.Position.Height {
				t.Fatal(stored)
			}
			repeated, err := f.service.PublishRebuiltWallet(f.ctx, leg.Wallet, replay.Height, l17.Repository{Pool: f.runtime})
			if err != nil || repeated.SnapshotID != summary.SnapshotID {
				t.Fatal(repeated, err)
			}
		})
	}
}

func TestIndexedMarkRejectsInvalidData(t *testing.T) {
	parallelDatabaseTest(t)
	f := newIndexedMarkFixture(t)
	fill := f.fills[0]
	for _, change := range []struct {
		sql    string
		absent bool
	}{
		{`UPDATE indexer.pool_state_snapshots SET token_reserve='0'`, true},
		{`UPDATE indexer.pool_state_snapshots SET virtual_reserve='0',real_reserve='0'`, true},
		{`UPDATE indexer.pool_state_snapshots SET virtual_reserve='invalid'`, false},
		{`UPDATE indexer.pool_state_snapshots SET virtual_reserve=repeat('9',78),real_reserve='1'`, false},
		{`UPDATE indexer.indexer_accounting_fills SET source_fact_digest=repeat('0',64)`, false},
	} {
		tx, err := f.admin.Begin(f.ctx)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(f.ctx, change.sql); err != nil {
			t.Fatal(err)
		}
		mark, err := indexedMarkPriceTx(f.ctx, tx, fill.Token.Address, indexedFillHeight(t, fill))
		if mark != nil || (err == nil) != change.absent {
			t.Fatal(mark, err)
		}
		if err = tx.Rollback(f.ctx); err != nil {
			t.Fatal(err)
		}
	}
	canceled, cancel := context.WithCancel(f.ctx)
	cancel()
	if _, err := indexedMarkPriceTx(canceled, f.transaction(t), fill.Token.Address, indexedFillHeight(t, fill)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestIndexedMarkWithoutUSDLSettlement(t *testing.T) {
	parallelDatabaseTest(t)
	f := newIndexedMarkFixture(t)
	fill := f.fills[0]
	fill.Settlement.Address = "0x" + strings.Repeat("e", 40)
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(fill)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.admin.Exec(f.ctx, `UPDATE indexer.indexer_accounting_fills SET payload=$1,source_fact_digest=$2 WHERE source_fill_id=$3`, raw, digest, fill.SourceFillID); err != nil {
		t.Fatal(err)
	}
	mark, err := indexedMarkPriceTx(f.ctx, f.transaction(t), fill.Token.Address, indexedFillHeight(t, fill))
	assertIndexedMark(t, mark, nil, err)
}
