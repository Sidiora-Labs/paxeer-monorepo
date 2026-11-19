package processor

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/publisher"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/receipt"
	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/store"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/delivery"
)

type standbyStore struct {
	ProcessorStore
	mu              sync.Mutex
	held            bool
	fence           int64
	watermark       int64
	acquireAttempts int
	commits         int
	committedFrom   int64
	firstAcquire    chan struct{}
	committed       chan struct{}
	cancel          context.CancelFunc
}

func (s *standbyStore) AcquireLease(context.Context, int32, string, string, time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.acquireAttempts++
	if s.acquireAttempts == 1 && s.firstAcquire != nil {
		close(s.firstAcquire)
		s.firstAcquire = nil
	}
	if s.held {
		return 0, store.ErrFenced
	}
	s.held = true
	s.fence++
	return s.fence, nil
}

func (s *standbyStore) RenewLease(context.Context, int32, string, string, int64, time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.held {
		return store.ErrFenced
	}
	return nil
}

func (s *standbyStore) ReleaseLease(context.Context, int32, string, string, int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.held = false
	return nil
}

func (s *standbyStore) GetCanonicalWatermark(context.Context, int32, string) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.watermark, nil
}

func (s *standbyStore) CanonicalEpochGuarded() bool { return true }

func (s *standbyStore) CommitProjectedCanonicalTx(_ context.Context, b store.CanonicalBatch, _ func(context.Context, store.CanonicalTx) ([]store.OutboxIntent, error)) error {
	s.mu.Lock()
	s.commits++
	s.committedFrom = b.PreviousHeight
	s.watermark = b.Blocks[len(b.Blocks)-1].Height
	if s.committed != nil {
		close(s.committed)
		s.committed = nil
	}
	cancel := s.cancel
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	return nil
}

type standbyBatchSource struct{}

func (standbyBatchSource) FetchQualifiedBatch(_ context.Context, from, to uint64) (store.CanonicalBatch, []*receipt.QualifiedReceipt, error) {
	return store.CanonicalBatch{ChainID: 125, Blocks: []store.BlockCompleteness{{Height: int64(from), Hash: "0x" + strings.Repeat("1", 64), RawCount: 0, FetchComplete: true, RequiredDecodeComplete: true}}}, nil, nil
}

type standbyHead struct{}

func (standbyHead) Head(context.Context) (int64, error) { return 101, nil }
func (standbyHead) Name() string                        { return "standby-test-head" }
func (standbyHead) Close() error                        { return nil }

func TestLeaseStandbyWaitsThenCommitsFromDurableWatermark(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	firstAcquire := make(chan struct{})
	committed := make(chan struct{})
	st := &standbyStore{held: true, watermark: 90, firstAcquire: firstAcquire, committed: committed, cancel: cancel}
	p := New(Deps{
		Store: st, QualifiedBatchSource: standbyBatchSource{}, CanonicalStreamID: "live", CanonicalOwner: "standby",
		ChainID: 125, BatchSize: 1, PollInterval: 5 * time.Millisecond, Head: standbyHead{},
		AccountingPartitions: &delivery.PartitionStore{}, Durable: &publisher.DurableStager{}, AccountingProjectionVersion: "v1",
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})
	done := make(chan error, 1)
	go func() { done <- p.Run(ctx) }()
	select {
	case <-firstAcquire:
	case <-time.After(time.Second):
		t.Fatal("standby did not attempt lease acquisition")
	}
	if got := p.LeaseState(); got != "standby" {
		t.Fatalf("waiting processor state = %q, want standby", got)
	}
	st.mu.Lock()
	if st.commits != 0 {
		t.Fatalf("standby committed %d batches", st.commits)
	}
	st.watermark = 100
	st.held = false
	st.mu.Unlock()
	select {
	case <-committed:
	case <-time.After(2 * time.Second):
		t.Fatal("standby did not commit after lease takeover")
	}
	if err := <-done; !errors.Is(err, context.Canceled) {
		t.Fatalf("processor exit = %v, want context cancellation", err)
	}
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.commits != 1 || st.committedFrom != 100 {
		t.Fatalf("commit count/from = %d/%d, want 1/100", st.commits, st.committedFrom)
	}
}
