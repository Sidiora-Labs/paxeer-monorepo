package consumer

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/chainread"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting/sourcebootstrap"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
)

// swapEventTopic is the Sidiora event hub Swap signature; topic1 is the pool.
var swapEventTopic = crypto.Keccak256Hash([]byte("Swap(bytes32,address,bool,uint256,uint256,uint256,uint256,uint256,uint256)"))

// poolCreation is the indexer's canonical MarketCreated fact for a pool joined
// to the raw log it was decoded from.
type poolCreation struct {
	Height, TransactionIndex, LogIndex int64
	BlockHash, TransactionHash         string
	Factory                            string
	Topics                             []string
	Data                               []byte
}

func (c poolCreation) position() accounting.SourcePosition {
	return accounting.SourcePosition{ChainID: "125", Height: strconv.FormatInt(c.Height, 10), TransactionIndex: strconv.FormatInt(c.TransactionIndex, 10), LogIndex: strconv.FormatInt(c.LogIndex, 10), TransactionHash: c.TransactionHash, BlockHash: c.BlockHash}
}

// chartSourceBootstrap is the evidence that partition sequence 1 of a pool is
// the pool's first fill: the replay runs from the indexer commit boundary
// before the pool's creation block through the commit holding the first fill.
type chartSourceBootstrap struct {
	Fill                  accounting.FillV1
	Position              accounting.PartitionPosition
	PublicationID         string
	Creation              poolCreation
	ReplayFromHeight      int64
	FirstCommitID         string
	FirstCommitLastHeight int64
	FirstCommitBlockHash  string
	Checkpoint            accounting.Digest
}

type sourceBootstrapper struct {
	config SourceVerifiedAccountingConfig
}

