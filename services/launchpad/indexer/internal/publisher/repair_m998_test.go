package publisher_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/Sidiora-Technologies/KindleLaunch/core/indexer/internal/publisher"
)

func TestRepairM_998_CompatibilityConfigurationRefused(t *testing.T) {
	var requests atomic.Int64
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer receiver.Close()

	for _, urls := range [][]string{{receiver.URL}, {receiver.URL, receiver.URL}, {""}} {
		for _, secrets := range [][]string{nil, {secret}, {secret, "other-secret"}} {
			targets, err := publisher.BuildTargets(urls, secret, secrets)
			if !errors.Is(err, publisher.ErrLegacyWebhookRetired) || targets != nil {
				t.Fatalf("BuildTargets accepted retired configuration: targets=%v err=%v", targets, err)
			}
		}
	}
	for _, opts := range []publisher.Options{
		{Targets: []publisher.Target{{URL: receiver.URL, Secret: secret}}},
		{Targets: []publisher.Target{{URL: receiver.URL}}, Attempts: 100},
		{Targets: []publisher.Target{{}}},
	} {
		assertLegacyWebhookRefused(t, opts)
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("retired configuration reached the real receiver %d times", got)
	}
}

func TestRepairM_998_RetirementSurvivesRestart(t *testing.T) {
	var requests atomic.Int64
	receiver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		// A missing target acknowledgement must never be hidden in a volatile
		// queue or treated as durable acceptance by the retired mode.
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer receiver.Close()
	opts := publisher.Options{Targets: []publisher.Target{{URL: receiver.URL, Secret: secret}}, Attempts: 1}

	for restart := 0; restart < 3; restart++ {
		// Both the application configuration path and direct construction must
		// refuse the mode before a processor can advance its source cursor.
		targets, err := publisher.BuildTargets([]string{receiver.URL}, secret, nil)
		if targets != nil || !errors.Is(err, publisher.ErrLegacyWebhookRetired) {
			t.Fatalf("restart %d revived compatibility delivery: %v, %v", restart, targets, err)
		}
		assertLegacyWebhookRefused(t, opts)

		// Stop and reopen the retained realtime component. Replaying the old
		// batch cannot revive compatibility delivery, even without Redis dedup.
		p := publisher.New(publisher.Options{})
		if got := p.Snapshot(); got != (publisher.Metrics{}) {
			t.Fatalf("restart %d recovered phantom webhook work: %+v", restart, got)
		}
		p.PublishBatch(context.Background(), sampleEvents())
		p.PublishBatch(context.Background(), sampleEvents())
		p.Disconnect()
		if got := p.Snapshot(); got != (publisher.Metrics{}) {
			t.Fatalf("restart %d accepted retired work: %+v", restart, got)
		}
		if p.DeadLetterCount() != 0 || len(p.DeadLetterQueue()) != 0 || len(p.DrainDeadLetterQueue()) != 0 {
			t.Fatalf("restart %d retained unacknowledged compatibility work in memory", restart)
		}
	}
	if got := requests.Load(); got != 0 {
		t.Fatalf("restart or replay revived retired target delivery: %d requests", got)
	}
}

func TestRepairM_998_RealtimeOnlyConfigurationRemainsAvailable(t *testing.T) {
	for _, urls := range [][]string{nil, {}} {
		targets, err := publisher.BuildTargets(urls, "unused-secret", []string{"unused-per-url-secret"})
		if err != nil || len(targets) != 0 {
			t.Fatalf("realtime-only startup refused: %v, %v", targets, err)
		}
		p := publisher.New(publisher.Options{Targets: targets})
		p.PublishBatch(context.Background(), nil)
		p.SetBackfillMode(true)
		p.PublishBatch(context.Background(), sampleEvents())
		p.SetBackfillMode(false)
		p.PublishBatch(context.Background(), sampleEvents())
		p.Disconnect()
		if got := p.Snapshot(); got != (publisher.Metrics{}) {
			t.Fatalf("no-transport publisher claims delivery: %+v", got)
		}
	}
}
