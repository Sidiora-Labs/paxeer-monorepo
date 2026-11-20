package projectionstats

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type publicStatsSyncRaw struct {
	Pool, Emitter string
	Record        syncRecord
}
type publicStatsSyncCheckpoint struct {
	Key    string
	Events json.RawMessage
}
type publicStatsSyncState struct {
	Raw          []publicStatsSyncRaw
	Commits      []accounting.PublicationV1
	Pools        []string
	Frames       map[string]json.RawMessage
	Checkpoints  map[string]publicStatsSyncCheckpoint
	ResetQueries int
}
func publicStatsSyncInt64(t *testing.T, raw string) int64 {
	t.Helper()
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

type publicStatsSyncDB struct {
	t            *testing.T
	path         string
	pool         *pgxpool.Pool
	resetQueries atomic.Int64
}

func (d *publicStatsSyncDB) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.HasPrefix(data.SQL, "SELECT height::text,transaction_index::text,log_index::text,topics,data FROM ") {
		d.resetQueries.Add(1)
	}
	return ctx
}

func (d *publicStatsSyncDB) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

func publicStatsSyncOpen(t *testing.T, path string, initial *publicStatsSyncState) *publicStatsSyncDB {
	t.Helper()
	d := &publicStatsSyncDB{t: t, path: path}
	var cfg *pgxpool.Config
	if initial != nil {
		f := newPublicFixture(t)
		cfg = f.pg.Config()
		for _, raw := range initial.Raw {
			r := raw.Record
			f.exec(`INSERT INTO indexer.indexer_raw_logs(chain_id,transaction_hash,log_index,height,transaction_index,block_hash,emitter,topics,data) VALUES(125,$1,$2,$3,$4,$5,$6,$7,$8)`, "0x"+fmt.Sprintf("%064x", publicStatsSyncInt64(t, r.Height)), r.LogIndex, r.Height, r.TransactionIndex, f.real.Transactions[1].BlockHash, raw.Emitter, string(r.Topics), r.Data)
		}
		pub, _ := publicStatsSyncReceipt(t)
		fill := pub.Fill
		// The recorded Swap keeps its receipt identity.
		f.exec(`UPDATE indexer.indexer_raw_logs SET transaction_hash=$1,block_hash=$2 WHERE height=$3 AND log_index=$4`, fill.Position.TransactionHash, fill.Position.BlockHash, fill.Position.Height, fill.Position.LogIndex)
		payload, err := json.Marshal(fill)
		if err != nil {
			t.Fatal(err)
		}
		f.exec(`INSERT INTO indexer.indexer_accounting_fills(source_fill_id,source_fact_digest,chain_id,transaction_hash,log_index,pool_address,payload) VALUES($1,$2,125,$3,$4,$5,$6)`, fill.SourceFillID, string(pub.SourceFactDigest), fill.Position.TransactionHash, fill.Position.LogIndex, fill.PoolAddress, string(payload))
		for i, pool := range initial.Pools {
			f.real.Pool = pool
			creation := f.real.Transactions[0]
			if i > 0 {
				f.real.Token = fmt.Sprintf("0x%040x", i)
				creation.Hash = fmt.Sprintf("0x%064x", i)
			}
			f.creation(creation, 14)
		}
		f.advance(publicStatsSyncInt64(t, fill.Position.Height), fill.Position.BlockHash)
		wire, err := json.Marshal(struct {
			DSN string
		}{cfg.ConnString()})
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(path, wire, 0600); err != nil {
			t.Fatal(err)
		}
	} else {
		wire, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		var saved struct {
			DSN          string
			ResetQueries int64
		}
		if err = json.Unmarshal(wire, &saved); err != nil {
			t.Fatal(err)
		}
		cfg, err = pgxpool.ParseConfig(saved.DSN)
		if err != nil {
			t.Fatal(err)
		}
		d.resetQueries.Store(saved.ResetQueries)
	}
	cfg.ConnConfig.Tracer = d
	var err error
	d.pool, err = pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(d.close)
	return d
}

func (d *publicStatsSyncDB) close() {
	if d.pool == nil {
		return
	}
	wire, err := json.Marshal(struct {
		DSN          string
		ResetQueries int64
	}{d.pool.Config().ConnString(), d.resetQueries.Load()})
	if err != nil {
		d.t.Error(err)
	} else if err = os.WriteFile(d.path, wire, 0600); err != nil {
		d.t.Error(err)
	}
	d.pool.Close()
	d.pool = nil
}