// load reads the bootstrap evidence for pool. A pool is bootstrapped from its
// own creation, at any distance from its first fill.
func (s sourceBootstrapper) load(ctx context.Context, tx sourcebootstrap.Querier, pool string) (chartSourceBootstrap, error) {
	config := s.config
	var b chartSourceBootstrap
	positionWire, err := sourcebootstrap.FirstMarketPosition(ctx, tx, config.SourcePartitionSchema, config.ProjectionVersion, pool)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrAccountingSourceUnavailable
	}
	if err != nil {
		return b, err
	}
	if err = json.Unmarshal(positionWire, &b.Position); err != nil {
		return b, err
	}
	if accounting.ValidatePartitionPosition(b.Position) != nil || b.Position.Sequence != "1" || b.Position.PreviousEffectIdentity != "" || b.Position.PartitionKind != marketPartitionKind || b.Position.PartitionKey != "125:"+pool || b.Position.ProjectionVersion != config.ProjectionVersion {
		return b, fmt.Errorf("%w: first partition position is not a market origin", ErrAccountingAuthority)
	}
	publicationWire, err := sourcebootstrap.MarketPublication(ctx, tx, b.Position.SourceEventID, config.ProjectionVersion)
	if err != nil {
		return b, err
	}
	publication, err := accounting.DecodePublicationV1(publicationWire)
	if err != nil {
		return b, err
	}
	if publication.Fill == nil || len(publication.Positions) != 1 || publication.Positions[0] != b.Position {
		return b, fmt.Errorf("%w: first publication does not carry the first position", ErrAccountingAuthority)
	}
	b.Fill = *publication.Fill
	b.PublicationID = publication.PublicationID
	if err = (sourceAuthority{reader: postgresSourceAuthority{queries: tx, config: config}}).authorizeFill(ctx, b.Fill); err != nil {
		return b, err
	}
	creation, err := sourcebootstrap.CanonicalCreation(ctx, tx, config.SourceStream, pool)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrAccountingSourceUnavailable
	}
	if err != nil {
		return b, err
	}
	topics := creation.TopicsWire
	b.Creation.Height, b.Creation.BlockHash, b.Creation.TransactionHash, b.Creation.TransactionIndex, b.Creation.LogIndex, b.Creation.Factory, b.Creation.Data = creation.Height, creation.BlockHash, creation.TransactionHash, creation.TransactionIndex, creation.LogIndex, creation.Factory, creation.Data
	b.Creation.Factory = strings.ToLower(b.Creation.Factory)
	if err = json.Unmarshal(topics, &b.Creation.Topics); err != nil {
		return b, ErrAccountingAuthority
	}
	if !logNamesAddress(b.Creation.Topics, b.Creation.Data, pool) {
		return b, fmt.Errorf("%w: creation log does not name the pool", ErrAccountingAuthority)
	}
	order, err := accounting.CompareSourcePosition(b.Creation.position(), b.Fill.Position)
	if err != nil {
		return b, err
	}
	if order >= 0 {
		return b, fmt.Errorf("%w: first fill does not follow pool creation", ErrAccountingAuthority)
	}
	b.ReplayFromHeight, err = sourcebootstrap.PreviousHeightContaining(ctx, tx, config.SourceStream, b.Creation.Height)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrAccountingSourceUnavailable
	}
	if err != nil {
		return b, err
	}
	height, err := strconv.ParseInt(b.Fill.Position.Height, 10, 64)
	if err != nil {
		return b, err
	}
	commit, err := sourcebootstrap.CommitContaining(ctx, tx, config.SourceStream, height)
	if errors.Is(err, pgx.ErrNoRows) {
		return b, ErrAccountingSourceUnavailable
	}
	if err != nil {
		return b, err
	}
	b.FirstCommitID, b.FirstCommitLastHeight, b.FirstCommitBlockHash = commit.ID, commit.LastCompleteHeight, commit.BlockHash
	if b.ReplayFromHeight < 0 || b.ReplayFromHeight >= b.Creation.Height || height < b.Creation.Height || b.FirstCommitLastHeight < height {
		return b, ErrAccountingSourceUnavailable
	}
	b.Checkpoint = sourcebootstrap.ChartsEvidence{SourceStream: config.SourceStream, Pool: pool, ProjectionVersion: config.ProjectionVersion, PublicationID: b.PublicationID, PayloadDigest: b.Position.PayloadDigest, ReplayFromHeight: b.ReplayFromHeight, CreationTransactionHash: b.Creation.TransactionHash, CreationLogIndex: b.Creation.LogIndex, FirstCommitID: b.FirstCommitID, FirstCommitLastHeight: b.FirstCommitLastHeight, FirstCommitBlockHash: b.FirstCommitBlockHash}.Checkpoint()
	return b, nil
}

type bootstrapProgress struct {
	Checkpoint                              accounting.Digest
	Stage                                   int
	LastHeight, Commits, Swaps, ReplayFills int64
	CommitPrevious, CommitLast, CommitRaw   int64
	PrimaryChain, FallbackChain             bool
	Used                                    time.Time
}

const bootstrapStatementTimeout = 3 * time.Second
const maxBootstrapProofs = 256

type bootstrapQueries struct{ sourcebootstrap.Querier }
type bootstrapRow struct {
	pgx.Row
	cancel context.CancelFunc
}

func (r bootstrapRow) Scan(dest ...any) error { defer r.cancel(); return r.Row.Scan(dest...) }
func (q bootstrapQueries) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	bounded, cancel := context.WithTimeout(ctx, bootstrapStatementTimeout)
	return bootstrapRow{Row: q.Querier.QueryRow(bounded, sql, args...), cancel: cancel}
}

