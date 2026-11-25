package consumer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"reflect"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/stats-workers/internal/projectionstats"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting/sourcebootstrap"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
)

const maxStatsBootstrapProgress = 1024

var statsSwapTopic = crypto.Keccak256Hash([]byte("Swap(bytes32,address,bool,uint256,uint256,uint256,uint256,uint256,uint256)"))

type statsSourceBootstrap struct {
	Fill     accounting.FillV1
	Position accounting.PartitionPosition
	Evidence sourcebootstrap.StatsEvidence
	Creation sourcebootstrap.Creation
}

type statsBootstrapProgress struct {
	gate                                        chan struct{}
	users                                       int
	initialized                                 bool
	bootstrap                                   statsSourceBootstrap
	last, commits, swaps, raw, records, openRaw int64
	rpcStep                                     int
	chains                                      [2]bool
}

type statsBootstrapper struct {
	config    SourceVerifiedAccountingConfig
	authorize func(context.Context, sourcebootstrap.Querier, accounting.FillV1) error
	mu        sync.Mutex
	pools     map[string]*statsBootstrapProgress
}

type statsBoundedQuery struct {
	sourcebootstrap.Querier
	timeout time.Duration
}

type statsBoundedRow struct {
	pgx.Row
	cancel context.CancelFunc
}

func (r statsBoundedRow) Scan(dest ...any) error {
	defer r.cancel()
	return r.Row.Scan(dest...)
}

func (q statsBoundedQuery) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	bounded, cancel := context.WithTimeout(ctx, q.timeout)
	return statsBoundedRow{Row: q.Querier.QueryRow(bounded, sql, args...), cancel: cancel}
}

func statsBootstrapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) || errors.Is(err, sourcebootstrap.ErrUnavailable) {
		return ErrAccountingSourceUnavailable
	}
	if errors.Is(err, sourcebootstrap.ErrIntegrity) {
		return projectionstats.ErrIntegrity
	}
	return err
}

func (s *statsBootstrapper) load(ctx context.Context, q sourcebootstrap.Querier, pool string) (statsSourceBootstrap, error) {
	var b statsSourceBootstrap
	q = statsBoundedQuery{q, s.config.StatementTimeout}
	wire, err := sourcebootstrap.FirstMarketPosition(ctx, q, s.config.SourcePartitionSchema, s.config.ProjectionVersion, pool)
	if err != nil {
		return b, statsBootstrapError(err)
	}
	if err = json.Unmarshal(wire, &b.Position); err != nil {
		return b, err
	}
	if accounting.ValidatePartitionPosition(b.Position) != nil || b.Position.Sequence != "1" || b.Position.PreviousEffectIdentity != "" || b.Position.PartitionKind != valueMarket || b.Position.PartitionKey != "125:"+pool || b.Position.ProjectionVersion != s.config.ProjectionVersion {
		return b, projectionstats.ErrIntegrity
	}
	wire, err = sourcebootstrap.MarketPublication(ctx, q, b.Position.SourceEventID, s.config.ProjectionVersion)
	if err != nil {
		return b, err
	}
	publication, err := accounting.DecodePublicationV1(wire)
	if err != nil {
		return b, err
	}
	if publication.Fill == nil || len(publication.Positions) != 1 || publication.Positions[0] != b.Position {
		return b, projectionstats.ErrIntegrity
	}
	b.Fill = *publication.Fill
	if err = s.authorize(ctx, q, b.Fill); err != nil {
		return b, err
	}
	b.Creation, err = sourcebootstrap.CanonicalCreation(ctx, q, s.config.SourceStream, pool)
	if err != nil {
		return b, statsBootstrapError(err)
	}
	var topics []string
	if json.Unmarshal(b.Creation.TopicsWire, &topics) != nil || !statsLogNamesPool(topics, b.Creation.Data, pool) {
		return b, projectionstats.ErrIntegrity
	}
	creationPosition := accounting.SourcePosition{ChainID: "125", Height: strconv.FormatInt(b.Creation.Height, 10), TransactionIndex: strconv.FormatInt(b.Creation.TransactionIndex, 10), LogIndex: strconv.FormatInt(b.Creation.LogIndex, 10), TransactionHash: b.Creation.TransactionHash, BlockHash: b.Creation.BlockHash}
	order, err := accounting.CompareSourcePosition(creationPosition, b.Fill.Position)
	if err != nil || order >= 0 {
		return b, projectionstats.ErrIntegrity
	}
	b.Evidence, err = sourcebootstrap.LoadStats(ctx, q, s.config.SourcePartitionSchema, s.config.SourceStream, s.config.ProjectionVersion, pool)
	if err != nil {
		return b, statsBootstrapError(err)
	}
	e := b.Evidence
	height, err := strconv.ParseInt(b.Fill.Position.Height, 10, 64)
	if err != nil || e.ReplayFromHeight < 0 || e.ReplayFromHeight >= b.Creation.Height || e.FirstCommitLastHeight < height {
		return b, ErrAccountingSourceUnavailable
	}
	if e.PublicationID != publication.PublicationID || e.PayloadDigest != b.Position.PayloadDigest || e.CreationTransactionHash != b.Creation.TransactionHash || e.CreationLogIndex != b.Creation.LogIndex {
		return b, ErrAccountingSourceUnavailable
	}
	return b, nil
}

