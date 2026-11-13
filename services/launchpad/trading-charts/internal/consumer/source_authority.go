package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/chainread"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
)

type sourceWatermark struct {
	Raw, Typed, Fence int64
	Owner             string
	Alive             bool
}

type sourceAuthorityReader interface {
	Watermark(context.Context) (sourceWatermark, error)
	MatchesFill(context.Context, accounting.FillV1, accounting.Digest, []byte) (bool, error)
	MatchesPublication(context.Context, accounting.PublicationV1, []byte, []byte) (bool, error)
	Head(context.Context, string) (int64, error)
	Clock(context.Context, string) (engine.SeriesCheckpoint, error)
	BlockHash(context.Context, int64) (string, error)
}

type sourceAuthority struct {
	reader     sourceAuthorityReader
	rpc        *ethclient.Client
	projection string
}

func (a sourceAuthority) watermark(ctx context.Context) (int64, error) {
	w, err := a.reader.Watermark(ctx)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrAccountingSourceUnavailable
	}
	if err != nil {
		return 0, err
	}
	if w.Raw < 0 || w.Raw != w.Typed || w.Fence <= 0 || w.Owner == "" || !w.Alive {
		return 0, ErrAccountingSourceUnavailable
	}
	return w.Typed, nil
}

func (a sourceAuthority) authorizeFill(ctx context.Context, fill accounting.FillV1) error {
	if err := accounting.ValidateFillV1(fill); err != nil {
		return fmt.Errorf("%w: %v", ErrAccountingAuthority, err)
	}
	watermark, err := a.watermark(ctx)
	if err != nil {
		return err
	}
	height, err := strconv.ParseInt(fill.Position.Height, 10, 64)
	if err != nil {
		return err
	}
	if height > watermark {
		return ErrAccountingSourceUnavailable
	}
	body, err := json.Marshal(fill)
	if err != nil {
		return err
	}
	digest, err := accounting.FillDigest(fill)
	if err != nil {
		return err
	}
	same, err := a.reader.MatchesFill(ctx, fill, digest, body)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && !same {
		return ErrAccountingAuthority
	}
	return err
}

func (a sourceAuthority) authorizePublication(ctx context.Context, p accounting.PublicationV1) error {
	body, err := accounting.CanonicalPublicationV1(p)
	if err != nil {
		return err
	}
	if len(p.Positions) != 1 || p.Positions[0].ProjectionVersion != a.projection {
		return ErrAccountingAuthority
	}
	position, err := json.Marshal(p.Positions[0])
	if err != nil {
		return err
	}
	same, err := a.reader.MatchesPublication(ctx, p, body, position)
	if errors.Is(err, pgx.ErrNoRows) || err == nil && !same {
		return ErrAccountingAuthority
	}
	return err
}

func (a sourceAuthority) checkpoint(ctx context.Context, p accounting.PublicationV1) (engine.SeriesCheckpoint, error) {
	watermark, err := a.watermark(ctx)
	if err != nil {
		return engine.SeriesCheckpoint{}, err
	}
	head, err := a.reader.Head(ctx, p.Fill.PoolAddress)
	if err != nil {
		return engine.SeriesCheckpoint{}, err
	}
	sequence, err := strconv.ParseInt(p.Positions[0].Sequence, 10, 64)
	if err != nil {
		return engine.SeriesCheckpoint{}, err
	}
	if sequence != head {
		prior, err := a.reader.Clock(ctx, p.Fill.PoolAddress)
		if errors.Is(err, pgx.ErrNoRows) {
			err = nil
		}
		prior.HasGaps = true
		return prior, err
	}
	bounded, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	header, err := (chainread.Reader{RPC: a.rpc}).Block(bounded, watermark)
	if err != nil {
		return engine.SeriesCheckpoint{}, err
	}
	expected, err := a.reader.BlockHash(ctx, watermark)
	if err != nil {
		return engine.SeriesCheckpoint{}, err
	}
	if watermark < 0 || header.Hash != expected {
		return engine.SeriesCheckpoint{}, ErrAccountingAuthority
	}
	return engine.SeriesCheckpoint{Height: uint64(watermark), Timestamp: header.Time}, nil
}

type postgresSourceAuthority struct {
	tx     pgx.Tx
	config SourceVerifiedAccountingConfig
}

