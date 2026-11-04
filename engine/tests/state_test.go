package giga_test

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"testing"
)

// maxFailuresToDisplay limits the number of individual failures shown in the summary
const maxFailuresToDisplay = 50

// TestSummary tracks test results for summary reporting
type TestSummary struct {
	mu sync.Mutex

	// Per-category stats
	CategoryStats map[string]*CategoryStats

	// Global failure tracking by type
	FailuresByType map[FailureType][]string

	// Total counts
	TotalTests   int
	TotalPassed  int
	TotalFailed  int
	TotalSkipped int
}

// CategoryStats tracks stats for a single category
type CategoryStats struct {
	Total    int
	Passed   int
	Failed   int
	Skipped  int
	Failures []FailureRecord
}

// FailureRecord records a single test failure
type FailureRecord struct {
	TestName    string
	FailureType FailureType
	Message     string
}

// NewTestSummary creates a new test summary tracker
func NewTestSummary() *TestSummary {
	return &TestSummary{
		CategoryStats:  make(map[string]*CategoryStats),
		FailuresByType: make(map[FailureType][]string),
	}
}

// RecordPass records a passing test
func (ts *TestSummary) RecordPass(category, testName string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.TotalTests++
	ts.TotalPassed++

	if ts.CategoryStats[category] == nil {
		ts.CategoryStats[category] = &CategoryStats{}
	}
	ts.CategoryStats[category].Total++
	ts.CategoryStats[category].Passed++
}

// RecordSkip records a skipped test
func (ts *TestSummary) RecordSkip(category, testName, reason string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.TotalTests++
	ts.TotalSkipped++

	if ts.CategoryStats[category] == nil {
		ts.CategoryStats[category] = &CategoryStats{}
	}
	ts.CategoryStats[category].Total++
	ts.CategoryStats[category].Skipped++
}

// RecordFailure records a failed test
func (ts *TestSummary) RecordFailure(category, testName string, failureType FailureType, message string) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	ts.TotalTests++
	ts.TotalFailed++

	if ts.CategoryStats[category] == nil {
		ts.CategoryStats[category] = &CategoryStats{}
	}
	cs := ts.CategoryStats[category]
	cs.Total++
	cs.Failed++
	cs.Failures = append(cs.Failures, FailureRecord{
		TestName:    testName,
		FailureType: failureType,
		Message:     message,
	})

	fullName := category + "/" + testName
	ts.FailuresByType[failureType] = append(ts.FailuresByType[failureType], fullName)
}

// PrintSummary prints the test summary to the test log
func (ts *TestSummary) PrintSummary(t *testing.T) {
	ts.mu.Lock()
	defer ts.mu.Unlock()

	var sb strings.Builder
	sb.WriteString("\n=== State Test Summary ===\n\n")

	// Sort categories for consistent output
	categories := make([]string, 0, len(ts.CategoryStats))
	for cat := range ts.CategoryStats {
		categories = append(categories, cat)
	}
	sort.Strings(categories)

	// Per-category summary
	for _, cat := range categories {
		stats := ts.CategoryStats[cat]
		sb.WriteString(fmt.Sprintf("Category: %s\n", cat))
		sb.WriteString(fmt.Sprintf("  Total: %d\n", stats.Total))
		sb.WriteString(fmt.Sprintf("  Passed: %d\n", stats.Passed))
		sb.WriteString(fmt.Sprintf("  Failed: %d\n", stats.Failed))
		sb.WriteString(fmt.Sprintf("  Skipped: %d\n", stats.Skipped))
		sb.WriteString("\n")
	}

	// Failures by type
	if len(ts.FailuresByType) > 0 {
		sb.WriteString("Failures by type:\n")
		// Sort failure types for consistent output
		types := make([]FailureType, 0, len(ts.FailuresByType))
		for ft := range ts.FailuresByType {
			types = append(types, ft)
		}
		sort.Slice(types, func(i, j int) bool {
			return string(types[i]) < string(types[j])
		})

		for _, ft := range types {
			sb.WriteString(fmt.Sprintf("  %s: %d\n", ft, len(ts.FailuresByType[ft])))
		}
		sb.WriteString("\n")

		// List failed tests (limit to avoid huge output)
		sb.WriteString("Failed tests:\n")
		failedCount := 0
		for _, cat := range categories {
			stats := ts.CategoryStats[cat]
			for _, f := range stats.Failures {
				if failedCount >= maxFailuresToDisplay {
					sb.WriteString(fmt.Sprintf("  ... and %d more failures\n", ts.TotalFailed-maxFailuresToDisplay))
					break
				}
				sb.WriteString(fmt.Sprintf("  - %s/%s: %s (%s)\n", cat, f.TestName, f.FailureType, f.Message))
				failedCount++
			}
			if failedCount >= maxFailuresToDisplay {
				break
			}
		}
	}

	// Overall summary
	sb.WriteString(fmt.Sprintf("\n=== Overall: %d total, %d passed, %d failed, %d skipped ===\n",
		ts.TotalTests, ts.TotalPassed, ts.TotalFailed, ts.TotalSkipped))

	t.Log(sb.String())
}

// TestGigaVsV2_StateTests qualifies an explicit, counted Cancun selection.
// Qualification always checks both local executors against explicit fixture state and logs.
// Paxeer uses a Cosmos store commitment rather than the fixture's Ethereum MPT root;
// the supported oracle is the explicit account allocation plus RLP logs hash.
// Transactions are re-signed for pax-test (713714), never the fixture chain ID 1.
// Public hyperpax_125-1 maps to 125; these local fixtures do not certify mainnet.
// Paxeer's fork schedule, fee policy and opcode gas costs remain active. Fixtures
// that depend on a different policy must fail, not have their expectations rewritten.
func TestGigaVsV2_StateTests(t *testing.T) {
	mode := ModeGigaSequential
	modeName := "Giga vs V2"
	if os.Getenv("USE_REGULAR_STORE") == "true" {
		mode = ModeGigaWithRegularStore
		modeName = "Giga with Regular Store"
	}
	runStateTestSuite(t, ComparisonConfig{GigaMode: mode, VerifyEthereumSpec: true}, modeName)
}
