package store_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/internaltest"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5/pgxpool"
)

type repair75Fixture struct {
	dsn        string
	db         *pgxpool.Pool
	st         *store.ExactStore
	fills      []accounting.FillV1
	checkpoint engine.SeriesCheckpoint
}

func repair75JSON(t *testing.T, v any) []byte {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
func repair75Digest(b []byte) string { sum := sha256.Sum256(b); return hex.EncodeToString(sum[:]) }
func repair75Exec(t *testing.T, db *pgxpool.Pool, sql string, args ...any) {
	t.Helper()
	if _, err := db.Exec(context.Background(), sql, args...); err != nil {
		t.Fatal(err)
	}
}
func repair75New(t *testing.T) *repair75Fixture {
	t.Helper()
	dsn := internaltest.NewDatabase(t)
	db := internaltest.OpenPool(t, dsn)
	for _, ddl := range []func(string) (string, error){store.ExactSchemaDDL, store.SeriesSchemaDDL, store.RebuildSchemaDDL} {
		sql, err := ddl("candles")
		if err != nil {
			t.Fatal(err)
		}
		repair75Exec(t, db, sql)
	}
	partitionDDL, err := delivery.PartitionSchemaDDL("indexer")
	if err != nil {
		t.Fatal(err)
	}
	repair75Exec(t, db, `CREATE SCHEMA indexer;`+partitionDDL)
	// These are the read contracts consumed by the production append function.
	// Positions and publications below come from the real canonical producers.
	repair75Exec(t, db, `CREATE TABLE indexer.indexer_accounting_fills_v1(source_fill_id text PRIMARY KEY,source_fact_digest text NOT NULL,pool_address text NOT NULL,payload jsonb NOT NULL);
 CREATE TABLE indexer.indexer_accounting_publications_v1(publication_id text PRIMARY KEY,payload jsonb NOT NULL);
 CREATE VIEW indexer.indexer_market_partition_positions_v1 AS SELECT * FROM indexer.delivery_partition_positions;
 CREATE TABLE indexer.indexer_block_completeness_v1(chain_id bigint,stream_id text,height bigint,block_hash text,fetch_complete boolean,required_decode_complete boolean,PRIMARY KEY(chain_id,stream_id,height));
 CREATE VIEW candles.chart_exact_v2 AS SELECT * FROM candles.chart_exact;
 CREATE VIEW candles.chart_outbox_v2 AS SELECT * FROM candles.chart_outbox;
 CREATE VIEW candles.chart_inbox_v2 AS SELECT * FROM candles.chart_inbox;
 CREATE VIEW candles.chart_stream_sources_v2 AS SELECT * FROM candles.chart_stream_sources;
 CREATE SCHEMA api_stream;
 CREATE TABLE api_stream.stream_projection(partition_id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,topic text NOT NULL UNIQUE,sequence bigint NOT NULL DEFAULT 0 CHECK(sequence>=0),frame bytea,updated_at timestamptz NOT NULL DEFAULT clock_timestamp(),CHECK(sequence=0 OR frame IS NOT NULL));
 CREATE TABLE api_stream.stream_delivery_outbox(partition_id bigint NOT NULL REFERENCES api_stream.stream_projection,sequence bigint NOT NULL CHECK(sequence>0),event_id text NOT NULL UNIQUE,frame bytea NOT NULL,created_at timestamptz NOT NULL DEFAULT clock_timestamp(),PRIMARY KEY(partition_id,sequence));`)
	repair75Exec(t, db, delivery.CandleStreamSourceDDL)
	st, err := store.NewExact(db, "candles")
	if err != nil {
		t.Fatal(err)
	}
	f := &repair75Fixture{dsn: dsn, db: db, st: st, checkpoint: engine.SeriesCheckpoint{Height: 102, Timestamp: 1700001000}}
	partitions, err := delivery.NewPartitionStore(db, "indexer")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= 2; i++ {
		source := accounting.FillV1{Version: accounting.FillVersion, Position: accounting.SourcePosition{ChainID: "125", Height: fmt.Sprint(100 + i), TransactionIndex: "0", LogIndex: "1", TransactionHash: fmt.Sprintf("0x%064x", i+1), BlockHash: fmt.Sprintf("0x%064x", i+1000)}, ActualEmitter: "0x" + strings.Repeat("1", 40), PoolAddress: "0x" + strings.Repeat("2", 40), Token: accounting.Asset{Address: "0x" + strings.Repeat("3", 40), Decimals: 6}, Settlement: accounting.Asset{Address: "0x" + strings.Repeat("4", 40), Decimals: 6}, Side: "buy", TokenAmountRaw: "1000000", USDLAmountRaw: fmt.Sprint(1000000 + i), PaidInputRaw: fmt.Sprint(1000000 + i), FeeRaw: "100", FeeDisposition: "distributed_usdl", InputDerivationQuality: "receipt_incomplete", BlockTimestampSeconds: fmt.Sprint(1700000000 + i), Attribution: accounting.Attribution{TransactionSender: "0x" + strings.Repeat("5", 40), Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, Execution: accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, Provenance: accounting.Provenance{DeploymentID: "repair-75", ABIRevision: "sidiora-v1", EmitterCodeEpoch: "epoch-1", PoolCodeEpoch: "epoch-1", ReceiptSHA256: strings.Repeat("c", 64), HeaderSHA256: strings.Repeat("d", 64)}}
		source.SourceFillID = accounting.SourceFillIdentity(source.Position, source.Provenance.DeploymentID)
		digest, err := accounting.FillDigest(source)
		if err != nil {
			t.Fatal(err)
		}
		tx, err := db.Begin(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = tx.Rollback(context.Background()) })
		position, err := partitions.AppendTx(context.Background(), tx, delivery.AppendInput{ProjectionVersion: "market-v1", PartitionKind: "market", PartitionKey: "125:" + source.PoolAddress, SourceEventID: source.SourceFillID, EffectIdentity: accounting.HashTuple("effect-v1", source.SourceFillID, "market", "market-v1", "", "market", "125:"+source.PoolAddress), PayloadDigest: digest, SourceOrder: source.Position})
		if err != nil {
			t.Fatal(err)
		}
		publication, err := accounting.NewMarketPublication(source, []accounting.PartitionPosition{position})
		if err != nil {
			t.Fatal(err)
		}
		wire, err := accounting.CanonicalPublicationV1(publication)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(context.Background(), `INSERT INTO indexer.indexer_accounting_fills_v1 VALUES($1,$2,$3,$4)`, source.SourceFillID, string(digest), source.PoolAddress, repair75JSON(t, source)); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(context.Background(), `INSERT INTO indexer.indexer_accounting_publications_v1 VALUES($1,$2)`, publication.PublicationID, wire); err != nil {
			t.Fatal(err)
		}
		if i == 1 {
			if _, err = tx.Exec(context.Background(), `INSERT INTO indexer.indexer_block_completeness_v1 VALUES(125,'canonical',102,$1,true,true)`, fmt.Sprintf("0x%064x", 1002)); err != nil {
				t.Fatal(err)
			}
			writer := delivery.StreamWriter{Schema: "api_stream"}
			if err = engine.ApplySeriesStreamTx(context.Background(), tx, st, source, 9, f.checkpoint, &writer, position, "canonical"); err != nil {
				t.Fatal(err)
			}
		}
		if err = tx.Commit(context.Background()); err != nil {
			t.Fatal(err)
		}
		f.fills = append(f.fills, source)
	}
	return f
}
func (f *repair75Fixture) stage(t *testing.T, id string, base uint64, alter func([]store.StagedCandle)) []engine.SeriesCandle {
	t.Helper()
	bars := []store.StagedCandle{}
	states := []engine.SeriesCandle{}
	for _, interval := range engine.ExactIntervals {
		var bar *engine.QualifiedCandle
		for _, source := range f.fills {
			next, err := engine.FoldQualified(bar, source, interval, false)
			if err != nil {
				t.Fatal(err)
			}
			bar = &next
		}
		state, err := engine.DescribeSeries(*bar, f.checkpoint, time.Unix(1700001001, 0))
		if err != nil {
			t.Fatal(err)
		}
		state.CorrectionReason = "archive_reconstruction"
		states = append(states, state)
		bars = append(bars, store.StagedCandle{Interval: interval, Start: state.Start, Snapshot: repair75JSON(t, state)})
	}
	if alter != nil {
		alter(bars)
	}
	// Archive authentication remains separate from market publication authority.
	// Decode an actual raw archive segment before staging its fills.
	records := []store.ArchiveRaw{}
	events := []store.ArchiveEvent{}
	blocks := []store.ArchiveBlock{}
	for i, fill := range f.fills {
		raw := store.ArchiveRaw{Height: int64(101 + i), TransactionHash: fill.Position.TransactionHash, BlockHash: fill.Position.BlockHash, TransactionIndex: 0, LogIndex: 1, Emitter: fill.ActualEmitter, Topics: []string{}, Data: []byte{}}
		records = append(records, raw)
		events = append(events, store.ArchiveEvent{Kind: accounting.MarketSubject, Payload: repair75JSON(t, fill), RawDigest: repair75Digest(repair75JSON(t, raw)), Raw: raw, Timestamp: 1700000001 + int64(i)})
		blocks = append(blocks, store.ArchiveBlock{Height: raw.Height, Hash: raw.BlockHash, RawCount: 1, FetchComplete: true, RequiredDecodeComplete: true})
	}
	segment := store.ArchiveSegment{FormatVersion: "raw-replay-v2", ChainID: 125, SourceStream: "canonical", FromHeight: 101, ToHeight: 102, Blocks: blocks, Records: records, Events: events}
	raw := repair75JSON(t, segment)
	digest := repair75Digest(raw)
	manifest := store.ArchiveManifest{FormatVersion: "raw-archive-v2", ChainID: 125, SourceStream: "canonical", FromHeight: 101, ToHeight: 102, EventCount: 2, TypedCount: 2, ByteLength: int64(len(raw)), SHA256: digest, Bucket: "private-archive", ObjectKey: "indexer/raw/v2/canonical/" + digest + ".json", Blocks: blocks}
	decoded, err := store.ValidateArchive(raw, manifest)
	if err != nil {
		t.Fatal(err)
	}
	sorted := append([]engine.SeriesCandle(nil), states...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Interval < sorted[j].Interval })
	economicDigests := []string{}
	for _, state := range sorted {
		digest, err := engine.EconomicDigest(state.QualifiedCandle)
		if err != nil {
			t.Fatal(err)
		}
		economicDigests = append(economicDigests, digest)
	}
	outputDigest := repair75Digest(repair75JSON(t, economicDigests))
	manifestDigests := []string{repair75Digest(repair75JSON(t, manifest))}
	manifestDigest := repair75Digest(repair75JSON(t, manifestDigests))
	if err = f.st.StageRebuild(context.Background(), id, f.fills[0].PoolAddress, states[0].Start, states[0].IntervalEnd, manifestDigest, outputDigest, outputDigest, base, bars, nil, decoded); err != nil {
		t.Fatal(err)
	}
	corrupt := append([]byte(nil), raw...)
	corrupt[len(corrupt)/2] ^= 1
	if _, err = store.ValidateArchive(corrupt, manifest); err == nil {
		t.Fatal("corrupt archive authorized")
	}
	return states
}
func (f *repair75Fixture) reopen(t *testing.T) {
	t.Helper()
	f.db.Close()
	f.db = internaltest.OpenPool(t, f.dsn)
	var err error
	f.st, err = store.NewExact(f.db, "candles")
	if err != nil {
		t.Fatal(err)
	}
}
func repair75EqualJSON(t *testing.T, a, b []byte) {
	t.Helper()
	var x, y any
	if err := json.Unmarshal(a, &x); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &y); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(x, y) {
		t.Fatalf("canonical payload mismatch:\n%s\n%s", a, b)
	}
}

