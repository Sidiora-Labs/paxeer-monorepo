package source

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDurableHistoricalPauseFollowsSteadyBlockDistance(t *testing.T) {
	type observation struct {
		second   int
		distance uint64
		paused   bool
	}
	for _, tc := range []struct {
		name         string
		observations []observation
	}{
		{"steady", []observation{
			{0, 12, false}, {29, 12, false}, {30, 12, true},
			{31, 3, true}, {90, 3, true}, {91, 3, false},
		}},
		{"falling behind", []observation{
			{0, 12, false}, {30, 12, true}, {31, 3, true},
			{60, 5, true}, {61, 3, true}, {120, 6, true},
			{121, 3, true}, {180, 4, true}, {181, 4, false},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, client, endpoint := genuineHistoricalNode(t)
			genesis, err := client.HeaderByNumber(ctx, nil)
			if err != nil {
				t.Fatal(err)
			}
			opts := fopts(endpoint)
			opts.GenesisHash = genesis.Hash()
			s, err := NewCommittedSource(ctx, opts)
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			start := time.Now().Add(-5 * time.Minute)
			var pausedAt int
			for _, observation := range tc.observations {
				if err := client.Client().CallContext(ctx, nil, "anvil_mine", "0x10", "0x1"); err != nil {
					t.Fatal(err)
				}
				head, err := s.ObserveHead(ctx, 0)
				if err != nil {
					t.Fatal(err)
				}
				headBlock, err := s.CheckpointBlockAt(ctx, head.Height)
				if err != nil {
					t.Fatal(err)
				}
				durable := head.Height - observation.distance
				block, err := s.CheckpointBlockAt(ctx, durable)
				if err != nil {
					t.Fatal(err)
				}
				if headBlock.TimestampSeconds-block.TimestampSeconds != int64(observation.distance) {
					t.Fatalf("block timestamp distance: head=%d durable=%d", headBlock.TimestampSeconds, block.TimestampSeconds)
				}
				if err := s.ReportDurableHeight(start.Add(time.Duration(observation.second)*time.Second), durable); err != nil {
					t.Fatal(err)
				}
				metrics := s.TransportMetrics()
				if metrics.HistoricalPaused != observation.paused {
					t.Fatalf("second=%d distance=%d paused=%v", observation.second, observation.distance, metrics.HistoricalPaused)
				}
				if observation.paused && pausedAt == 0 {
					pausedAt = observation.second
				}
				if pausedAt != 0 && !observation.paused && metrics.PausedBackfillNanoseconds != int64(time.Duration(observation.second-pausedAt)*time.Second) {
					t.Fatalf("pause duration=%d", metrics.PausedBackfillNanoseconds)
				}
				if metrics.HistoricalQueueDepth != 0 || metrics.InFlightRequests != 0 || metrics.PendingRanges != 0 {
					t.Fatalf("completed observation retains work: %+v", metrics)
				}
			}
		})
	}
}

func TestCheckpointBlockAtWhileHistoricalPaused(t *testing.T) {
	server := fixture(t, "0x7d")
	s, err := NewCommittedSource(context.Background(), fopts(server.URL))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	start := time.Now().Add(-2 * time.Minute)
	for _, at := range []time.Duration{0, 30 * time.Second} {
		if err := s.ReportLiveLag(start.Add(at), 3*time.Second); err != nil {
			t.Fatal(err)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	historical, stopHistorical := context.WithCancel(context.Background())
	defer stopHistorical()
	result := make(chan error, 1)
	go func() { _, err := s.BlockAt(historical, 100); result <- err }()
	for s.TransportMetrics().HistoricalQueueDepth != 1 {
		select {
		case err := <-result:
			t.Fatalf("historical read returned during pause: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(time.Millisecond):
		}
	}
	block, err := s.CheckpointBlockAt(ctx, 100)
	if err != nil || block.Height != 100 || block.Hash != committedFixtureHash(100) {
		t.Fatalf("checkpoint=%+v: %v", block, err)
	}
	if err := s.Ready(); err != nil {
		t.Fatalf("source readiness while paused: %v", err)
	}
	if metrics := s.TransportMetrics(); !metrics.HistoricalPaused || metrics.HistoricalQueueDepth != 1 {
		t.Fatalf("checkpoint changed historical work: %+v", metrics)
	}
	select {
	case err := <-result:
		t.Fatalf("historical read returned during checkpoint: %v", err)
	default:
	}
	stopHistorical()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("historical cancellation: %v", err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if metrics := s.TransportMetrics(); metrics.HistoricalQueueDepth != 0 || metrics.PendingRanges != 0 {
		t.Fatalf("canceled work retained: %+v", metrics)
	}
	canceled, cancelCheckpoint := context.WithCancel(context.Background())
	cancelCheckpoint()
	if _, err := s.CheckpointBlockAt(canceled, 100); !errors.Is(err, context.Canceled) {
		t.Fatalf("checkpoint cancellation: %v", err)
	}
	if _, err := s.CheckpointBlockAt(ctx, ^uint64(0)); err == nil {
		t.Fatal("invalid checkpoint height accepted")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CheckpointBlockAt(ctx, 100); err == nil {
		t.Fatal("closed checkpoint source accepted a read")
	}
}
