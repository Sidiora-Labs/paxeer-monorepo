package config

import (
	"bytes"
	"errors"
	"testing"
	"text/template"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/require"

	flatkvconfig "github.com/Sidiora-Labs/Paxeer-X-Network/storage/state_db/sc/flatkv/config"
)

type flatKVAppOptions map[string]interface{}

func (opts flatKVAppOptions) Get(key string) interface{} {
	return opts[key]
}

func TestParseFlatKVConfigReadsTemplateKeys(t *testing.T) {
	t.Run("template values", func(t *testing.T) {
		stateCommit := DefaultStateCommitConfig()
		stateCommit.FlatKVConfig.Fsync = true
		stateCommit.FlatKVConfig.AsyncWriteBuffer = 64
		stateCommit.FlatKVConfig.SnapshotInterval = 250
		stateCommit.FlatKVConfig.SnapshotKeepRecent = 7

		tmpl, err := template.New("state-commit").Parse(StateCommitConfigTemplate)
		require.NoError(t, err)
		var rendered bytes.Buffer
		require.NoError(t, tmpl.Execute(&rendered, struct {
			StateCommit StateCommitConfig
		}{StateCommit: stateCommit}))

		var document map[string]interface{}
		_, err = toml.Decode(rendered.String(), &document)
		require.NoError(t, err)
		commitTable, ok := document["state-commit"].(map[string]interface{})
		require.True(t, ok)
		flatkvTable, ok := commitTable["flatkv"].(map[string]interface{})
		require.True(t, ok)
		opts := flatKVAppOptions{}
		for _, key := range []string{"fsync", "async-write-buffer", "snapshot-interval", "snapshot-keep-recent"} {
			value, exists := flatkvTable[key]
			require.True(t, exists, "template must define %s", key)
			opts["state-commit.flatkv."+key] = value
		}

		cfg, err := ParseFlatKVConfig(opts)
		require.Equal(t, stateCommit.FlatKVConfig, cfg)
		require.EqualError(t, err, "data dir is required")
	})

	for _, tc := range []struct {
		name string
		opts flatKVAppOptions
		want flatkvconfig.Config
	}{
		{
			name: "string values",
			opts: flatKVAppOptions{
				"state-commit.flatkv.fsync":                "true",
				"state-commit.flatkv.async-write-buffer":   "32",
				"state-commit.flatkv.snapshot-interval":    "500",
				"state-commit.flatkv.snapshot-keep-recent": "5",
			},
			want: func() flatkvconfig.Config {
				cfg := *flatkvconfig.DefaultConfig()
				cfg.Fsync = true
				cfg.AsyncWriteBuffer = 32
				cfg.SnapshotInterval = 500
				cfg.SnapshotKeepRecent = 5
				return cfg
			}(),
		},
		{
			name: "explicit zero and synchronous writes",
			opts: flatKVAppOptions{
				"state-commit.flatkv.fsync":                false,
				"state-commit.flatkv.async-write-buffer":   -1,
				"state-commit.flatkv.snapshot-interval":    0,
				"state-commit.flatkv.snapshot-keep-recent": 0,
			},
			want: func() flatkvconfig.Config {
				cfg := *flatkvconfig.DefaultConfig()
				cfg.AsyncWriteBuffer = -1
				cfg.SnapshotInterval = 0
				cfg.SnapshotKeepRecent = 0
				return cfg
			}(),
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseFlatKVConfig(tc.opts)
			require.Equal(t, tc.want, cfg)
			require.EqualError(t, err, "data dir is required")
		})
	}
}

func TestParseFlatKVConfigKeepsDefaultsForAbsentKeys(t *testing.T) {
	for _, tc := range []struct {
		name string
		opts flatKVAppOptions
	}{
		{name: "absent keys", opts: flatKVAppOptions{}},
		{name: "nil map"},
		{
			name: "explicit nil values",
			opts: flatKVAppOptions{
				"state-commit.flatkv.fsync":                nil,
				"state-commit.flatkv.async-write-buffer":   nil,
				"state-commit.flatkv.snapshot-interval":    nil,
				"state-commit.flatkv.snapshot-keep-recent": nil,
			},
		},
		{
			name: "other namespaces",
			opts: flatKVAppOptions{
				"flatkv.fsync":                      true,
				"state-commit.async-write-buffer":   64,
				"state-commit.sc-snapshot-interval": 99,
				"state-commit.sc-keep-recent":       4,
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseFlatKVConfig(tc.opts)
			require.Equal(t, *flatkvconfig.DefaultConfig(), cfg)
			require.EqualError(t, err, "data dir is required")
		})
	}

	t.Run("partially specified", func(t *testing.T) {
		want := *flatkvconfig.DefaultConfig()
		want.Fsync = true
		cfg, err := ParseFlatKVConfig(flatKVAppOptions{"state-commit.flatkv.fsync": true})
		require.Equal(t, want, cfg)
		require.EqualError(t, err, "data dir is required")
	})
}

func TestParseFlatKVConfigRefusesInvalidValues(t *testing.T) {
	for _, tc := range []struct {
		key   string
		value interface{}
	}{
		{key: "state-commit.flatkv.fsync", value: "invalid"},
		{key: "state-commit.flatkv.async-write-buffer", value: "invalid"},
		{key: "state-commit.flatkv.snapshot-interval", value: "invalid"},
		{key: "state-commit.flatkv.snapshot-keep-recent", value: "invalid"},
		{key: "state-commit.flatkv.fsync", value: []int{1}},
		{key: "state-commit.flatkv.async-write-buffer", value: []int{1}},
		{key: "state-commit.flatkv.snapshot-interval", value: []int{1}},
		{key: "state-commit.flatkv.snapshot-keep-recent", value: []int{1}},
		{key: "state-commit.flatkv.snapshot-interval", value: -1},
		{key: "state-commit.flatkv.snapshot-keep-recent", value: -1},
	} {
		t.Run(tc.key+"/"+testingValueName(tc.value), func(t *testing.T) {
			_, err := ParseFlatKVConfig(flatKVAppOptions{tc.key: tc.value})
			require.ErrorContains(t, err, tc.key+": ")
			require.NotNil(t, errors.Unwrap(err), "cast error must be wrapped")
		})
	}

	t.Run("config validation", func(t *testing.T) {
		cfg, err := ParseFlatKVConfig(flatKVAppOptions{})
		require.Error(t, err)
		require.ErrorContains(t, err, "data dir")
		require.EqualError(t, err, cfg.Validate().Error())
	})
}

func testingValueName(value interface{}) string {
	switch value.(type) {
	case string:
		return "invalid string"
	case []int:
		return "invalid type"
	default:
		return "negative value"
	}
}