func (s sourceBootstrapper) nodeRead(ctx context.Context, b chartSourceBootstrap, p *bootstrapProgress) error {
	height, err := strconv.ParseInt(b.Fill.Position.Height, 10, 64)
	if err != nil {
		return err
	}
	if p.Stage == 0 || p.Stage == 1 || p.Stage == 2 {
		height = b.Creation.Height
	}
	if p.Stage == 3 {
		height = b.FirstCommitLastHeight
	}
	for i, rpc := range []*ethclient.Client{s.config.RPC, s.config.RPCFallback} {
		if rpc == nil {
			continue
		}
		checked := &p.PrimaryChain
		if i == 1 {
			checked = &p.FallbackChain
		}
		if !*checked {
			bounded, cancel := context.WithTimeout(ctx, bootstrapStatementTimeout)
			chain, e := rpc.ChainID(bounded)
			cancel()
			if e != nil {
				continue
			}
			if chain.Cmp(big.NewInt(125)) != 0 {
				return ErrAccountingAuthority
			}
			*checked = true
		}
		if p.Stage == 0 {
			return nil
		}
		bounded, cancel := context.WithTimeout(ctx, bootstrapStatementTimeout)
		if p.Stage == 2 {
			receipt, e := rpc.TransactionReceipt(bounded, common.HexToHash(b.Creation.TransactionHash))
			cancel()
			if e != nil {
				continue
			}
			return verifyCreationReceipt(b, receipt)
		}
		block, e := (chainread.Reader{RPC: rpc}).Block(bounded, height)
		cancel()
		if e != nil {
			if errors.Is(e, chainread.ErrConflict) {
				return ErrAccountingAuthority
			}
			continue
		}
		expected := b.Fill.Position.BlockHash
		if p.Stage == 1 {
			expected = b.Creation.BlockHash
		}
		if p.Stage == 3 {
			expected = b.FirstCommitBlockHash
		}
		if block.Hash != expected || p.Stage == 4 && strconv.FormatInt(block.Time, 10) != b.Fill.BlockTimestampSeconds {
			return ErrAccountingAuthority
		}
		return nil
	}
	logger := s.config.Logger
	if logger == nil {
		logger = slog.Default()
	}
	logger.WarnContext(ctx, "charts: historical source unavailable; a node with the full history is required", slog.Int64("height", height))
	return fmt.Errorf("%w: height %d requires a node with the full history", ErrAccountingSourceUnavailable, height)
}

func verifyCreationReceipt(b chartSourceBootstrap, receipt *types.Receipt) error {
	if receipt == nil || receipt.Status != types.ReceiptStatusSuccessful || receipt.BlockHash.Hex() != b.Creation.BlockHash || receipt.BlockNumber == nil || receipt.BlockNumber.Int64() != b.Creation.Height || b.Creation.TransactionIndex < 0 || uint64(receipt.TransactionIndex) != uint64(b.Creation.TransactionIndex) {
		return fmt.Errorf("%w: creation receipt conflicts with indexed creation", ErrAccountingAuthority)
	}
	var created *types.Log
	for _, log := range receipt.Logs {
		if log != nil && b.Creation.LogIndex >= 0 && uint64(log.Index) == uint64(b.Creation.LogIndex) {
			created = log
		}
	}
	if created == nil || created.Removed || strings.ToLower(created.Address.Hex()) != b.Creation.Factory || !bytes.Equal(created.Data, b.Creation.Data) || len(created.Topics) != len(b.Creation.Topics) {
		return fmt.Errorf("%w: creation log conflicts with indexed raw log", ErrAccountingAuthority)
	}
	for i, topic := range created.Topics {
		if !strings.EqualFold(topic.Hex(), b.Creation.Topics[i]) {
			return fmt.Errorf("%w: creation log conflicts with indexed raw log", ErrAccountingAuthority)
		}
	}
	return nil
}

type bootstrapCommitSlice struct {
	Previous, Last, Records, Blocks, Raw, Violations int64
}

type bootstrapSlice struct {
	Commits               []bootstrapCommitSlice
	Swaps, Earlier, First int64
}