func statsLogNamesPool(topics []string, data []byte, pool string) bool {
	word := common.BytesToHash(common.HexToAddress(pool).Bytes())
	for _, topic := range topics {
		if strings.EqualFold(topic, word.Hex()) {
			return true
		}
	}
	for i := 0; i+32 <= len(data); i += 32 {
		if bytes.Equal(data[i:i+32], word.Bytes()) {
			return true
		}
	}
	return false
}

func (s *statsBootstrapper) nodeRead(ctx context.Context, p *statsBootstrapProgress, height int64, read func(context.Context, *ethclient.Client) error) error {
	for i, node := range []*ethclient.Client{s.config.RPC, s.config.RPCFallback} {
		if node == nil {
			continue
		}
		bounded, cancel := context.WithTimeout(ctx, s.config.StatementTimeout)
		if !p.chains[i] {
			chain, err := node.ChainID(bounded)
			if err != nil {
				cancel()
				continue
			}
			if chain.Cmp(big.NewInt(125)) != 0 {
				cancel()
				return projectionstats.ErrIntegrity
			}
			p.chains[i] = true
		}
		err := read(bounded, node)
		cancel()
		if err == nil || errors.Is(err, projectionstats.ErrIntegrity) {
			return err
		}
		if ctx.Err() != nil {
			return ErrAccountingSourceUnavailable
		}
	}
	s.config.Logger.WarnContext(ctx, "stats bootstrap requires a node with the full history", "height", height)
	return fmt.Errorf("%w: height %d requires a node with the full history", ErrAccountingSourceUnavailable, height)
}

func (s *statsBootstrapper) canonical(ctx context.Context, p *statsBootstrapProgress) error {
	b := p.bootstrap
	fillHeight, _ := strconv.ParseInt(b.Fill.Position.Height, 10, 64)
	for p.rpcStep < 4 {
		height := b.Creation.Height
		switch p.rpcStep {
		case 2:
			height = b.Evidence.FirstCommitLastHeight
		case 3:
			height = fillHeight
		}
		err := s.nodeRead(ctx, p, height, func(ctx context.Context, node *ethclient.Client) error {
			if p.rpcStep == 1 {
				receipt, err := node.TransactionReceipt(ctx, common.HexToHash(b.Creation.TransactionHash))
				if err != nil {
					return err
				}
				return statsVerifyCreationReceipt(b.Creation, receipt)
			}
			hash, err := reportedBlockHash(ctx, node, height)
			if err != nil {
				return err
			}
			want := b.Creation.BlockHash
			switch p.rpcStep {
			case 2:
				want = b.Evidence.FirstCommitBlockHash
			case 3:
				want = b.Fill.Position.BlockHash
			}
			if hash.Hex() != want {
				return projectionstats.ErrIntegrity
			}
			return nil
		})
		if err != nil {
			return err
		}
		p.rpcStep++
	}
	return nil
}

