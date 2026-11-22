package source

import (
	"context"
	"sync"
	"testing"
	"time"
)

func rejoinSnapshotStart(t *testing.T, s *CommittedSource, node *sourceRecoveryNode) (*committedEndpoint, <-chan error, func()) {
	t.Helper()
	var endpoint *committedEndpoint
	s.mu.Lock()
	for _, candidate := range s.endpoints {
		if candidate.id == "fixture" {
			endpoint = candidate
			break
		}
	}
	if endpoint != nil {
		endpoint.quarantined = true
		endpoint.health.QuarantineReason = string(TimeoutError)
	}
	s.mu.Unlock()
	if endpoint == nil {
		t.Fatal("rejoin endpoint absent")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	var once sync.Once
	release := func() { once.Do(func() { close(node.release) }) }
	t.Cleanup(func() { release(); cancel() })
	node.gate.Store(true)
	finished := make(chan error, 1)
	go func() { finished <- s.RejoinEndpoint(ctx, endpoint.id) }()
	for i := 0; i < cap(endpoint.bulkPermits); i++ {
		select {
		case <-node.entered:
		case err := <-finished:
			t.Fatalf("rejoin ended before bulk permits filled: %v", err)
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	if len(endpoint.bulkPermits) != cap(endpoint.bulkPermits) || len(endpoint.permits) != cap(endpoint.bulkPermits) {
		t.Fatalf("rejoin permits: bulk=%d requests=%d", len(endpoint.bulkPermits), len(endpoint.permits))
	}
	select {
	case <-node.entered:
		t.Fatal("rejoin exceeded bulk permits")
	default:
	}
	return endpoint, finished, release
}

func rejoinSnapshotWait(t *testing.T, finished <-chan error) error {
	t.Helper()
	select {
	case err := <-finished:
		return err
	case <-time.After(5 * time.Second):
		t.Fatal("rejoin did not finish")
		return nil
	}
}

func TestRejoinSnapshotAcceptsConcurrentHistory(t *testing.T) {
	node, healthy := sourceRecoveryServe(t), sourceRecoveryServe(t)
	options := fopts(node.server.URL)
	options.Endpoints["healthy"] = healthy.server.URL
	s := sourceRecoveryOpen(t, options)
	sourceRecoverySeed(t, s)
	before := node.requests.Load()
	endpoint, finished, release := rejoinSnapshotStart(t, s, node)
	s.mu.Lock()
	generation := endpoint.generation
	s.mu.Unlock()
	for height := uint64(129); height <= 144; height++ {
		record, err := s.BlockAt(context.Background(), height)
		if err != nil || record.Height != height || record.Hash != committedFixtureHash(height) || record.SourceID != "healthy" {
			t.Fatalf("concurrent accepted block: %+v %v", record, err)
		}
	}
	release()
	if err := rejoinSnapshotWait(t, finished); err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if endpoint.quarantined || endpoint.recovering || endpoint.generation != generation+1 || endpoint.health.RecoveryChecks != 128 {
		t.Fatalf("rejoined endpoint: %+v", endpoint)
	}
	if len(s.accepted) != 144 || s.accepted[144] != committedFixtureHash(144) {
		t.Fatal("rejoin changed accepted history")
	}
	if reads := node.requests.Load() - before; reads != 130 {
		t.Fatalf("identity admission and snapshot requests=%d, want 130", reads)
	}
}

func TestRejoinSnapshotRejectsChangedOrRemovedEntries(t *testing.T) {
	for _, mode := range []string{"changed", "removed"} {
		t.Run(mode, func(t *testing.T) {
			node := sourceRecoveryServe(t)
			s := sourceRecoveryOpen(t, fopts(node.server.URL))
			sourceRecoverySeed(t, s)
			endpoint, finished, release := rejoinSnapshotStart(t, s, node)
			s.mu.Lock()
			generation := endpoint.generation
			if mode == "changed" {
				s.accepted[128] = committedFixtureHash(129)
			} else {
				delete(s.accepted, 128)
			}
			s.mu.Unlock()
			release()
			sourceRecoveryKind(t, rejoinSnapshotWait(t, finished), IntegrityError)
			s.mu.Lock()
			defer s.mu.Unlock()
			if !endpoint.quarantined || endpoint.recovering || endpoint.generation != generation || endpoint.health.RecoveryChecks != 128 {
				t.Fatalf("snapshot entry refusal: %+v", endpoint)
			}
		})
	}
}

func TestRejoinSnapshotRejectsReadFailures(t *testing.T) {
	for _, mode := range []string{"hash", "rpc"} {
		t.Run(mode, func(t *testing.T) {
			node := sourceRecoveryServe(t)
			s := sourceRecoveryOpen(t, fopts(node.server.URL))
			sourceRecoverySeed(t, s)
			endpoint, finished, release := rejoinSnapshotStart(t, s, node)
			want := IntegrityError
			if mode == "hash" {
				node.conflicting.Store(true)
			} else {
				node.pruned.Store(true)
				want = RPCError
			}
			release()
			sourceRecoveryKind(t, rejoinSnapshotWait(t, finished), want)
			s.mu.Lock()
			defer s.mu.Unlock()
			if !endpoint.quarantined || endpoint.recovering || endpoint.health.RecoveryChecks >= 128 {
				t.Fatalf("read failure recovery: %+v", endpoint)
			}
			if len(endpoint.bulkPermits) != 0 || len(endpoint.permits) != 0 {
				t.Fatal("rejoin retained request permits")
			}
		})
	}
}

func TestRejoinSnapshotCompletesWithinBulkDeadline(t *testing.T) {
	node := sourceRecoveryServe(t)
	options := fopts(node.server.URL)
	options.BulkDeadline = 15 * time.Second
	s := sourceRecoveryOpen(t, options)
	sourceRecoverySeed(t, s)
	endpoint := s.endpoints[0]
	s.mu.Lock()
	endpoint.quarantined = true
	endpoint.health.QuarantineReason = string(TimeoutError)
	generation := endpoint.generation
	s.mu.Unlock()
	before := node.requests.Load()
	node.maxInFlight.Store(0)
	node.delay.Store(int64(250 * time.Millisecond))
	started := time.Now()
	if err := s.RejoinEndpoint(context.Background(), endpoint.id); err != nil {
		t.Fatal(err)
	}
	if elapsed := time.Since(started); elapsed >= 15*time.Second {
		t.Fatalf("rejoin elapsed %s, deadline 15s", elapsed)
	}
	if reads := node.requests.Load() - before; reads != 130 {
		t.Fatalf("identity admission and snapshot requests=%d, want 130", reads)
	}
	if peak := node.maxInFlight.Load(); peak <= 1 || peak > int64(cap(endpoint.bulkPermits)) {
		t.Fatalf("parallel requests=%d, bulk permits=%d", peak, cap(endpoint.bulkPermits))
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if endpoint.quarantined || endpoint.recovering || endpoint.health.RecoveryChecks != 128 || endpoint.generation != generation+1 {
		t.Fatalf("delayed endpoint recovery: %+v", endpoint)
	}
	if len(endpoint.bulkPermits) != 0 || len(endpoint.permits) != 0 {
		t.Fatal("rejoin retained request permits")
	}
}
