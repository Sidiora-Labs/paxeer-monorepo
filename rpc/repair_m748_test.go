package evmrpc_test

// Task 22.33.748, audit 22.30.172: rpc-fork-default-guide is proved by
// TestRepairM_748_ManualForkCommand and TestRepairM_748_RequiredPinnedBlock.
// Baseline: not run; the ordered implementation-first instructions permit
// verification only through hf-done after hf-build. The inherited README
// already required both variables and included block 21000000; no baseline
// failure is claimed. Wiring requests: none required by the repair proposal.
// The runner's gate ledger records the verified revision and private evidence.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/dop251/goja"
	"github.com/stretchr/testify/require"
)

func repairM748Read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "tests", "integration", "rpc_tests", path))
	require.NoError(t, err)
	return string(data)
}

func repairM748LoadConfig(t *testing.T, env map[string]string) (goja.Value, error) {
	t.Helper()
	var manifest struct {
		Scripts map[string]string `json:"scripts"`
	}
	require.NoError(t, json.Unmarshal([]byte(repairM748Read(t, "package.json")), &manifest))
	// Follow the documented npm command to its real configuration file. Only
	// module imports, the TypeScript type annotation and the default export
	// are omitted: the environment validation and config object run unchanged
	// in the embedded JavaScript engine, without starting Hardhat or an RPC.
	args := strings.Fields(manifest.Scripts["rpc:fork"])
	require.Equal(t, []string{"hardhat", "--config", "hardhat/hardhat.config.ts", "node", "--port", "9546"}, args)
	source := repairM748Read(t, args[2])
	start := strings.Index(source, "const UPSTREAM =")
	end := strings.Index(source, "export default config;")
	require.GreaterOrEqual(t, start, 0)
	require.Greater(t, end, start)
	source = source[start:end]
	require.Equal(t, 1, strings.Count(source, "const config: HardhatUserConfig ="))
	source = strings.Replace(source, "const config: HardhatUserConfig =", "const config =", 1)
	vm := goja.New()
	require.NoError(t, vm.Set("process", map[string]interface{}{"env": env}))
	return vm.RunString(source + "\nJSON.stringify(config);")
}

func TestRepairM_748_ManualForkCommand(t *testing.T) {
	readme := repairM748Read(t, "README.md")
	manual := strings.SplitN(readme, "## Running manually", 2)
	require.Len(t, manual, 2)
	manual = strings.SplitN(manual[1], "## Configuration", 2)
	require.Len(t, manual, 2)
	command := regexp.MustCompile(`ETH_MAINNET_UPSTREAM='([^']+)'\s*\\\s*\nETH_MAINNET_FORK_BLOCK=([0-9]+) npm run rpc:fork`).FindStringSubmatch(manual[0])
	require.Len(t, command, 3, "the complete manual command must supply the upstream and pinned block together")
	block, err := strconv.ParseUint(command[2], 10, 64)
	require.NoError(t, err)
	require.Greater(t, block, uint64(0))
	require.LessOrEqual(t, block, uint64(9007199254740991))
	for _, statement := range []string{
		"Both fork variables are required", "positive", "safe-integer block number",
		"9007199254740991", "historical state your upstream serves",
		"never defaults to the latest block",
	} {
		require.Contains(t, manual[0], statement)
	}
	row := regexp.MustCompile("(?m)^\\| `ETH_MAINNET_FORK_BLOCK`[^\\n]*").FindString(readme)
	require.Contains(t, row, "required positive safe integer")
	require.Contains(t, row, "no latest default")
	var manifest struct {
		Description string `json:"description"`
	}
	require.NoError(t, json.Unmarshal([]byte(repairM748Read(t, "package.json")), &manifest))
	require.Contains(t, manifest.Description, "require ETH_MAINNET_UPSTREAM and a positive safe-integer ETH_MAINNET_FORK_BLOCK")

	produced, err := repairM748LoadConfig(t, map[string]string{
		"ETH_MAINNET_UPSTREAM": command[1], "ETH_MAINNET_FORK_BLOCK": command[2],
	})
	require.NoError(t, err, "the documented inputs must load the actual production fork config")
	var config struct {
		Networks struct {
			Hardhat struct {
				ChainID int `json:"chainId"`
				Forking struct {
					URL         string `json:"url"`
					BlockNumber uint64 `json:"blockNumber"`
				} `json:"forking"`
			} `json:"hardhat"`
		} `json:"networks"`
	}
	require.NoError(t, json.Unmarshal([]byte(produced.String()), &config))
	require.Equal(t, command[1], config.Networks.Hardhat.Forking.URL)
	require.Equal(t, block, config.Networks.Hardhat.Forking.BlockNumber)
	require.Equal(t, 1, config.Networks.Hardhat.ChainID)
}

func TestRepairM_748_RequiredPinnedBlock(t *testing.T) {
	for _, block := range []string{"", "0", "-1", "1.5", "NaN", "Infinity", "latest", "9007199254740992", "9007199254740993"} {
		t.Run("reject_"+block, func(t *testing.T) {
			env := map[string]string{"ETH_MAINNET_UPSTREAM": "https://your-ethereum-provider.example/"}
			if block != "" {
				env["ETH_MAINNET_FORK_BLOCK"] = block
			}
			_, err := repairM748LoadConfig(t, env)
			require.ErrorContains(t, err, "ETH_MAINNET_FORK_BLOCK must be a positive pinned block number")
		})
	}
	_, err := repairM748LoadConfig(t, map[string]string{"ETH_MAINNET_FORK_BLOCK": "21000000"})
	require.ErrorContains(t, err, "ETH_MAINNET_UPSTREAM is not set")
	for _, block := range []string{"1", "21000000", "9007199254740991"} {
		t.Run("accept_"+block, func(t *testing.T) {
			produced, err := repairM748LoadConfig(t, map[string]string{
				"ETH_MAINNET_UPSTREAM": "https://your-ethereum-provider.example/", "ETH_MAINNET_FORK_BLOCK": block,
			})
			require.NoError(t, err)
			require.Contains(t, produced.String(), `"blockNumber":`+block)
		})
	}
}
