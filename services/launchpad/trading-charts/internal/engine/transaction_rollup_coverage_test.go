package engine

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

func TestQualifiedSeriesTransactionsAndRollups(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	for _, schema := range []func(string) (string, error){store.ExactSchemaDDL, store.SeriesSchemaDDL} {
		ddl, err := schema("candles")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	exact, err := store.NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	for index, timestamp := range []int64{1704067260, 1704067320} {
		source := seriesSource(index, timestamp, "")
		tx, err := pool.Begin(ctx)
		if err != nil {
			t.Fatal(err)
		}
		checkpoint := SeriesCheckpoint{Height: uint64(100 + index), Timestamp: timestamp + 120}
		if err = ApplySeriesTx(ctx, tx, exact, source, 1, checkpoint); err != nil {
			_ = tx.Rollback(ctx)
			t.Fatal(err)
		}
		if err = tx.Commit(ctx); err != nil {
			t.Fatal(err)
		}
		var raw []byte
		if err = pool.QueryRow(ctx, `SELECT snapshot FROM candles.chart_exact WHERE pool=$1 AND interval='1m' AND start=$2`, source.PoolAddress, timestamp).Scan(&raw); err != nil {
			t.Fatal(err)
		}
		var candle SeriesCandle
		if err = json.Unmarshal(raw, &candle); err != nil {
			t.Fatal(err)
		}
		if candle.Trades != 1 || candle.SourceWatermarkHeight != checkpoint.Height || candle.DisplayRevision != uint64(index+1) || candle.OpenSource.SourceFillID != source.SourceFillID {
			t.Fatalf("committed source candle: %+v", candle)
		}
		var pending int
		if err = pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_rollup_pending WHERE pool=$1`, source.PoolAddress).Scan(&pending); err != nil || pending != len(ExactIntervals)-1 {
			t.Fatalf("pending parent windows: %d, %v", pending, err)
		}
		completed, err := RecomputeRollups(ctx, exact, len(ExactIntervals)-1)
		if err != nil || completed != len(ExactIntervals)-1 {
			t.Fatalf("recomputed parent windows: %d, %v", completed, err)
		}
		var lineage int
		if err = pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_rollup_lineage WHERE pool=$1`, source.PoolAddress).Scan(&lineage); err != nil || lineage != len(ExactIntervals)-1 {
			t.Fatalf("qualified parent lineage: %d, %v", lineage, err)
		}
	}
	completed, err := RecomputeRollups(ctx, exact, 1)
	if err != nil || completed != 0 {
		t.Fatalf("empty rollup queue: %d, %v", completed, err)
	}
}

