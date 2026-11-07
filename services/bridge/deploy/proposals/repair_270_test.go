package proposals

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/contracts/bridge/vectors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge"
	bridgetestutil "github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/testutil"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/layerxbridge/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/services/bridge/deploy/chainconfig"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm/runtime"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

func repair270WriteJSON(t *testing.T, name string, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	require.NoError(t, err)
	path := filepath.Join(t.TempDir(), name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, raw, 0o600))
	return path
}

// Keep both command schemas on the same asset list, including mutations that
// would otherwise bypass the canonical loader through the legacy interface.
func repair270Input(t *testing.T, chain, schema string, mutate func(*ChainConfig)) (ChainConfig, string, []string) {
	t.Helper()
	fixture := ethereumPath
	if chain == "solana" {
		fixture = solanaPath
	}
	legacy, err := LoadChainConfig(fixture)
	require.NoError(t, err)
	if schema == "legacy" {
		mutate(&legacy)
		return legacy, repair270WriteJSON(t, "config.json", legacy), nil
	}
	canonical := loadCanonical(t, deployableConfiguration(t, chain))
	vault := legacy.Vault
	if canonical.Solana != nil {
		program, err := vectors.Key(canonical.Solana.ProgramID)
		require.NoError(t, err)
		handle, err := vectors.VaultHandle(program)
		require.NoError(t, err)
		vault = handle.Hex()
	}
	cfg, err := FromChainConfig(canonical, governanceAuthority, vault)
	require.NoError(t, err)
	mutate(&cfg)
	canonical.Assets = make([]chainconfig.Asset, len(cfg.Assets))
	for i, asset := range cfg.Assets {
		symbol := fmt.Sprintf("TOKEN%d", i)
		if i == 0 {
			symbol = canonical.Native.Symbol
		}
		canonical.Assets[i] = chainconfig.Asset{
			Symbol: symbol, Address: asset.Address, AssetID: asset.AssetID,
			Decimals: uint8(asset.Decimals), PerTxCap: asset.MaxPerTx, TotalCap: asset.MaxTotal,
		}
	}
	canonical.Attestors = cfg.Attestors
	canonical.Threshold = cfg.Threshold
	return cfg, repair270WriteJSON(t, filepath.Join(chain, "config.json"), canonical), []string{"-authority", cfg.Authority, "-vault", cfg.Vault}
}

func TestRepair_270_RejectMismatchedBindings(t *testing.T) {
	t.Setenv(chainconfig.OverlayEnv, "")
	other := "0x7a3e5c81b04d296f8e1a7c35d92b6f04e8c1a37d"
	for _, tc := range []struct {
		name, chain, field string
		mutate             func(*ChainConfig)
	}{
		{"evm-native-id", "ethereum", "assets[0].asset_id", func(c *ChainConfig) { c.Assets[0].AssetID = other }},
		{"evm-token-id", "ethereum", "assets[1].asset_id", func(c *ChainConfig) {
			c.Assets = append(c.Assets, Asset{Address: other, AssetID: "0x2b7e151628aed2a6abf7158809cf4f3c762e7160", Decimals: 18, MaxPerTx: "100", MaxTotal: "1000"})
		}},
		{"evm-malformed-address", "ethereum", "assets[1].address", func(c *ChainConfig) {
			c.Assets = append(c.Assets, Asset{Address: "0x1234", AssetID: other, Decimals: 18, MaxPerTx: "100", MaxTotal: "1000"})
		}},
		{"solana-native-id", "solana", "assets[0].asset_id", func(c *ChainConfig) { c.Assets[0].AssetID = other }},
		{"solana-mint-handle", "solana", "assets[2].asset_id", func(c *ChainConfig) {
			c.Assets = append(c.Assets, Asset{Address: solanaOwner(), AssetID: other, Decimals: 6, MaxPerTx: "100", MaxTotal: "1000"})
		}},
		{"sidiora-id-on-other-mint", "solana", "assets[1].asset_id", func(c *ChainConfig) { c.Assets[1].Address = solanaOwner() }},
		{"sidiora-derived-handle", "solana", "assets[1].asset_id", func(c *ChainConfig) {
			handle, err := chainconfig.SolanaHandle(chainconfig.SidioraMint)
			require.NoError(t, err)
			c.Assets[1].AssetID = handle
		}},
		{"sidiora-decimals", "solana", "assets[1].decimals", func(c *ChainConfig) { c.Assets[1].Decimals = 9 }},
		{"solana-zero-mint", "solana", "assets[1].address", func(c *ChainConfig) { c.Assets[1].Address = chainconfig.SolanaZeroPubkey }},
	} {
		for _, schema := range []string{"legacy", "canonical"} {
			t.Run(tc.name+"/"+schema, func(t *testing.T) {
				cfg, input, flags := repair270Input(t, tc.chain, schema, tc.mutate)
				caps, err := SetCaps(cfg)
				require.Error(t, err)
				require.Nil(t, caps)
				require.Equal(t, tc.field, fieldError(t, err).Field)
				bundle, err := Generate(cfg, loadManifest(t, testManifest))
				require.Error(t, err)
				require.Equal(t, Bundle{}, bundle)
				out := filepath.Join(t.TempDir(), "bodies")
				proposals := filepath.Join(t.TempDir(), "proposals")
				args := append([]string{"-manifest", testManifest, "-proposals", proposals}, flags...)
				var report bytes.Buffer
				err = Run(append(args, input, out), &report)
				require.ErrorContains(t, err, tc.field)
				require.NoDirExists(t, out)
				require.NoDirExists(t, proposals)
				require.Empty(t, report.String())
			})
		}
	}
}