const bootstrapSliceSQL = `WITH bounds AS MATERIALIZED (SELECT $2::bigint start_height,$3::bigint end_height), raw AS MATERIALIZED (
 SELECT r.height,count(*) n,bool_and(r.block_hash=c.block_hash) canonical
 FROM indexer.indexer_raw_logs_v1 r
 LEFT JOIN indexer.indexer_block_completeness_v1 c USING(chain_id,height)
 WHERE r.chain_id=125 AND c.stream_id=$1 AND r.height>$2 AND r.height<=$3 GROUP BY r.height
), blocks AS MATERIALIZED (
 SELECT c.height,c.block_hash,COALESCE(r.n,0) actual,
 CASE WHEN c.fetch_complete AND c.required_decode_complete AND COALESCE(r.canonical,true) AND c.raw_count=COALESCE(r.n,0) THEN 0 ELSE 1 END bad,1 n
 FROM indexer.indexer_block_completeness_v1 c LEFT JOIN raw r USING(height)
 WHERE c.chain_id=125 AND c.stream_id=$1 AND c.height>$2 AND c.height<=$3
 UNION ALL SELECT $2,NULL,0,0,0
), totals AS MATERIALIZED (
 SELECT height,block_hash,sum(n) OVER w n,sum(actual) OVER w raw,sum(bad) OVER w bad FROM blocks WINDOW w AS (ORDER BY height ROWS UNBOUNDED PRECEDING)
), slices AS (
 SELECT a.previous_height AS "Previous",a.last_complete_height AS "Last",a.record_count AS "Records",
 COALESCE(hi.n-lo.n,-1) AS "Blocks",COALESCE(hi.raw-lo.raw,0) AS "Raw",
 COALESCE(hi.bad-lo.bad,1)+CASE WHEN a.last_complete_height<=$3 AND hi.block_hash IS DISTINCT FROM a.block_hash THEN 1 ELSE 0 END AS "Violations"
 FROM (
  (SELECT previous_height,last_complete_height,block_hash,record_count FROM indexer.indexer_ingest_commits_v1
  WHERE chain_id=125 AND stream_id=$1 AND previous_height<(SELECT end_height FROM bounds) AND last_complete_height>(SELECT start_height FROM bounds) AND numrange(least(previous_height,last_complete_height)::numeric,greatest(previous_height,last_complete_height)::numeric,'[]') && numrange($2::numeric,$3::numeric,'[]') ORDER BY previous_height,last_complete_height)
  UNION ALL
  (SELECT previous_height,last_complete_height,block_hash,record_count FROM indexer.indexer_ingest_commits_v1
  WHERE chain_id=125 AND stream_id=$1 AND previous_height>=(SELECT start_height FROM bounds) AND last_complete_height<=(SELECT end_height FROM bounds) AND numrange(least(previous_height,last_complete_height)::numeric,greatest(previous_height,last_complete_height)::numeric,'[]') && numrange($2::numeric,$3::numeric,'[]') AND previous_height>=(SELECT end_height FROM bounds) ORDER BY previous_height,last_complete_height)
  UNION ALL
  (SELECT previous_height,last_complete_height,block_hash,record_count FROM indexer.indexer_ingest_commits_v1
  WHERE chain_id=125 AND stream_id=$1 AND previous_height>=(SELECT start_height FROM bounds) AND last_complete_height<=(SELECT end_height FROM bounds) AND numrange(least(previous_height,last_complete_height)::numeric,greatest(previous_height,last_complete_height)::numeric,'[]') && numrange($2::numeric,$3::numeric,'[]') AND previous_height<(SELECT end_height FROM bounds) AND last_complete_height<=(SELECT start_height FROM bounds) ORDER BY previous_height,last_complete_height)
 ) a
 LEFT JOIN totals lo ON lo.height=greatest(a.previous_height,$2)
 LEFT JOIN totals hi ON hi.height=least(a.last_complete_height,$3)
), swaps AS (
 SELECT count(*) AS "Swaps",count(*) FILTER(WHERE (height,transaction_index,log_index)<($5::bigint,$6::bigint,$7::bigint)) AS "Earlier",
 count(*) FILTER(WHERE (height,transaction_index,log_index)=($5::bigint,$6::bigint,$7::bigint) AND transaction_hash=$8 AND block_hash=$9 AND emitter=$10) AS "First"
 FROM indexer.indexer_raw_logs_v1 WHERE chain_id=125 AND height>=$4 AND height>$2 AND height<=least($3,$5) AND topics->>0=$11 AND topics->>1=$12
) SELECT jsonb_build_object('Commits',(SELECT COALESCE(jsonb_agg(slices ORDER BY "Previous","Last"),'[]'::jsonb) FROM slices),'Swaps',"Swaps",'Earlier',"Earlier",'First',"First") FROM swaps`

