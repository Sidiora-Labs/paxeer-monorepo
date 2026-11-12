package chain

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/profile"
	"github.com/Sidiora-Technologies/KindleLaunch/shared/chain/readpolicy"
)

func TestReadAdapterObservesCommittedHead(t *testing.T) {
	header := testHeader(100)
	var calls atomic.Int64
	server := pinnedServer(t, header, false, &calls)
	client, err := NewVerifiedClient(context.Background(), server.URL, "", VerifiedConfig{profile.Paxeer("e1", "d1"), 100, consensusHash(header)})
	if err != nil {
		t.Fatal(err)
	}
	adapter := &ReadAdapter{client: client, headPermit: make(chan struct{}, 1), historicalPermit: make(chan struct{}, 1)}
	defer adapter.Close()
	if _, ok := adapter.LastHead(); ok {
		t.Fatal("head observed before a read")
	}
	if adapter.Name() != "verified-paxeer-read" {
		t.Fatal("read adapter identity changed")
	}
	height, err := adapter.Head(context.Background())
	if err != nil || height != 100 {
		t.Fatalf("committed head = %d, %v", height, err)
	}
	last, ok := adapter.LastHead()
	if !ok || last.Height != 100 || last.Hash != consensusHash(header) || last.ParentHash != header.ParentHash || last.EndpointID != valuePrimary {
		t.Fatalf("last committed head = %+v, %t", last, ok)
	}
	if adapter.ReadStats().UpstreamGroups != 0 {
		t.Fatal("head observation counted as pooled snapshot")
	}
	ctx, cancel := context.WithCancel(context.Background())
	var delivered int
	err = adapter.PollHeads(ctx, func(_ context.Context, observation readpolicy.HeadObservation) error {
		if observation.Err != nil || observation.Height != 100 || observation.ObservedAt.IsZero() {
			t.Fatalf("polled head = %+v", observation)
		}
		delivered++
		cancel()
		return nil
	})
	if !errors.Is(err, context.Canceled) || delivered != 1 {
		t.Fatalf("head polling = %d deliveries, %v", delivered, err)
	}
	adapter.mu.Lock()
	adapter.lastHead = 101
	adapter.mu.Unlock()
	if _, err := adapter.ObserveHead(context.Background()); err == nil {
		t.Fatal("lagging committed head accepted")
	}
}