func (r postgresSourceAuthority) Watermark(ctx context.Context) (w sourceWatermark, err error) {
	err = r.tx.QueryRow(ctx, `SELECT w.raw_complete_height,w.typed_complete_height,l.fence_token,l.owner_instance,l.expires_at>clock_timestamp() FROM indexer.indexer_ingest_watermarks_v1 w JOIN indexer.indexer_ingest_leases_v1 l USING(chain_id,stream_id) WHERE w.chain_id=125 AND w.stream_id=$1`, r.config.SourceStream).Scan(&w.Raw, &w.Typed, &w.Fence, &w.Owner, &w.Alive)
	return
}
func (r postgresSourceAuthority) MatchesFill(ctx context.Context, fill accounting.FillV1, digest accounting.Digest, body []byte) (same bool, err error) {
	height, err := strconv.ParseInt(fill.Position.Height, 10, 64)
	if err != nil {
		return false, err
	}
	transactionIndex, err := strconv.ParseInt(fill.Position.TransactionIndex, 10, 64)
	if err != nil {
		return false, err
	}
	logIndex, err := strconv.ParseInt(fill.Position.LogIndex, 10, 64)
	if err != nil {
		return false, err
	}
	err = r.tx.QueryRow(ctx, `SELECT f.source_fact_digest=$2 AND f.payload=$3::jsonb AND f.pool_address=$4 AND r.height=$5 AND r.transaction_index=$6 AND r.block_hash=$7 AND r.emitter=$8 AND c.block_hash=r.block_hash AND c.fetch_complete AND c.required_decode_complete AND EXISTS(SELECT 1 FROM indexer.indexer_ingest_commits_v1 a WHERE a.chain_id=125 AND a.stream_id=$9 AND a.previous_height<r.height AND a.last_complete_height>=r.height) FROM indexer.indexer_accounting_fills_v1 f JOIN indexer.indexer_raw_logs_v1 r ON r.chain_id=f.chain_id AND r.transaction_hash=f.transaction_hash AND r.log_index=f.log_index JOIN indexer.indexer_block_completeness_v1 c ON c.chain_id=r.chain_id AND c.stream_id=$9 AND c.height=r.height WHERE f.source_fill_id=$1 AND f.chain_id=125 AND f.transaction_hash=$10 AND f.log_index=$11`, fill.SourceFillID, string(digest), body, fill.PoolAddress, height, transactionIndex, fill.Position.BlockHash, fill.ActualEmitter, r.config.SourceStream, fill.Position.TransactionHash, logIndex).Scan(&same)
	return
}
func (r postgresSourceAuthority) MatchesPublication(ctx context.Context, p accounting.PublicationV1, body, position []byte) (same bool, err error) {
	sequence, err := strconv.ParseInt(p.Positions[0].Sequence, 10, 64)
	if err != nil {
		return false, err
	}
	q := pgx.Identifier{r.config.SourcePartitionSchema}.Sanitize()
	err = r.tx.QueryRow(ctx, `SELECT p.payload=$2::jsonb AND EXISTS(SELECT 1 FROM `+q+`.indexer_market_partition_positions_v1 s WHERE s.projection_version=$3 AND s.partition_kind='market' AND s.partition_key=$4 AND s.sequence=$5 AND s.position=$6) FROM indexer.indexer_accounting_publications_v1 p WHERE p.publication_id=$1`, p.PublicationID, body, p.Positions[0].ProjectionVersion, p.Positions[0].PartitionKey, sequence, position).Scan(&same)
	return
}
func (r postgresSourceAuthority) Head(ctx context.Context, pool string) (head int64, err error) {
	q := pgx.Identifier{r.config.SourcePartitionSchema}.Sanitize()
	err = r.tx.QueryRow(ctx, `SELECT COALESCE(max(sequence),0) FROM `+q+`.indexer_market_partition_positions_v1 WHERE projection_version=$1 AND partition_kind='market' AND partition_key=$2`, r.config.ProjectionVersion, "125:"+pool).Scan(&head)
	return
}
func (r postgresSourceAuthority) Clock(ctx context.Context, pool string) (clock engine.SeriesCheckpoint, err error) {
	err = r.tx.QueryRow(ctx, `SELECT watermark_height,watermark_timestamp,has_gaps FROM `+r.config.Exact.Schema+`.chart_series_clock WHERE chain_id='125' AND pool=$1`, pool).Scan(&clock.Height, &clock.Timestamp, &clock.HasGaps)
	return
}
func (r postgresSourceAuthority) BlockHash(ctx context.Context, height int64) (hash string, err error) {
	err = r.tx.QueryRow(ctx, `SELECT block_hash FROM indexer.indexer_block_completeness_v1 WHERE chain_id=125 AND stream_id=$1 AND height=$2 AND fetch_complete AND required_decode_complete`, r.config.SourceStream, height).Scan(&hash)
	return
}
