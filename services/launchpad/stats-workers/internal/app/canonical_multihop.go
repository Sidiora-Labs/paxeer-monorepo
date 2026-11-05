package app

import (
	"context"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/config"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/consumer"
	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/config/deployment"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/queue"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Installed by the migration coordinator, never by the production runtime.
//
//go:embed canonical_multihop.sql
var canonicalMultihopSchemaSQL string

var (
	multihopTopic       = crypto.Keccak256Hash([]byte("MultihopSwap(address,address,address,uint256,uint256,uint256)")).Hex()
	multihopMirrorTopic = crypto.Keccak256Hash([]byte("RouterTrade(uint8,address,address,address,address,uint256,uint256,uint256,uint256,uint256)")).Hex()
	multihopSwapTopic   = crypto.Keccak256Hash([]byte("Swap(bytes32,address,bool,uint256,uint256,uint256,uint256,uint256,uint256)")).Hex()
)

type canonicalMultihopLog struct {
	TxHash, BlockHash, Emitter         string
	Height, TransactionIndex, LogIndex int64
	Topics                             []string
	Data                               []byte
}

func (l canonicalMultihopLog) position() accounting.SourcePosition {
	return accounting.SourcePosition{ChainID: "125", Height: strconv.FormatInt(l.Height, 10), TransactionIndex: strconv.FormatInt(l.TransactionIndex, 10), LogIndex: strconv.FormatInt(l.LogIndex, 10), TransactionHash: l.TxHash, BlockHash: l.BlockHash}
}

func scanMultihopLog(row interface{ Scan(...any) error }) (canonicalMultihopLog, error) {
	var l canonicalMultihopLog
	var topics []byte
	err := row.Scan(&l.TxHash, &l.BlockHash, &l.Emitter, &l.Height, &l.TransactionIndex, &l.LogIndex, &topics, &l.Data)
	if err != nil {
		return l, err
	}
	if json.Unmarshal(topics, &l.Topics) != nil || accounting.ValidateSourcePosition(l.position()) != nil || !common.IsHexAddress(l.Emitter) || l.Emitter != strings.ToLower(l.Emitter) {
		return l, consumer.ErrLegacyBridgeIntegrity
	}
	return l, nil
}

const multihopLogColumns = "transaction_hash,block_hash,emitter,height,transaction_index,log_index,topics,data"

type canonicalMultihopProjection struct {
	pool                    *pgxpool.Pool
	stream, router, emitter string
	batch                   int
	guard                   *queue.EpochGuard
	logger                  *slog.Logger
}

func canonicalMultihopJob(pool *pgxpool.Pool, cfg config.Config, d statsRuntimeDocument, guard *queue.EpochGuard, logger *slog.Logger, health *statsRuntimeHealth) (statsRuntimeJob, error) {
	if pool == nil || guard == nil || logger == nil || health == nil || d.SourceStream == "" || len(d.SourceStream) > 128 || !common.IsHexAddress(cfg.RouterAddress) || !common.IsHexAddress(cfg.EventEmitterAddress) || common.HexToAddress(cfg.RouterAddress) == (common.Address{}) || common.HexToAddress(cfg.EventEmitterAddress) == (common.Address{}) || cfg.LegacyBridgeBatch < 1 || cfg.LegacyBridgeBatch > 1000 || cfg.LegacyBridgeIntervalMS < 100 || cfg.LegacyBridgeIntervalMS > 60000 {
		return statsRuntimeJob{}, errors.New("stats: canonical multihop dependencies required")
	}
	p := &canonicalMultihopProjection{pool: pool, stream: d.SourceStream, router: strings.ToLower(cfg.RouterAddress), emitter: strings.ToLower(cfg.EventEmitterAddress), batch: cfg.LegacyBridgeBatch, guard: guard, logger: logger}
	health.set("canonical multihop", errors.New("canonical multihop awaiting source replay"))
	return statsRuntimeJob{name: "canonical multihop", run: func(ctx context.Context) error {
		return retryStatsJob(ctx, time.Duration(cfg.LegacyBridgeIntervalMS)*time.Millisecond, health, "canonical multihop", func(ctx context.Context) error { _, err := p.RunOnce(ctx); return err })
	}}, nil
}

func (p *canonicalMultihopProjection) sourceState(ctx context.Context, tx pgx.Tx) (int64, error) {
	var raw, typed, fence int64
	var owner string
	var alive bool
	err := tx.QueryRow(ctx, `SELECT w.raw_complete_height,w.typed_complete_height,l.fence_token,l.owner_instance,l.expires_at>clock_timestamp() FROM indexer.indexer_ingest_watermarks_v1 w JOIN indexer.indexer_ingest_leases_v1 l USING(chain_id,stream_id) WHERE w.chain_id=125 AND w.stream_id=$1`, p.stream).Scan(&raw, &typed, &fence, &owner, &alive)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, consumer.ErrAccountingSourceUnavailable
	}
	if err != nil {
		return 0, err
	}
	if raw < 0 || raw != typed || fence <= 0 || owner == "" || !alive {
		return 0, consumer.ErrAccountingSourceUnavailable
	}
	return typed, nil
}

