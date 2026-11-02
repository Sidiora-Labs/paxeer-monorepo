package app

import (
	"bytes"
	"testing"
	"text/template"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
)

func TestAppTomlStateCommitKeysRoundTrip(t *testing.T) {
	want := config.DefaultStateCommitConfig()
	want.Enable = false
	want.Directory = t.TempDir()
	want.HistoricalProofMaxInFlight = 7
	want.HistoricalProofRateLimit = 12.5
	want.HistoricalProofBurst = 9
	want.MemIAVLConfig.AsyncCommitBuffer = 23
	want.MemIAVLConfig.SnapshotKeepRecent = 3
	want.MemIAVLConfig.SnapshotInterval = 257
	want.MemIAVLConfig.SnapshotMinTimeInterval = 127
	want.MemIAVLConfig.SnapshotWriterLimit = 7
	want.MemIAVLConfig.SnapshotPrefetchThreshold = 0.625
	want.MemIAVLConfig.SnapshotWriteRateMBps = 37
	want.WriteMode = config.MigrateEVM
	want.KeysToMigratePerBlock = 73
	want.FlatKVConfig.Fsync = true
	want.FlatKVConfig.AsyncWriteBuffer = 29
	want.FlatKVConfig.SnapshotInterval = 271
	want.FlatKVConfig.SnapshotKeepRecent = 5

	tmpl, err := template.New("app.toml").Parse(config.StateCommitConfigTemplate)
	require.NoError(t, err)
	var rendered bytes.Buffer
	require.NoError(t, tmpl.Execute(&rendered, struct {
		StateCommit config.StateCommitConfig
	}{StateCommit: want}))

	opts := viper.New()
	opts.SetConfigType("toml")
	require.NoError(t, opts.ReadConfig(&rendered))
	for _, key := range []string{
		FlagSCEnable,
		FlagSCDirectory,
		FlagSCHistoricalProofMaxInFlight,
		FlagSCHistoricalProofRateLimit,
		FlagSCHistoricalProofBurst,
		FlagSCAsyncCommitBuffer,
		FlagSCSnapshotKeepRecent,
		FlagSCSnapshotInterval,
		FlagSCSnapshotMinTimeInterval,
		FlagSCSnapshotWriterLimit,
		FlagSCSnapshotPrefetchThreshold,
		FlagSCSnapshotWriteRateMBps,
		FlagSCWriteMode,
		FlagSCKeysToMigratePerBlock,
		"state-commit.flatkv.fsync",
		"state-commit.flatkv.async-write-buffer",
		"state-commit.flatkv.snapshot-interval",
		"state-commit.flatkv.snapshot-keep-recent",
	} {
		require.True(t, opts.IsSet(key), "template must write %s", key)
	}

	require.Equal(t, want, parseSCConfigs(opts))
}
