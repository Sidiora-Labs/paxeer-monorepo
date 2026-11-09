package evmrpc_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPxNodeCatalogueWalletSDK(t *testing.T) {
	h := newPxRootAccountHarness(t)
	methods, err := json.Marshal(h.http.HTTPMethods())
	require.NoError(t, err)
	reportPath := filepath.Join(t.TempDir(), "wallet-node-catalogue.json")
	ctx, cancel := context.WithTimeout(t.Context(), 90*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, "pnpm", "--dir", "../sdk/wallet", "exec", "vitest", "run",
		"test/node-route-catalogue.test.ts", "-t", "^nodeRouteCatalogueDecodesAndValidates$",
		"--reporter=default", "--reporter=json", "--outputFile="+reportPath)
	command.Env = append(os.Environ(), "PX_NODE_CATALOGUE_URL=http://"+h.http.ListenAddr()+"/",
		"PX_NODE_CATALOGUE_METHODS="+string(methods))
	output, err := command.CombinedOutput()
	require.NoError(t, err, "wallet SDK node catalogue test failed:\n%s", output)
	reportBytes, err := os.ReadFile(reportPath)
	require.NoError(t, err, "vitest must produce an execution report:\n%s", output)
	var report struct {
		Success         bool `json:"success"`
		NumTotalTests   int  `json:"numTotalTests"`
		NumPassedTests  int  `json:"numPassedTests"`
		NumPendingTests int  `json:"numPendingTests"`
		TestResults     []struct {
			AssertionResults []struct {
				FullName string `json:"fullName"`
				Status   string `json:"status"`
			} `json:"assertionResults"`
		} `json:"testResults"`
	}
	require.NoError(t, json.Unmarshal(reportBytes, &report))
	require.True(t, report.Success, "%s", output)
	require.Equal(t, 1, report.NumTotalTests, "%s", output)
	require.Equal(t, 1, report.NumPassedTests, "%s", output)
	require.Zero(t, report.NumPendingTests, "%s", output)
	require.Len(t, report.TestResults, 1)
	require.Len(t, report.TestResults[0].AssertionResults, 1)
	assertion := report.TestResults[0].AssertionResults[0]
	require.Equal(t, "nodeRouteCatalogueDecodesAndValidates", assertion.FullName)
	require.Equal(t, "passed", assertion.Status)
}