func TestRepair_270_AcceptSolanaBindings(t *testing.T) {
	t.Setenv(chainconfig.OverlayEnv, "")
	for _, schema := range []string{"legacy", "canonical"} {
		t.Run(schema, func(t *testing.T) {
			mint := solanaOwner()
			handle, err := chainconfig.SolanaHandle(mint)
			require.NoError(t, err)
			cfg, input, flags := repair270Input(t, "solana", schema, func(c *ChainConfig) {
				c.Assets = append(c.Assets, Asset{Address: mint, AssetID: "0x" + strings.ToUpper(handle[2:]), Decimals: 6, MaxPerTx: "100", MaxTotal: "1000"})
			})
			out := filepath.Join(t.TempDir(), "bodies")
			require.NoError(t, Run(append(append([]string{"-manifest", testManifest}, flags...), input, out), &bytes.Buffer{}))
			bundle, err := Generate(cfg, loadManifest(t, testManifest))
			require.NoError(t, err)
			require.Len(t, bundle.Caps, 3)
			require.Equal(t, wrappedSOLAssetID, bundle.Caps[0].Asset.Hex())
			require.Equal(t, chainconfig.SidioraMint, cfg.Assets[1].Address)
			require.Equal(t, SidioraAssetID(), bundle.Caps[1].Asset)
			require.Equal(t, handle, bundle.Caps[2].Asset.Hex())
			files, err := bundle.Files()
			require.NoError(t, err)
			for _, file := range files {
				raw, err := os.ReadFile(filepath.Join(out, file.Name))
				require.NoError(t, err)
				require.JSONEq(t, string(file.Body), string(raw))
			}
		})
	}
}

