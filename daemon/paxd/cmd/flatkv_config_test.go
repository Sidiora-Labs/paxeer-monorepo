package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"text/template"

	paxdbconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestAppTomlFlatKVSectionRoundTrips(t *testing.T) {
	appTemplate, appConfig := initAppConfig()
	config := appConfig.(CustomAppConfig)
	config.StateCommit.FlatKVConfig.Fsync = true
	config.StateCommit.FlatKVConfig.AsyncWriteBuffer = 257
	config.StateCommit.FlatKVConfig.SnapshotInterval = 1234
	config.StateCommit.FlatKVConfig.SnapshotKeepRecent = 7

	tmpl, err := template.New("app").Parse(appTemplate)
	require.NoError(t, err)
	var rendered bytes.Buffer
	require.NoError(t, tmpl.Execute(&rendered, config))

	appTomlPath := filepath.Join(t.TempDir(), "app.toml")
	require.NoError(t, os.WriteFile(appTomlPath, rendered.Bytes(), 0o600))
	v := viper.New()
	v.SetConfigFile(appTomlPath)
	require.NoError(t, v.ReadInConfig())

	got, err := paxdbconfig.ParseFlatKVConfig(v)
	require.NoError(t, err)
	require.Equal(t, config.StateCommit.FlatKVConfig, got)
}
