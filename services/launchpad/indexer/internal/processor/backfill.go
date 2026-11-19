package processor

import (
	"context"
	"errors"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
)

type Backfill struct {
	p         *Processor
	fromBlock *int64
	batchSize int
}

func NewBackfill(p *Processor, fromBlock *int64, batchSize int) *Backfill {
	if batchSize <= 0 {
		batchSize = 1000
	}
	return &Backfill{p: p, fromBlock: fromBlock, batchSize: batchSize}
}
func (b *Backfill) Run(ctx context.Context) error {
	if b == nil || b.p == nil || b.p.deps.HistoricalReplay == nil {
		return errors.New("backfill requires an authorized explicit historical replay request and real durable replay runner")
	}
	requests := b.p.deps.HistoricalRequests
	if b.p.deps.HistoricalRequest != nil {
		if len(requests) > 0 {
			return errors.New("backfill request sources are exclusive")
		}
		requests = []store.ReplayRequest{*b.p.deps.HistoricalRequest}
	}
	if len(requests) == 0 || len(requests) > 16 {
		return errors.New("bounded explicit backfill requests required")
	}
	for i, request := range requests {
		if err := store.ValidateReplayRequest(request); err != nil {
			return err
		}
		if b.p.deps.CanonicalStreamID != "" && request.SourceStream != b.p.deps.CanonicalStreamID {
			return errors.New("backfill source stream mismatch")
		}
		if request.Mode != valueBackfill {
			return errors.New("backfill request mode required")
		}
		if i == 0 && b.fromBlock != nil && request.FromHeight != *b.fromBlock {
			return errors.New("backfill requested range conflicts with configured operation")
		}
		if i > 0 && (request.FromHeight != requests[i-1].ToHeight+1 || request.SourceStream != requests[0].SourceStream || request.CheckpointHeight != requests[0].CheckpointHeight || request.CheckpointHash != requests[0].CheckpointHash || request.Target != requests[0].Target || request.Generation != requests[0].Generation || request.ProjectionRevision != requests[0].ProjectionRevision) {
			return errors.New("backfill requests must form one checkpoint-bound range")
		}
	}
	runner := *b.p.deps.HistoricalReplay
	if b.batchSize < 1 || b.batchSize > 1000 {
		return errors.New("backfill batch bound must be at most 1000")
	}
	runner.BatchSize = b.batchSize
	for _, request := range requests {
		var canonical *Processor
		if request.Target == valueIndexer {
			canonical = b.p
		}
		if err := runner.run(ctx, request, canonical); err != nil {
			return err
		}
	}
	return nil
}
