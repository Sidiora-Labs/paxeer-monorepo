package telemetry

import (
	"bytes"
	"fmt"
	"testing"

	metrics "github.com/armon/go-metrics"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/require"
)

func TestRepairM_624_GlobalLabelValidation(t *testing.T) {
	m := repairM624Metrics(t, Config{
		Enabled:                 true,
		ServiceName:             "validation",
		PrometheusRetentionTime: 60,
		GlobalLabels:            [][]string{{"chain_id", "valid-chain"}},
	})
	previousMetrics := metrics.Default()
	previousLabels := append([]metrics.Label(nil), globalLabels...)

	for _, tc := range []struct {
		name string
		row  []string
	}{
		{name: "nil"},
		{name: "empty", row: []string{}},
		{name: "one_element", row: []string{"private-label-name"}},
		{name: "extra_element", row: []string{"private-label-name", "private-label-value", "private-extra-value"}},
	} {
		for _, rowIndex := range []int{0, 1} {
			t.Run(fmt.Sprintf("%s/row_%d", tc.name, rowIndex+1), func(t *testing.T) {
				rows := [][]string{{"unpublished", "private-valid-value"}, tc.row}
				if rowIndex == 0 {
					rows = rows[1:]
				}
				var rejected *Metrics
				var err error
				require.NotPanics(t, func() {
					rejected, err = New(Config{Enabled: true, GlobalLabels: rows})
				})
				require.Nil(t, rejected)
				require.EqualError(t, err, fmt.Sprintf("invalid telemetry configuration: global-labels row %d must contain exactly 2 elements", rowIndex+1))
				require.Same(t, previousMetrics, metrics.Default())
				require.Equal(t, previousLabels, globalLabels)
			})
		}
	}

	IncrCounter(4, "after_rejection")
	series := repairM624Gather(t, m)
	require.Contains(t, series, "validation_after_rejection")
	require.Len(t, series["validation_after_rejection"].Metric, 1)
	emitted := series["validation_after_rejection"].Metric[0]
	require.Equal(t, float64(4), emitted.GetCounter().GetValue())
	repairM624Labels(t, emitted, map[string]string{"chain_id": "valid-chain"})
}

func TestRepairM_624_ValidGlobalLabelPair(t *testing.T) {
	m := repairM624Metrics(t, Config{
		Enabled:                 true,
		ServiceName:             "valid",
		PrometheusRetentionTime: 60,
		GlobalLabels:            [][]string{{"chain_id", "chain-624"}, {"region", "local"}},
	})
	IncrCounter(2, "requests")
	IncrCounter(3, "requests")
	SetGauge(7, "height")
	series := repairM624Gather(t, m)
	for _, name := range []string{"valid_requests", "valid_height"} {
		require.Contains(t, series, name)
		require.Len(t, series[name].Metric, 1)
		repairM624Labels(t, series[name].Metric[0], map[string]string{"chain_id": "chain-624", "region": "local"})
	}
	require.Equal(t, float64(5), series["valid_requests"].Metric[0].GetCounter().GetValue())
	require.Equal(t, float64(7), series["valid_height"].Metric[0].GetGauge().GetValue())
}

func TestRepairM_624_EnableServiceLabel(t *testing.T) {
	for _, enabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("enabled_%t", enabled), func(t *testing.T) {
			m := repairM624Metrics(t, Config{
				Enabled:                 true,
				ServiceName:             "telemetry624",
				EnableServiceLabel:      enabled,
				PrometheusRetentionTime: 60,
				GlobalLabels:            [][]string{{"chain_id", "chain-624"}},
			})
			IncrCounter(2, "requests")
			IncrCounter(3, "requests")
			SetGauge(7, "height")
			series := repairM624Gather(t, m)
			labels := map[string]string{"chain_id": "chain-624"}
			prefix, otherPrefix := "telemetry624_", ""
			if enabled {
				labels["service"] = "telemetry624"
				prefix, otherPrefix = "", "telemetry624_"
			}
			for _, key := range []string{"requests", "height"} {
				name := prefix + key
				require.Contains(t, series, name)
				require.NotContains(t, series, otherPrefix+key)
				require.Len(t, series[name].Metric, 1)
				repairM624Labels(t, series[name].Metric[0], labels)
			}
			require.Equal(t, float64(5), series[prefix+"requests"].Metric[0].GetCounter().GetValue())
			require.Equal(t, float64(7), series[prefix+"height"].Metric[0].GetGauge().GetValue())
		})
	}
}

func repairM624Metrics(t *testing.T, cfg Config) *Metrics {
	t.Helper()
	previousRegisterer, previousGatherer := prometheus.DefaultRegisterer, prometheus.DefaultGatherer
	previousLabels := globalLabels
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer, prometheus.DefaultGatherer = registry, registry
	t.Cleanup(func() {
		metrics.Shutdown()
		prometheus.DefaultRegisterer, prometheus.DefaultGatherer = previousRegisterer, previousGatherer
		globalLabels = previousLabels
	})
	m, err := New(cfg)
	require.NoError(t, err)
	require.NotNil(t, m)
	return m
}

func repairM624Gather(t *testing.T, m *Metrics) map[string]*dto.MetricFamily {
	t.Helper()
	response, err := m.Gather(FormatPrometheus)
	require.NoError(t, err)
	require.Equal(t, string(expfmt.NewFormat(expfmt.TypeTextPlain)), response.ContentType)
	parser := expfmt.NewTextParser(model.LegacyValidation)
	series, err := parser.TextToMetricFamilies(bytes.NewReader(response.Metrics))
	require.NoError(t, err)
	return series
}

func repairM624Labels(t *testing.T, emitted *dto.Metric, expected map[string]string) {
	t.Helper()
	actual := make(map[string]string)
	for _, label := range emitted.Label {
		require.NotContains(t, actual, label.GetName())
		actual[label.GetName()] = label.GetValue()
	}
	require.Equal(t, expected, actual)
}