// Every route has its own source checkpoint; a failed route cannot advance past
// its missing wallet row. PostgreSQL is the canonical catchup transport.
func (p *canonicalMultihopProjection) RunOnce(ctx context.Context) (int, error) {
	if p == nil || p.pool == nil || p.guard == nil || p.batch < 1 || p.batch > 1000 {
		return 0, errors.New("stats: canonical multihop dependencies required")
	}
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer func() {
		cleanup, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
		defer cancel()
		_ = tx.Rollback(cleanup)
	}()
	if err = p.guard.Assert(ctx, tx); err != nil {
		return 0, err
	}
	var locked bool
	if err = tx.QueryRow(ctx, `SELECT pg_try_advisory_xact_lock(hashtextextended('stats:canonical-multihop:v1',0))`).Scan(&locked); err != nil || !locked {
		return 0, err
	}
	typed, err := p.sourceState(ctx, tx)
	if err != nil {
		return 0, err
	}
	rows, err := tx.Query(ctx, `SELECT `+multihopLogColumns+` FROM indexer.indexer_raw_logs_v1 r
 WHERE r.chain_id=125 AND r.height<=$1 AND r.topics->>0=$2 AND r.emitter IN ($3,$4)
 AND NOT EXISTS(SELECT 1 FROM stats.canonical_multihop_checkpoints c WHERE c.source_stream=$5 AND c.transaction_hash=r.transaction_hash AND c.log_index=r.log_index)
 ORDER BY r.height,r.transaction_index,r.log_index LIMIT $6`, typed, multihopTopic, p.router, deployment.LaunchpadPrecompileAddress, p.stream, p.batch)
	if err != nil {
		return 0, err
	}
	pending := []canonicalMultihopLog{}
	for rows.Next() {
		l, e := scanMultihopLog(rows)
		if e != nil {
			rows.Close()
			return 0, e
		}
		pending = append(pending, l)
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return 0, err
	}
	for _, l := range pending {
		if err = p.projectRoute(ctx, tx, l); err != nil {
			return 0, err
		}
	}
	if _, err = p.sourceState(ctx, tx); err != nil {
		return 0, err
	}
	if err = p.guard.Assert(ctx, tx); err != nil {
		return 0, err
	}
	if err = tx.Commit(ctx); err != nil {
		return 0, err
	}
	return len(pending), nil
}

func multihopAddress(topic string) (string, error) {
	if len(topic) != 66 || !strings.HasPrefix(topic, "0x000000000000000000000000") || topic != strings.ToLower(topic) || !common.IsHexAddress("0x"+topic[26:]) || common.HexToAddress("0x"+topic[26:]) == (common.Address{}) {
		return "", consumer.ErrLegacyBridgeIntegrity
	}
	return "0x" + topic[26:], nil
}
func multihopWord(data []byte, i int) string {
	return new(big.Int).SetBytes(data[i*32 : (i+1)*32]).String()
}

