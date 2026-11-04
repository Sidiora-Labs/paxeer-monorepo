package evmrpc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/dop251/goja"
)

// Task 22.33.753 closure proofs:
// fork-readme-latest -> TestRepairM_753_ForkReadmeLatest
// fork-optional-validation -> TestRepairM_753_ForkOptionalValidation
// Wiring request (rpc/domain 16): align the out-of-scope fork instructions in
// tests/integration/rpc_tests/README.md with the required positive pinned block.
// Baseline verification is not run: the bounded gate order requires the entire
// implementation before hf-build and hf-done, and forbids a separate verify run.

func repairM753Read(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("cannot locate the committed Hardhat configuration")
	}
	data, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "tests", "integration", "rpc_tests", "hardhat", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

type repairM753Config struct {
	Networks struct {
		Hardhat struct {
			ChainID int `json:"chainId"`
			Forking struct {
				URL         string  `json:"url"`
				BlockNumber float64 `json:"blockNumber"`
			} `json:"forking"`
		} `json:"hardhat"`
	} `json:"networks"`
}

func repairM753Load(t *testing.T, env map[string]string) (repairM753Config, error) {
	t.Helper()
	source := repairM753Read(t, "hardhat.config.ts")
	// Only remove module declarations and the TypeScript type annotation. The
	// actual environment reads, validation and exported config execute in Goja;
	// no Node subprocess, Hardhat node, provider or reimplemented validator runs.
	for _, replacement := range [][2]string{
		{"import { HardhatUserConfig } from 'hardhat/config';", ""},
		{"import '@nomicfoundation/hardhat-toolbox';", ""},
		{"const config: HardhatUserConfig =", "const config ="},
		{"export default config;", "JSON.stringify(config);"},
	} {
		if strings.Count(source, replacement[0]) != 1 {
			t.Fatalf("expected exactly one module/type declaration %q", replacement[0])
		}
		source = strings.Replace(source, replacement[0], replacement[1], 1)
	}
	vm := goja.New()
	if err := vm.Set("process", map[string]interface{}{"env": env}); err != nil {
		t.Fatal(err)
	}
	value, err := vm.RunScript("hardhat.config.ts", source)
	var config repairM753Config
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal([]byte(value.String()), &config); err != nil {
		t.Fatalf("decode the exported Hardhat config: %v", err)
	}
	return config, nil
}

func repairM753AssertFork(t *testing.T, config repairM753Config, upstream string, block float64) {
	t.Helper()
	hardhat := config.Networks.Hardhat
	if hardhat.ChainID != 1 || hardhat.Forking.URL != upstream || hardhat.Forking.BlockNumber != block {
		t.Fatalf("exported fork settings = %+v, want chainId 1, upstream %q, pinned block %.0f", hardhat, upstream, block)
	}
}

func TestRepairM_753_ForkReadmeLatest(t *testing.T) {
	readme := repairM753Read(t, "README.md")
	for _, required := range []string{
		"**required pinned block**",
		"| `ETH_MAINNET_FORK_BLOCK` | None | Required positive safe integer",
		"There is no latest-block default.",
		"ETH_MAINNET_FORK_BLOCK is not set",
	} {
		if !strings.Contains(readme, required) {
			t.Errorf("fork guide must state %q", required)
		}
	}
	commands := 0
	for _, fenced := range strings.Split(readme, "```bash\n")[1:] {
		end := strings.Index(fenced, "```")
		if end < 0 {
			t.Fatal("unterminated bash example")
		}
		command := strings.ReplaceAll(fenced[:end], "\\\n", " ")
		if !strings.Contains(command, "npm run rpc:fork") {
			continue
		}
		commands++
		env := map[string]string{}
		fields := strings.Fields(command)
		for len(fields) > 0 && strings.Contains(fields[0], "=") {
			key, value, _ := strings.Cut(fields[0], "=")
			env[key] = strings.Trim(value, "\"'")
			fields = fields[1:]
		}
		if strings.Join(fields, " ") != "npm run rpc:fork" {
			t.Fatalf("unexpected startup command %q", command)
		}
		block, err := strconv.ParseUint(env["ETH_MAINNET_FORK_BLOCK"], 10, 64)
		if err != nil || block == 0 || block > 9007199254740991 {
			t.Fatalf("startup command must supply a positive safe pinned block: %q (%v)", command, err)
		}
		if env["ETH_MAINNET_UPSTREAM"] == "" {
			t.Fatal("startup command must supply its upstream")
		}
		// The displayed command produces the environment consumed by the real
		// config; assert the settings exported to Hardhat preserve that pin.
		config, err := repairM753Load(t, env)
		if err != nil {
			t.Fatalf("documented startup command rejected: %v", err)
		}
		repairM753AssertFork(t, config, env["ETH_MAINNET_UPSTREAM"], float64(block))
	}
	if commands == 0 {
		t.Fatal("fork guide has no executable startup example")
	}
}

