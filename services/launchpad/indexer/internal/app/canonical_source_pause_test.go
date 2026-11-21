package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestCanonicalSourceReadinessRecoversWhileHistoricalPaused(t *testing.T) {
	r, clock := canonicalSourceReadinessCase(t)
	transport := canonicalReadinessRecordedTransport(t, r)
	transport.head.Store(r.doc.InitialCheckpointHeight + 3)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	height := uint64(r.doc.InitialCheckpointHeight)
	if _, err := r.provider.ObserveHead(ctx, height); err != nil {
		t.Fatal(err)
	}
	block, err := r.provider.CheckpointBlockAt(ctx, height)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := r.pool.Exec(ctx, `INSERT INTO indexer.indexer_block_completeness(chain_id,stream_id,height,block_hash,raw_count,fetch_complete,required_decode_complete) VALUES(125,$1,$2,$3,0,true,true)`, r.stream, r.doc.InitialCheckpointHeight, "0x"+strings.Repeat("b", 64)); err != nil {
		t.Fatal(err)
	}
	start := time.Now().Add(-3 * time.Minute)
	for _, at := range []time.Duration{0, 30 * time.Second} {
		if err := r.provider.ReportLiveLag(start.Add(at), 3*time.Second); err != nil {
			t.Fatal(err)
		}
	}
	historical, stopHistorical := context.WithCancel(context.Background())
	defer stopHistorical()
	result := make(chan error, 1)
	go func() { _, err := r.provider.BlockAt(historical, height); result <- err }()
	for r.provider.TransportMetrics().HistoricalQueueDepth != 1 {
		select {
		case err := <-result:
			t.Fatalf("historical read returned during pause: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Millisecond):
		}
	}
	if err := r.refresh(ctx); !errors.Is(err, errCanonicalCheckpointMismatch) {
		t.Fatalf("paused checkpoint refresh: %v", err)
	}
	if err := r.Ready(ctx); !errors.Is(err, errCanonicalCheckpointMismatch) {
		t.Fatalf("paused checkpoint readiness: %v", err)
	}
	if _, err := r.pool.Exec(ctx, `UPDATE indexer.indexer_block_completeness SET block_hash=$2 WHERE stream_id=$1 AND height=$3`, r.stream, strings.ToLower(block.Hash.Hex()), r.doc.InitialCheckpointHeight); err != nil {
		t.Fatal(err)
	}
	clock.Add(int64(time.Second))
	if err := r.refresh(ctx); err != nil {
		t.Fatalf("paused checkpoint recovery: %v", err)
	}
	if err := r.Ready(ctx); err != nil {
		t.Fatalf("recovered canonical source readiness: %v", err)
	}
	if err := r.provider.Ready(); err != nil {
		t.Fatalf("paused transport readiness: %v", err)
	}
	if metrics := r.provider.TransportMetrics(); !metrics.HistoricalPaused || metrics.HistoricalQueueDepth != 1 {
		t.Fatalf("refresh changed pause: %+v", metrics)
	}
	select {
	case err := <-result:
		t.Fatalf("historical read returned before release: %v", err)
	default:
	}
	for _, at := range []time.Duration{31 * time.Second, 90 * time.Second, 91 * time.Second} {
		if err := r.provider.ReportDurableHeight(start.Add(at), height); err != nil {
			t.Fatal(err)
		}
		if got, want := r.provider.TransportMetrics().HistoricalPaused, at < 91*time.Second; got != want {
			t.Fatalf("pause at %s: got %v want %v", at, got, want)
		}
	}
	select {
	case err := <-result:
		if err != nil {
			t.Fatalf("released historical read: %v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := r.refresh(ctx); err != nil {
		t.Fatalf("refresh after release: %v", err)
	}
	if err := r.Ready(ctx); err != nil {
		t.Fatalf("readiness after release: %v", err)
	}
}
