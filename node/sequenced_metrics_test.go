package app

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.opentelemetry.io/otel/attribute"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/metricdata"
)

func installSequencedMetricsReader(t *testing.T) *sdkmetric.ManualReader {
	t.Helper()
	reader := sdkmetric.NewManualReader()
	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(reader))
	previous := sequencedMetrics
	t.Cleanup(func() {
		sequencedMetrics = previous
		require.NoError(t, provider.Shutdown(context.Background()))
	})
	sequencedMetrics = newSequencedMetrics(provider.Meter("app"))
	return reader
}

func collectSequencedCounter(t *testing.T, reader *sdkmetric.ManualReader, name string) metricdata.Sum[int64] {
	t.Helper()
	var data metricdata.ResourceMetrics
	require.NoError(t, reader.Collect(context.Background(), &data))
	var found []metricdata.Metrics
	for _, scope := range data.ScopeMetrics {
		for _, measurement := range scope.Metrics {
			if measurement.Name == name {
				require.Equal(t, "app", scope.Scope.Name)
				found = append(found, measurement)
			}
		}
	}
	require.Len(t, found, 1, "expected exactly one counter named %s", name)
	require.Equal(t, "{count}", found[0].Unit)
	sum, ok := found[0].Data.(metricdata.Sum[int64])
	require.True(t, ok, "%s must be an Int64Counter", name)
	require.True(t, sum.IsMonotonic)
	require.Equal(t, metricdata.CumulativeTemporality, sum.Temporality)
	return sum
}

func TestSequencedMetricsRejectedCounter(t *testing.T) {
	reader := installSequencedMetricsReader(t)
	ctx := context.Background()
	reasons := []struct {
		value string
		want  string
	}{
		{sequencedRejectNoBatch, "no_batch"},
		{sequencedRejectBatchDecode, "batch_decode"},
		{sequencedRejectBatchTxs, "batch_txs"},
		{sequencedRejectOrder, "order"},
		{sequencedRejectProposer, "proposer"},
		{"custom_reason", "custom_reason"},
	}
	for _, reason := range reasons {
		require.Equal(t, reason.want, reason.value)
	}
	for count := int64(1); count <= 2; count++ {
		for _, reason := range reasons {
			recordSequencedReject(ctx, reason.value)
		}
		sum := collectSequencedCounter(t, reader, "app_sequenced_proposal_rejected")
		require.Len(t, sum.DataPoints, len(reasons))
		got := make(map[string]int64, len(reasons))
		for _, point := range sum.DataPoints {
			require.Equal(t, 1, point.Attributes.Len())
			reason, ok := point.Attributes.Value(attribute.Key("reason"))
			require.True(t, ok, "rejection counter must carry the reason attribute")
			require.Equal(t, attribute.STRING, reason.Type())
			_, duplicate := got[reason.AsString()]
			require.False(t, duplicate, "duplicate reason data point")
			got[reason.AsString()] = point.Value
		}
		for _, reason := range reasons {
			value, ok := got[reason.want]
			require.True(t, ok, "missing reason %s", reason.want)
			require.Equal(t, count, value, "reason %s must increment by one per call", reason.want)
		}
	}
}

func TestSequencedMetricsHaltCounter(t *testing.T) {
	reader := installSequencedMetricsReader(t)
	ctx := context.Background()
	for count := int64(1); count <= 2; count++ {
		recordSequencedHalt(ctx)
		sum := collectSequencedCounter(t, reader, "app_sequenced_block_halt")
		require.Len(t, sum.DataPoints, 1)
		require.Equal(t, count, sum.DataPoints[0].Value)
		require.Zero(t, sum.DataPoints[0].Attributes.Len())
	}
}
