package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

func TestPeerCallObservesRoundTrip(t *testing.T) {
	registries := make(map[string]*prometheus.Registry)
	tune := metricOptions(t, registries)
	c := newTestClusterWith(t, 2, false, testPolicy(), func(opts *Options) {
		if opts.NodeID == "node-1" {
			tune(opts)
		}
	})
	sender, receiver := c.nodes[0], c.nodes[1]
	registry := registries[sender.id]
	const keyID = "peer-rtt"
	// A third share satisfies the key threshold; only two peers exchange announcements.
	participants := []string{sender.id, receiver.id, "node-3"}
	bundles := fixedBundles(t, participants)
	account := secpAddress(bundles[0].PublicKey)
	for i, node := range c.nodes {
		bundle := EncodeBundle(bundles[i])
		if err := node.server.saveShare(keyID, bundles[i].Curve, bundles[i].PublicKey, 0, participants, storedShare{
			Owner: testOwner, Account: account, Bundle: &bundle,
		}); err != nil {
			t.Fatal(err)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	for i := 1; i <= 2; i++ {
		started := time.Now()
		acked, silent := sender.server.Announce(ctx, keyID, fmt.Sprintf("rtt-%d", i), account, c.ids, nil)
		elapsed := time.Since(started)
		if acked != 2 || len(silent) != 0 {
			t.Fatalf("announcement %d: acked %d, silent %v", i, acked, silent)
		}
		metric := gatheredMetric(t, registry, "attestor_peer_rtt_seconds", "peer", receiver.id)
		if metric.Histogram == nil || metric.GetHistogram().GetSampleCount() != uint64(i) || metric.GetHistogram().GetSampleSum() <= 0 {
			t.Fatalf("announcement %d: RTT = %v, want %d positive observations", i, metric, i)
		}
		if i == 1 && metric.GetHistogram().GetSampleSum() > elapsed.Seconds() {
			t.Fatalf("RTT %g seconds exceeds announcement duration %v", metric.GetHistogram().GetSampleSum(), elapsed)
		}
	}

	body, err := json.Marshal(Announcement{SessionID: "rtt-refused", KeyID: "missing-key", Account: account})
	if err != nil {
		t.Fatal(err)
	}
	if err := sender.server.peerCall(ctx, receiver.id, PathPeerAnnounce, body, false, nil); !errors.Is(err, errPeerRefused) {
		t.Fatalf("missing-key announcement: error %v, want peer refusal", err)
	}
	metric := gatheredMetric(t, registry, "attestor_peer_rtt_seconds", "peer", receiver.id)
	if metric.Histogram == nil || metric.GetHistogram().GetSampleCount() != 3 || metric.GetHistogram().GetSampleSum() <= 0 {
		t.Fatalf("refused announcement: RTT = %v, want three positive observations", metric)
	}
	sum := metric.GetHistogram().GetSampleSum()

	canceled, cancelRequest := context.WithCancel(ctx)
	cancelRequest()
	if err := sender.server.peerCall(canceled, receiver.id, PathPeerAnnounce, body, false, nil); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled announcement: error %v, want context cancellation", err)
	}
	metric = gatheredMetric(t, registry, "attestor_peer_rtt_seconds", "peer", receiver.id)
	if metric.GetHistogram().GetSampleCount() != 3 || metric.GetHistogram().GetSampleSum() != sum {
		t.Fatalf("transport error changed RTT observations: %v", metric)
	}

	if receiver.server.opts.Metrics != nil {
		t.Fatal("expected receiver without metrics")
	}
	if acked, silent := receiver.server.Announce(ctx, keyID, "rtt-no-metrics", account, c.ids, nil); acked != 2 || len(silent) != 0 {
		t.Fatalf("announcement without metrics: acked %d, silent %v", acked, silent)
	}
}
