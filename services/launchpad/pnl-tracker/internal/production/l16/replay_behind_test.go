package l16

import (
	"context"
	"encoding/json"
	"errors"
	"math/big"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/pnl-tracker/internal/production/l17"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	interfaces "github.com/Sidiora-Technologies/KindleLaunch/shared/interfaces/v2"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type portfolioReplayFixture struct {
	ctx             context.Context
	pool            *pgxpool.Pool
	consumer        *AccountingConsumer
	buy, sale       portfolioRow
	buyLeg, saleLeg Leg
}

func portfolioReplayConsumer(t *testing.T, service *Service, pool *pgxpool.Pool) *AccountingConsumer {
	t.Helper()
	a := service.authority
	c, err := NewAccountingConsumerWithRuntime(pool, "pnl", a.consumerID, a.generation, a.owner, a.projectionVersion, a, a.epochGuard, delivery.StreamWriter{Schema: "api_stream"})
	if err != nil {
		t.Fatal(err)
	}
	c, err = c.WithFullPortfolio(l17.Repository{Pool: pool}, service.outbox, "portfolio.snapshot.v2")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func readPortfolioReplayRow(t *testing.T, ctx context.Context, pool *pgxpool.Pool, generation string) portfolioRow {
	t.Helper()
	var row portfolioRow
	var state, position, fill []byte
	version, err := ownedVersion(generation)
	if err != nil {
		t.Fatal(err)
	}
	err = pool.QueryRow(ctx, `SELECT e.partition,s.revision,s.state,e.position,f.payload FROM pnl.inventory_l16 s JOIN pnl.inventory_l16_effects e ON e.projection_generation=$1 AND s.partition=$2||':'||e.partition AND e.leg_identity=s.last_identity JOIN indexer.indexer_accounting_fills_v1 f ON f.source_fill_id=convert_from(e.position,'UTF8')::jsonb->>'sourceEventId'`, generation, version).Scan(&row.LogicalPartition, &row.Revision, &state, &position, &fill)
	if err != nil {
		t.Fatal(err)
	}
	row.Inventory, err = DecodeInventoryState(state)
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(position, &row.Position); err != nil {
		t.Fatal(err)
	}
	row.Fill, err = accounting.DecodeFillV1(fill)
	if err != nil {
		t.Fatal(err)
	}
	return row
}

func newPortfolioReplayFixture(t *testing.T, throughSale bool) portfolioReplayFixture {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	t.Cleanup(cancel)
	service, pool, frames := currentRecordedInventoryFrames(t, ctx)
	f := portfolioReplayFixture{ctx: ctx, pool: pool, consumer: portfolioReplayConsumer(t, service, pool)}
	for i, frame := range frames {
		var publication accounting.PublicationV1
		if err := json.Unmarshal(frame.Payload, &publication); err != nil {
			t.Fatal(err)
		}
		var row portfolioRow
		if i == 0 || throughSale {
			consumeCurrentInventoryFrames(t, ctx, service, []delivery.Envelope{frame})
			row = readPortfolioReplayRow(t, ctx, pool, f.consumer.generation)
		} else {
			var raw []byte
			if err := pool.QueryRow(ctx, `SELECT payload FROM indexer.indexer_accounting_fills_v1 WHERE source_fill_id=$1`, publication.SourceFillID).Scan(&raw); err != nil {
				t.Fatal(err)
			}
			fill, err := accounting.DecodeFillV1(raw)
			if err != nil {
				t.Fatal(err)
			}
			row = portfolioRow{LogicalPartition: f.buy.LogicalPartition, Revision: 2, Position: publication.Positions[0], Fill: fill}
		}
		leg, err := InventoryLeg(row.Fill, publication)
		if err != nil {
			t.Fatal(err)
		}
		if i == 0 {
			f.buy, f.buyLeg = row, leg
		} else {
			if !throughSale {
				row.Inventory, err = Apply(f.buy.Inventory, leg)
				if err != nil {
					t.Fatal(err)
				}
			}
			f.sale, f.saleLeg = row, leg
		}
	}
	if f.buyLeg.Kind != sideBuy || f.saleLeg.Kind != sideSell || f.buy.Inventory == f.sale.Inventory {
		t.Fatal(f.buy, f.sale)
	}
	return f
}

func (f portfolioReplayFixture) transaction(t *testing.T) pgx.Tx {
	t.Helper()
	tx, err := f.pool.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { inventoryRollback(tx) })
	if _, err = tx.Exec(f.ctx, "SET LOCAL search_path TO pnl,pg_catalog"); err != nil {
		t.Fatal(err)
	}
	return tx
}