func TestRepair_270_VaultDepositReachesKeeper(t *testing.T) {
	t.Setenv(chainconfig.OverlayEnv, "")
	// Compile the maintained Solidity producer, never a substitute event emitter.
	compiler := exec.Command("node", "testdata/repair_270/compile-vault.cjs")
	var stderr bytes.Buffer
	compiler.Stderr = &stderr
	compiled, err := compiler.Output()
	require.NoError(t, err, "%s", stderr.String())
	var contract struct {
		ABI      json.RawMessage `json:"abi"`
		Bytecode string          `json:"bytecode"`
	}
	require.NoError(t, json.Unmarshal(compiled, &contract))
	vaultABI, err := abi.JSON(bytes.NewReader(contract.ABI))
	require.NoError(t, err)
	bytecode, err := hex.DecodeString(contract.Bytecode)
	require.NoError(t, err)
	require.NotEmpty(t, bytecode)
	attestors := bridgetestutil.Attestors(3)
	signers := make([]string, len(attestors))
	evmSigners := make([]common.Address, len(attestors))
	for i, attestor := range attestors {
		signers[i] = attestor.Signer.Hex()
		evmSigners[i] = common.Address(attestor.Signer)
	}
	manifest := repair270WriteJSON(t, "attestors.json", AttestorManifest{Attestors: signers, Threshold: 2})
	for _, schema := range []string{"legacy", "canonical"} {
		t.Run(schema, func(t *testing.T) {
			owner := common.HexToAddress("0x2b7e151628aed2a6abf7158809cf4f3c762e7160")
			remote, err := state.New(ethtypes.EmptyRootHash, state.NewDatabaseForTesting())
			require.NoError(t, err)
			remote.SetBalance(owner, uint256.NewInt(10000), tracing.BalanceChangeUnspecified)
			evm := &runtime.Config{Origin: owner, State: remote, GasLimit: 15000000}
			constructor, err := vaultABI.Pack("", owner, evmSigners, big.NewInt(2))
			require.NoError(t, err)
			_, vault, _, err := runtime.Create(append(append([]byte(nil), bytecode...), constructor...), evm)
			require.NoError(t, err)
			cfg, input, flags := repair270Input(t, "ethereum", schema, func(c *ChainConfig) {
				c.Vault = vault.Hex()
				c.Attestors, c.Threshold = signers, 2
				c.Assets[0].MaxPerTx, c.Assets[0].MaxTotal = "100", "1000"
			})
			out, proposals := filepath.Join(t.TempDir(), "bodies"), filepath.Join(t.TempDir(), "proposals")
			args := append([]string{"-manifest", manifest, "-proposals", proposals}, flags...)
			require.NoError(t, Run(append(args, input, out), &bytes.Buffer{}))
			testApp := app.Setup(t, false, false, false)
			ctx := testApp.GetContextForDeliverTx(nil).WithBlockHeight(8)
			k, ctx := bridgetestutil.NewKeeper(testApp, ctx)
			k.InitGenesis(ctx, *types.DefaultGenesis())
			proposalJSON, err := os.ReadFile(filepath.Join(proposals, OpenChainProposalFile))
			require.NoError(t, err)
			proposal, err := DecodeProposal(proposalJSON)
			require.NoError(t, err)
			require.NoError(t, layerxbridge.NewProposalHandler(k)(ctx, proposal))
			// Configure the remote side from the address and limits, independently
			// of the asset ID governance read from the proposal input.
			capCall, err := vaultABI.Pack("setCap", common.HexToAddress(cfg.Assets[0].Address), big.NewInt(100), big.NewInt(1000))
			require.NoError(t, err)
			_, _, err = runtime.Call(vault, capCall, evm)
			require.NoError(t, err)
			recipient := common.HexToAddress("0x00000000000000000000000000000000000a11ce")
			padded := common.BytesToHash(recipient.Bytes())
			depositCall, err := vaultABI.Pack("depositNative", [32]byte(padded))
			require.NoError(t, err)
			tx := ethtypes.NewTx(&ethtypes.LegacyTx{Nonce: 1, To: &vault, Value: big.NewInt(42), Gas: evm.GasLimit, Data: depositCall})
			remote.SetTxContext(tx.Hash(), 0)
			evm.Value = tx.Value()
			_, _, err = runtime.Call(vault, tx.Data(), evm)
			require.NoError(t, err)
			logs := remote.GetLogs(tx.Hash(), 1, crypto.Keccak256Hash([]byte("repair-270-block")))
			require.Len(t, logs, 1)
			wire, err := json.Marshal(logs[0])
			require.NoError(t, err)
			var deposit ethtypes.Log
			require.NoError(t, json.Unmarshal(wire, &deposit))
			event := vaultABI.Events["BridgeDeposit"]
			require.Len(t, deposit.Topics, 4)
			require.Equal(t, event.ID, deposit.Topics[0])
			fields := map[string]any{}
			require.NoError(t, event.Inputs.NonIndexed().UnpackIntoMap(fields, deposit.Data))
			var indexed abi.Arguments
			for _, input := range event.Inputs {
				if input.Indexed {
					indexed = append(indexed, input)
				}
			}
			require.NoError(t, abi.ParseTopicsIntoMap(fields, indexed, deposit.Topics[1:]))
			in := types.BridgeIn{ChainID: cfg.ChainID, Vault: types.Address20(deposit.Address),
				TxHash: types.Hash32(deposit.TxHash), LogIndex: uint64(deposit.Index),
				Recipient: types.Hash32(fields["paxeerRecipient"].([32]byte)),
				Asset:     types.Address20(fields["asset"].(common.Address)), Amount: fields["amount"].(*big.Int)}
			require.Equal(t, types.Address20{}, in.Asset)
			require.Equal(t, big.NewInt(42), in.Amount)
			record, found := k.GetAsset(ctx, in.ChainID, in.Asset)
			require.True(t, found, "the emitted asset must be the capped asset")
			limits, found := k.GetCap(ctx, record.Denom)
			require.True(t, found)
			require.Equal(t, "100", limits.MaxPerTx.String())
			require.Equal(t, "1000", limits.MaxInFlight.String())
			result, err := k.BridgeIn(ctx, in, bridgetestutil.Sign(types.InboundDigest(in), attestors[:2]...))
			require.NoError(t, err)
			require.Equal(t, types.Denom(cfg.ChainID, in.Asset), result.Denom)
			account := testApp.EvmKeeper.GetPaxAddressOrDefault(ctx, recipient)
			require.Equal(t, sdk.NewInt(42), testApp.BankKeeper.GetBalance(ctx, account, result.Denom).Amount)
			require.Equal(t, sdk.NewInt(42), k.InFlight(ctx, result.Denom))
			require.True(t, k.IsNullified(ctx, in.Nullifier()))
			_, err = k.BridgeIn(ctx, in, bridgetestutil.Sign(types.InboundDigest(in), attestors[:2]...))
			require.ErrorIs(t, err, types.ErrNullified)
		})
	}
}
