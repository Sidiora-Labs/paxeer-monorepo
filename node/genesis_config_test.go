package app

import (
	"bytes"
	"strings"
	"testing"
	"text/template"

	serverconfig "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/config"
	genesistypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/genesis"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestReadGenesisImportConfigReadsTemplateKey(t *testing.T) {
	cfg := serverconfig.DefaultConfig()
	cfg.Genesis.StreamImport = true
	cfg.Genesis.GenesisStreamFile = "config/genesis-stream.json"

	tmpl, err := template.New("app").Parse(serverconfig.AutoManagedConfigTemplate)
	require.NoError(t, err)
	var rendered bytes.Buffer
	require.NoError(t, tmpl.Execute(&rendered, cfg))

	opts := viper.New()
	opts.SetConfigType("toml")
	require.NoError(t, opts.ReadConfig(&rendered))

	got, err := ReadGenesisImportConfig(opts)
	require.NoError(t, err)
	require.Equal(t, genesistypes.GenesisImportConfig{
		StreamGenesisImport: true,
		GenesisStreamFile:   cfg.Genesis.GenesisStreamFile,
	}, got)
}

func TestReadGenesisImportConfigRefusesNonString(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value string
	}{
		{name: "array", value: `["genesis.json"]`},
		{name: "table", value: `{ path = "genesis.json" }`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := viper.New()
			opts.SetConfigType("toml")
			require.NoError(t, opts.ReadConfig(strings.NewReader(
				"[genesis]\nstream-import = true\ngenesis-stream-file = "+tc.value+"\n",
			)))

			got, err := ReadGenesisImportConfig(opts)
			require.Error(t, err)
			require.True(t, got.StreamGenesisImport)
			require.Empty(t, got.GenesisStreamFile)
		})
	}
}
