// Proves: StateCommitConfig + StateCommitConfigTemplate -> app.toml + viper.Viper -> ParseFlatKVConfig + config.Config.
// The snapshot-writer limit crosses the same file and dotted-key lookup used by
// node/paxdb.go:parseSCConfigs; that unexported function is outside this package.
// Repairs storage/config/flatkv_parse.go:ParseFlatKVConfig silently wrapping
// snapshot counts above uint32 and truncating fractional counts. The template in
// storage/config/toml.go now documents the enforced bounds. Malformed cases edit
// a real producer's persisted document, then use the same Viper/parser crossing.
package config

import (
	"bytes"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"text/template"

	"github.com/spf13/cast"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func TestProofNodeConfig(t *testing.T) {
	defaults := DefaultStateCommitConfig()
	custom := DefaultStateCommitConfig()
	custom.FlatKVConfig.Fsync = true
	custom.FlatKVConfig.AsyncWriteBuffer = 257
	custom.FlatKVConfig.SnapshotInterval = 1234
	custom.FlatKVConfig.SnapshotKeepRecent = 7
	custom.MemIAVLConfig.SnapshotWriterLimit = 9
	// Distinct values make an accidental read from the MemIAVL namespace visible.
	custom.MemIAVLConfig.SnapshotInterval = 999
	custom.MemIAVLConfig.SnapshotKeepRecent = 13

	synchronous := DefaultStateCommitConfig()
	synchronous.FlatKVConfig.AsyncWriteBuffer = -1
	synchronous.FlatKVConfig.SnapshotInterval = 0
	synchronous.FlatKVConfig.SnapshotKeepRecent = 0
	synchronous.MemIAVLConfig.SnapshotWriterLimit = 1

	boundary := DefaultStateCommitConfig()
	boundary.FlatKVConfig.SnapshotInterval = math.MaxUint32
	boundary.FlatKVConfig.SnapshotKeepRecent = math.MaxUint32

	for _, tc := range []struct {
		name string
		want StateCommitConfig
	}{
		{"defaults", defaults},
		{"operator overrides", custom},
		{"synchronous writes and disabled snapshots", synchronous},
		{"maximum snapshot counts", boundary},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "app.toml")
			writeProofNodeConfig(t, path, tc.want)
			original, err := os.ReadFile(path)
			require.NoError(t, err)

			// The producer has returned and closed its file. Every observation
			// below constructs a new loader, so it cannot reuse in-memory state.
			for restart := 0; restart < 2; restart++ {
				opts := readProofNodeConfig(t, path)
				for _, key := range []string{
					"state-commit.flatkv.fsync",
					"state-commit.flatkv.async-write-buffer",
					"state-commit.flatkv.snapshot-interval",
					"state-commit.flatkv.snapshot-keep-recent",
					"state-commit.sc-snapshot-writer-limit",
				} {
					require.True(t, opts.IsSet(key), "producer must persist %s", key)
				}
				got, err := ParseFlatKVConfig(opts)
				require.NoError(t, err)
				require.Equal(t, tc.want.FlatKVConfig, got)
				// This is the consumer expression in node/paxdb.go:parseSCConfigs.
				limit, err := cast.ToIntE(opts.Get("state-commit.sc-snapshot-writer-limit"))
				require.NoError(t, err)
				require.Equal(t, tc.want.MemIAVLConfig.SnapshotWriterLimit, limit)
			}

			// Repeating the producer is byte-for-byte idempotent and remains
			// consumable after another close and reopen.
			writeProofNodeConfig(t, path, tc.want)
			repeated, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, original, repeated)
			opts := readProofNodeConfig(t, path)
			got, err := ParseFlatKVConfig(opts)
			require.NoError(t, err)
			require.Equal(t, tc.want.FlatKVConfig, got)
			require.Equal(t, tc.want.MemIAVLConfig.SnapshotWriterLimit,
				cast.ToInt(opts.Get("state-commit.sc-snapshot-writer-limit")))
		})
	}

	t.Run("refuses malformed persisted options", func(t *testing.T) {
		for _, tc := range []struct {
			name        string
			line        string
			replacement string
			key         string
			rangeError  bool
		}{
			{"invalid fsync", "fsync = true", `fsync = "invalid"`, "fsync", false},
			{"invalid buffer", "async-write-buffer = 257", `async-write-buffer = "invalid"`, "async-write-buffer", false},
			{"negative interval", "snapshot-interval = 1234", "snapshot-interval = -1", "snapshot-interval", false},
			{"negative retention", "snapshot-keep-recent = 7", "snapshot-keep-recent = -1", "snapshot-keep-recent", false},
			{"overflow interval", "snapshot-interval = 1234", "snapshot-interval = 4294967296", "snapshot-interval", true},
			{"overflow retention", "snapshot-keep-recent = 7", "snapshot-keep-recent = 4294967296", "snapshot-keep-recent", true},
			{"fractional interval", "snapshot-interval = 1234", "snapshot-interval = 1.5", "snapshot-interval", false},
			{"fractional retention", "snapshot-keep-recent = 7", "snapshot-keep-recent = 1.5", "snapshot-keep-recent", false},
		} {
			t.Run(tc.name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "app.toml")
				writeProofNodeConfig(t, path, custom)
				original, err := os.ReadFile(path)
				require.NoError(t, err)
				before := []byte("\n" + tc.line + "\n")
				require.Equal(t, 1, bytes.Count(original, before))
				malformed := bytes.Replace(original, before, []byte("\n"+tc.replacement+"\n"), 1)
				require.NoError(t, os.WriteFile(path, malformed, 0o600))
				for restart := 0; restart < 2; restart++ {
					opts := readProofNodeConfig(t, path)
					_, err := ParseFlatKVConfig(opts)
					require.ErrorContains(t, err, "state-commit.flatkv."+tc.key+": ")
					require.NotNil(t, errors.Unwrap(err))
					if tc.rangeError {
						require.ErrorIs(t, err, strconv.ErrRange)
					}
				}
				persisted, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, malformed, persisted, "refusal must not rewrite operator input")
			})
		}
	})
}

func writeProofNodeConfig(t *testing.T, path string, state StateCommitConfig) {
	t.Helper()
	tmpl, err := template.New("app.toml").Parse(StateCommitConfigTemplate)
	require.NoError(t, err)
	file, err := os.OpenFile(path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	require.NoError(t, err)
	err = tmpl.Execute(file, struct{ StateCommit StateCommitConfig }{StateCommit: state})
	if err != nil {
		file.Close()
		t.Fatal(err)
	}
	syncErr := file.Sync()
	closeErr := file.Close()
	require.NoError(t, syncErr)
	require.NoError(t, closeErr)
}

func readProofNodeConfig(t *testing.T, path string) *viper.Viper {
	t.Helper()
	opts := viper.New()
	opts.SetConfigFile(path)
	require.NoError(t, opts.ReadInConfig())
	return opts
}