func TestRepair_75_HistorySnapshotReplayConvergeAfterRestart(t *testing.T) {
	for _, removeHistory := range []bool{false, true} {
		t.Run(fmt.Sprint("retained_away=", removeHistory), func(t *testing.T) {
			f := repair75New(t)
			states := f.stage(t, "reconstruction", 1, nil)
			if removeHistory {
				repair75Exec(t, f.db, `DELETE FROM candles.chart_exact`)
			}
			f.reopen(t)
			revision, err := f.st.PromoteRebuild(context.Background(), "reconstruction")
			if err != nil || revision != 2 {
				t.Fatalf("promotion revision=%d: %v", revision, err)
			}
			f.reopen(t)
			sources := map[string]accounting.FillV1{}
			for _, fill := range f.fills {
				sources[fill.SourceFillID] = fill
			}
			for _, state := range states {
				history, err := engine.ReadSeries(context.Background(), f.st, state.PoolAddress, state.Interval, state.Start, state.IntervalEnd, false)
				if err != nil || len(history) != 1 {
					t.Fatalf("history: %v %v", history, err)
				}
				if history[0].Revision != 2 || history[0].DisplayRevision != 2 || history[0].Trades != 2 || history[0].CorrectionReason != "archive_reconstruction" {
					t.Fatal(history[0])
				}
				expected, err := engine.CanonicalCandle(history[0], sources)
				if err != nil {
					t.Fatal(err)
				}
				topic := "market.candles." + state.PoolAddress + "." + state.Interval
				var snapshot []byte
				var sequence int64
				if err = f.db.QueryRow(context.Background(), `SELECT sequence,frame FROM api_stream.stream_projection WHERE topic=$1`, topic).Scan(&sequence, &snapshot); err != nil {
					t.Fatal(err)
				}
				if sequence != 2 {
					t.Fatalf("stream sequence=%d", sequence)
				}
				frame, err := delivery.ParseCommittedStreamFrame(snapshot)
				if err != nil {
					t.Fatal(err)
				}
				if frame.Revision != "2" || frame.Sequence != "2" {
					t.Fatal(frame)
				}
				repair75EqualJSON(t, frame.Payload, repair75JSON(t, expected))
				var resumed []byte
				if err = f.db.QueryRow(context.Background(), `SELECT o.frame FROM api_stream.stream_delivery_outbox o JOIN api_stream.stream_projection p USING(partition_id) WHERE p.topic=$1 AND o.sequence>1 ORDER BY o.sequence`, topic).Scan(&resumed); err != nil {
					t.Fatal(err)
				}
				replay, err := delivery.ParseCommittedStreamFrame(resumed)
				if err != nil {
					t.Fatal(err)
				}
				if replay.EventID != frame.EventID || replay.Sequence != "2" {
					t.Fatal(replay)
				}
				repair75EqualJSON(t, replay.Payload, frame.Payload)
				var proof bool
				if err = f.db.QueryRow(context.Background(), `SELECT EXISTS(SELECT 1 FROM candles.chart_rebuild_stream_sources r JOIN candles.chart_stream_sources s USING(event_id) JOIN candles.chart_rebuild_runs b USING(run_id) WHERE s.event_id=$1 AND s.source_sequence=2 AND s.revision=2 AND r.manifest_digest=b.manifest_digest AND r.output_digest=b.expected_digest AND r.owner_epoch=9 AND b.status='promoted' AND b.promotion_epoch=2)`, frame.EventID).Scan(&proof); err != nil || !proof {
					t.Fatalf("durable reconstruction proof: %t %v", proof, err)
				}
			}
			if _, err = f.st.PromoteRebuild(context.Background(), "reconstruction"); err == nil {
				t.Fatal("re-promoted run")
			}
			if _, err = f.db.Exec(context.Background(), `UPDATE candles.chart_rebuild_stream_sources SET manifest_digest=$1`, strings.Repeat("f", 64)); err == nil {
				t.Fatal("immutable reconstruction proof changed")
			}
		})
	}
}

