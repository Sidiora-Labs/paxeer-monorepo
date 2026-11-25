// Package consumer implements the Redis pub/sub swap consumer that feeds the
// candle builder, porting candles/src/engine/swap-consumer.ts. It subscribes to
// the indexer swap channel and forwards Swap events to the Builder.
package consumer

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"regexp"
	"time"

	chartcleanup "github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/cleanup"
	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/accounting"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery/natsdriver"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/constants"
	sharedredis "github.com/Sidiora-Technologies/KindleLaunch/shared/redis"

	"github.com/Sidiora-Technologies/KindleLaunch/core/trading-charts/internal/engine"
)

// SwapConsumer subscribes to the Redis swap channel and processes swaps through
// the candle builder.
type SwapConsumer struct {
	source     *DevelopmentSource
	builder    *engine.Builder
	subscriber *sharedredis.Subscriber
	logger     *slog.Logger
}

// New creates a SwapConsumer.
func New(builder *engine.Builder, redisURL string, logger *slog.Logger) (*SwapConsumer, error) {
	sub, err := sharedredis.NewSubscriber(redisURL)
	if err != nil {
		return nil, fmt.Errorf("consumer: new subscriber: %w", err)
	}
	return &SwapConsumer{
		builder:    builder,
		subscriber: sub,
		logger:     logger,
	}, nil
}