func TestQualifiedSeriesStreamsCanonicalCommittedSource(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	for _, schema := range []func(string) (string, error){store.ExactSchemaDDL, store.SeriesSchemaDDL} {
		ddl, err := schema("candles")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	partitionDDL, err := delivery.PartitionSchemaDDL("indexer")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `CREATE SCHEMA indexer;`+partitionDDL+`
CREATE TABLE indexer.indexer_block_completeness_v1(chain_id bigint,stream_id text,height bigint,block_hash text,fetch_complete boolean,required_decode_complete boolean,PRIMARY KEY(chain_id,stream_id,height));
CREATE TABLE indexer.indexer_accounting_fills_v1(source_fill_id text PRIMARY KEY,source_fact_digest text NOT NULL,pool_address text NOT NULL,payload jsonb NOT NULL);
CREATE TABLE indexer.indexer_accounting_publications_v1(publication_id text PRIMARY KEY,payload jsonb NOT NULL);
CREATE VIEW indexer.indexer_market_partition_positions_v1 AS SELECT * FROM indexer.delivery_partition_positions;
CREATE VIEW candles.chart_exact_v2 AS SELECT * FROM candles.chart_exact;
CREATE VIEW candles.chart_outbox_v2 AS SELECT * FROM candles.chart_outbox;
CREATE VIEW candles.chart_inbox_v2 AS SELECT * FROM candles.chart_inbox;
CREATE VIEW candles.chart_stream_sources_v2 AS SELECT * FROM candles.chart_stream_sources;
CREATE SCHEMA api_stream;
CREATE TABLE api_stream.stream_projection(partition_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,topic text NOT NULL UNIQUE,sequence bigint NOT NULL DEFAULT 0 CHECK(sequence>=0),frame bytea,updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),CHECK(sequence=0 OR frame IS NOT NULL));
CREATE TABLE api_stream.stream_delivery_outbox(partition_id bigint NOT NULL REFERENCES api_stream.stream_projection,sequence bigint NOT NULL CHECK(sequence>0),event_id text NOT NULL UNIQUE,frame bytea NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),PRIMARY KEY(partition_id,sequence));`); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, delivery.CandleStreamSourceDDL); err != nil {
		t.Fatal(err)
	}
	exact, err := store.NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	partitions, err := delivery.NewPartitionStore(pool, "indexer")
	if err != nil {
		t.Fatal(err)
	}
	source := seriesSource(0, 1704067260, "")
	digest, err := accounting.FillDigest(source)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness_v1 VALUES(125,'canonical',100,$1,true,true)`, source.Position.BlockHash); err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	position, err := partitions.AppendTx(ctx, tx, delivery.AppendInput{ProjectionVersion: "market-v1", PartitionKind: "market", PartitionKey: "125:" + source.PoolAddress, SourceEventID: source.SourceFillID, EffectIdentity: accounting.EffectIdentity(source.SourceFillID, "market", "market-v1", "", "market", "125:"+source.PoolAddress), PayloadDigest: digest, SourceOrder: source.Position})
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	publication, err := accounting.NewMarketPublication(source, []accounting.PartitionPosition{position})
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	publicationWire, err := accounting.CanonicalPublicationV1(publication)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	fillWire, err := json.Marshal(source)
	if err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO indexer.indexer_accounting_fills_v1 VALUES($1,$2,$3,$4)`, source.SourceFillID, string(digest), source.PoolAddress, fillWire); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO indexer.indexer_accounting_publications_v1 VALUES($1,$2)`, publication.PublicationID, publicationWire); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	checkpoint := SeriesCheckpoint{Height: 100, Timestamp: 1704067380}
	if err = ApplySeriesStreamTx(ctx, tx, exact, source, 1, checkpoint, &delivery.StreamWriter{Schema: "api_stream"}, position, "canonical"); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var eventID, topic string
	var frame []byte
	if err = pool.QueryRow(ctx, `SELECT event_id,topic,api_stream.stream_delivery_outbox.frame FROM api_stream.stream_projection JOIN api_stream.stream_delivery_outbox USING(partition_id) WHERE topic=$1`, "market.candles."+source.PoolAddress+".1m").Scan(&eventID, &topic, &frame); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(eventID, "candle:") || topic != "market.candles."+source.PoolAddress+".1m" || len(frame) == 0 {
		t.Fatalf("committed candle stream: %q %q %d bytes", eventID, topic, len(frame))
	}
	var snapshot []byte
	if err := pool.QueryRow(ctx, `SELECT snapshot FROM candles.chart_exact WHERE pool=$1 AND interval='1m'`, source.PoolAddress).Scan(&snapshot); err != nil {
		t.Fatal(err)
	}
	var bar SeriesCandle
	if err := json.Unmarshal(snapshot, &bar); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, sql, want string
		change          func(*SeriesCandle)
	}{
		{"missing source fill", `DELETE FROM candles.chart_inbox`, "no rows", nil},
		{"invalid source fill", `UPDATE candles.chart_inbox SET fill='{}'::jsonb`, "invalid", nil},
		{"changed candle price", "", ErrInvalidFill.Error(), func(c *SeriesCandle) { c.Open.Numerator = "999" }},
		{"missing source watermark", `DELETE FROM indexer.indexer_block_completeness_v1`, "no rows", nil},
		{"invalid source watermark time", "", ErrInvalidFill.Error(), func(c *SeriesCandle) { c.SourceWatermarkTimestamp = -62135596801 }},
		{"unavailable candle source", `ALTER TABLE candles.chart_stream_sources RENAME TO chart_stream_sources_held`, "chart_stream_sources", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			if tc.sql != "" {
				if _, err := tx.Exec(ctx, tc.sql); err != nil {
					t.Fatal(err)
				}
			}
			candidate := bar
			candidate.Revision = bar.Revision + 1
			candidate.DisplayRevision = bar.DisplayRevision + 1
			if tc.change != nil {
				tc.change(&candidate)
			}
			err = appendCandleStreamTx(ctx, tx, exact, delivery.StreamWriter{Schema: "api_stream"}, candidate, source, position, "canonical", snapshot)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("candle stream authority %s: %v", tc.name, err)
			}
		})
	}
}

func TestExactFillOuterTransaction(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	ddl, err := store.ExactSchemaDDL("candles")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	exact, err := store.NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	fill := fillFixture()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = ProcessExactFillTx(ctx, tx, exact, fill, 1); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_exact WHERE pool=$1`, fill.PoolAddress).Scan(&count); err != nil || count != len(ExactIntervals) {
		t.Fatalf("persisted exact intervals: %d, %v", count, err)
	}
}

