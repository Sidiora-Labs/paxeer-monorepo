package telemetry

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/expfmt"
	"github.com/stretchr/testify/require"
)

func TestPrometheusSinkPredeclaresNoLegacyDefinitions(t *testing.T) {
	previousRegisterer, previousGatherer := prometheus.DefaultRegisterer, prometheus.DefaultGatherer
	registry := prometheus.NewRegistry()
	prometheus.DefaultRegisterer, prometheus.DefaultGatherer = registry, registry
	t.Cleanup(func() {
		prometheus.DefaultRegisterer, prometheus.DefaultGatherer = previousRegisterer, previousGatherer
	})

	m, err := New(Config{Enabled: true, PrometheusRetentionTime: 60})
	require.NoError(t, err)
	require.NotNil(t, m)
	require.True(t, m.prometheusEnabled)

	response, err := m.Gather(FormatPrometheus)
	require.NoError(t, err)
	require.Equal(t, string(expfmt.NewFormat(expfmt.TypeTextPlain)), response.ContentType)
	require.NotContains(t, string(response.Metrics), "cosmos_upgrade_plan_height")
	require.NotContains(t, string(response.Metrics), "pax_cosmos_validator_slashed")

	IncrCounter(2, "active_counter")
	SetGauge(3, "active_gauge")
	response, err = m.Gather(FormatPrometheus)
	require.NoError(t, err)
	require.Contains(t, string(response.Metrics), "active_counter 2")
	require.Contains(t, string(response.Metrics), "active_gauge 3")
	require.NotContains(t, string(response.Metrics), "cosmos_upgrade_plan_height")
	require.NotContains(t, string(response.Metrics), "pax_cosmos_validator_slashed")
}

func TestTelemetryPackageExportsNoValidatorSlashedHelper(t *testing.T) {
	packages, err := parser.ParseDir(token.NewFileSet(), ".", func(info fs.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, parser.SkipObjectResolution)
	require.NoError(t, err)
	require.Contains(t, packages, "telemetry")
	require.NotEmpty(t, packages["telemetry"].Files)

	var exportedFunctions []string
	for _, file := range packages["telemetry"].Files {
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if ok && function.Recv == nil && function.Name.IsExported() {
				exportedFunctions = append(exportedFunctions, function.Name.Name)
			}
		}
	}
	require.Contains(t, exportedFunctions, "IncrCounter")
	require.Contains(t, exportedFunctions, "SetGauge")
	require.NotContains(t, exportedFunctions, "IncrValidatorSlashedCounter")
}
