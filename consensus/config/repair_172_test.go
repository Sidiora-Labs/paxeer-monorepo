package config_test

import (
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/cmd/tendermint/commands"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
)

func repair172WriteAndReadConfig(t *testing.T, cfg *config.Config) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)

	rootDir := t.TempDir()
	config.EnsureRoot(rootDir)
	require.NoError(t, config.WriteConfigFile(rootDir, cfg))
	viper.SetConfigFile(filepath.Join(rootDir, "config", "config.toml"))
	require.NoError(t, viper.ReadInConfig())
}

func TestRepair_172_AutobahnPathRoundTrip(t *testing.T) {
	for _, path := range []string{"", "config/autobahn network.json", filepath.Join(t.TempDir(), "autobahn.json")} {
		t.Run(path, func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.AutobahnConfigFile = path
			repair172WriteAndReadConfig(t, cfg)

			decoded, err := commands.ParseConfig(config.DefaultConfig())
			require.NoError(t, err)
			require.Equal(t, path, decoded.AutobahnConfigFile)
			require.Equal(t, cfg.SelfRemediation, decoded.SelfRemediation)
		})
	}
}

func TestRepair_172_SequencerAutobahnConflict(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Consensus.Sequencer.Enable = true
	require.NoError(t, cfg.ValidateBasic())
	cfg.AutobahnConfigFile = "config/autobahn.json"
	repair172WriteAndReadConfig(t, cfg)

	decoded := config.DefaultConfig()
	require.NoError(t, viper.Unmarshal(decoded))
	require.Equal(t, cfg.AutobahnConfigFile, decoded.AutobahnConfigFile)
	require.True(t, decoded.Consensus.Sequencer.Enable)
	require.Error(t, decoded.ValidateBasic())

	parsed, err := commands.ParseConfig(config.DefaultConfig())
	require.Error(t, err)
	require.Nil(t, parsed)

	decoded.Consensus.Sequencer.Enable = false
	require.NoError(t, decoded.ValidateBasic())
	decoded.Consensus.Sequencer.Enable = true
	decoded.AutobahnConfigFile = ""
	require.NoError(t, decoded.ValidateBasic())
}