func bootstrapUnavailable(reason string) error {
	return fmt.Errorf("%w: %w: %s", ErrAccountingSourceUnavailable, ErrAccountingAuthority, reason)
}

func (s sourceBootstrapper) advance(ctx context.Context, q sourcebootstrap.Querier, b chartSourceBootstrap, p *bootstrapProgress) error {
	q = bootstrapQueries{q}
	for p.Stage < 5 {
		if err := ctx.Err(); err != nil {
			return errors.Join(ErrAccountingSourceUnavailable, err)
		}
		if err := s.nodeRead(ctx, b, p); err != nil {
			return err
		}
		p.Stage++
	}
	height, err := strconv.ParseInt(b.Fill.Position.Height, 10, 64)
	if err != nil {
		return err
	}
	ti, err := strconv.ParseInt(b.Fill.Position.TransactionIndex, 10, 64)
	if err != nil {
		return err
	}
	li, err := strconv.ParseInt(b.Fill.Position.LogIndex, 10, 64)
	if err != nil {
		return err
	}
	for p.LastHeight < b.FirstCommitLastHeight {
		if err := ctx.Err(); err != nil {
			return errors.Join(ErrAccountingSourceUnavailable, err)
		}
		to := p.LastHeight + min(int64(s.config.MaxReplayBlocks), b.FirstCommitLastHeight-p.LastHeight)
		var wire []byte
		if err := q.QueryRow(ctx, bootstrapSliceSQL, s.config.SourceStream, p.LastHeight, to, b.Creation.Height, height, ti, li, b.Fill.Position.TransactionHash, b.Fill.Position.BlockHash, b.Fill.ActualEmitter, swapEventTopic.Hex(), common.BytesToHash(common.HexToAddress(b.Fill.PoolAddress).Bytes()).Hex()).Scan(&wire); err != nil {
			return err
		}
		var slice bootstrapSlice
		if err := json.Unmarshal(wire, &slice); err != nil {
			return err
		}
		next := *p
		cursor := p.LastHeight
		for _, part := range slice.Commits {
			start, end := max(part.Previous, p.LastHeight), min(part.Last, to)
			if part.Last <= part.Previous || start != cursor || part.Blocks != end-start || part.Violations != 0 {
				return bootstrapUnavailable("source replay range is not completely ingested")
			}
			if next.CommitLast != part.Last || next.CommitPrevious != part.Previous {
				if part.Previous != cursor {
					return bootstrapUnavailable("source commit boundaries conflict")
				}
				next.CommitPrevious, next.CommitLast, next.CommitRaw = part.Previous, part.Last, 0
			}
			next.CommitRaw += part.Raw
			if end == part.Last {
				if next.CommitRaw != part.Records {
					return bootstrapUnavailable("source commit record count conflicts")
				}
				next.Commits++
			}
			cursor = end
		}
		if cursor != to {
			return bootstrapUnavailable("source completeness rows are missing")
		}
		if slice.Earlier != 0 {
			return bootstrapUnavailable("indexed replay found an earlier pool swap")
		}
		next.ReplayFills += slice.Swaps
		next.Swaps += slice.First
		if next.ReplayFills > int64(s.config.MaxReplayFills) {
			return ErrAccountingSourceUnavailable
		}
		next.LastHeight = to
		*p = next
	}
	if p.Stage == 5 {
		if p.Swaps != 1 {
			return ErrAccountingAuthority
		}
		var data []byte
		if err := q.QueryRow(ctx, `SELECT data FROM indexer.indexer_raw_logs_v1 WHERE chain_id=125 AND transaction_hash=$1 AND log_index=$2`, b.Fill.Position.TransactionHash, li).Scan(&data); err != nil {
			return err
		}
		if !bootstrapSwapMatches(b.Fill, data) {
			return ErrAccountingAuthority
		}
		var count int64
		if err := q.QueryRow(ctx, `SELECT count(*) FROM indexer.indexer_accounting_fills_v1 f JOIN indexer.indexer_raw_logs_v1 r USING(chain_id,transaction_hash,log_index) WHERE f.chain_id=125 AND f.pool_address=$1 AND (r.height,r.transaction_index,r.log_index)<=($2::bigint,$3::bigint,$4::bigint)`, b.Fill.PoolAddress, height, ti, li).Scan(&count); err != nil {
			return err
		}
		if count != 1 {
			return ErrAccountingAuthority
		}
		p.Stage++
	}
	return nil
}