func TestRepairM_753_ForkOptionalValidation(t *testing.T) {
	const upstream = "https://your-ethereum-provider.example/"
	for _, tc := range []struct {
		name, value string
		missing     bool
	}{
		{"unset", "", true},
		{"empty", "", true},
		{"whitespace", " \t\n", true},
		{"zero", "0", false},
		{"negative", "-1", false},
		{"fractional", "1.5", false},
		{"nonnumeric", "not-a-block", false},
		{"latest", "latest", false},
		{"nan", "NaN", false},
		{"infinite", "Infinity", false},
		{"unsafe", "9007199254740992", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{"ETH_MAINNET_UPSTREAM": upstream}
			if tc.name != "unset" {
				env["ETH_MAINNET_FORK_BLOCK"] = tc.value
			}
			_, err := repairM753Load(t, env)
			if err == nil {
				t.Fatalf("invalid pinned block %q was accepted", tc.value)
			}
			if tc.missing {
				for _, required := range []string{"ETH_MAINNET_FORK_BLOCK is not set", "positive pinned block number", "ETH_MAINNET_UPSTREAM=", "ETH_MAINNET_FORK_BLOCK=21000000 npm run rpc:fork"} {
					if !strings.Contains(err.Error(), required) {
						t.Errorf("missing-value error %q lacks %q", err, required)
					}
				}
			} else if !strings.Contains(err.Error(), "ETH_MAINNET_FORK_BLOCK must be a positive pinned block number") {
				t.Fatalf("invalid-value error = %v", err)
			}
		})
	}
	for _, tc := range []struct {
		value string
		block float64
	}{
		{"1", 1},
		{"21000000", 21000000},
		{"9007199254740991", 9007199254740991},
		{" 21000000 ", 21000000},
		{"2.1e7", 21000000},
		{"0x1406f40", 21000000},
	} {
		t.Run("valid_"+tc.value, func(t *testing.T) {
			config, err := repairM753Load(t, map[string]string{"ETH_MAINNET_UPSTREAM": upstream, "ETH_MAINNET_FORK_BLOCK": tc.value})
			if err != nil {
				t.Fatalf("valid pinned block rejected: %v", err)
			}
			repairM753AssertFork(t, config, upstream, tc.block)
		})
	}
	for _, env := range []map[string]string{
		{"ETH_MAINNET_FORK_BLOCK": "21000000"},
		{"ETH_MAINNET_UPSTREAM": "", "ETH_MAINNET_FORK_BLOCK": "21000000"},
	} {
		_, err := repairM753Load(t, env)
		if err == nil || !strings.Contains(err.Error(), "ETH_MAINNET_UPSTREAM is not set") || !strings.Contains(err.Error(), "ETH_MAINNET_FORK_BLOCK=21000000 npm run rpc:fork") {
			t.Fatalf("missing-upstream error must include both startup variables: %v", err)
		}
	}
	header := strings.SplitN(repairM753Read(t, "hardhat.config.ts"), "*/", 2)[0]
	for _, required := range []string{"Required env (no defaults)", "positive pinned block number", "ETH_MAINNET_FORK_BLOCK=21000000 npm run rpc:fork"} {
		if !strings.Contains(header, required) {
			t.Errorf("configuration header must state %q", required)
		}
	}
}
