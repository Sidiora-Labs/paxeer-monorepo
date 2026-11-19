package app

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	canonicalSourceRefreshInterval = 10 * time.Second
	canonicalSourceRefreshTimeout  = 10 * time.Second
	canonicalSourceMaxAge          = time.Minute
)

var (
	errCanonicalCheckpointMismatch   = errors.New("indexer: persisted checkpoint differs from canonical source")
	errCanonicalCheckpointIncomplete = errors.New("indexer: canonical checkpoint has no complete source block or issued initial checkpoint")
	errCanonicalSourceUnrefreshed    = errors.New("indexer: canonical source has no successful refresh")
	errCanonicalSourceStale          = errors.New("indexer: canonical source successful refresh is older than one minute")
)

type canonicalSourceReadiness struct {
	store    *store.Store
	pool     *pgxpool.Pool
	provider *source.CommittedSource
	stream   string
	doc      lifecycleRuntimeDocument
	now      func() time.Time
	logger   *slog.Logger

	mu           sync.RWMutex
	lastSuccess  time.Time
	invalid      error
	failureStep  string
	failureClass string
	cancel       context.CancelFunc
	done         chan struct{}
}

func newCanonicalSourceReadiness(st *store.Store, pool *pgxpool.Pool, provider *source.CommittedSource, stream string, doc lifecycleRuntimeDocument, now func() time.Time, logger *slog.Logger) *canonicalSourceReadiness {
	return &canonicalSourceReadiness{store: st, pool: pool, provider: provider, stream: stream, doc: doc, now: now, logger: logger}
}

func (r *canonicalSourceReadiness) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	r.cancel = cancel
	r.done = make(chan struct{})
	go func() {
		defer close(r.done)
		ticker := time.NewTicker(canonicalSourceRefreshInterval)
		defer ticker.Stop()
		for {
			if ctx.Err() != nil {
				return
			}
			_ = r.refresh(ctx)
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (r *canonicalSourceReadiness) refresh(parent context.Context) error {
	ctx, cancel := context.WithTimeout(parent, canonicalSourceRefreshTimeout)
	defer cancel()
	step, err := r.read(ctx)
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err == nil {
		r.lastSuccess = r.now()
		r.invalid = nil
		if r.failureStep != "" {
			r.logger.Info("canonical source refresh", slog.String("step", r.failureStep), slog.String("class", "none"), slog.String("state", "ready"))
		}
		r.failureStep, r.failureClass = "", ""
	} else {
		if errors.Is(err, errCanonicalCheckpointMismatch) || errors.Is(err, errCanonicalCheckpointIncomplete) {
			r.invalid = err
		}
		class := canonicalSourceReadErrorClass(err)
		if step != r.failureStep || class != r.failureClass {
			r.logger.Warn("canonical source refresh", slog.String("step", step), slog.String("class", class), slog.String("state", "unavailable"))
		}
		r.failureStep, r.failureClass = step, class
	}
	return err
}

func (r *canonicalSourceReadiness) read(ctx context.Context) (string, error) {
	height, err := r.provider.Head(ctx)
	if err != nil {
		return "source_height", err
	}
	watermark, err := r.store.GetCanonicalWatermark(ctx, 125, r.stream)
	if err != nil {
		return "watermark", err
	}
	return "checkpoint", verifyCanonicalRuntimeCheckpoint(ctx, r.pool, r.provider, r.stream, min(height, watermark), r.doc)
}

func canonicalSourceReadErrorClass(err error) string {
	switch {
	case errors.Is(err, context.Canceled):
		return "canceled"
	case errors.Is(err, context.DeadlineExceeded):
		return "timeout"
	case errors.Is(err, errCanonicalCheckpointMismatch):
		return "checkpoint_mismatch"
	case errors.Is(err, errCanonicalCheckpointIncomplete):
		return "checkpoint_incomplete"
	case errors.Is(err, store.ErrWatermark):
		return "invalid_watermark"
	case errors.Is(err, pgx.ErrNoRows):
		return "missing_watermark"
	}
	var failure *source.SourceError
	if errors.As(err, &failure) {
		switch failure.Kind {
		case source.IdentityError, source.InvalidError, source.OversizeError, source.TimeoutError,
			source.ThrottleError, source.RPCError, source.RangeLimitError, source.IncompleteError,
			source.IntegrityError, source.ClosedError, source.OverloadError, source.AuthorizationError:
			return string(failure.Kind)
		}
	}
	var databaseFailure *pgconn.PgError
	if errors.As(err, &databaseFailure) {
		return "database"
	}
	return "read_error"
}

func (r *canonicalSourceReadiness) Ready(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.invalid != nil {
		return r.invalid
	}
	if r.lastSuccess.IsZero() {
		return errCanonicalSourceUnrefreshed
	}
	if r.now().Sub(r.lastSuccess) > canonicalSourceMaxAge {
		return errCanonicalSourceStale
	}
	return nil
}

func (r *canonicalSourceReadiness) Close() {
	if r.cancel != nil {
		r.cancel()
		<-r.done
	}
}
