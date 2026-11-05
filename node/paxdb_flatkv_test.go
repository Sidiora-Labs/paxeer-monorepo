package app

import (
	"strings"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/storage/config"
)

func TestParseSCConfigsReadsFlatKVKeys(t *testing.T) {
	for _, tc := range []struct {
		name               string
		input              string
		fsync              bool
		asyncWriteBuffer   int
		snapshotInterval   uint32
		snapshotKeepRecent uint32
	}{
		{
			name: "configured values",
			input: `[state-commit.flatkv]
fsync = true
async-write-buffer = 64
snapshot-interval = 250
snapshot-keep-recent = 7
`,
			fsync: true, asyncWriteBuffer: 64, snapshotInterval: 250, snapshotKeepRecent: 7,
		},
		{
			name: "explicit zero values",
			input: `[state-commit.flatkv]
fsync = false
async-write-buffer = 0
snapshot-interval = 0
snapshot-keep-recent = 0
`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := viper.New()
			opts.SetConfigType("toml")
			require.NoError(t, opts.ReadConfig(strings.NewReader(tc.input)))
			want := config.DefaultStateCommitConfig().FlatKVConfig
			want.Fsync = tc.fsync
			want.AsyncWriteBuffer = tc.asyncWriteBuffer
			want.SnapshotInterval = tc.snapshotInterval
			want.SnapshotKeepRecent = tc.snapshotKeepRecent

			var got config.StateCommitConfig
			require.NotPanics(t, func() { got = parseSCConfigs(opts) })
			require.Equal(t, want, got.FlatKVConfig)
		})
	}
}

func TestParseSCConfigsKeepsFlatKVDefaults(t *testing.T) {
	for _, partial := range []bool{false, true} {
		name := "absent keys"
		if partial {
			name = "partially configured"
		}
		t.Run(name, func(t *testing.T) {
			opts := viper.New()
			want := config.DefaultStateCommitConfig().FlatKVConfig
			if partial {
				opts.Set("state-commit.flatkv.fsync", true)
				want.Fsync = true
			}
			var got config.StateCommitConfig
			require.NotPanics(t, func() { got = parseSCConfigs(opts) })
			require.Equal(t, want, got.FlatKVConfig)
		})
	}
}

func TestParseSCConfigsPanicsOnBadFlatKVValue(t *testing.T) {
	for _, key := range []string{
		"state-commit.flatkv.fsync",
		"state-commit.flatkv.async-write-buffer",
		"state-commit.flatkv.snapshot-interval",
		"state-commit.flatkv.snapshot-keep-recent",
	} {
		t.Run(key, func(t *testing.T) {
			opts := viper.New()
			opts.Set(key, []int{1})
			_, err := config.ParseFlatKVConfig(opts)
			require.ErrorContains(t, err, key+": ")
			require.PanicsWithError(t, "error reading state-commit.flatkv config: "+err.Error(), func() {
				parseSCConfigs(opts)
			})
		})
	}
}