// Start subscribes to the swap channel and processes events until ctx is cancelled.
func (sc *SwapConsumer) Start(ctx context.Context) error {
	errc, err := sc.subscriber.Subscribe(ctx, constants.ChannelSwap, func(ctx context.Context, payload []byte) error {
		swap, err := DecodeDevelopmentSwap(payload)
		if err != nil {
			return err
		}

		if err := sc.builder.ProcessSwap(ctx, swap); err != nil {
			return fmt.Errorf("consumer: process swap: %w", err)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("consumer: subscribe: %w", err)
	}

	if sc.source != nil {
		go sc.source.Run(ctx)
	}
	sc.logger.Info("candle swap consumer started")

	go func() {
		for err := range errc {
			sc.logger.Error("consumer: receive error", slog.String("err", err.Error()))
		}
	}()

	return nil
}

// Close releases the subscriber connection.
func (sc *SwapConsumer) Close() error {
	return sc.subscriber.Close()
}

func asString(v interface{}) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case float64:
		return fmt.Sprintf("%v", val)
	case json.Number:
		return val.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func asBool(v interface{}) bool {
	if v == nil {
		return false
	}
	switch val := v.(type) {
	case bool:
		return val
	case string:
		return val == "true"
	default:
		return false
	}
}

func asInt64(v interface{}) int64 {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case float64:
		return int64(val)
	case json.Number:
		n, err := val.Int64()
		if err != nil {
			return 0
		}
		return n
	case string:
		var n int64
		if _, err := fmt.Sscanf(val, "%d", &n); err != nil {
			return 0
		}
		return n
	default:
		return 0
	}
}

var ErrAccountingAuthority = errors.New("charts: accounting authority rejected")

type AccountingConfig struct {
	Deliveries                                *delivery.Store
	Partitions                                *delivery.PartitionStore
	Exact                                     *store.ExactStore
	ConsumerID, Generation, ProjectionVersion string
	Lease                                     func(context.Context, accounting.FillV1) (delivery.PartitionLease, error)
	Authorize                                 func(context.Context, pgx.Tx, accounting.FillV1) error
	AuthorizePublication                      func(context.Context, pgx.Tx, accounting.PublicationV1) error
	StreamWriter                              *delivery.StreamWriter
	SourceStream                              string
	SeriesCheckpoint                          func(context.Context, pgx.Tx, accounting.PublicationV1) (engine.SeriesCheckpoint, error)
}
type AccountingConsumer struct {
	config         AccountingConfig
	preflightLease bool
	reprocess      *ReprocessRequest
	completeness   *completenessRuntime
}

func NewAccountingConsumer(config AccountingConfig) (*AccountingConsumer, error) {
	if config.Deliveries == nil || config.Partitions == nil || config.Exact == nil || config.ConsumerID == "" || config.Generation == "" || config.ProjectionVersion == "" || config.Lease == nil || config.Authorize == nil {
		return nil, errors.New("charts: complete accounting authority and ordered stores required")
	}
	return &AccountingConsumer{config: config}, nil
}
func (c *AccountingConsumer) Consume(ctx context.Context, message *natsdriver.Message) error {
	if c == nil || message == nil || message.Subject != accounting.MarketSubject || message.Stream == "" || message.Sequence == 0 || message.EventID == "" {
		return errors.New("charts: real bounded market broker message required")
	}
	cfg := c.config
	if c.reprocess != nil {
		if message.EventID != c.reprocess.PublicationID || delivery.Hash(message.Wire) != c.reprocess.CorrectedDigest {
			return ErrAccountingAuthority
		}
		if err := cfg.Exact.CheckReprocess(ctx, cfg.ConsumerID, cfg.Generation, message.EventID, delivery.Hash(message.Wire)); err != nil {
			return err
		}
	}
	if cfg.SeriesCheckpoint != nil {
		if message.Sequence > math.MaxInt64 {
			return ErrAccountingAuthority
		}
		if err := cfg.Exact.RecordAttempt(ctx, cfg.ConsumerID, cfg.Generation, message.EventID, delivery.Hash(message.Wire), int64(message.Sequence), "received", false); err != nil {
			if errors.Is(err, store.ErrIntegrity) {
				pools := []string{}
				var envelope delivery.Envelope
				if json.Unmarshal(message.Wire, &envelope) == nil {
					publication, decodeErr := accounting.DecodePublicationV1(envelope.Payload)
					if decodeErr == nil && publication.Fill != nil {
						fill := publication.Fill
						pools = append(pools, fill.PoolAddress)
						var originalPool string
						lookup := cfg.Exact.Pool.QueryRow(ctx, `SELECT pool FROM `+cfg.Exact.Schema+`.chart_inbox WHERE chain_id='125' AND emitter=$1 AND code_epoch=$2 AND tx_hash=$3 AND log_index=$4::numeric`, fill.ActualEmitter, fill.Provenance.EmitterCodeEpoch, fill.Position.TransactionHash, fill.Position.LogIndex).Scan(&originalPool)
						if lookup != nil && !errors.Is(lookup, pgx.ErrNoRows) {
							return errors.Join(err, lookup)
						}
						if lookup == nil && originalPool != fill.PoolAddress {
							pools = append(pools, originalPool)
						}
					}
				}
				in := delivery.QuarantineInput{ConsumerID: cfg.ConsumerID, Generation: cfg.Generation, Subject: message.Subject, PublicationID: message.EventID, Wire: message.Wire, WireSHA256: accounting.Digest(delivery.Hash(message.Wire)), Class: delivery.IdentityConflict, AffectedPools: pools, EvidenceDigest: accounting.Digest(delivery.Hash(message.Wire))}
				qerr := cfg.Deliveries.QuarantineAfterRollback(ctx, in, func(ctx context.Context, tx pgx.Tx, affected []string) error {
					return cfg.Exact.FreezePoolsTx(ctx, tx, affected, in.EvidenceDigest)
				})
				return errors.Join(err, qerr)
			}
			return err
		}
		if err := cfg.Exact.WaitRetryBackoff(ctx, cfg.ConsumerID, cfg.Generation, message.EventID); err != nil {
			return err
		}
	}
	decode := func(envelope delivery.Envelope) (accounting.PublicationV1, error) {
		publication, err := accounting.DecodePublicationV1(envelope.Payload)
		if err != nil {
			return publication, &delivery.IntegrityFailure{Class: delivery.InnerSchema, Cause: err}
		}
		pools := []string{}
		if publication.Fill != nil {
			pools = append(pools, publication.Fill.PoolAddress)
		}
		if publication.Facet != marketPartitionKind || publication.Fill == nil || publication.Enrichment != nil || len(publication.Positions) != 1 || publication.Positions[0].ProjectionVersion != cfg.ProjectionVersion || publication.Positions[0].PartitionKind != marketPartitionKind || publication.Positions[0].PartitionKey != "125:"+publication.Fill.PoolAddress || publication.PublicationID != message.EventID || publication.PublicationID != envelope.EventID {
			return publication, &delivery.IntegrityFailure{Class: delivery.PartitionBinding, AffectedPools: pools, Cause: delivery.ErrIntegrity}
		}
		return publication, nil
	}
	var prepared *delivery.PartitionLease
	if c.preflightLease {
		envelope, err := delivery.Decode(message.Wire, delivery.Policy{Subjects: map[string]bool{accounting.MarketSubject: true}, EventTypes: map[string]bool{accounting.MarketSubject: true}}, message.Subject)
		if err == nil {
			publication, decodeErr := decode(envelope)
			if decodeErr == nil && publication.Positions[0].SourceOrder.ChainID == "125" && publication.Positions[0].SourceOrder.Height == envelope.SourceOrder.Height && publication.Positions[0].SourceOrder.TransactionIndex == envelope.SourceOrder.TransactionIndex && publication.Positions[0].SourceOrder.LogIndex == envelope.SourceOrder.LogIndex {
				lease, err := cfg.Lease(ctx, *publication.Fill)
				if err != nil {
					return err
				}
				prepared = &lease
			}
		}
	}
	hooks := delivery.ConsumeHooks{
		Guard: func(ctx context.Context, tx pgx.Tx, envelope delivery.Envelope) (delivery.GuardedPosition, error) {
			publication, err := decode(envelope)
			if err != nil {
				return delivery.GuardedPosition{}, err
			}
			var lease delivery.PartitionLease
			if prepared != nil {
				lease = *prepared
			} else {
				if c.preflightLease {
					return delivery.GuardedPosition{}, &delivery.IntegrityFailure{Class: delivery.PartitionBinding, AffectedPools: []string{publication.Fill.PoolAddress}, Cause: delivery.ErrIntegrity}
				}
				lease, err = cfg.Lease(ctx, *publication.Fill)
				if err != nil {
					return delivery.GuardedPosition{}, err
				}
			}
			if lease.Key.ConsumerID != cfg.ConsumerID || lease.Key.ProjectionGeneration != cfg.Generation {
				return delivery.GuardedPosition{}, delivery.ErrLease
			}
			guard, err := cfg.Partitions.CheckAndLockTx(ctx, tx, lease, publication.Positions[0])
			if err != nil {
				return guard, err
			}
			if err = cfg.Authorize(ctx, tx, *publication.Fill); err != nil {
				if !errors.Is(err, ErrAccountingAuthority) {
					return guard, err
				}
				return guard, &delivery.IntegrityFailure{Class: delivery.Authority, AffectedPools: []string{publication.Fill.PoolAddress}, EvidenceDigest: accounting.Digest(delivery.Hash(envelope.Payload)), Cause: err}
			}
			if cfg.AuthorizePublication != nil {
				if err = cfg.AuthorizePublication(ctx, tx, publication); err != nil {
					if !errors.Is(err, ErrAccountingAuthority) {
						return guard, err
					}
					return guard, &delivery.IntegrityFailure{Class: delivery.Authority, AffectedPools: []string{publication.Fill.PoolAddress}, Cause: err}
				}
			}
			return guard, nil
		},
		Effect: func(ctx context.Context, tx pgx.Tx, envelope delivery.Envelope, g delivery.GuardedPosition) error {
			publication, err := decode(envelope)
			if err != nil {
				return err
			}
			if cfg.SeriesCheckpoint != nil {
				if err := cfg.Exact.RecordAttemptResultTx(ctx, tx, cfg.ConsumerID, cfg.Generation, message.EventID, "applying", true); err != nil {
					return err
				}
				checkpoint, err := cfg.SeriesCheckpoint(ctx, tx, publication)
				if err != nil {
					return err
				}
				if err = engine.ApplySeriesStreamTx(ctx, tx, cfg.Exact, *publication.Fill, g.Lease.Epoch, checkpoint, cfg.StreamWriter, publication.Positions[0], cfg.SourceStream); err != nil {
					if errors.Is(err, store.ErrSourceWatermarkRegression) {
						return &delivery.IntegrityFailure{Class: delivery.IdentityConflict, AffectedPools: []string{publication.Fill.PoolAddress}, Cause: err}
					}
					return err
				}
				return nil
			}
			return engine.ApplyQualifiedTx(ctx, tx, cfg.Exact, *publication.Fill, g.Lease.Epoch)
		},
		Advance: func(ctx context.Context, tx pgx.Tx, envelope delivery.Envelope, g delivery.GuardedPosition) error {
			if err := cfg.Partitions.AdvanceTx(ctx, tx, g); err != nil {
				return err
			}
			if cfg.SeriesCheckpoint != nil {
				return cfg.Exact.RecordAttemptResultTx(ctx, tx, cfg.ConsumerID, cfg.Generation, message.EventID, "committed", true)
			}
			return nil
		},
		Freeze: func(ctx context.Context, tx pgx.Tx, pools []string) error {
			return cfg.Exact.FreezePoolsTx(ctx, tx, pools, accounting.Digest(delivery.Hash(message.Wire)))
		},
	}
	err := cfg.Deliveries.ConsumeOrdered(ctx, cfg.ConsumerID, cfg.Generation, message.Subject, message.Wire, message, hooks)
	if err != nil && cfg.SeriesCheckpoint != nil {
		category := "dependency_unavailable"
		var integrity *delivery.IntegrityFailure
		if errors.As(err, &integrity) {
			category = string(integrity.Class)
		}
		if e := cfg.Exact.RecordAttemptResult(ctx, cfg.ConsumerID, cfg.Generation, message.EventID, category, false); e != nil {
			return errors.Join(err, e)
		}
	}
	return err
}
func (c *AccountingConsumer) Run(ctx context.Context, driver *natsdriver.Consumer) error {
	if driver == nil {
		return errors.New("charts: real broker driver required")
	}
	return driver.Run(ctx, c.Consume)
}

var ErrAccountingSourceUnavailable = errors.New("charts: canonical source unavailable or bootstrap proof incomplete")

type SourceVerifiedAccountingConfig struct {
	Pool                                                                                  *pgxpool.Pool
	Deliveries                                                                            *delivery.Store
	Partitions                                                                            *delivery.PartitionStore
	Exact                                                                                 *store.ExactStore
	ConsumerID, Generation, ProjectionVersion, Owner, SourceStream, SourcePartitionSchema string
	RPC                                                                                   *ethclient.Client
	RPCFallback                                                                           *ethclient.Client
	Logger                                                                                *slog.Logger
	ProofTimeout, LeaseDuration                                                           time.Duration
	MaxReplayBlocks, MaxReplayFills                                                       int
	SeriesV2                                                                              bool
	StreamWriter                                                                          *delivery.StreamWriter
}

func NewSourceVerifiedAccountingConsumer(config SourceVerifiedAccountingConfig) (*AccountingConsumer, error) {
	if config.Pool == nil || config.Deliveries == nil || config.Partitions == nil || config.Exact == nil || config.RPC == nil || config.Owner == "" || config.SourceStream == "" || len(config.SourceStream) > 128 || !regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`).MatchString(config.SourcePartitionSchema) || config.MaxReplayBlocks < 1 || config.MaxReplayBlocks > 4096 || config.MaxReplayFills < 1 || config.MaxReplayFills > 10000 {
		return nil, errors.New("charts: concrete canonical RPC/source stores and bounded replay configuration required")
	}
	if config.SeriesV2 && (config.StreamWriter == nil || config.StreamWriter.Schema != "api_stream") {
		return nil, errors.New("charts: canonical candle stream writer required")
	}
	if config.LeaseDuration == 0 {
		config.LeaseDuration = 30 * time.Second
	}
	if config.ProofTimeout == 0 {
		config.ProofTimeout = 5 * time.Second
	}
	if config.LeaseDuration < time.Second || config.LeaseDuration > 5*time.Minute || config.ProofTimeout < time.Millisecond || config.ProofTimeout > 15*time.Second {
		return nil, errors.New("charts: bounded bootstrap time and lease duration required")
	}
	serial := make(chan struct{}, 1)
	initialized := map[string]bool{}
	progress := map[string]*bootstrapProgress{}
	authority := func(tx pgx.Tx) sourceAuthority {
		return sourceAuthority{reader: postgresSourceAuthority{tx: tx, config: config}, rpc: config.RPC, projection: config.ProjectionVersion}
	}
	authorize := func(ctx context.Context, tx pgx.Tx, fill accounting.FillV1) error {
		return authority(tx).authorizeFill(ctx, fill)
	}
	bootstrapper := sourceBootstrapper{config: config}
	lease := func(ctx context.Context, fill accounting.FillV1) (delivery.PartitionLease, error) {
		var l delivery.PartitionLease
		proofCtx, stop := context.WithTimeout(ctx, config.ProofTimeout)
		defer stop()
		select {
		case serial <- struct{}{}:
			defer func() { <-serial }()
		case <-proofCtx.Done():
			return l, errors.Join(ErrAccountingSourceUnavailable, proofCtx.Err())
		}
		key := delivery.ConsumerKey{ConsumerID: config.ConsumerID, ProjectionGeneration: config.Generation, PartitionKind: marketPartitionKind, PartitionKey: "125:" + fill.PoolAddress}
		if initialized[fill.PoolAddress] {
			return config.Partitions.Acquire(ctx, key, config.Owner, config.LeaseDuration)
		}
		b, err := bootstrapper.load(proofCtx, bootstrapQueries{config.Pool}, fill.PoolAddress)
		if err != nil {
			if proofCtx.Err() == nil {
				delete(progress, fill.PoolAddress)
				return l, err
			}
			return l, errors.Join(ErrAccountingSourceUnavailable, err)
		}
		p := progress[fill.PoolAddress]
		if p == nil || p.Checkpoint != b.Checkpoint {
			if len(progress) >= maxBootstrapProofs {
				oldest := fill.PoolAddress
				var used time.Time
				for pool, candidate := range progress {
					if used.IsZero() || candidate.Used.Before(used) {
						oldest, used = pool, candidate.Used
					}
				}
				delete(progress, oldest)
			}
			p = &bootstrapProgress{Checkpoint: b.Checkpoint, LastHeight: b.ReplayFromHeight}
			progress[fill.PoolAddress] = p
		}
		p.Used = time.Now()
		if err = bootstrapper.advance(proofCtx, config.Pool, b, p); err != nil {
			if proofCtx.Err() == nil && !errors.Is(err, context.DeadlineExceeded) {
				delete(progress, fill.PoolAddress)
				return l, err
			}
			return l, errors.Join(ErrAccountingSourceUnavailable, err)
		}
		if err = proofCtx.Err(); err != nil {
			return l, errors.Join(ErrAccountingSourceUnavailable, err)
		}
		l, err = config.Partitions.Acquire(ctx, key, config.Owner, config.LeaseDuration)
		if err != nil {
			return l, err
		}
		bounded, cancel := context.WithTimeout(ctx, min(config.LeaseDuration, bootstrapStatementTimeout))
		defer cancel()
		tx, err := config.Pool.BeginTx(bounded, pgx.TxOptions{IsoLevel: pgx.RepeatableRead})
		if err != nil {
			return l, err
		}
		defer func() {
			cleanup, stop := context.WithTimeout(context.Background(), time.Second)
			defer stop()
			chartcleanup.Rollback(cleanup, tx)
		}()
		input := delivery.BootstrapInput{Lease: l, CheckpointDigest: b.Checkpoint, NextSequence: "1", PreviousEffectIdentity: ""}
		err = config.Partitions.BootstrapTx(bounded, tx, input, func(ctx context.Context, tx pgx.Tx, in delivery.BootstrapInput) error {
			current, err := bootstrapper.load(ctx, bootstrapQueries{tx}, fill.PoolAddress)
			if err != nil {
				delete(progress, fill.PoolAddress)
				return errors.Join(ErrAccountingSourceUnavailable, err)
			}
			if current.Checkpoint != p.Checkpoint || in.CheckpointDigest != p.Checkpoint {
				delete(progress, fill.PoolAddress)
				return ErrAccountingSourceUnavailable
			}
			if err = bootstrapper.recheck(ctx, bootstrapQueries{tx}, current, *p); err != nil {
				delete(progress, fill.PoolAddress)
				return err
			}
			return nil
		})
		if err != nil {
			return l, err
		}
		if err = tx.Commit(bounded); err != nil {
			return l, err
		}
		initialized[fill.PoolAddress] = true
		delete(progress, fill.PoolAddress)
		return l, nil
	}
	authorizePublication := func(ctx context.Context, tx pgx.Tx, publication accounting.PublicationV1) error {
		return authority(tx).authorizePublication(ctx, publication)
	}
	var checkpoint func(context.Context, pgx.Tx, accounting.PublicationV1) (engine.SeriesCheckpoint, error)
	if config.SeriesV2 {
		checkpoint = func(ctx context.Context, tx pgx.Tx, publication accounting.PublicationV1) (engine.SeriesCheckpoint, error) {
			return authority(tx).checkpoint(ctx, publication)
		}
	}
	c, err := NewAccountingConsumer(AccountingConfig{Deliveries: config.Deliveries, Partitions: config.Partitions, Exact: config.Exact, ConsumerID: config.ConsumerID, Generation: config.Generation, ProjectionVersion: config.ProjectionVersion, Lease: lease, Authorize: authorize, AuthorizePublication: authorizePublication, SeriesCheckpoint: checkpoint, StreamWriter: config.StreamWriter, SourceStream: config.SourceStream})
	if err != nil {
		return nil, err
	}
	c.preflightLease = true
	if config.SeriesV2 {
		c.completeness = &completenessRuntime{store: postgresCompletenessStore{config: config}, rpc: config.RPC, projection: config.ProjectionVersion}
	}
	return c, nil
}