func bootstrapSwapMatches(fill accounting.FillV1, data []byte) bool {
	if len(data) != 7*32 {
		return false
	}
	word := func(i int) string { return new(big.Int).SetBytes(data[i*32 : (i+1)*32]).String() }
	side, input, output := "1", fill.USDLAmountRaw, fill.TokenAmountRaw
	if fill.Side == "sell" {
		side, input, output = "0", fill.TokenAmountRaw, fill.USDLAmountRaw
	}
	return word(0) == side && word(1) == input && word(2) == output && word(3) == fill.FeeRaw && word(5) == fill.BlockTimestampSeconds && word(6) == fill.Position.Height
}

func (s sourceBootstrapper) recheck(ctx context.Context, q sourcebootstrap.Querier, b chartSourceBootstrap, p bootstrapProgress) error {
	var commits, swaps int64
	height, err := strconv.ParseInt(b.Fill.Position.Height, 10, 64)
	if err != nil {
		return err
	}
	ti, err := strconv.ParseInt(b.Fill.Position.TransactionIndex, 10, 64)
	if err != nil {
		return err
	}
	li, err := strconv.ParseInt(b.Fill.Position.LogIndex, 10, 64)
	if err != nil {
		return err
	}
	err = q.QueryRow(ctx, `WITH bounds AS MATERIALIZED (SELECT $2::bigint start_height,$3::bigint end_height) SELECT (SELECT count(*) FROM (SELECT previous_height,last_complete_height FROM indexer.indexer_ingest_commits_v1 WHERE (chain_id,stream_id)=(125,$1) AND previous_height>=(SELECT start_height FROM bounds) AND last_complete_height<=(SELECT end_height FROM bounds) AND numrange(least(previous_height,last_complete_height)::numeric,greatest(previous_height,last_complete_height)::numeric,'[]') && numrange($2::numeric,$3::numeric,'[]') ORDER BY previous_height,last_complete_height) commits), (SELECT count(*) FROM indexer.indexer_raw_logs_v1 WHERE chain_id=125 AND height>=$4 AND (height,transaction_index,log_index)<=($5::bigint,$6::bigint,$7::bigint) AND topics->>0=$8 AND topics->>1=$9)`, s.config.SourceStream, b.ReplayFromHeight, b.FirstCommitLastHeight, b.Creation.Height, height, ti, li, swapEventTopic.Hex(), common.BytesToHash(common.HexToAddress(b.Fill.PoolAddress).Bytes()).Hex()).Scan(&commits, &swaps)
	if err != nil {
		return err
	}
	if commits != p.Commits || swaps != p.Swaps {
		return ErrAccountingSourceUnavailable
	}
	return nil
}

// logNamesAddress reports whether address appears as a 32-byte word in the
// log's topics or data.
func logNamesAddress(topics []string, data []byte, address string) bool {
	word := common.BytesToHash(common.HexToAddress(address).Bytes())
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
