package pebbledb

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cockroachdb/pebble/v2"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/db_engine/types"
)

func TestRepairM_1176_TickOverlapsClose(t *testing.T) {
	entered := make(chan context.Context, 1)
	release := make(chan struct{})
	var enterOnce, releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithExemplarFilter(func(ctx context.Context) bool {
			// The SDK receives this measurement from the real periodic scrape,
			// after Pebble.Metrics has produced its snapshot.
			enterOnce.Do(func() {
				entered <- ctx
				<-release
			})
			return false
		}),
	)
	previous := otel.GetMeterProvider()
	otel.SetMeterProvider(provider)
	t.Cleanup(func() {
		otel.SetMeterProvider(previous)
		require.NoError(t, provider.Shutdown(context.Background()))
	})

	cfg := DefaultTestConfig(t)
	cfg.EnableMetrics = true
	cfg.MetricsScrapeInterval = time.Millisecond
	store, err := Open(t.Context(), &cfg)
	require.NoError(t, err)
	p := store.(*pebbleDB)
	t.Cleanup(func() {
		unblock()
		require.NoError(t, p.Close())
	})
	require.NotNil(t, p.metrics)
	raw := p.db
	require.NoError(t, store.Set([]byte("key"), []byte("value"), types.WriteOptions{Sync: true}))

	var scrapeCtx context.Context
	select {
	case scrapeCtx = <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("periodic collector did not reach the real OTel SDK")
	}

	const callers = 8
	results := make(chan error, callers)
	for i := 0; i < callers; i++ {
		go func() { results <- store.Close() }()
	}
	select {
	case <-scrapeCtx.Done():
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not cancel the active scrape")
	}
	// Leave another ticker event pending while the active tick is blocked.
	select {
	case err := <-results:
		t.Fatalf("Close returned before the active scrape finished: %v", err)
	case <-time.After(20 * time.Millisecond):
	}
	select {
	case <-p.metrics.done:
		t.Fatal("collector exited while its scrape was still blocked")
	default:
	}
	value, closer, err := raw.Get([]byte("key"))
	require.NoError(t, err, "Pebble must remain open until the scrape exits")
	require.Equal(t, "value", string(value))
	require.NoError(t, closer.Close())

	unblock()
	for i := 0; i < callers; i++ {
		select {
		case err := <-results:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("Close did not join the released collector")
		}
	}
	select {
	case <-p.metrics.done:
	default:
		t.Fatal("Close returned before collector termination")
	}
	require.Nil(t, p.db)
	require.PanicsWithError(t, pebble.ErrClosed.Error(), func() { _, _, _ = raw.Get([]byte("key")) })
	require.NoError(t, store.Close())
	p.metrics.Stop()

	var exported metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &exported))
	found := false
	for _, scope := range exported.ScopeMetrics {
		if scope.Scope.Name != pebbleMeterName {
			continue
		}
		for _, m := range scope.Metrics {
			if m.Name != "pebble_compaction_duration" {
				continue
			}
			found = true
			histogram, ok := m.Data.(metricdata.Histogram[float64])
			require.True(t, ok, "collector must export the real duration histogram")
			require.Len(t, histogram.DataPoints, 1)
			point := histogram.DataPoints[0]
			require.Equal(t, uint64(1), point.Count, "a queued tick must not scrape after cancellation")
			database, ok := point.Attributes.Value("db")
			require.True(t, ok)
			require.Equal(t, filepath.Base(cfg.DataDir), database.AsString())
		}
	}
	require.True(t, found, "the completed tick must reach the SDK reader")
}

func TestRepairM_1176_ParentCancellation(t *testing.T) {
	for _, cancelBeforeOpen := range []bool{false, true} {
		name := "after_open"
		if cancelBeforeOpen {
			name = "before_open"
		}
		t.Run(name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if cancelBeforeOpen {
				cancel()
			}
			cfg := DefaultTestConfig(t)
			cfg.EnableMetrics = true
			cfg.MetricsScrapeInterval = time.Millisecond
			store, err := Open(ctx, &cfg)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, store.Close()) })
			p := store.(*pebbleDB)
			cancel()
			select {
			case <-p.metrics.done:
			case <-time.After(5 * time.Second):
				t.Fatal("parent cancellation did not stop the collector")
			}
			p.metrics.Stop()
			p.metrics.Stop()
			require.NoError(t, store.Set([]byte("key"), []byte("value"), types.WriteOptions{}))
			value, err := store.Get([]byte("key"))
			require.NoError(t, err)
			require.Equal(t, []byte("value"), value)
			require.NoError(t, store.Close())
			require.NoError(t, store.Close())
		})
	}
}

func TestRepairM_1176_CloseWithoutMetrics(t *testing.T) {
	cfg := DefaultTestConfig(t)
	store, err := Open(t.Context(), &cfg)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, store.Close()) })
	p := store.(*pebbleDB)
	require.Nil(t, p.metrics)
	const callers = 8
	results := make(chan error, callers)
	for i := 0; i < callers; i++ {
		go func() { results <- store.Close() }()
	}
	for i := 0; i < callers; i++ {
		select {
		case err := <-results:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Fatal("Close blocked with metrics disabled")
		}
	}
	require.Nil(t, p.db)
	require.NoError(t, store.Close())
}