func TestRepair_75_PromotionRollbackPreservesPublicationAuthority(t *testing.T) {
	for _, failure := range []string{"missing_publication", "changed_authoritative_fill", "invalid_last_candle", "parity_mismatch", "invalid_manifest", "stale_revision", "frozen_owner"} {
		t.Run(failure, func(t *testing.T) {
			f := repair75New(t)
			var alter func([]store.StagedCandle)
			if failure == "invalid_last_candle" {
				alter = func(bars []store.StagedCandle) {
					var state engine.SeriesCandle
					if err := json.Unmarshal(bars[4].Snapshot, &state); err != nil {
						t.Fatal(err)
					}
					state.High.Numerator = "999999999"
					bars[4].Snapshot = repair75JSON(t, state)
				}
			}
			f.stage(t, "rejected", 1, alter)
			switch failure {
			case "missing_publication":
				repair75Exec(t, f.db, `DELETE FROM indexer.indexer_accounting_publications_v1 WHERE payload->>'sourceFillId'=$1`, f.fills[1].SourceFillID)
			case "changed_authoritative_fill":
				repair75Exec(t, f.db, `UPDATE indexer.indexer_accounting_fills_v1 SET source_fact_digest=$1 WHERE source_fill_id=$2`, strings.Repeat("f", 64), f.fills[0].SourceFillID)
			case "parity_mismatch":
				repair75Exec(t, f.db, `UPDATE candles.chart_rebuild_runs SET output_digest=$1 WHERE run_id='rejected'`, strings.Repeat("f", 64))
			case "invalid_manifest":
				repair75Exec(t, f.db, `UPDATE candles.chart_rebuild_runs SET manifest_digest='untrusted' WHERE run_id='rejected'`)
			case "stale_revision":
				repair75Exec(t, f.db, `UPDATE candles.chart_rebuild_runs SET base_revision=0 WHERE run_id='rejected'`)
			case "frozen_owner":
				repair75Exec(t, f.db, `UPDATE candles.chart_pool_fences SET frozen=true`)
			}
			if _, err := f.st.PromoteRebuild(context.Background(), "rejected"); err == nil {
				t.Fatal("unauthorized promotion succeeded")
			}
			f.reopen(t)
			var exact, outbox, sources, proofs, frames, clock int64
			var status string
			err := f.db.QueryRow(context.Background(), `SELECT (SELECT count(*) FROM candles.chart_exact WHERE snapshot->>'revision'='1'),(SELECT count(*) FROM candles.chart_outbox),(SELECT count(*) FROM candles.chart_stream_sources),(SELECT count(*) FROM candles.chart_rebuild_stream_sources),(SELECT count(*) FROM api_stream.stream_delivery_outbox),(SELECT revision FROM candles.chart_series_clock),(SELECT status FROM candles.chart_rebuild_runs WHERE run_id='rejected')`).Scan(&exact, &outbox, &sources, &proofs, &frames, &clock, &status)
			if err != nil {
				t.Fatal(err)
			}
			if exact != 7 || outbox != 7 || sources != 7 || frames != 7 || proofs != 0 || clock != 1 || status != "validated" {
				t.Fatalf("partial promotion leaked: %d %d %d %d %d %d %s", exact, outbox, sources, proofs, frames, clock, status)
			}
		})
	}
}
