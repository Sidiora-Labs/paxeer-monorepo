package sequencer_test

import (
	"context"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/stretchr/testify/require"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func newSequencerMetricsReader(t *testing.T) (*sequencer.Metrics, *sdkmetric.ManualReader) {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	t.Cleanup(func() {
		require.NoError(t, provider.Shutdown(context.Background()))
	})
	m, err := sequencer.NewMetrics(provider.Meter("sequencer"))
	require.NoError(t, err)
	require.NotNil(t, m)
	return m, reader
}

func collectSequencerMetrics(t *testing.T, reader *sdkmetric.ManualReader) map[string]metricdata.Metrics {
	t.Helper()
	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &data))
	require.Len(t, data.ScopeMetrics, 1)
	scope := data.ScopeMetrics[0]
	require.Equal(t, "sequencer", scope.Scope.Name)
	metrics := make(map[string]metricdata.Metrics, len(scope.Metrics))
	for _, m := range scope.Metrics {
		require.NotContains(t, metrics, m.Name)
		metrics[m.Name] = m
	}
	return metrics
}

func TestSequencerMetricsInstrumentNames(t *testing.T) {
	m, reader := newSequencerMetricsReader(t)
	m.ObserveReceipt(250 * time.Millisecond)
	m.ObserveReceipt(500 * time.Millisecond)
	m.ObserveWALSync(125 * time.Microsecond)
	m.ObserveStandbyAck(1500 * time.Millisecond)

	metrics := collectSequencerMetrics(t, reader)
	require.Len(t, metrics, 5)
	for _, want := range []struct {
		name  string
		count uint64
		sum   float64
	}{
		{"sequencer_receipt_latency_seconds", 2, 0.75},
		{"sequencer_wal_sync_seconds", 1, 0.000125},
		{"sequencer_standby_ack_seconds", 1, 1.5},
	} {
		require.Contains(t, metrics, want.name)
		instrument := metrics[want.name]
		require.Equal(t, "s", instrument.Unit, want.name)
		histogram, ok := instrument.Data.(metricdata.Histogram[float64])
		require.True(t, ok, "%s must be a Float64Histogram", want.name)
		require.Len(t, histogram.DataPoints, 1, want.name)
		require.Equal(t, want.count, histogram.DataPoints[0].Count, want.name)
		require.InDelta(t, want.sum, histogram.DataPoints[0].Sum, 1e-12, want.name)
	}
	for _, name := range []string{"sequencer_open_seq", "sequencer_sealed_height"} {
		require.Contains(t, metrics, name)
		gauge, ok := metrics[name].Data.(metricdata.Gauge[int64])
		require.True(t, ok, "%s must be an Int64ObservableGauge", name)
		require.Len(t, gauge.DataPoints, 1, name)
		require.Zero(t, gauge.DataPoints[0].Value, name)
	}
}

func TestSequencerMetricsRecordOpenSeqAndSealedHeight(t *testing.T) {
	m, reader := newSequencerMetricsReader(t)
	assertValues := func(seq, height int64) {
		t.Helper()
		metrics := collectSequencerMetrics(t, reader)
		for name, want := range map[string]int64{
			"sequencer_open_seq":      seq,
			"sequencer_sealed_height": height,
		} {
			require.Contains(t, metrics, name)
			gauge, ok := metrics[name].Data.(metricdata.Gauge[int64])
			require.True(t, ok, "%s must be an Int64ObservableGauge", name)
			require.Len(t, gauge.DataPoints, 1, name)
			require.Equal(t, want, gauge.DataPoints[0].Value, name)
		}
	}

	assertValues(0, 0)
	m.SetOpenSeq(41)
	m.SetOpenSeq(42)
	m.SetSealedHeight(99)
	m.SetSealedHeight(100)
	assertValues(42, 100)
	assertValues(42, 100)

	m.SetOpenSeq(7)
	assertValues(7, 100)
	m.SetSealedHeight(-1)
	assertValues(7, -1)
	m.SetOpenSeq(0)
	m.SetSealedHeight(0)
	assertValues(0, 0)
}
