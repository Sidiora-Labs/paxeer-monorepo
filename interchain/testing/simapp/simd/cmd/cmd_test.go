package cmd_test

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	svrcmd "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/cmd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/genutil/client/cli"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/simapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/interchain/testing/simapp/simd/cmd"
)

func TestInitCmd(t *testing.T) {
	home := t.TempDir()
	defaultHome := t.TempDir()
	previousDefault := simapp.DefaultNodeHome
	simapp.DefaultNodeHome = defaultHome
	t.Cleanup(func() { simapp.DefaultNodeHome = previousDefault })
	t.Cleanup(viper.Reset)
	defaultGenesis := filepath.Join(defaultHome, "config", "genesis.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(defaultGenesis), 0o700))
	preserved := []byte("existing developer genesis")
	require.NoError(t, os.WriteFile(defaultGenesis, preserved, 0o600))

	for _, chainID := range []string{"init-before-overwrite", "init-after-overwrite"} {
		viper.Reset()
		rootCmd, _ := cmd.NewRootCmd()
		rootCmd.SetOut(io.Discard)
		rootCmd.SetErr(io.Discard)
		rootCmd.SetArgs([]string{
			"init",
			"simapp-test",
			fmt.Sprintf("--%s=true", cli.FlagOverwrite),
			fmt.Sprintf("--%s=%s", flags.FlagHome, home),
			fmt.Sprintf("--%s=%s", flags.FlagChainID, chainID),
		})
		require.NoError(t, svrcmd.Execute(rootCmd, home))
		genesis, err := types.GenesisDocFromFile(filepath.Join(home, "config", "genesis.json"))
		require.NoError(t, err)
		require.Equal(t, chainID, genesis.ChainID)
		actual, err := os.ReadFile(defaultGenesis)
		require.NoError(t, err)
		require.Equal(t, preserved, actual)
		entries, err := os.ReadDir(defaultHome)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		entries, err = os.ReadDir(filepath.Dir(defaultGenesis))
		require.NoError(t, err)
		require.Len(t, entries, 1)
	}
}