func TestQualifiedFillOuterTransaction(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	ddl, err := store.ExactSchemaDDL("candles")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, ddl); err != nil {
		t.Fatal(err)
	}
	exact, err := store.NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	source := seriesSource(0, 1704067260, "")
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if err = ApplyQualifiedTx(ctx, tx, exact, source, 1); err != nil {
		_ = tx.Rollback(ctx)
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_exact WHERE pool=$1`, source.PoolAddress).Scan(&count); err != nil || count != len(ExactIntervals) {
		t.Fatalf("qualified exact intervals: %d, %v", count, err)
	}
	var raw []byte
	if err = pool.QueryRow(ctx, `SELECT snapshot FROM candles.chart_exact WHERE pool=$1 AND interval='1m'`, source.PoolAddress).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var candle QualifiedCandle
	if err = json.Unmarshal(raw, &candle); err != nil || candle.OpenSource.SourceFillID != source.SourceFillID || candle.Trades != 1 {
		t.Fatalf("qualified candle: %+v, %v", candle, err)
	}
}

func TestRollupWorkerRetainsUnqualifiedParentWindows(t *testing.T) {
	ctx := context.Background()
	pool := internaltest.OpenPool(t, internaltest.NewDatabase(t))
	for _, schema := range []func(string) (string, error){store.ExactSchemaDDL, store.SeriesSchemaDDL} {
		ddl, err := schema("candles")
		if err != nil {
			t.Fatal(err)
		}
		if _, err = pool.Exec(ctx, ddl); err != nil {
			t.Fatal(err)
		}
	}
	exact, err := store.NewExact(pool, "candles")
	if err != nil {
		t.Fatal(err)
	}
	address := seriesSource(0, 60, "").PoolAddress
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_rollup_pending VALUES('125',$1,'5m',0,1)`, address); err != nil {
		t.Fatal(err)
	}
	if count, err := RecomputeRollups(ctx, exact, 1); err != nil || count != 0 {
		t.Fatalf("unowned parent window: %d, %v", count, err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_pool_fences VALUES('125',$1,1,false)`, address); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE candles.chart_rollup_pending SET interval='invalid' WHERE pool=$1`, address); err != nil {
		t.Fatal(err)
	}
	if count, err := RecomputeRollups(ctx, exact, 1); count != 0 || !errors.Is(err, ErrUnsupportedResolution) {
		t.Fatalf("unsupported parent interval: %d, %v", count, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE candles.chart_rollup_pending SET interval='5m' WHERE pool=$1`, address); err != nil {
		t.Fatal(err)
	}
	if count, err := RecomputeRollups(ctx, exact, 1); count != 0 || !errors.Is(err, ErrInvalidFill) {
		t.Fatalf("parent without minute children: %d, %v", count, err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_exact VALUES('125',$1,'1m',60,'"invalid"')`, address); err != nil {
		t.Fatal(err)
	}
	if count, err := RecomputeRollups(ctx, exact, 1); count != 0 || err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Fatalf("invalid minute snapshot: %d, %v", count, err)
	}
	fill := seriesSource(0, 60, "")
	child, err := FoldQualified(nil, fill, "1m", false)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := FoldQualified(nil, fill, "5m", false)
	if err != nil {
		t.Fatal(err)
	}
	correctParentWire, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	parent.VolumeUsdlRaw = "999"
	childWire, err := json.Marshal(child)
	if err != nil {
		t.Fatal(err)
	}
	parentWire, err := json.Marshal(parent)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `UPDATE candles.chart_exact SET snapshot=$2 WHERE pool=$1 AND interval='1m'`, address, childWire); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_exact VALUES('125',$1,'5m',0,$2)`, address, parentWire); err != nil {
		t.Fatal(err)
	}
	if count, err := RecomputeRollups(ctx, exact, 1); count != 0 || !errors.Is(err, store.ErrIntegrity) {
		t.Fatalf("parent economic parity: %d, %v", count, err)
	}
	var pending int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM candles.chart_rollup_pending WHERE pool=$1`, address).Scan(&pending); err != nil || pending != 1 {
		t.Fatalf("unqualified parent retained: %d, %v", pending, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE candles.chart_exact SET snapshot=$2 WHERE pool=$1 AND interval='5m'`, address, correctParentWire); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO candles.chart_series_clock(chain_id,pool,revision,watermark_height,watermark_timestamp,has_gaps) VALUES('125',$1,1,1,300,false)`, address); err != nil {
		t.Fatal(err)
	}
	for _, relation := range []string{"chart_traders", "chart_series_clock", "chart_rollup_lineage"} {
		if _, err = pool.Exec(ctx, `ALTER TABLE candles.`+relation+` RENAME TO `+relation+`_held`); err != nil {
			t.Fatal(err)
		}
		count, failure := RecomputeRollups(ctx, exact, 1)
		if _, err = pool.Exec(ctx, `ALTER TABLE candles.`+relation+`_held RENAME TO `+relation); err != nil {
			t.Fatal(err)
		}
		if count != 0 || failure == nil || !strings.Contains(failure.Error(), relation) {
			t.Fatalf("parent without %s: count=%d err=%v", relation, count, failure)
		}
	}
	if _, err = pool.Exec(ctx, `UPDATE candles.chart_exact SET snapshot='"invalid"'::jsonb WHERE pool=$1 AND interval='5m'`, address); err != nil {
		t.Fatal(err)
	}
	if count, err := RecomputeRollups(ctx, exact, 1); count != 0 || err == nil || !strings.Contains(err.Error(), "unmarshal") {
		t.Fatalf("invalid retained parent: count=%d err=%v", count, err)
	}
}
