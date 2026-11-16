package app

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/source"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
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
	head     source.HeadSource
	store    *store.Store
	pool     *pgxpool.Pool
	provider *source.CommittedSource
	stream   string
	doc      lifecycleRuntimeDocument
	now      func() time.Time

	mu          sync.RWMutex
	lastSuccess time.Time
	invalid     error
	cancel      context.CancelFunc
	done        chan struct{}
}

func newCanonicalSourceReadiness(head source.HeadSource, st *store.Store, pool *pgxpool.Pool, provider *source.CommittedSource, stream string, doc lifecycleRuntimeDocument, now func() time.Time) *canonicalSourceReadiness {
	return &canonicalSourceReadiness{head: head, store: st, pool: pool, provider: provider, stream: stream, doc: doc, now: now}
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
			r.refresh(ctx)
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
	err := r.read(ctx)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err == nil {
		r.lastSuccess = r.now()
		r.invalid = nil
	} else if errors.Is(err, errCanonicalCheckpointMismatch) || errors.Is(err, errCanonicalCheckpointIncomplete) {
		r.invalid = err
	}
	return err
}

func (r *canonicalSourceReadiness) read(ctx context.Context) error {
	if _, err := r.head.Head(ctx); err != nil {
		return err
	}
	watermark, err := r.store.GetCanonicalWatermark(ctx, 125, r.stream)
	if err != nil {
		return err
	}
	return verifyCanonicalRuntimeCheckpoint(ctx, r.pool, r.provider, r.stream, watermark, r.doc)
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