func statsVerifyCreationReceipt(c sourcebootstrap.Creation, r *types.Receipt) error {
	if c.Height < 0 || r == nil || r.Status != types.ReceiptStatusSuccessful || r.TxHash.Hex() != c.TransactionHash || r.BlockHash.Hex() != c.BlockHash || r.BlockNumber == nil || !r.BlockNumber.IsInt64() || r.BlockNumber.Int64() != c.Height || c.TransactionIndex < 0 || uint64(r.TransactionIndex) != uint64(c.TransactionIndex) {
		return projectionstats.ErrIntegrity
	}
	var topics []string
	if json.Unmarshal(c.TopicsWire, &topics) != nil {
		return projectionstats.ErrIntegrity
	}
	matches := 0
	for _, log := range r.Logs {
		if log == nil || c.LogIndex < 0 || uint64(log.Index) != uint64(c.LogIndex) {
			continue
		}
		if log.Removed || log.BlockNumber != uint64(c.Height) || log.BlockHash != r.BlockHash || log.TxHash != r.TxHash || log.TxIndex != r.TransactionIndex || !strings.EqualFold(log.Address.Hex(), c.Factory) || !bytes.Equal(log.Data, c.Data) || len(log.Topics) != len(topics) {
			return projectionstats.ErrIntegrity
		}
		for i, topic := range log.Topics {
			if !strings.EqualFold(topic.Hex(), topics[i]) {
				return projectionstats.ErrIntegrity
			}
		}
		matches++
	}
	if matches != 1 {
		return projectionstats.ErrIntegrity
	}
	return nil
}

func (s *statsBootstrapper) slice(ctx context.Context, p *statsBootstrapProgress, to int64) error {
	b := p.bootstrap
	q := statsBoundedQuery{s.config.Pool, s.config.StatementTimeout}
	var complete, raw, records, commits, swaps, violations, openRaw, recordViolations int64
	err := q.QueryRow(ctx, `WITH blocks AS MATERIALIZED (
 SELECT height,block_hash,raw_count,fetch_complete,required_decode_complete FROM indexer.indexer_block_completeness_v1
 WHERE chain_id=125 AND stream_id=$1 AND height>$2 AND height<=$3
), logs AS MATERIALIZED (
 SELECT * FROM indexer.indexer_raw_logs_v1 WHERE chain_id=125 AND height>$2 AND height<=$3
), raw AS (
 SELECT height,count(*) n FROM logs GROUP BY height
), ranges AS (
 SELECT previous_height p,last_complete_height l,block_hash h,record_count n,
 lag(last_complete_height) OVER (ORDER BY previous_height,last_complete_height) prior
 FROM indexer.indexer_ingest_commits_v1 WHERE chain_id=125 AND stream_id=$1 AND previous_height<$3 AND last_complete_height>$2
), ranged AS (
 SELECT a.p,a.l,a.n,COALESCE(sum(b.raw_count),0) seen FROM ranges a
 LEFT JOIN blocks b ON b.height>a.p AND b.height<=a.l GROUP BY a.p,a.l,a.n
), swaps AS (
 SELECT * FROM logs WHERE topics->>0=$4 AND topics->>1=$5 AND (height,transaction_index,log_index)<=($6::bigint,$7::bigint,$8::bigint)
)
SELECT (SELECT count(*) FROM blocks WHERE fetch_complete AND required_decode_complete),
 (SELECT COALESCE(sum(raw_count),0) FROM blocks),
 (SELECT COALESCE(sum(n) FILTER (WHERE l<=$3),0) FROM ranges),
 (SELECT count(*) FROM ranges WHERE l<=$3), (SELECT count(*) FROM swaps),
 (SELECT count(*) FROM blocks b LEFT JOIN raw r USING(height) WHERE b.raw_count<>COALESCE(r.n,0))+
 (SELECT count(*) FROM logs r LEFT JOIN blocks b USING(height) WHERE b.block_hash IS DISTINCT FROM r.block_hash)+
 (SELECT count(*) FROM ranges a LEFT JOIN blocks b ON b.height=a.l WHERE a.l<=a.p OR (prior IS NOT NULL AND prior<>a.p) OR a.p<$9 OR a.l>$10 OR (a.l<=$3 AND b.block_hash IS DISTINCT FROM a.h))+
 CASE WHEN (SELECT min(p) FROM ranges)<=$2 AND (SELECT max(l) FROM ranges)>=$3 THEN 0 ELSE 1 END+
 (SELECT count(*) FROM swaps WHERE (height,transaction_index,log_index)<($6::bigint,$7::bigint,$8::bigint)),
 (SELECT count(*) FROM ranged WHERE l<=$3 AND n<>seen+CASE WHEN p<$2 THEN $11::bigint ELSE 0 END),
 (SELECT COALESCE(sum(seen+CASE WHEN p<$2 THEN $11::bigint ELSE 0 END),0) FROM ranged WHERE l>$3)`, s.config.SourceStream, p.last, to, statsSwapTopic.Hex(), common.BytesToHash(common.HexToAddress(b.Fill.PoolAddress).Bytes()).Hex(), b.Fill.Position.Height, b.Fill.Position.TransactionIndex, b.Fill.Position.LogIndex, b.Evidence.ReplayFromHeight, b.Evidence.FirstCommitLastHeight, p.openRaw).Scan(&complete, &raw, &records, &commits, &swaps, &violations, &recordViolations, &openRaw)
	if err != nil {
		return err
	}
	if complete != to-p.last || violations != 0 || p.swaps+swaps > int64(s.config.MaxReplayFills) {
		return ErrAccountingSourceUnavailable
	}
	if recordViolations != 0 {
		return projectionstats.ErrIntegrity
	}
	p.openRaw = openRaw
	p.last, p.raw, p.records, p.commits, p.swaps = to, p.raw+raw, p.records+records, p.commits+commits, p.swaps+swaps
	return nil
}

