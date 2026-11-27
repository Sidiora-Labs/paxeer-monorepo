package projectionstats

import (
	"bytes"
	"context"
	"encoding/json"
	"math/big"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type publicStatsClockTrace struct {
	statements atomic.Int64
	baselines  atomic.Int64
}

func (r *publicStatsClockTrace) TraceQueryStart(ctx context.Context, _ *pgx.Conn, q pgx.TraceQueryStartData) context.Context {
	r.statements.Add(1)
	if strings.HasPrefix(q.SQL, "SELECT height::text,transaction_index::text,log_index::text,topics,data FROM ") {
		r.baselines.Add(1)
	}
	return ctx
}

func (*publicStatsClockTrace) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func publicStatsClockBuy(t *testing.T) (*publicFixture, publicAnchor) {
	t.Helper()
	f := newPublicFixture(t)
	f.real.Transactions = f.real.Transactions[:2]
	fill := f.ingestReal()["buy"]
	stamp, err := strconv.ParseInt(fill.BlockTimestampSeconds, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	f.advance(f.real.Transactions[1].BlockNumber, fill.Position.BlockHash)
	return f, publicAnchor{Position: Position(fill.Position), timestamp: stamp}
}

func TestPublicStatsClockWindowAgesFill(t *testing.T) {
	f, anchor := publicStatsClockBuy(t)
	p := f.projector()
	if result, err := p.RefreshOnce(f.ctx, time.Unix(anchor.timestamp+120, 0)); err != nil || result.Appended != 1 {
		t.Fatal(result, err)
	}
	before := f.frame(f.real.Pool)
	if before.frame.Payload.Volume24hRaw != "100000000" || before.frame.Payload.Buys24h != "1" || before.frame.Payload.TxCount24h != "1" || before.frame.Payload.UniqueTraders24h != "1" {
		t.Fatal(before.frame.Payload)
	}
	now := time.Unix(anchor.timestamp+86400+120, 0)
	if result, err := p.RefreshOnce(f.ctx, now); err != nil || result.Appended != 1 || result.Prices != 0 {
		t.Fatal(result, err)
	}
	after := f.frame(f.real.Pool)
	doc := after.frame.Payload
	if doc.Volume24hRaw != "0" || doc.Buys24h != "0" || doc.Sells24h != "0" || doc.TxCount24h != "0" || doc.UniqueTraders24h != "0" || doc.SourceUpperPosition != before.frame.Payload.SourceUpperPosition || after.frame.Revision != "2" || after.sequence != 2 || after.outbox != 2 || after.frame.OccurredAt != publicTime(anchor.timestamp) || doc.WindowEnd != publicTime(now.Unix()/60*60) {
		t.Fatal(after.frame)
	}
	if result, err := p.RefreshOnce(f.ctx, now.Add(-time.Hour)); err != nil || result.Appended != 0 {
		t.Fatal("window must retain the current frame's end", result, err)
	}
}

func TestPublicStatsClockWindowIdleRefresh(t *testing.T) {
	f, anchor := publicStatsClockBuy(t)
	p := f.projector()
	trace := &publicStatsClockTrace{}
	cfg := f.pg.Config()
	cfg.ConnConfig.Tracer = trace
	pool, err := pgxpool.NewWithConfig(f.ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	p.reads.repository.pool = pool
	now := time.Unix((anchor.timestamp+7200)/60*60, 0)
	if result, err := p.RefreshOnce(f.ctx, now); err != nil || result.Appended != 1 {
		t.Fatal(result, err)
	}
	before := f.frame(f.real.Pool)
	checkpoint := func() string {
		var xmin string
		if err := f.pg.QueryRow(f.ctx, "SELECT xmin::text FROM stats.public_stats_sync_baselines WHERE projection_version=$1 AND pool=$2", publicTestVersion, f.real.Pool).Scan(&xmin); err != nil {
			t.Fatal(err)
		}
		return xmin
	}
	version := checkpoint()
	for _, seconds := range []int64{10, 50} {
		trace.statements.Store(0)
		trace.baselines.Store(0)
		if result, err := p.RefreshOnce(f.ctx, now.Add(time.Duration(seconds)*time.Second)); err != nil || result.Appended != 0 || checkpoint() != version {
			t.Fatal(result, err, "baseline changed within a minute")
		}
		if trace.baselines.Load() != 0 {
			t.Fatal("cached minute read reserve baselines", trace.baselines.Load())
		}
	}
	sameMinuteStatements := trace.statements.Load()
	for i := 1; i <= 30; i++ {
		trace.statements.Store(0)
		trace.baselines.Store(0)
		if result, err := p.RefreshOnce(f.ctx, now.Add(time.Duration(i)*20*time.Second)); err != nil || result.Appended != 0 || result.Prices != 0 {
			t.Fatal(i, result, err)
		}
		if i == 3 {
			if trace.baselines.Load() != 5 || trace.statements.Load() != sameMinuteStatements+6 {
				t.Fatal("minute baseline queries", trace.baselines.Load(), trace.statements.Load(), sameMinuteStatements)
			}
			t.Logf("one pool refresh statements: same minute=%d, next minute=%d", sameMinuteStatements, trace.statements.Load())
		}
		if after := f.frame(f.real.Pool); after.frame.Revision != before.frame.Revision || after.sequence != before.sequence || !bytes.Equal(after.wire, before.wire) {
			t.Fatal(after.frame)
		}
	}
	if checkpoint() == version {
		t.Fatal("baseline must advance with the minute")
	}
}

func TestPublicStatsClockWindowPoolIsolation(t *testing.T) {
	f := newPublicFixture(t)
	first := f.real.Transactions[0]
	f.creation(first, 14)
	f.creationHoldings()
	other := *f
	other.real = f.real
	other.real.Pool = "0x" + strings.Repeat("1", 40)
	other.real.Token = "0x" + strings.Repeat("2", 40)
	creation := first
	creation.Hash = "0x" + strings.Repeat("3", 64)
	other.creation(creation, 14)
	f.advance(first.BlockNumber, first.BlockHash)
	now := time.Unix(f.real.Blocks[strconv.FormatInt(first.BlockNumber, 10)].Timestamp+60, 0)
	p := f.projector()
	if result, err := p.RefreshOnce(f.ctx, now); err != nil || result.Appended != 2 {
		t.Fatal(result, err)
	}
	before := f.frame(other.real.Pool)
	buy := f.real.Transactions[1]
	f.block(buy.BlockNumber, buy.BlockHash)
	for _, log := range buy.Logs {
		f.rawLog(buy, log)
		if log.Address == f.real.Token && log.Topics[0] == transferTopic {
			f.transfer(buy, log)
		}
		if log.Address == f.real.EventEmitter && log.Topics[0] == swapEventTopic {
			f.fill(buy, log, f.real.Blocks[strconv.FormatInt(buy.BlockNumber, 10)].Timestamp)
		}
	}
	f.advance(buy.BlockNumber, buy.BlockHash)
	if result, err := p.RefreshOnce(f.ctx, now.Add(time.Minute)); err != nil || result.Appended != 1 || result.Pools != 2 {
		t.Fatal(result, err)
	}
	if after := f.frame(other.real.Pool); !bytes.Equal(before.wire, after.wire) || after.frame.Revision != "1" {
		t.Fatal(after.frame)
	}
	if after := f.frame(f.real.Pool); after.frame.Revision != "2" || after.frame.Payload.Buys24h != "1" {
		t.Fatal(after.frame)
	}
}

func TestPublicStatsClockWindowPriceChange(t *testing.T) {
	f, anchor := publicStatsClockBuy(t)
	first := f.real.Transactions[0]
	f.openingState("10000000000", "0", "1000000000000000", first.BlockNumber, 14)
	p := f.projector()
	now := time.Unix(anchor.timestamp+240, 0)
	if result, err := p.RefreshOnce(f.ctx, now); err != nil || result.Appended != 1 {
		t.Fatal(result, err)
	}
	before := f.frame(f.real.Pool)
	if before.frame.Payload.PriceChangeBps5m == nil || *before.frame.Payload.PriceChangeBps5m == "0" || before.frame.Payload.PriceChangeBps1h == nil {
		t.Fatal(before.frame.Payload)
	}
	if result, err := p.RefreshOnce(f.ctx, now.Add(2*time.Minute)); err != nil || result.Appended != 1 {
		t.Fatal(result, err)
	}
	after := f.frame(f.real.Pool)
	if after.frame.Revision != "2" || after.frame.Payload.SourceUpperPosition != before.frame.Payload.SourceUpperPosition || after.frame.Payload.PriceChangeBps5m == nil || *after.frame.Payload.PriceChangeBps5m != "0" || after.frame.Payload.ChangeBaselineQuality5m != valueComplete || *after.frame.Payload.PriceChangeBps1h != *before.frame.Payload.PriceChangeBps1h {
		t.Fatal(after.frame.Payload)
	}
	tx, err := f.pg.Begin(f.ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(f.ctx)
	current := &priceEvent{price: new(big.Int)}
	current.price.SetString(*after.frame.Payload.SpotPriceWad, 10)
	end, err := time.Parse("2006-01-02T15:04:05.000Z", after.frame.Payload.WindowEnd)
	if err != nil {
		t.Fatal(err)
	}
	change, quality, err := p.windowChange(f.ctx, tx, anchor, f.real.Pool, nil, current, big.NewInt(10000000000000), anchor.timestamp-2, end.Unix()-300)
	if err != nil || change == nil || *change != *after.frame.Payload.PriceChangeBps5m || quality != after.frame.Payload.ChangeBaselineQuality5m {
		t.Fatal(change, quality, err)
	}
}

func publicStatsClockUpdate(t *testing.T, f *publicFixture, tx pgx.Tx, anchor publicAnchor, end int64, revision string) delivery.StreamUpdate {
	t.Helper()
	first := f.real.Transactions[0]
	created := f.real.Blocks[strconv.FormatInt(first.BlockNumber, 10)].Timestamp
	doc, _, err := f.projector().poolStatsTx(f.ctx, tx, anchor, []string{f.real.EventEmitter}, f.real.Pool, f.real.Token, first.From, created, end)
	if err != nil {
		t.Fatal(err)
	}
	doc.Watermark = PublicWatermark{ChainID: "125", Height: anchor.Height, BlockHash: anchor.BlockHash, ProjectionRevision: revision, ObservedAt: publicTime(end), Integrity: valueHealthy}
	frame := publicStatsFrame{EventID: "stats:" + f.real.Pool + ":" + revision, Topic: publicStatsTopicPrefix + f.real.Pool, Sequence: revision, Revision: revision, OccurredAt: publicTime(anchor.timestamp), Watermark: doc.Watermark, Payload: doc}
	wire, err := json.Marshal(frame)
	if err != nil || ValidatePublicStatsFrame(wire) != nil {
		t.Fatal("frame binding", err, string(wire))
	}
	watermark, err := json.Marshal(doc.Watermark)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	return delivery.StreamUpdate{EventID: frame.EventID, Topic: frame.Topic, Revision: revision, OccurredAt: frame.OccurredAt, Watermark: watermark, Payload: payload, Source: delivery.StreamSourceProof{Kind: "stats_commit", Pool: f.real.Pool, ProjectionVersion: publicTestVersion}}
}

func TestPublicStatsClockWindowDatabaseBounds(t *testing.T) {
	f, anchor := publicStatsClockBuy(t)
	writer := delivery.StreamWriter{Schema: "api_stream"}
	for _, delta := range []int64{1, 120, 0} {
		t.Run("accepted_"+strconv.FormatInt(delta, 10), func(t *testing.T) {
			tx, err := f.pg.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(f.ctx)
			end := anchor.timestamp + delta
			if delta == 0 {
				var clock time.Time
				if err = tx.QueryRow(f.ctx, "SELECT clock_timestamp()").Scan(&clock); err != nil {
					t.Fatal(err)
				}
				end = clock.Unix()
			}
			update := publicStatsClockUpdate(t, f, tx, anchor, end, "1")
			if err = writer.AppendTx(f.ctx, tx, update); err != nil {
				t.Fatal(err)
			}
			var raw []byte
			if err = tx.QueryRow(f.ctx, "SELECT frame FROM api_stream.stream_projection WHERE topic=$1", update.Topic).Scan(&raw); err != nil || ValidatePublicStatsFrame(raw) != nil {
				t.Fatal(err, string(raw))
			}
		})
	}
	if result, err := f.projector().RefreshOnce(f.ctx, time.Unix(anchor.timestamp+120, 0)); err != nil || result.Appended != 1 {
		t.Fatal(result, err)
	}
	prior := f.frame(f.real.Pool)
	priorEnd, err := time.Parse("2006-01-02T15:04:05.000Z", prior.frame.Payload.WindowEnd)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		end     int64
		edit    func(*delivery.StreamUpdate, *PublicPoolStats)
		message string
	}{
		{name: "before anchor", end: anchor.timestamp, message: "stats window binding mismatch"},
		{name: "before current window", end: priorEnd.Unix() - 1, message: "stats window binding mismatch"},
		{name: "future clock", message: "stats window binding mismatch"},
		{name: "window duration", end: anchor.timestamp + 180, edit: func(_ *delivery.StreamUpdate, d *PublicPoolStats) { d.WindowStart = publicTime(anchor.timestamp) }, message: "stats window binding mismatch"},
		{name: "occurrence time", end: anchor.timestamp + 180, edit: func(u *delivery.StreamUpdate, _ *PublicPoolStats) { u.OccurredAt = publicTime(anchor.timestamp + 1) }, message: "stats window binding mismatch"},
		{name: "aggregate window", end: anchor.timestamp + 180, edit: func(_ *delivery.StreamUpdate, d *PublicPoolStats) {
			d.WindowEnd = publicTime(anchor.timestamp + 86400 + 120)
			d.WindowStart = publicTime(anchor.timestamp + 120)
		}, message: "stats window aggregate differs from committed fills"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := f.pg.Begin(f.ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer tx.Rollback(f.ctx)
			end := tc.end
			if tc.name == "future clock" {
				var clock time.Time
				if err = tx.QueryRow(f.ctx, "SELECT clock_timestamp()").Scan(&clock); err != nil {
					t.Fatal(err)
				}
				end = clock.Unix() + 3600
			}
			update := publicStatsClockUpdate(t, f, tx, anchor, end, "2")
			var doc PublicPoolStats
			if err = json.Unmarshal(update.Payload, &doc); err != nil {
				t.Fatal(err)
			}
			if tc.edit != nil {
				tc.edit(&update, &doc)
			}
			update.Payload, err = json.Marshal(doc)
			if err != nil {
				t.Fatal(err)
			}
			if err = writer.AppendTx(f.ctx, tx, update); err == nil || !strings.Contains(err.Error(), tc.message) {
				t.Fatalf("%s: %v", tc.name, err)
			}
		})
	}
	if after := f.frame(f.real.Pool); !bytes.Equal(prior.wire, after.wire) {
		t.Fatal(after.frame)
	}
}

func TestPublicStatsClockWindowLegacyFrame(t *testing.T) {
	f, anchor := publicStatsClockBuy(t)
	p := f.projector()
	if result, err := p.RefreshOnce(f.ctx, time.Unix(anchor.timestamp+1, 0)); err != nil || result.Appended != 1 {
		t.Fatal(result, err)
	}
	legacy := f.frame(f.real.Pool)
	if legacy.frame.Payload.WindowEnd != publicTime(anchor.timestamp+1) || ValidatePublicStatsFrame(legacy.wire) != nil {
		t.Fatal(legacy.frame)
	}
	if result, err := p.RefreshOnce(f.ctx, time.Unix(anchor.timestamp+86400+120, 0)); err != nil || result.Appended != 1 {
		t.Fatal(result, err)
	}
	next := f.frame(f.real.Pool)
	if next.frame.Revision != "2" || next.frame.Payload.Volume24hRaw != "0" || next.frame.Payload.SourceUpperPosition != legacy.frame.Payload.SourceUpperPosition || next.frame.Payload.WindowEnd <= legacy.frame.Payload.WindowEnd || ValidatePublicStatsFrame(next.wire) != nil || ValidatePublicStatsFrame(legacy.wire) != nil {
		t.Fatal(next.frame)
	}
}

func TestPublicStatsClockWindowDatabaseClock(t *testing.T) {
	f, anchor := publicStatsClockBuy(t)
	var before, after time.Time
	if err := f.pg.QueryRow(f.ctx, "SELECT clock_timestamp()").Scan(&before); err != nil {
		t.Fatal(err)
	}
	if result, err := f.projector().RefreshOnce(f.ctx, before.Add(time.Hour)); err != nil || result.Appended != 1 {
		t.Fatal(result, err)
	}
	if err := f.pg.QueryRow(f.ctx, "SELECT clock_timestamp()").Scan(&after); err != nil {
		t.Fatal(err)
	}
	frame := f.frame(f.real.Pool).frame
	end, err := time.Parse("2006-01-02T15:04:05.000Z", frame.Payload.WindowEnd)
	if err != nil || end.Before(before.Truncate(publicStatsClockStep)) || end.After(after.Truncate(publicStatsClockStep)) || frame.Payload.SourceUpperPosition != anchor.Position || frame.OccurredAt != publicTime(anchor.timestamp) || frame.Payload.Volume24hRaw != "0" {
		t.Fatal(frame, err)
	}
}