func (d *publicStatsSyncDB) snapshot() publicStatsSyncState {
	d.t.Helper()
	out := publicStatsSyncState{Frames: map[string]json.RawMessage{}, Checkpoints: map[string]publicStatsSyncCheckpoint{}, ResetQueries: int(d.resetQueries.Load())}
	ctx := context.Background()
	rows, err := d.pool.Query(ctx, "SELECT topic,frame FROM api_stream.stream_projection WHERE frame IS NOT NULL")
	if err != nil {
		d.t.Fatal(err)
	}
	for rows.Next() {
		var topic string
		var frame []byte
		if err = rows.Scan(&topic, &frame); err != nil {
			d.t.Fatal(err)
		}
		out.Frames[topic] = frame
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		d.t.Fatal(err)
	}
	rows, err = d.pool.Query(ctx, "SELECT pool,checkpoint_key,events FROM stats.public_stats_sync_baselines")
	if err != nil {
		d.t.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var pool string
		var c publicStatsSyncCheckpoint
		if err = rows.Scan(&pool, &c.Key, &c.Events); err != nil {
			d.t.Fatal(err)
		}
		out.Checkpoints[pool] = c
	}
	if err = rows.Err(); err != nil {
		d.t.Fatal(err)
	}
	return out
}

func publicStatsSyncReceipt(t *testing.T) (accounting.PublicationV1, publicStatsSyncRaw) {
	t.Helper()
	wire, err := os.ReadFile("testdata/real_pool_125.json")
	if err != nil {
		t.Fatal(err)
	}
	var tape realPool125
	if err = json.Unmarshal(wire, &tape); err != nil {
		t.Fatal(err)
	}
	for _, tx := range tape.Transactions {
		for _, log := range tx.Logs {
			if len(log.Topics) != 3 || log.Topics[0] != swapEventTopic {
				continue
			}
			data, err := hex.DecodeString(strings.TrimPrefix(log.Data, "0x"))
			if err != nil {
				t.Fatal(err)
			}
			words, err := abiWords(data, 7)
			if err != nil {
				t.Fatal(err)
			}
			if words[0].String() != "1" {
				continue
			}
			timestamp := tape.Blocks[strconv.FormatInt(tx.BlockNumber, 10)].Timestamp
			f := accounting.FillV1{Version: accounting.FillVersion,
				Position:      accounting.SourcePosition{ChainID: "125", Height: strconv.FormatInt(tx.BlockNumber, 10), TransactionIndex: strconv.FormatInt(tx.TransactionIndex, 10), LogIndex: strconv.FormatInt(log.LogIndex, 10), TransactionHash: tx.Hash, BlockHash: tx.BlockHash},
				ActualEmitter: log.Address, PoolAddress: tape.Pool, Token: accounting.Asset{Address: tape.Token, Decimals: tape.TokenDecimals}, Settlement: accounting.Asset{Address: tape.USDL, Decimals: 6},
				Side: "buy", TokenAmountRaw: words[2].String(), USDLAmountRaw: words[1].String(), PaidInputRaw: words[1].String(), FeeRaw: words[3].String(), FeeDisposition: "distributed_usdl", InputDerivationQuality: "receipt_incomplete",
				Attribution: accounting.Attribution{TransactionSender: tx.From, Quality: "unavailable", Evidence: []accounting.EvidenceRef{}}, Execution: accounting.ExecutionPolicy{Quality: "unavailable", Evidence: []accounting.EvidenceRef{}},
				Provenance: accounting.Provenance{DeploymentID: publicTestDeployment, ABIRevision: "sidiora-v1", EmitterCodeEpoch: "event-emitter-1", PoolCodeEpoch: "pool-1", ReceiptSHA256: strings.Repeat("a", 64), HeaderSHA256: strings.Repeat("b", 64)}, BlockTimestampSeconds: strconv.FormatInt(timestamp, 10)}
			f.SourceFillID = accounting.SourceFillIdentity(f.Position, f.Provenance.DeploymentID)
			digest, err := accounting.FillDigest(f)
			if err != nil {
				t.Fatal(err)
			}
			p := accounting.PublicationV1{Version: accounting.PublicationVersion, Facet: "market", SourceFillID: f.SourceFillID, SourceFactDigest: digest, Fill: &f,
				Positions: []accounting.PartitionPosition{{ProjectionVersion: publicTestVersion, PartitionKind: "market", PartitionKey: "125:" + f.PoolAddress, Sequence: "1", SourceEventID: f.SourceFillID, PayloadDigest: digest, SourceOrder: f.Position}}}
			p.PublicationID = accounting.PublicationIdentity(f.SourceFillID, digest, "market", publicTestVersion, "initial", "")
			p.Positions[0].EffectIdentity = accounting.EffectIdentity(f.SourceFillID, "market", publicTestVersion, "", "market", p.Positions[0].PartitionKey)
			// Drive the real producer codec and consumer decoder before repository
			// persistence, preserving the source receipt's exact amounts/position.
			wire, err = accounting.CanonicalPublicationV1(p)
			if err != nil {
				t.Fatal(err)
			}
			p, err = accounting.DecodePublicationV1(wire)
			if err != nil {
				t.Fatal(err)
			}
			topics, err := json.Marshal(log.Topics)
			if err != nil {
				t.Fatal(err)
			}
			return p, publicStatsSyncRaw{Pool: f.PoolAddress, Emitter: f.ActualEmitter, Record: syncRecord{Height: f.Position.Height, TransactionIndex: f.Position.TransactionIndex, LogIndex: f.Position.LogIndex, Topics: topics, Data: data}}
		}
	}
	t.Fatal("recorded receipt contains no canonical buy")
	return accounting.PublicationV1{}, publicStatsSyncRaw{}
}