func putPortfolioReplay(t *testing.T, ctx context.Context, tx pgx.Tx, snapshot RebuildSnapshot) {
	t.Helper()
	raw, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	digest := delivery.Hash(raw)
	if _, err = tx.Exec(ctx, `INSERT INTO inventory_l16_snapshots_v2(generation,partition,snapshot_digest,payload,height,block_hash,release_digest) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT DO NOTHING`, snapshot.Generation, snapshot.Partition, digest, raw, snapshot.Height, snapshot.BlockHash, strings.Repeat("c", 64)); err != nil {
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO inventory_l16_active_v2(partition,generation,snapshot_digest,height) VALUES($1,$2,$3,$4) ON CONFLICT(partition) DO UPDATE SET generation=EXCLUDED.generation,snapshot_digest=EXCLUDED.snapshot_digest,height=EXCLUDED.height`, snapshot.Partition, snapshot.Generation, digest, snapshot.Height); err != nil {
		t.Fatal(err)
	}
}

func (f portfolioReplayFixture) replay() RebuildSnapshot {
	return RebuildSnapshot{Generation: f.consumer.generation, Partition: f.sale.LogicalPartition, Height: int64(f.saleLeg.Height), BlockHash: f.sale.Fill.Position.BlockHash, State: f.sale.Inventory}
}

func (f portfolioReplayFixture) commit(tx pgx.Tx, row portfolioRow, leg Leg) (interfaces.PortfolioUpdate, error) {
	update, _, _, err := f.consumer.commitFullPortfolioTx(f.ctx, tx, delivery.GuardedPosition{Position: row.Position}, leg, []portfolioRow{row})
	return update, err
}

func assertPortfolioReplayState(t *testing.T, update interfaces.PortfolioUpdate, row portfolioRow) {
	t.Helper()
	if len(update.Changes) != 1 || !update.CompleteBatch || update.Changes[0].Position == nil {
		t.Fatal(update)
	}
	position := update.Changes[0].Position
	if string(position.KnownQuantityRaw) != row.Inventory.Known || string(position.UnknownQuantityRaw) != row.Inventory.Unknown || string(position.KnownRemainingCostUsdlRaw) != row.Inventory.Cost || string(position.KnownRealizedSubtotalUsdlRaw) != row.Inventory.Realized || string(position.SourceWatermark.Height) != row.Fill.Position.Height {
		t.Fatal(position, row)
	}
}

func assertPortfolioReplayGap(t *testing.T, err error, site string) {
	t.Helper()
	if !errors.Is(err, delivery.ErrGap) || !strings.Contains(err.Error(), site) {
		t.Fatalf("gap at %q: %v", site, err)
	}
	if strings.Contains(err.Error(), realMarketCreator) || strings.Contains(err.Error(), realMarketToken) {
		t.Fatal(err)
	}
}

func TestPortfolioReplayBehindAppliesAFillBelowALaterFill(t *testing.T) {
	f := newPortfolioReplayFixture(t, false)
	tx := f.transaction(t)
	replay := f.replay()
	replay.Height++
	putPortfolioReplay(t, f.ctx, tx, replay)
	// The wallet may already have published its replay before the ledger catches up.
	prior, err := f.commit(tx, f.sale, f.saleLeg)
	if err != nil {
		t.Fatal(err)
	}
	update, err := f.commit(tx, f.buy, f.buyLeg)
	if err != nil {
		t.Fatal(err)
	}
	assertPortfolioReplayState(t, update, f.buy)
	previous, ok := new(big.Int).SetString(string(prior.SummaryRevision), 10)
	if !ok {
		t.Fatal(prior)
	}
	current, ok := new(big.Int).SetString(string(update.SummaryRevision), 10)
	if !ok || current.Cmp(previous) <= 0 {
		t.Fatal(prior, update)
	}
}

func TestPortfolioReplayBehindComparesAtTheLastFill(t *testing.T) {
	f := newPortfolioReplayFixture(t, true)
	for _, mismatch := range []bool{false, true} {
		t.Run(strconv.FormatBool(mismatch), func(t *testing.T) {
			tx := f.transaction(t)
			replay := f.replay()
			if mismatch {
				replay.State.Known = f.buy.Inventory.Known
			}
			putPortfolioReplay(t, f.ctx, tx, replay)
			update, err := f.commit(tx, f.sale, f.saleLeg)
			if mismatch {
				assertPortfolioReplayGap(t, err, "portfolio replay state mismatch")
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			assertPortfolioReplayState(t, update, f.sale)
		})
	}
}

func TestPortfolioReplayBehindKeepsTheGapWithoutALaterFill(t *testing.T) {
	f := newPortfolioReplayFixture(t, false)
	for _, unrelated := range []string{"none", "wallet", "token"} {
		t.Run(unrelated, func(t *testing.T) {
			tx := f.transaction(t)
			switch unrelated {
			case "none":
				if _, err := tx.Exec(f.ctx, `DELETE FROM indexer.indexer_accounting_fills WHERE source_fill_id=$1`, f.sale.Fill.SourceFillID); err != nil {
					t.Fatal(err)
				}
			case "wallet":
				if _, err := tx.Exec(f.ctx, `UPDATE indexer.indexer_accounting_fills SET payload=jsonb_set(payload,'{attribution}',jsonb_build_object('transactionSender',$2::text,'payer',$2::text,'recipient',$2::text,'trader',$2::text)) WHERE source_fill_id=$1`, f.sale.Fill.SourceFillID, "0x"+strings.Repeat("e", 40)); err != nil {
					t.Fatal(err)
				}
			case "token":
				if _, err := tx.Exec(f.ctx, `UPDATE indexer.indexer_accounting_fills SET payload=jsonb_set(payload,'{token,address}',to_jsonb($2::text)) WHERE source_fill_id=$1`, f.sale.Fill.SourceFillID, "0x"+strings.Repeat("e", 40)); err != nil {
					t.Fatal(err)
				}
			}
			putPortfolioReplay(t, f.ctx, tx, f.replay())
			_, err := f.commit(tx, f.buy, f.buyLeg)
			assertPortfolioReplayGap(t, err, "portfolio replay state mismatch")
		})
	}
}

func TestPortfolioReplayBehindAcceptsAReplayWithoutARow(t *testing.T) {
	f := newPortfolioReplayFixture(t, false)
	for _, indexed := range []bool{true, false} {
		t.Run(strconv.FormatBool(indexed), func(t *testing.T) {
			tx := f.transaction(t)
			replay := f.replay()
			putPortfolioReplay(t, f.ctx, tx, replay)
			token := "0x" + strings.Repeat("e", 40)
			replay.Partition = "125:" + f.saleLeg.Wallet + ":" + token
			putPortfolioReplay(t, f.ctx, tx, replay)
			if indexed {
				fill := f.sale.Fill
				fill.Token.Address = token
				raw, err := json.Marshal(fill)
				if err != nil {
					t.Fatal(err)
				}
				digest, err := accounting.FillDigest(fill)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = tx.Exec(f.ctx, `UPDATE indexer.indexer_accounting_fills SET payload=$2,source_fact_digest=$3 WHERE source_fill_id=$1`, f.sale.Fill.SourceFillID, raw, string(digest)); err != nil {
					t.Fatal(err)
				}
			}
			// Use the buy row's state for its own replay so only the second coin is behind.
			own := f.replay()
			own.State = f.buy.Inventory
			putPortfolioReplay(t, f.ctx, tx, own)
			update, err := f.commit(tx, f.buy, f.buyLeg)
			if !indexed {
				assertPortfolioReplayGap(t, err, "portfolio replay without ledger row")
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			assertPortfolioReplayState(t, update, f.buy)
		})
	}
}

func TestPortfolioReplayBehindCountsOnlyAFillUpToTheReplay(t *testing.T) {
	f := newPortfolioReplayFixture(t, false)
	for _, sameBlock := range []bool{false, true} {
		t.Run(strconv.FormatBool(sameBlock), func(t *testing.T) {
			tx := f.transaction(t)
			replay := f.replay()
			replay.Height = int64(f.buyLeg.Height)
			if sameBlock {
				// Keep the sale's identity while placing it later in the buy's block.
				fill := f.sale.Fill
				fill.Position.Height = f.buy.Fill.Position.Height
				fill.Position.BlockHash = f.buy.Fill.Position.BlockHash
				fill.Position.TransactionIndex = f.buy.Fill.Position.TransactionIndex
				index, err := strconv.ParseUint(f.buy.Fill.Position.LogIndex, 10, 32)
				if err != nil {
					t.Fatal(err)
				}
				fill.Position.LogIndex = strconv.FormatUint(index+1, 10)
				fill.Position.TransactionHash = f.buy.Fill.Position.TransactionHash
				fill.SourceFillID = accounting.SourceFillIdentity(fill.Position, fill.Provenance.DeploymentID)
				raw, err := json.Marshal(fill)
				if err != nil {
					t.Fatal(err)
				}
				digest, err := accounting.FillDigest(fill)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = tx.Exec(f.ctx, `UPDATE indexer.indexer_accounting_fills SET payload=$2,source_fact_digest=$3 WHERE source_fill_id=$1`, f.sale.Fill.SourceFillID, raw, string(digest)); err != nil {
					t.Fatal(err)
				}
			}
			putPortfolioReplay(t, f.ctx, tx, replay)
			update, err := f.commit(tx, f.buy, f.buyLeg)
			if !sameBlock {
				assertPortfolioReplayGap(t, err, "portfolio replay state mismatch")
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			assertPortfolioReplayState(t, update, f.buy)
		})
	}
}

func TestPortfolioReplayBehindConsumerAppliesBuyThenSale(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	service, pool, frames := currentRecordedInventoryFrames(t, ctx)
	replay, err := service.Rebuild(ctx, realMarketCreator, realMarketToken, 33963749)
	if err != nil {
		t.Fatal(err)
	}
	consumer := portfolioReplayConsumer(t, service, pool)
	consumeCurrentInventoryFrames(t, ctx, service, frames, consumer)
	row := readPortfolioReplayRow(t, ctx, pool, consumer.generation)
	if row.Inventory != replay.State || row.Revision != 2 {
		t.Fatal(row, replay)
	}
	var snapshots, deliveries, effects int
	if err = pool.QueryRow(ctx, `SELECT (SELECT count(*) FROM pnl.portfolio_snapshots WHERE wallet=$1),(SELECT count(*) FROM pnl.delivery_inbox WHERE consumer_id=$2 AND projection_generation=$3),(SELECT count(*) FROM pnl.inventory_l16_effects WHERE projection_generation=$3)`, realMarketCreator, consumer.consumerID, consumer.generation).Scan(&snapshots, &deliveries, &effects); err != nil || snapshots != 2 || deliveries != 2 || effects != 2 {
		t.Fatal(snapshots, deliveries, effects, err)
	}
	revisions, err := pool.Query(ctx, `SELECT revision::text FROM pnl.portfolio_snapshots WHERE wallet=$1 ORDER BY revision`, realMarketCreator)
	if err != nil {
		t.Fatal(err)
	}
	defer revisions.Close()
	previous := new(big.Int)
	count := 0
	for revisions.Next() {
		var revision string
		if err = revisions.Scan(&revision); err != nil {
			t.Fatal(err)
		}
		next, ok := new(big.Int).SetString(revision, 10)
		if !ok || next.Cmp(previous) <= 0 {
			t.Fatal(revision, previous)
		}
		previous = next
		count++
	}
	if err = revisions.Err(); err != nil || count != 2 {
		t.Fatal(count, err)
	}
}