func (s *statsBootstrapper) counts(ctx context.Context, q sourcebootstrap.Querier, b statsSourceBootstrap) (int64, int64, error) {
	var commits, swaps int64
	q = statsBoundedQuery{q, s.config.StatementTimeout}
	err := q.QueryRow(ctx, `SELECT
 (SELECT count(*) FROM indexer.indexer_ingest_commits_v1 WHERE chain_id=125 AND stream_id=$1 AND previous_height>=$2 AND last_complete_height<=$3),
 (SELECT count(*) FROM indexer.indexer_raw_logs_v1 WHERE chain_id=125 AND height>$2 AND height<=$4 AND topics->>0=$7 AND topics->>1=$8 AND (height,transaction_index,log_index)<=($4::bigint,$5::bigint,$6::bigint))`, s.config.SourceStream, b.Evidence.ReplayFromHeight, b.Evidence.FirstCommitLastHeight, b.Fill.Position.Height, b.Fill.Position.TransactionIndex, b.Fill.Position.LogIndex, statsSwapTopic.Hex(), common.BytesToHash(common.HexToAddress(b.Fill.PoolAddress).Bytes()).Hex()).Scan(&commits, &swaps)
	return commits, swaps, err
}

func (s *statsBootstrapper) firstFill(ctx context.Context, b statsSourceBootstrap) error {
	var count int64
	q := statsBoundedQuery{s.config.Pool, s.config.StatementTimeout}
	err := q.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_accounting_fills_v1 f JOIN indexer.indexer_raw_logs_v1 r USING(chain_id,transaction_hash,log_index) WHERE f.chain_id=125 AND f.pool_address=$1 AND (r.height,r.transaction_index,r.log_index)<=($2::bigint,$3::bigint,$4::bigint)`, b.Fill.PoolAddress, b.Fill.Position.Height, b.Fill.Position.TransactionIndex, b.Fill.Position.LogIndex).Scan(&count)
	if err != nil {
		return err
	}
	if count != 1 {
		return ErrAccountingSourceUnavailable
	}
	return nil
}

func (s *statsBootstrapper) progress(pool string) (*statsBootstrapProgress, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p := s.pools[pool]
	if p == nil {
		if len(s.pools) >= maxStatsBootstrapProgress {
			for key, entry := range s.pools {
				if entry.users == 0 {
					delete(s.pools, key)
					break
				}
			}
			if len(s.pools) >= maxStatsBootstrapProgress {
				return nil, ErrAccountingSourceUnavailable
			}
		}
		p = &statsBootstrapProgress{gate: make(chan struct{}, 1)}
		s.pools[pool] = p
	}
	p.users++
	return p, nil
}

func (s *statsBootstrapper) lease(ctx context.Context, fill accounting.FillV1) (delivery.PartitionLease, error) {
	var lease delivery.PartitionLease
	proof, cancel := context.WithTimeout(ctx, s.config.ProofTimeout)
	defer cancel()
	p, err := s.progress(fill.PoolAddress)
	if err != nil {
		return lease, err
	}
	defer func() {
		s.mu.Lock()
		p.users--
		s.mu.Unlock()
	}()
	select {
	case p.gate <- struct{}{}:
		defer func() { <-p.gate }()
	case <-proof.Done():
		return lease, ErrAccountingSourceUnavailable
	}
	key := delivery.ConsumerKey{ConsumerID: s.config.ConsumerID, ProjectionGeneration: s.config.Generation, PartitionKind: valueMarket, PartitionKey: "125:" + fill.PoolAddress}
	if p.initialized {
		return s.config.Partitions.Acquire(ctx, key, s.config.Owner, s.config.LeaseDuration)
	}
	b, err := s.load(proof, s.config.Pool, fill.PoolAddress)
	if err != nil {
		return lease, err
	}
	if !reflect.DeepEqual(p.bootstrap, b) {
		p.bootstrap, p.last, p.commits, p.swaps, p.raw, p.records, p.openRaw, p.rpcStep, p.chains = b, b.Evidence.ReplayFromHeight, 0, 0, 0, 0, 0, 0, [2]bool{}
	}
	if err = s.canonical(proof, p); err != nil {
		return lease, err
	}
	for p.last < b.Evidence.FirstCommitLastHeight {
		if proof.Err() != nil {
			return lease, ErrAccountingSourceUnavailable
		}
		to := p.last + min(int64(s.config.MaxReplayBlocks), b.Evidence.FirstCommitLastHeight-p.last)
		if err = s.slice(proof, p, to); err != nil {
			if proof.Err() != nil {
				return lease, ErrAccountingSourceUnavailable
			}
			return lease, err
		}
	}
	if p.raw != p.records {
		return lease, projectionstats.ErrIntegrity
	}
	if p.swaps != 1 || p.commits == 0 {
		return lease, ErrAccountingSourceUnavailable
	}
	if err = s.firstFill(proof, b); err != nil {
		if proof.Err() != nil {
			return lease, ErrAccountingSourceUnavailable
		}
		return lease, err
	}
	lease, err = s.config.Partitions.Acquire(ctx, key, s.config.Owner, s.config.LeaseDuration)
	if err != nil {
		return lease, err
	}
	tx, err := s.config.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return lease, err
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		rollbackTransaction(cleanup, tx)
	}()
	current, err := s.load(ctx, tx, fill.PoolAddress)
	if err != nil {
		p.bootstrap = statsSourceBootstrap{}
		return lease, err
	}
	if current.Evidence.Checkpoint() != b.Evidence.Checkpoint() || !reflect.DeepEqual(current, b) {
		p.bootstrap = statsSourceBootstrap{}
		return lease, ErrAccountingSourceUnavailable
	}
	commits, swaps, err := s.counts(ctx, tx, current)
	if err != nil {
		return lease, err
	}
	if commits != p.commits || swaps != p.swaps {
		p.bootstrap = statsSourceBootstrap{}
		return lease, ErrAccountingSourceUnavailable
	}
	input := delivery.BootstrapInput{Lease: lease, CheckpointDigest: current.Evidence.Checkpoint(), NextSequence: "1"}
	if err = s.config.Partitions.BootstrapTx(ctx, tx, input, func(_ context.Context, _ pgx.Tx, in delivery.BootstrapInput) error {
		if in.CheckpointDigest != b.Evidence.Checkpoint() || in.NextSequence != "1" || in.PreviousEffectIdentity != "" {
			return projectionstats.ErrIntegrity
		}
		return nil
	}); err != nil {
		return lease, err
	}
	if err = tx.Commit(ctx); err != nil {
		return lease, err
	}
	p.initialized = true
	return lease, nil
}
