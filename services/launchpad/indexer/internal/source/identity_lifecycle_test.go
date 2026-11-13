package source

import (
	"context"
	"math"
	"testing"
	"time"
)

func TestCommittedBlockReadsRequireAvailableUnfencedIdentity(t *testing.T) {
	ctx := context.Background()
	node := repair997Serve(t)
	s := repair997Open(t, fopts(node.server.URL))
	if _, err := s.BlockAt(ctx, math.MaxUint64); err == nil {
		t.Fatal("unbounded height admitted")
	}
	if record, err := s.BlockAt(ctx, 128); err != nil || record.Hash != committedFixtureHash(128) {
		t.Fatalf("committed block %+v: %v", record, err)
	}
	node.invalid.Store(true)
	if _, err := s.BlockAt(ctx, 129); err == nil {
		t.Fatal("invalid block admitted")
	}
	node.invalid.Store(false)
	node.conflicting.Store(true)
	if _, err := s.BlockAt(ctx, 128); err == nil {
		t.Fatal("changed accepted identity admitted")
	}
	if _, err := s.BlockAt(ctx, 129); err == nil {
		t.Fatal("quarantined endpoint served block")
	}
	if _, err := s.FetchCommitted(ctx, FetchOptions{FromBlock: 129, ToBlock: 129}, false, nil); err == nil {
		t.Fatal("quarantined endpoint served range")
	}
	if _, err := s.ObserveHead(ctx, 128); err == nil {
		t.Fatal("quarantined endpoint decided head")
	}
	if err := s.RejoinEndpoint(ctx, "absent"); err == nil {
		t.Fatal("unknown endpoint recovered")
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.RejoinEndpoint(ctx, s.endpoints[0].id); err == nil {
		t.Fatal("closed source recovered")
	}
}

func TestRecoveryRejectsIdentityChangesWhileVerifyingAcceptedBlocks(t *testing.T) {
	for _, mode := range []string{"endpoint generation", "accepted history"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			node := repair997Serve(t)
			s := repair997Open(t, fopts(node.server.URL))
			repair997Seed(t, s)
			endpoint := s.endpoints[0]
			s.mu.Lock()
			endpoint.quarantined = true
			endpoint.health.QuarantineReason = string(TimeoutError)
			s.mu.Unlock()
			node.gate.Store(true)
			released := false
			defer func() {
				if !released {
					close(node.release)
				}
			}()
			finished := make(chan error, 1)
			go func() { finished <- s.RejoinEndpoint(ctx, endpoint.id) }()
			select {
			case <-node.entered:
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			if err := s.RejoinEndpoint(ctx, endpoint.id); err == nil {
				t.Fatal("concurrent rejoin admitted")
			}
			if mode == "endpoint generation" {
				s.mu.Lock()
				s.disagreeLocked(endpoint, 128, committedFixtureHash(128), committedFixtureHash(129))
				s.mu.Unlock()
			} else {
				if err := s.retainBlock(endpoint, BlockRecord{ChainID: 125, Height: 129, Hash: committedFixtureHash(129), ParentHash: committedFixtureHash(128), TimestampSeconds: 129, SourceID: endpoint.id}); err != nil {
					t.Fatal(err)
				}
			}
			close(node.release)
			released = true
			select {
			case err := <-finished:
				repair997Kind(t, err, IntegrityError)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			if s.EndpointHealth()[0].QuarantineReason == "" {
				t.Fatal("changing recovery snapshot admitted")
			}
		})
	}
}

func TestCommittedConfigurationPreservesTransportAndEpochBounds(t *testing.T) {
	node := repair997Serve(t)
	for _, tc := range []struct {
		name string
		edit func(*CommittedOptions)
	}{
		{"live deadline", func(o *CommittedOptions) { o.LiveDeadline = 3 * time.Second }},
		{"bulk deadline", func(o *CommittedOptions) { o.BulkDeadline = 16 * time.Second }},
		{"response bound", func(o *CommittedOptions) { o.MaxResponseBytes = 33 << 20 }},
		{"epoch manifest", func(o *CommittedOptions) { o.Epochs[0].ManifestDigest = "other" }},
		{"source identifier", func(o *CommittedOptions) { o.Endpoints = map[string]string{"bad id": node.server.URL} }},
		{"duplicate archive", func(o *CommittedOptions) { o.HistoricalEndpoints = map[string]string{"fixture": node.server.URL} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			o := fopts(node.server.URL)
			tc.edit(&o)
			if s, err := NewCommittedSource(context.Background(), o); err == nil {
				_ = s.Close()
				t.Fatal("unbounded source configuration admitted")
			}
		})
	}
}