func publicStatsSyncStateFor(t *testing.T, resets int) (publicStatsSyncState, accounting.PublicationV1) {
	t.Helper()
	pub, swap := publicStatsSyncReceipt(t)
	f := pub.Fill
	s := publicStatsSyncState{Raw: []publicStatsSyncRaw{swap}, Pools: []string{f.PoolAddress}, Frames: map[string]json.RawMessage{}, Checkpoints: map[string]publicStatsSyncCheckpoint{}}
	height, _ := strconv.ParseInt(f.Position.Height, 10, 64)
	timestamp, _ := strconv.ParseInt(f.BlockTimestampSeconds, 10, 64)
	topics, err := json.Marshal([]string{poolStateUpdatedTopic, poolTopic(f.PoolAddress)})
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < resets; i++ {
		h := height - int64(resets-i)
		// 400 resets span 33 hours and leave 288 within the last 24 hours.
		ts := timestamp - int64(resets-i)*300
		data := make([]byte, 192)
		for j, n := range []int64{100000, 1000000 + int64(i)*100, 2000000, 100 + int64(i), ts, h} {
			big.NewInt(n).FillBytes(data[j*32 : (j+1)*32])
		}
		s.Raw = append(s.Raw, publicStatsSyncRaw{Pool: f.PoolAddress, Emitter: f.ActualEmitter, Record: syncRecord{Height: strconv.FormatInt(h, 10), TransactionIndex: "0", LogIndex: "0", Topics: topics, Data: data}})
	}
	return s, pub
}
func publicStatsSyncProjector(t *testing.T, d *publicStatsSyncDB, batch int) (*Repository, *PublicStatsProjector) {
	t.Helper()
	r, err := NewRepository(d.pool, "stats")
	if err != nil {
		t.Fatal(err)
	}
	reads, err := NewReadService(r, ReadConfig{SourceSchema: "indexer", SourceStream: publicTestStream, ProjectionVersion: publicTestVersion, CursorKey: bytes.Repeat([]byte{7}, 32), CursorTTL: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	p, err := NewPublicStatsProjector(PublicStatsConfig{Reads: reads, HolderSchema: publicTestHolders, StreamSchema: "api_stream", Batch: batch})
	if err != nil {
		t.Fatal(err)
	}
	return r, p
}
func publicStatsSyncCommit(t *testing.T, r *Repository, p accounting.PublicationV1) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	in, err := qualifiedInput(*p.Fill, "checkpoint-consumer", publicTestVersion)
	if err != nil {
		t.Fatal(err)
	}
	result, err := r.ApplyQualifiedTx(ctx, tx, in)
	if err != nil || !result.Accepted {
		t.Fatal(result, err)
	}
	if err = r.RecordMarketPublicationTx(ctx, tx, p, result); err != nil {
		t.Fatal(err)
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}
func publicStatsSyncStats(t *testing.T, s publicStatsSyncState, pool string) publicStatsFrame {
	t.Helper()
	raw := s.Frames["market.stats."+pool]
	if err := ValidatePublicStatsFrame(raw); err != nil {
		t.Fatalf("stats frame: %v %s", err, raw)
	}
	consumed, err := delivery.ParseCommittedStreamFrame(raw)
	if err != nil {
		t.Fatal(err)
	}
	if consumed.Topic != "market.stats."+pool || consumed.Sequence != "1" {
		t.Fatal(consumed)
	}
	var frame publicStatsFrame
	if err = json.Unmarshal(raw, &frame); err != nil {
		t.Fatal(err)
	}
	return frame
}

func TestPublicStatsCheckpointStreamSchemaSplit(t *testing.T) {
	state, pub := publicStatsSyncStateFor(t, 0)
	d := publicStatsSyncOpen(t, filepath.Join(t.TempDir(), "committed.json"), &state)
	repo, p := publicStatsSyncProjector(t, d, 2)
	for _, schema := range []string{"other_stream", "API_STREAM", "api_stream;drop schema stats", ""} {
		if _, err := NewPublicStatsProjector(PublicStatsConfig{Reads: p.reads, HolderSchema: publicTestHolders, StreamSchema: schema, Batch: 2}); !errors.Is(err, ErrInvalid) {
			t.Fatalf("override %q accepted: %v", schema, err)
		}
	}
	publicStatsSyncCommit(t, repo, pub)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	result, err := p.RefreshOnce(ctx, time.Unix(1800000000, 0))
	if err != nil || result.Appended != 1 || result.Prices != 1 || result.Pools != 1 {
		t.Fatal(result, err)
	}
	stored := d.snapshot()
	trade, err := delivery.ParseCommittedStreamFrame(stored.Frames["market.trades."+pub.Fill.PoolAddress])
	if err != nil {
		t.Fatal(err)
	}
	stats := publicStatsSyncStats(t, stored, pub.Fill.PoolAddress)
	var payload struct {
		USDL  string `json:"usdlAmountRaw"`
		Token string `json:"tokenAmountRaw"`
	}
	if err = json.Unmarshal(trade.Payload, &payload); err != nil {
		t.Fatal(err)
	}
	if len(stored.Frames) != 3 || payload.USDL != pub.Fill.USDLAmountRaw || payload.Token != pub.Fill.TokenAmountRaw || stats.Payload.Volume24hRaw != pub.Fill.USDLAmountRaw || stats.Payload.SourceUpperPosition != Position(pub.Fill.Position) {
		t.Fatal(trade, stats)
	}
	if err = ValidatePublicPriceFrame(stored.Frames["market.price."+pub.Fill.PoolAddress]); err != nil {
		t.Fatal(err)
	}
}

func TestPublicStatsCheckpointSyncLifetimeCapRestart(t *testing.T) {
	state, pub := publicStatsSyncStateFor(t, 400)
	path := filepath.Join(t.TempDir(), "committed.json")
	d := publicStatsSyncOpen(t, path, &state)
	repo, p := publicStatsSyncProjector(t, d, 2)
	publicStatsSyncCommit(t, repo, pub)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := p.RefreshOnce(ctx, time.Unix(1800000000, 0))
	if err != nil || result.Appended != 1 || result.Pools != 1 {
		t.Fatal(result, err)
	}
	stored := d.snapshot()
	frame := publicStatsSyncStats(t, stored, pub.Fill.PoolAddress)
	reserve := big.NewInt(1000000 + 399*100)
	paid, _ := new(big.Int).SetString(pub.Fill.PaidInputRaw, 10)
	fee, _ := new(big.Int).SetString(pub.Fill.FeeRaw, 10)
	reserve.Add(reserve, new(big.Int).Sub(paid, fee))
	if frame.Payload.RealUsdlBalanceRaw != reserve.String() {
		t.Fatal(frame.Payload.RealUsdlBalanceRaw, reserve)
	}
	swapWords, err := abiWords(state.Raw[0].Record.Data, 7)
	if err != nil {
		t.Fatal(err)
	}
	timestamp, _ := strconv.ParseInt(pub.Fill.BlockTimestampSeconds, 10, 64)
	changes := []*string{frame.Payload.PriceChangeBps24h, frame.Payload.PriceChangeBps5m, frame.Payload.PriceChangeBps1h, frame.Payload.PriceChangeBps6h}
	qualities := []string{frame.Payload.ChangeBaselineQuality, frame.Payload.ChangeBaselineQuality5m, frame.Payload.ChangeBaselineQuality1h, frame.Payload.ChangeBaselineQuality6h}
	for i, seconds := range []int64{86400, 300, 3600, 21600} {
		var baseline *big.Int
		for _, r := range state.Raw[1:] {
			words, err := abiWords(r.Record.Data, 6)
			if err != nil {
				t.Fatal(err)
			}
			if words[4].Int64() < timestamp+1-seconds {
				baseline = words[3]
			}
		}
		if baseline == nil {
			t.Fatal("missing fixture baseline")
		}
		expected := new(big.Int).Sub(swapWords[4], baseline)
		expected.Mul(expected, big.NewInt(10000))
		expected.Quo(expected, baseline)
		if changes[i] == nil || *changes[i] != expected.String() || qualities[i] != "complete" {
			t.Fatalf("window %d: %v %s expected %s", seconds, changes[i], qualities[i], expected)
		}
	}
	checkpoint := stored.Checkpoints[pub.Fill.PoolAddress]
	var records []syncRecord
	if err = json.Unmarshal(checkpoint.Events, &records); err != nil {
		t.Fatal(err)
	}
	if len(state.Raw) != 401 || len(records) != 5 || stored.ResetQueries != 5 || publicStatsMaxSyncEvents != 256 {
		t.Fatal(len(state.Raw), len(records), stored.ResetQueries, publicStatsMaxSyncEvents)
	}
	first := append([]byte(nil), stored.Frames["market.stats."+pub.Fill.PoolAddress]...)
	d.close()
	reopened := publicStatsSyncOpen(t, path, nil)
	_, p = publicStatsSyncProjector(t, reopened, 2)
	result, err = p.RefreshOnce(ctx, time.Unix(1800000060, 0))
	if err != nil || result.Pools != 1 || result.Appended != 0 || result.Prices != 0 {
		t.Fatal(result, err)
	}
	recovered := reopened.snapshot()
	if recovered.ResetQueries != 5 || !bytes.Equal(first, recovered.Frames["market.stats."+pub.Fill.PoolAddress]) || len(recovered.Checkpoints) != 1 {
		t.Fatal("restart lost verified checkpoint or duplicated frame", recovered.ResetQueries)
	}
}

func TestPublicStatsCheckpointFailedPoolReadinessAndLaterPage(t *testing.T) {
	for _, batch := range []int{1, 2} {
		t.Run(fmt.Sprintf("batch_%d", batch), func(t *testing.T) {
			state, pub := publicStatsSyncStateFor(t, 400)
			bad := "0x" + strings.Repeat("1", 40)
			state.Pools = append(state.Pools, bad)
			topics, err := json.Marshal([]string{poolStateUpdatedTopic, poolTopic(bad)})
			if err != nil {
				t.Fatal(err)
			}
			// An invalid authorized raw reset must remain an integrity failure, while
			// the healthy pool in the next page still commits its frame.
			state.Raw = append(state.Raw, publicStatsSyncRaw{Pool: bad, Emitter: pub.Fill.ActualEmitter, Record: syncRecord{Height: pub.Fill.Position.Height, TransactionIndex: "0", LogIndex: "0", Topics: topics, Data: []byte{1}}})
			d := publicStatsSyncOpen(t, filepath.Join(t.TempDir(), "committed.json"), &state)
			repo, p := publicStatsSyncProjector(t, d, batch)
			publicStatsSyncCommit(t, repo, pub)
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			result, err := p.RefreshOnce(ctx, time.Unix(1800000000, 0))
			if !errors.Is(err, ErrIntegrity) || len(result.FailedPools) != 1 || result.FailedPools[0] != bad || result.Pools != 1 || result.Appended != 1 {
				t.Fatal(result, err)
			}
			stored := d.snapshot()
			publicStatsSyncStats(t, stored, pub.Fill.PoolAddress)
			if len(stored.Frames["market.stats."+bad]) != 0 || len(stored.Checkpoints[bad].Events) != 0 {
				t.Fatal("failed pool published or checkpointed invalid evidence")
			}
		})
	}
}

func TestPublicStatsCheckpointSyncSafetyBounds(t *testing.T) {
	state, pub := publicStatsSyncStateFor(t, 1)
	ts, _ := strconv.ParseInt(pub.Fill.BlockTimestampSeconds, 10, 64)
	anchor := publicAnchor{Position: Position(pub.Fill.Position), timestamp: ts}
	good := state.Raw[1].Record
	tooMany := make([]syncRecord, publicStatsMaxSyncEvents+1)
	for i := range tooMany {
		tooMany[i] = good
	}
	if _, err := decodeSyncRecords(tooMany, anchor, pub.Fill.PoolAddress); !errors.Is(err, ErrUnavailable) {
		t.Fatal("safety bound changed", err)
	}
	for _, bad := range []syncRecord{
		{Height: good.Height, TransactionIndex: "0", LogIndex: "0", Topics: good.Topics, Data: good.Data[:191]},
		{Height: pub.Fill.Position.Height, TransactionIndex: pub.Fill.Position.TransactionIndex, LogIndex: "4294967295", Topics: good.Topics, Data: good.Data},
		{Height: good.Height, TransactionIndex: "0", LogIndex: "0", Topics: []byte(`[]`), Data: good.Data},
	} {
		if _, err := decodeSyncRecords([]syncRecord{bad}, anchor, pub.Fill.PoolAddress); !errors.Is(err, ErrIntegrity) {
			t.Fatal("invalid reset admitted", err)
		}
	}
}
