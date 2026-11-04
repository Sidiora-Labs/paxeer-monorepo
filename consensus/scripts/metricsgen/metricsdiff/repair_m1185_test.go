package main_test

import (
	"strings"
	"testing"

	metricsdiff "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/scripts/metricsgen/metricsdiff"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/proto"
)

func repairM1185Exposition(t *testing.T, labels []string) string {
	t.Helper()
	pairs := make([]*dto.LabelPair, len(labels))
	for i, name := range labels {
		pairs[i] = &dto.LabelPair{Name: proto.String(name), Value: proto.String("fixture")}
	}
	family := &dto.MetricFamily{
		Name: proto.String("requests"),
		Type: dto.MetricType_GAUGE.Enum(),
		Metric: []*dto.Metric{{
			Label: pairs,
			Gauge: &dto.Gauge{Value: proto.Float64(1)},
		}},
	}
	var text strings.Builder
	_, err := expfmt.MetricFamilyToText(&text, family)
	require.NoError(t, err)
	return text.String()
}

func TestRepairM_1185_ReorderedLabels(t *testing.T) {
	orders := [][]string{
		{"host", "method", "zone"},
		{"host", "zone", "method"},
		{"method", "host", "zone"},
		{"method", "zone", "host"},
		{"zone", "host", "method"},
		{"zone", "method", "host"},
	}
	for _, before := range orders {
		for _, after := range orders {
			t.Run(strings.Join(before, "_")+"_to_"+strings.Join(after, "_"), func(t *testing.T) {
				diff, err := metricsdiff.DiffFromReaders(
					strings.NewReader(repairM1185Exposition(t, before)),
					strings.NewReader(repairM1185Exposition(t, after)),
				)
				require.NoError(t, err)
				require.Empty(t, diff.Adds)
				require.Empty(t, diff.Removes)
				require.Empty(t, diff.Changes)
				require.Empty(t, diff.String())
			})
		}
	}
}

func TestRepairM_1185_ChangedLabels(t *testing.T) {
	for _, tc := range []struct {
		name    string
		before  []string
		after   []string
		adds    []string
		removes []string
		output  string
	}{
		{
			name:   "replace",
			before: []string{"zone", "method", "host"},
			after:  []string{"method", "chain", "host"},
			adds:   []string{"chain"}, removes: []string{"zone"},
			output: "Label changes:\nMetric: requests\n+++ chain\n--- zone\n",
		},
		{
			name:   "add",
			before: []string{"zone", "host"},
			after:  []string{"method", "zone", "host"},
			adds:   []string{"method"}, removes: []string{},
			output: "Label changes:\nMetric: requests\n+++ method\n",
		},
		{
			name:   "remove",
			before: []string{"zone", "host", "method"},
			after:  []string{"host", "zone"},
			adds:   []string{}, removes: []string{"method"},
			output: "Label changes:\nMetric: requests\n--- method\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			diff, err := metricsdiff.DiffFromReaders(
				strings.NewReader(repairM1185Exposition(t, tc.before)),
				strings.NewReader(repairM1185Exposition(t, tc.after)),
			)
			require.NoError(t, err)
			require.Empty(t, diff.Adds)
			require.Empty(t, diff.Removes)
			require.Equal(t, []metricsdiff.LabelDiff{{
				Metric: "requests", Adds: tc.adds, Removes: tc.removes,
			}}, diff.Changes)
			require.Equal(t, tc.output, diff.String())
		})
	}
}
