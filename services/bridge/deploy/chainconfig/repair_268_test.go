package chainconfig

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	gethmath "github.com/ethereum/go-ethereum/common/math"
)

func TestRepair_268_CapBoundaries(t *testing.T) {
	testCapDestinationBoundaries(t)
}

func TestRepair_268_DecimalInputBound(t *testing.T) {
	maximum, _ := capBoundaryValues(KindEVM)
	for _, method := range []struct {
		name string
		read func(Asset) (*big.Int, error)
	}{
		{"PerTx", Asset.PerTx},
		{"Total", Asset.Total},
	} {
		t.Run(method.name, func(t *testing.T) {
			amount, err := method.read(Asset{PerTxCap: maximum, TotalCap: maximum})
			if err != nil || amount == nil || amount.String() != maximum {
				t.Fatalf("the 78-digit uint256 maximum must parse exactly: %v, %v", amount, err)
			}
			for _, size := range []int{79, 1 << 20} {
				t.Run(strconv.Itoa(size), func(t *testing.T) {
					value := strings.Repeat("9", size)
					amount, err := method.read(Asset{PerTxCap: value, TotalCap: value})
					if err == nil || amount != nil {
						t.Fatalf("an overlong cap produced a big.Int: %v", err)
					}
					if !strings.Contains(err.Error(), "at most 78 decimal digits") || len(err.Error()) > 100 {
						t.Fatalf("the early length refusal must be bounded: %v", err)
					}
				})
			}
			for _, value := range []string{"", "01", "+1", "-1", "1.0", "1e3", " 1", "1 ", "0x01", "１"} {
				if amount, err := method.read(Asset{PerTxCap: value, TotalCap: value}); err == nil || amount != nil {
					t.Errorf("noncanonical decimal %q accepted: %v, %v", value, amount, err)
				}
			}
		})
	}
}

func transportCapConfig(t *testing.T, name string) *ChainConfig {
	t.Helper()
	t.Setenv(OverlayEnv, "")
	chain, ok := ChainByName(name)
	if !ok {
		t.Fatalf("unknown transport chain %s", name)
	}
	path := capTestConfigPath(t, chain)
	config, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	maximum, _ := capBoundaryValues(chain.Kind)
	limits := make(map[string]AssetCap, len(config.Assets))
	for index, asset := range config.Assets {
		perTx := maximum
		if index != 0 {
			perTx = "1"
		}
		limits[asset.Symbol] = AssetCap{PerTxCap: perTx, TotalCap: maximum}
	}
	overlay := Overlay{Chains: map[string]ChainOverlay{name: {Caps: limits}}}
	if err := overlay.Apply(config); err != nil {
		t.Fatal(err)
	}
	if err := config.RequireDeployable(); err != nil {
		t.Fatal(err)
	}
	return config
}

func deploymentCapStrings(t *testing.T, config *ChainConfig) (perTx, total []string) {
	t.Helper()
	// These are the deployment scripts' real JSON extraction operations. Keep
	// caps as strings throughout; JSON numeric conversion would lose precision.
	for _, field := range []string{"per_tx_cap", "total_cap"} {
		body, err := json.Marshal(config)
		if err != nil {
			t.Fatal(err)
		}
		cmd := exec.Command("jq", "-r", ".assets[]."+field)
		cmd.Stdin = bytes.NewReader(body)
		output, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("deployment JSON extraction failed: %v: %s", err, output)
		}
		values := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
		if len(values) != len(config.Assets) {
			t.Fatalf("transport emitted %d %s values for %d assets", len(values), field, len(config.Assets))
		}
		for index, value := range values {
			want := config.Assets[index].PerTxCap
			if field == "total_cap" {
				want = config.Assets[index].TotalCap
			}
			if value != want {
				t.Fatalf("transport changed assets[%d].%s: got %q, want %q", index, field, value, want)
			}
		}
		if field == "per_tx_cap" {
			perTx = values
		} else {
			total = values
		}
	}
	return perTx, total
}

