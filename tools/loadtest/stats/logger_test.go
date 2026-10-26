package stats

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLogFinalStatsWritesJSONReport(t *testing.T) {
	collector := NewCollector()
	collector.RecordTransaction("transfer", "endpoint-a", 10*time.Millisecond, true)
	collector.RecordTransaction("transfer", "endpoint-b", 20*time.Millisecond, true)
	collector.RecordTransaction("transfer", "endpoint-a", 30*time.Millisecond, true)
	reportPath := filepath.Join(t.TempDir(), "report.json")
	logger := NewLogger(collector, time.Second, reportPath, false)

	finalStats, err := logger.LogFinalStats()
	if err != nil {
		t.Fatalf("LogFinalStats: %v", err)
	}
	if finalStats == nil {
		t.Fatal("LogFinalStats returned nil statistics")
	}
	data, err := os.ReadFile(reportPath)
	if err != nil {
		t.Fatalf("read report: %v", err)
	}
	var report FinalStats
	if err := json.Unmarshal(data, &report); err != nil {
		t.Fatalf("decode report as FinalStats: %v", err)
	}
	if report.LoadTestStatistics.TotalTxs != 3 {
		t.Errorf("report total transactions = %d, want 3", report.LoadTestStatistics.TotalTxs)
	}
	if report.ScenarioDistribution["transfer"] != 3 {
		t.Errorf("report scenario transactions = %d, want 3", report.ScenarioDistribution["transfer"])
	}
	if report.EndpointPerformance["endpoint-a"].SampleCount != 2 || report.EndpointPerformance["endpoint-b"].SampleCount != 1 {
		t.Errorf("report endpoint samples = %+v, want 2 for endpoint-a and 1 for endpoint-b", report.EndpointPerformance)
	}
	expected, err := json.MarshalIndent(finalStats, "", "  ")
	if err != nil {
		t.Fatalf("marshal returned statistics: %v", err)
	}
	if !bytes.Equal(data, expected) {
		t.Errorf("report differs from the indented JSON of the returned statistics:\ngot: %s\nwant: %s", data, expected)
	}
}

func TestLogFinalStatsReturnsReportError(t *testing.T) {
	collector := NewCollector()
	collector.RecordTransaction("transfer", "endpoint-a", 10*time.Millisecond, true)
	reportPath := filepath.Join(t.TempDir(), "missing", "report.json")
	logger := NewLogger(collector, time.Second, reportPath, false)

	_, err := logger.LogFinalStats()
	if err == nil {
		t.Fatal("LogFinalStats succeeded for a report path in a missing directory")
	}
	if !strings.Contains(err.Error(), "write report "+reportPath+": ") {
		t.Errorf("error %q does not identify the report write and path %q", err, reportPath)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Errorf("error %v does not wrap os.ErrNotExist", err)
	}
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) {
		t.Fatalf("error %v does not wrap an os.PathError", err)
	}
	if pathErr.Path != reportPath {
		t.Errorf("wrapped error path = %q, want %q", pathErr.Path, reportPath)
	}
}