func (p *canonicalMultihopProjection) projectRoute(ctx context.Context, tx pgx.Tx, route canonicalMultihopLog) error {
	integrity := consumer.ErrLegacyBridgeIntegrity
	if accounting.ValidateSourcePosition(route.position()) != nil || (route.Emitter != p.router && route.Emitter != deployment.LaunchpadPrecompileAddress) || len(route.Topics) != 4 || route.Topics[0] != multihopTopic || len(route.Data) != 96 || (route.Emitter == deployment.LaunchpadPrecompileAddress && route.Height < int64(deployment.Current().ActivationHeight)) {
		return integrity
	}
	var complete bool
	err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM indexer.indexer_block_completeness_v1 b WHERE b.chain_id=125 AND b.stream_id=$1 AND b.height=$2 AND b.block_hash=$3 AND b.fetch_complete AND b.required_decode_complete AND b.raw_count=(SELECT count(*) FROM indexer.indexer_raw_logs_v1 r WHERE r.chain_id=b.chain_id AND r.height=b.height AND r.block_hash=b.block_hash) AND EXISTS(SELECT 1 FROM indexer.indexer_ingest_commits_v1 c WHERE c.chain_id=b.chain_id AND c.stream_id=b.stream_id AND c.previous_height<b.height AND c.last_complete_height>=b.height))`, p.stream, route.Height, route.BlockHash).Scan(&complete)
	if err != nil {
		return err
	}
	if !complete {
		return integrity
	}
	sender, err := multihopAddress(route.Topics[1])
	if err != nil {
		return err
	}
	tokenIn, err := multihopAddress(route.Topics[2])
	if err != nil {
		return err
	}
	tokenOut, err := multihopAddress(route.Topics[3])
	if err != nil {
		return err
	}
	if tokenIn == tokenOut {
		return integrity
	}
	var poolIn, poolOut string
	err = tx.QueryRow(ctx, `SELECT a.pool,b.pool FROM indexer.indexer_canonical_creations_v1 a CROSS JOIN indexer.indexer_canonical_creations_v1 b WHERE a.token=$1 AND b.token=$2 AND (a.height,a.transaction_index,a.log_index)<=($3,$4,$5) AND (b.height,b.transaction_index,b.log_index)<=($3,$4,$5)`, tokenIn, tokenOut, route.Height, route.TransactionIndex, route.LogIndex).Scan(&poolIn, &poolOut)
	if errors.Is(err, pgx.ErrNoRows) {
		return integrity
	}
	if err != nil {
		return err
	}
	if !common.IsHexAddress(poolIn) || !common.IsHexAddress(poolOut) || poolIn == poolOut || poolIn != strings.ToLower(poolIn) || poolOut != strings.ToLower(poolOut) {
		return integrity
	}
	rows, err := tx.Query(ctx, `SELECT `+multihopLogColumns+` FROM indexer.indexer_raw_logs_v1 WHERE chain_id=125 AND transaction_hash=$1 ORDER BY log_index LIMIT 10001`, route.TxHash)
	if err != nil {
		return err
	}
	var mirror *canonicalMultihopLog
	legs := []canonicalMultihopLog{}
	var persisted bool
	count := 0
	for rows.Next() {
		l, e := scanMultihopLog(rows)
		if e != nil {
			rows.Close()
			return e
		}
		count++
		if l.Height != route.Height || l.TransactionIndex != route.TransactionIndex || l.BlockHash != route.BlockHash {
			rows.Close()
			return integrity
		}
		if l.LogIndex == route.LogIndex {
			a, _ := json.Marshal(l)
			b, _ := json.Marshal(route)
			persisted = string(a) == string(b)
		}
		if len(l.Topics) == 0 {
			continue
		}
		if l.LogIndex < route.LogIndex && ((l.Emitter == p.emitter && l.Topics[0] == multihopMirrorTopic) || ((l.Emitter == p.router || l.Emitter == deployment.LaunchpadPrecompileAddress) && l.Topics[0] == multihopTopic)) {
			legs = nil
		}
		if l.Emitter != p.emitter {
			continue
		}
		if l.LogIndex < route.LogIndex && l.Topics[0] == multihopSwapTopic {
			legs = append(legs, l)
			if len(legs) > 2 {
				legs = legs[1:]
			}
		}
		if l.LogIndex == route.LogIndex+1 && l.Topics[0] == multihopMirrorTopic {
			v := l
			mirror = &v
		}
	}
	rows.Close()
	if err = rows.Err(); err != nil {
		return err
	}
	if count > 10000 || !persisted || mirror == nil || len(legs) != 2 {
		return integrity
	}
	m := *mirror
	if len(m.Topics) != 4 || len(m.Data) != 224 || m.Topics[1] != common.BigToHash(big.NewInt(2)).Hex() {
		return integrity
	}
	mirrorPool, e := multihopAddress(m.Topics[2])
	if e != nil {
		return e
	}
	mirrorSender, e := multihopAddress(m.Topics[3])
	if e != nil {
		return e
	}
	mirrorIn, e := multihopAddress("0x" + common.Bytes2Hex(m.Data[:32]))
	if e != nil {
		return e
	}
	mirrorOut, e := multihopAddress("0x" + common.Bytes2Hex(m.Data[32:64]))
	if e != nil {
		return e
	}
	amountIn, intermediate, amountOut := multihopWord(route.Data, 0), multihopWord(route.Data, 1), multihopWord(route.Data, 2)
	if amountIn == "0" || intermediate == "0" || amountOut == "0" || mirrorPool != poolIn || mirrorSender != sender || mirrorIn != tokenIn || mirrorOut != tokenOut || multihopWord(m.Data, 2) != amountIn || multihopWord(m.Data, 3) != amountOut || multihopWord(m.Data, 4) != intermediate || multihopWord(m.Data, 6) != strconv.FormatInt(route.Height, 10) {
		return integrity
	}
	timestamp, e := strconv.ParseInt(multihopWord(m.Data, 5), 10, 64)
	if e != nil || timestamp <= 0 {
		return integrity
	}
	for i, l := range legs {
		if len(l.Topics) != 3 || len(l.Data) != 224 {
			return integrity
		}
		pool, e := multihopAddress(l.Topics[1])
		if e != nil {
			return e
		}
		wallet, e := multihopAddress(l.Topics[2])
		if e != nil {
			return e
		}
		wantPool, wantSide, wantIn, wantOut := poolIn, "0", amountIn, intermediate
		if i == 1 {
			wantPool, wantSide, wantIn, wantOut = poolOut, "1", intermediate, amountOut
		}
		if pool != wantPool || wallet != sender || multihopWord(l.Data, 0) != wantSide || multihopWord(l.Data, 1) != wantIn || multihopWord(l.Data, 2) != wantOut || multihopWord(l.Data, 5) != strconv.FormatInt(timestamp, 10) || multihopWord(l.Data, 6) != strconv.FormatInt(route.Height, 10) {
			return integrity
		}
		fee, _ := new(big.Int).SetString(multihopWord(l.Data, 3), 10)
		input, _ := new(big.Int).SetString(wantIn, 10)
		if fee.Cmp(input) > 0 {
			return integrity
		}
	}
	row := store.CrossTokenSwapRow{ID: fmt.Sprintf("%s-%d", route.TxHash, route.LogIndex), Sender: sender, TokenIn: tokenIn, TokenOut: tokenOut, PoolIn: poolIn, PoolOut: poolOut, AmountIn: amountIn, IntermediateUsdl: intermediate, AmountOut: amountOut, FeeIn: multihopWord(legs[0].Data, 3), FeeOut: multihopWord(legs[1].Data, 3), BlockTimestamp: timestamp, TxHash: route.TxHash}
	evidence, _ := json.Marshal(struct {
		Stream string
		Logs   []canonicalMultihopLog
		Row    store.CrossTokenSwapRow
	}{p.stream, []canonicalMultihopLog{legs[0], legs[1], route, m}, row})
	digest := delivery.Hash(evidence)
	bound := store.New(p.pool).WithTx(tx)
	writer := consumer.NewMultihopConsumer(bound, p.logger)
	if err = writer.ProcessEvent(ctx, consumer.MultihopEvent{Sender: row.Sender, TokenIn: row.TokenIn, TokenOut: row.TokenOut, PoolIn: row.PoolIn, PoolOut: row.PoolOut, AmountIn: row.AmountIn, IntermediateUsdl: row.IntermediateUsdl, AmountOut: row.AmountOut, FeeIn: row.FeeIn, FeeOut: row.FeeOut, BlockTimestamp: row.BlockTimestamp, TxHash: row.TxHash, LogIndex: int(route.LogIndex)}); err != nil {
		return err
	}
	var same bool
	err = tx.QueryRow(ctx, `SELECT sender=$2 AND token_in=$3 AND token_out=$4 AND pool_in=$5 AND pool_out=$6 AND amount_in=$7 AND intermediate_usdl=$8 AND amount_out=$9 AND fee_in=$10 AND fee_out=$11 AND block_timestamp=$12 AND tx_hash=$13 FROM stats.cross_token_swaps WHERE id=$1`, row.ID, row.Sender, row.TokenIn, row.TokenOut, row.PoolIn, row.PoolOut, row.AmountIn, row.IntermediateUsdl, row.AmountOut, row.FeeIn, row.FeeOut, row.BlockTimestamp, row.TxHash).Scan(&same)
	if err != nil {
		return err
	}
	if !same {
		return integrity
	}
	_, err = tx.Exec(ctx, `INSERT INTO stats.canonical_multihop_checkpoints(source_stream,transaction_hash,log_index,height,transaction_index,block_hash,source_sha256,swap_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT DO NOTHING`, p.stream, route.TxHash, route.LogIndex, route.Height, route.TransactionIndex, route.BlockHash, digest, row.ID)
	if err != nil {
		return err
	}
	err = tx.QueryRow(ctx, `SELECT height=$4 AND transaction_index=$5 AND block_hash=$6 AND source_sha256=$7 AND swap_id=$8 FROM stats.canonical_multihop_checkpoints WHERE source_stream=$1 AND transaction_hash=$2 AND log_index=$3`, p.stream, route.TxHash, route.LogIndex, route.Height, route.TransactionIndex, route.BlockHash, digest, row.ID).Scan(&same)
	if err != nil {
		return err
	}
	if !same {
		return integrity
	}
	return nil
}
