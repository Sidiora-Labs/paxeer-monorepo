package cmd_test

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/simapp/simd/cmd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	svrcmd "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/cmd"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authsigning "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/signing"
	banktypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/bank/types"
	stakingtypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/staking/types"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestRepairM_1279_InitTestHome(t *testing.T) {
	TestInitCmd(t)
}

func TestRepairM_1279_PrivateSeedPermissions(t *testing.T) {
	previousMask := syscall.Umask(0o022)
	t.Cleanup(func() { syscall.Umask(previousMask) })
	for _, existing := range []string{"new", "private", "legacy"} {
		t.Run(existing, func(t *testing.T) {
			home := t.TempDir()
			output := filepath.Join(home, "testnet")
			nodeDir := filepath.Join(output, "node0", "simd")
			seedDir := filepath.Join(nodeDir, "private")
			seedPath := filepath.Join(seedDir, "key_seed.json")
			if existing != "new" {
				require.NoError(t, os.MkdirAll(seedDir, 0o755))
				require.NoError(t, os.Chmod(seedDir, 0o755))
				path := seedPath
				if existing == "legacy" {
					path = filepath.Join(nodeDir, "key_seed.json")
				}
				require.NoError(t, os.WriteFile(path, []byte(strings.Repeat("old seed data", 1024)), 0o644))
				require.NoError(t, os.Chmod(path, 0o644))
			}

			viper.Reset()
			t.Cleanup(viper.Reset)
			root, enc := cmd.NewRootCmd()
			root.SetOut(io.Discard)
			root.SetErr(io.Discard)
			root.SetArgs([]string{
				"testnet", "--v=1", "--output-dir=" + output,
				"--home=" + home, "--chain-id=repair-m1279",
				"--keyring-backend=test", "--starting-ip-address=127.0.0.1",
			})
			require.NoError(t, svrcmd.Execute(root, home))

			info, err := os.Stat(seedDir)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0o700), info.Mode().Perm())
			info, err = os.Stat(seedPath)
			require.NoError(t, err)
			require.Equal(t, os.FileMode(0o600), info.Mode().Perm())
			_, err = os.Stat(filepath.Join(nodeDir, "key_seed.json"))
			require.True(t, os.IsNotExist(err), "no seed may remain in the public node directory")

			seedJSON, err := os.ReadFile(seedPath)
			require.NoError(t, err)
			var seed struct {
				Secret string `json:"secret"`
			}
			require.NoError(t, json.Unmarshal(seedJSON, &seed))
			require.NotEmpty(t, seed.Secret)
			recovered, err := keyring.NewInMemory().NewAccount("recovered", seed.Secret,
				keyring.DefaultBIP39Passphrase, sdk.GetConfig().GetFullBIP44Path(), hd.Secp256k1)
			require.NoError(t, err)
			kb, err := keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, nodeDir, strings.NewReader(""))
			require.NoError(t, err)
			stored, err := kb.Key("node0")
			require.NoError(t, err)
			require.Equal(t, stored.GetPubKey().Bytes(), recovered.GetPubKey().Bytes())

			gentxPath := filepath.Join(output, "gentxs", "node0.json")
			info, err = os.Stat(gentxPath)
			require.NoError(t, err)
			require.NotZero(t, info.Mode().Perm()&0o044, "public transactions remain readable")
			gentxJSON, err := os.ReadFile(gentxPath)
			require.NoError(t, err)
			gentx, err := enc.TxConfig.TxJSONDecoder()(gentxJSON)
			require.NoError(t, err)
			require.Len(t, gentx.GetMsgs(), 1)
			validator, ok := gentx.GetMsgs()[0].(*stakingtypes.MsgCreateValidator)
			require.True(t, ok)
			require.Equal(t, recovered.GetAddress().String(), validator.DelegatorAddress)
			signed, ok := gentx.(authsigning.SigVerifiableTx)
			require.True(t, ok)
			signatures, err := signed.GetSignaturesV2()
			require.NoError(t, err)
			require.Len(t, signatures, 1)
			require.Equal(t, recovered.GetPubKey().Bytes(), signatures[0].PubKey.Bytes())

			genesis, err := types.GenesisDocFromFile(filepath.Join(nodeDir, "config", "genesis.json"))
			require.NoError(t, err)
			require.Equal(t, "repair-m1279", genesis.ChainID)
			var state map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(genesis.AppState, &state))
			var bank banktypes.GenesisState
			require.NoError(t, enc.Marshaler.UnmarshalAsJSON(state[banktypes.ModuleName], &bank))
			require.Len(t, bank.Balances, 1)
			require.Equal(t, recovered.GetAddress().String(), bank.Balances[0].Address)
			require.True(t, bank.Balances[0].Coins.IsAllPositive())
		})
	}
}

func TestRepairM_1279_PrivateSeedWriteFailure(t *testing.T) {
	home := t.TempDir()
	output := filepath.Join(home, "testnet")
	nodeDir := filepath.Join(output, "node0", "simd")
	require.NoError(t, os.MkdirAll(nodeDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(nodeDir, "private"), []byte("directory obstruction"), 0o600))
	viper.Reset()
	t.Cleanup(viper.Reset)
	root, _ := cmd.NewRootCmd()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{
		"testnet", "--v=1", "--output-dir=" + output,
		"--home=" + home, "--chain-id=repair-m1279",
		"--keyring-backend=test", "--starting-ip-address=127.0.0.1",
	})
	err := svrcmd.Execute(root, home)
	require.Error(t, err)
	require.Contains(t, err.Error(), "private")
	_, err = os.Stat(filepath.Join(nodeDir, "key_seed.json"))
	require.True(t, os.IsNotExist(err))
	_, err = os.Stat(filepath.Join(output, "gentxs", "node0.json"))
	require.True(t, os.IsNotExist(err))
}
