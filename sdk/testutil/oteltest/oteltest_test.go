package oteltest_test

import (
	"context"
	"testing"
	"time"

	metrics "github.com/armon/go-metrics"
	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"

	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/telemetry"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/oteltest"
)

const beforeMeterName = "oteltest.before_install"

var (
	beforeMeter                               = otel.Meter(beforeMeterName)
	beforeHistogram, beforeHistogramErr       = beforeMeter.Float64Histogram("duration")
	beforeIntHistogram, beforeIntHistogramErr = beforeMeter.Int64Histogram("size")
	beforeCounter, beforeCounterErr           = beforeMeter.Int64Counter("events")
	beforeGauge, beforeGaugeErr               = beforeMeter.Int64Gauge("height")
)

func TestReaderSeesInstrumentCreatedBeforeInstall(t *testing.T) {
	require.NoError(t, beforeHistogramErr)
	require.NoError(t, beforeIntHistogramErr)
	require.NoError(t, beforeCounterErr)
	require.NoError(t, beforeGaugeErr)

	otherMeter := otel.Meter("oteltest.other")
	otherHistogram, err := otherMeter.Float64Histogram("duration")
	require.NoError(t, err)
	otherCounter, err := otherMeter.Int64Counter("events")
	require.NoError(t, err)
	otherGauge, err := otherMeter.Int64Gauge("height")
	require.NoError(t, err)

	reader := oteltest.Install()
	require.Same(t, reader, oteltest.Install())
	provider := otel.GetMeterProvider()
	ctx := context.Background()
	first := metric.WithAttributes(attribute.String("partition", "first"))
	second := metric.WithAttributes(attribute.String("partition", "second"))

	require.Zero(t, reader.HistogramCount(beforeMeterName, "missing"))
	require.Zero(t, reader.CounterSum(beforeMeterName, "missing"))
	value, found := reader.GaugeInt(beforeMeterName, "missing")
	require.False(t, found)
	require.Zero(t, value)

	beforeHistogram.Record(ctx, 0.25, first)
	beforeHistogram.Record(ctx, 0.5, second)
	beforeIntHistogram.Record(ctx, 10, first)
	beforeIntHistogram.Record(ctx, 20, second)
	beforeCounter.Add(ctx, 3, first)
	beforeCounter.Add(ctx, 5, second)
	beforeGauge.Record(ctx, 0, first)
	value, found = reader.GaugeInt(beforeMeterName, "height")
	require.True(t, found)
	require.Zero(t, value)
	beforeGauge.Record(ctx, 7, second)
	otherHistogram.Record(ctx, 1)
	otherCounter.Add(ctx, 100)
	otherGauge.Record(ctx, 100)

	require.Equal(t, uint64(2), reader.HistogramCount(beforeMeterName, "duration"))
	require.Equal(t, uint64(2), reader.HistogramCount(beforeMeterName, "size"))
	require.Equal(t, int64(8), reader.CounterSum(beforeMeterName, "events"))
	value, found = reader.GaugeInt(beforeMeterName, "height")
	require.True(t, found)
	require.Equal(t, int64(7), value)

	require.Same(t, reader, oteltest.Install())
	require.Same(t, provider, otel.GetMeterProvider())
	beforeHistogram.Record(ctx, 0.75, first)
	beforeCounter.Add(ctx, 2, second)
	require.Equal(t, uint64(3), reader.HistogramCount(beforeMeterName, "duration"))
	require.Equal(t, int64(10), reader.CounterSum(beforeMeterName, "events"))
	require.Zero(t, reader.HistogramCount("oteltest.absent", "duration"))
	require.Zero(t, reader.CounterSum("oteltest.absent", "events"))
	value, found = reader.GaugeInt("oteltest.absent", "height")
	require.False(t, found)
	require.Zero(t, value)
}

func TestLegacySinkCapturesTelemetryCounter(t *testing.T) {
	var sink *oteltest.LegacySink
	expected := []string{"oteltest.legacy.counter", "oteltest.legacy.gauge", "oteltest.legacy.sample"}
	t.Run("capture", func(t *testing.T) {
		sink = oteltest.InstallLegacySink(t)
		require.Empty(t, sink.Keys())
		telemetry.IncrCounter(1, "legacy", "counter")
		require.Equal(t, []string{"oteltest.legacy.counter"}, sink.Keys())
		telemetry.IncrCounterWithLabels([]string{"legacy", "counter"}, 2,
			[]metrics.Label{telemetry.NewLabel("partition", "first")})
		telemetry.SetGauge(7, "legacy", "gauge")
		telemetry.MeasureSince(time.Now().Add(-time.Millisecond), "legacy", "sample")
		require.Equal(t, expected, sink.Keys())
	})
	require.NotNil(t, sink)
	telemetry.IncrCounter(1, "after", "cleanup")
	require.Equal(t, expected, sink.Keys())
}
