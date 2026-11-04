package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strconv"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
)

const completenessBatchSize = 32
const completenessInterval = 5 * time.Second
const completenessBatchTimeout = 10 * time.Second

type persistedPoolPublication struct {
	Pool string
	Wire []byte
}

type completenessStore interface {
	Publications(context.Context, string, int) ([]persistedPoolPublication, error)
	Begin(context.Context) (completenessTransaction, error)
}

type completenessTransaction interface {
	sourceAuthorityReader
	Applied(context.Context, accounting.PublicationV1) error
	Unresolved(context.Context) (bool, error)
	Advance(context.Context, string, engine.SeriesCheckpoint) error
	Commit(context.Context) error
	Rollback(context.Context) error
}

type completenessRuntime struct {
	store      completenessStore
	rpc        *ethclient.Client
	projection string
}

func (r *completenessRuntime) refresh(ctx context.Context, p accounting.PublicationV1) error {
	if accounting.ValidatePublicationV1(p) != nil || p.Fill == nil || p.Facet != "market" || p.Positions[0].ProjectionVersion != r.projection {
		return ErrAccountingAuthority
	}
	tx, err := r.store.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() {
		cleanup, stop := context.WithTimeout(context.Background(), time.Second)
		defer stop()
		_ = tx.Rollback(cleanup)
	}()
	if err = tx.Applied(ctx, p); err != nil {
		return err
	}
	authority := sourceAuthority{reader: tx, rpc: r.rpc, projection: r.projection}
	if err = authority.authorizeFill(ctx, *p.Fill); err != nil {
		return err
	}
	if err = authority.authorizePublication(ctx, p); err != nil {
		return err
	}
	checkpoint, err := authority.checkpoint(ctx, p)
	if err != nil {
		return err
	}
	unresolved, err := tx.Unresolved(ctx)
	if err != nil {
		return err
	}
	checkpoint.HasGaps = checkpoint.HasGaps || unresolved
	prior, err := tx.Clock(ctx, p.Fill.PoolAddress)
	if err != nil {
		return err
	}
	if prior != checkpoint {
		if err = tx.Advance(ctx, p.Fill.PoolAddress, checkpoint); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (c *AccountingConsumer) refreshCompletenessBatch(ctx context.Context, after string, limit int) (string, error) {
	if c == nil || c.completeness == nil || limit < 1 || limit > completenessBatchSize {
		return after, ErrAccountingAuthority
	}
	entries, err := c.completeness.store.Publications(ctx, after, limit)
	if err != nil {
		return after, err
	}
	var failures error
	for _, entry := range entries {
		if ctx.Err() != nil {
			return after, errors.Join(failures, ctx.Err())
		}
		// Move past refused pools too, so one broken authority cannot starve others.
		after = entry.Pool
		p, decodeErr := accounting.DecodePublicationV1(entry.Wire)
		if decodeErr != nil || p.Fill == nil || p.Fill.PoolAddress != entry.Pool {
			failures = errors.Join(failures, ErrAccountingAuthority)
			continue
		}
		if err := c.RefreshCompleteness(ctx, p); err != nil {
			failures = errors.Join(failures, err)
		}
	}
	if len(entries) < limit {
		after = ""
	}
	return after, failures
}

// RunCompleteness resumes authenticated applied publications, including idle
// pools, in bounded pages. Transient source failures leave the clock unchanged
// and are retried on the next sweep.
func (c *AccountingConsumer) RunCompleteness(ctx context.Context, logger *slog.Logger) error {
	if c == nil || c.completeness == nil || logger == nil {
		return ErrAccountingAuthority
	}
	return c.runCompleteness(ctx, logger, completenessInterval)
}
func (c *AccountingConsumer) runCompleteness(ctx context.Context, logger *slog.Logger, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	after := ""
	for ctx.Err() == nil {
		bounded, stop := context.WithTimeout(ctx, completenessBatchTimeout)
		next, err := c.refreshCompletenessBatch(bounded, after, completenessBatchSize)
		stop()
		after = next
		if err != nil && ctx.Err() == nil {
			logger.Error("chart completeness refresh failed", slog.Any("err", err))
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
	return ctx.Err()
}

type postgresCompletenessStore struct {
	config SourceVerifiedAccountingConfig
}

func (s postgresCompletenessStore) Publications(ctx context.Context, after string, limit int) ([]persistedPoolPublication, error) {
	q := pgx.Identifier{s.config.Exact.Schema}.Sanitize()
	rows, err := s.config.Pool.Query(ctx, `SELECT clock.pool,pub.payload FROM `+q+`.chart_series_clock clock LEFT JOIN LATERAL (
 SELECT p.payload FROM `+q+`.delivery_partition_offsets o JOIN `+q+`.delivery_partition_commits c USING(consumer_id,projection_generation,partition_kind,partition_key)
 JOIN indexer.indexer_accounting_publications_v1 p ON p.payload->'positions'->0=convert_from(c.position,'UTF8')::jsonb
 WHERE o.consumer_id=$1 AND o.projection_generation=$2 AND o.partition_kind='market' AND o.partition_key='125:'||clock.pool AND c.sequence=o.next_sequence-1 AND c.effect_identity=o.previous_effect_identity AND p.payload->>'facet'='market' AND p.payload->'positions'->0->>'projectionVersion'=$3
 ) pub ON true WHERE clock.chain_id='125' AND clock.pool>$4 ORDER BY clock.pool LIMIT $5`, s.config.ConsumerID, s.config.Generation, s.config.ProjectionVersion, after, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []persistedPoolPublication{}
	for rows.Next() {
		var entry persistedPoolPublication
		if err = rows.Scan(&entry.Pool, &entry.Wire); err != nil {
			return nil, err
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}
func (s postgresCompletenessStore) Begin(ctx context.Context) (completenessTransaction, error) {
	tx, err := s.config.Pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
	if err != nil {
		return nil, err
	}
	return &postgresCompletenessTransaction{postgresSourceAuthority: postgresSourceAuthority{tx: tx, config: s.config}}, nil
}

type postgresCompletenessTransaction struct{ postgresSourceAuthority }

func (t *postgresCompletenessTransaction) Applied(ctx context.Context, p accounting.PublicationV1) error {
	cfg := t.config
	var epoch int64
	if err := t.tx.QueryRow(ctx, `SELECT epoch FROM `+cfg.Exact.Schema+`.chart_pool_fences WHERE chain_id='125' AND pool=$1 AND NOT frozen FOR UPDATE`, p.Fill.PoolAddress).Scan(&epoch); err != nil {
		return err
	}
	raw, err := json.Marshal(*p.Fill)
	if err != nil {
		return err
	}
	var applied bool
	err = t.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM `+cfg.Exact.Schema+`.chart_inbox WHERE chain_id='125' AND pool=$1 AND fill=$2::jsonb)`, p.Fill.PoolAddress, raw).Scan(&applied)
	if err != nil {
		return err
	}
	if !applied {
		return ErrAccountingAuthority
	}
	sequence, err := strconv.ParseInt(p.Positions[0].Sequence, 10, 64)
	if err != nil {
		return err
	}
	position, err := json.Marshal(p.Positions[0])
	if err != nil {
		return err
	}
	q := pgx.Identifier{cfg.Exact.Schema}.Sanitize()
	err = t.tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM `+q+`.delivery_partition_offsets o JOIN `+q+`.delivery_partition_commits c USING(consumer_id,projection_generation,partition_kind,partition_key) WHERE o.consumer_id=$1 AND o.projection_generation=$2 AND o.partition_kind='market' AND o.partition_key=$3 AND c.sequence=$4 AND c.position=$5 AND c.effect_identity=$6 AND o.next_sequence-1=c.sequence AND o.previous_effect_identity=c.effect_identity)`, cfg.ConsumerID, cfg.Generation, p.Positions[0].PartitionKey, sequence, position, p.Positions[0].EffectIdentity).Scan(&applied)
	if err != nil {
		return err
	}
	if !applied {
		return ErrAccountingAuthority
	}
	return nil
}
func (t *postgresCompletenessTransaction) Unresolved(ctx context.Context) (bool, error) {
	return t.config.Exact.HasUnresolvedDeliveriesTx(ctx, t.tx)
}
func (t *postgresCompletenessTransaction) Advance(ctx context.Context, pool string, checkpoint engine.SeriesCheckpoint) error {
	_, err := t.config.Exact.AdvanceSeriesTx(ctx, t.tx, pool, checkpoint.Height, checkpoint.Timestamp, checkpoint.HasGaps)
	return err
}
func (t *postgresCompletenessTransaction) Commit(ctx context.Context) error { return t.tx.Commit(ctx) }
func (t *postgresCompletenessTransaction) Rollback(ctx context.Context) error {
	return t.tx.Rollback(ctx)
}

func (t *postgresCompletenessTransaction) Clock(ctx context.Context, pool string) (clock engine.SeriesCheckpoint, err error) {
	err = t.tx.QueryRow(ctx, `SELECT watermark_height,watermark_timestamp,has_gaps FROM `+t.config.Exact.Schema+`.chart_series_clock WHERE chain_id='125' AND pool=$1 FOR UPDATE`, pool).Scan(&clock.Height, &clock.Timestamp, &clock.HasGaps)
	return
}