func TestRepair_268_EVMTransport(t *testing.T) {
	config := transportCapConfig(t, "ethereum")
	perTx, total := deploymentCapStrings(t, config)
	script, err := os.ReadFile(filepath.Join(repositoryRoot(t), "contracts/bridge/evm/script/DeployPaxeerXVault.s.sol"))
	if err != nil {
		t.Fatal(err)
	}
	for _, declaration := range []string{
		`uint256[] memory perTxCaps = vm.envUint("PAXEER_BRIDGE_VAULT_PER_TX_CAPS", ",");`,
		`uint256[] memory totalCaps = vm.envUint("PAXEER_BRIDGE_VAULT_TOTAL_CAPS", ",");`,
	} {
		if !bytes.Contains(script, []byte(declaration)) {
			t.Fatalf("the destination no longer consumes the checked uint256 array: %s", declaration)
		}
	}
	arrayType, err := abi.NewType("uint256[]", "", nil)
	if err != nil {
		t.Fatal(err)
	}
	args := abi.Arguments{{Name: "perTxCaps", Type: arrayType}, {Name: "totalCaps", Type: arrayType}}
	values := make([][]*big.Int, 2)
	for field, decimals := range [][]string{perTx, total} {
		// The deployment exports these arrays as comma-separated environment values.
		for _, decimal := range strings.Split(strings.Join(decimals, ","), ",") {
			amount, ok := gethmath.ParseBig256(decimal)
			if !ok {
				t.Fatalf("destination uint256 parser refused %s", decimal)
			}
			values[field] = append(values[field], amount)
		}
	}
	payload, err := args.Pack(values[0], values[1])
	if err != nil {
		t.Fatal(err)
	}
	recovered, err := args.Unpack(payload)
	if err != nil {
		t.Fatal(err)
	}
	for field, want := range [][]string{perTx, total} {
		got, ok := recovered[field].([]*big.Int)
		if !ok || len(got) != len(want) {
			t.Fatalf("destination decoded an unexpected array: %v", recovered[field])
		}
		for index := range want {
			if got[index].String() != want[index] {
				t.Fatalf("uint256 transport changed cap %d/%d: %s != %s", field, index, got[index], want[index])
			}
		}
	}
	_, overflow := capBoundaryValues(KindEVM)
	if _, ok := gethmath.ParseBig256(overflow); ok {
		t.Fatal("the destination parser accepted uint256 maximum+1")
	}
}

func TestRepair_268_SolanaTransport(t *testing.T) {
	config := transportCapConfig(t, "solana")
	perTx, total := deploymentCapStrings(t, config)
	root := repositoryRoot(t)
	for _, check := range []struct{ path, declaration string }{
		{"contracts/bridge/solana/admin/src/main.rs", "value.parse::<u64>()"},
		{"contracts/bridge/solana/admin/src/main.rs", "asset.per_tx_cap.to_be_bytes()"},
		{"contracts/bridge/solana/admin/src/main.rs", "asset.total_cap.to_be_bytes()"},
		{"contracts/bridge/solana/src/admin.rs", "let per_tx_cap = reader.u64()?;"},
		{"contracts/bridge/solana/src/admin.rs", "let total_cap = reader.u64()?;"},
		{"contracts/bridge/solana/src/state.rs", "pub per_tx_cap: u64,"},
		{"contracts/bridge/solana/src/state.rs", "pub total_cap: u64,"},
	} {
		source, err := os.ReadFile(filepath.Join(root, check.path))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(source, []byte(check.declaration)) {
			t.Fatalf("the destination no longer uses the checked u64 transport: %s: %s", check.path, check.declaration)
		}
	}
	for index := range config.Assets {
		var payload bytes.Buffer
		for _, decimal := range []string{perTx[index], total[index]} {
			amount, err := strconv.ParseUint(decimal, 10, 64)
			if err != nil {
				t.Fatalf("destination u64 parser refused %s: %v", decimal, err)
			}
			if err := binary.Write(&payload, binary.BigEndian, amount); err != nil {
				t.Fatal(err)
			}
		}
		if payload.Len() != 16 {
			t.Fatalf("two u64 caps occupied %d bytes", payload.Len())
		}
		for field, want := range []string{perTx[index], total[index]} {
			var recovered uint64
			if err := binary.Read(&payload, binary.BigEndian, &recovered); err != nil {
				t.Fatal(err)
			}
			if got := strconv.FormatUint(recovered, 10); got != want {
				t.Fatalf("u64 transport changed cap %d/%d: %s != %s", index, field, got, want)
			}
		}
	}
	_, overflow := capBoundaryValues(KindSolana)
	if _, err := strconv.ParseUint(overflow, 10, 64); err == nil {
		t.Fatal("the destination parser accepted u64 maximum+1")
	}
}
