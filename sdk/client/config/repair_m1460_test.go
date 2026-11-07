package config

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	tmcli "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
)

func TestRepairM_1460_StringRoundTrip(t *testing.T) {
	fields := []struct {
		key string
		set func(*ClientConfig, string)
	}{
		{flags.FlagChainID, func(c *ClientConfig, s string) { c.ChainID = s }},
		{flags.FlagKeyringBackend, func(c *ClientConfig, s string) { c.KeyringBackend = s }},
		{tmcli.OutputFlag, func(c *ClientConfig, s string) { c.Output = s }},
		{flags.FlagNode, func(c *ClientConfig, s string) { c.Node = s }},
		{flags.FlagBroadcastMode, func(c *ClientConfig, s string) { c.BroadcastMode = s }},
	}
	values := []struct {
		name, value string
	}{
		{"quotes", `chain-"quoted"-'literal'`},
		{"backslashes", `C:\node\config\new\test\`},
		{"newlines", "first\nsecond\r\nthird"},
		{"injection", "value\"\nnode = \"tcp://injected:26657\"\n#"},
		{"controls", "\x00\x01\b\t\f\r\x1f\x7f"},
		{"unicode", "網絡-α-🚀-\u2028-\u2029"},
		{"empty", ""},
	}
	for _, field := range fields {
		for _, value := range values {
			t.Run(field.key+"/"+value.name, func(t *testing.T) {
				path := t.TempDir()
				conf := defaultClientConfig()
				want := *conf
				field.set(&want, value.value)
				require.NoError(t, SetClientConfig(field.key, value.value, path, conf))

				// A fresh reader has no cached values from the producer.
				reopened, err := GetClientConfig(path, viper.New())
				require.NoError(t, err)
				require.Equal(t, want, *reopened)
				info, err := os.Stat(filepath.Join(path, "client.toml"))
				require.NoError(t, err)
				require.Equal(t, os.FileMode(0600), info.Mode().Perm())
			})
		}
	}
}

func TestRepairM_1460_InvalidStringPreservesConfig(t *testing.T) {
	for _, key := range []string{flags.FlagChainID, flags.FlagKeyringBackend, tmcli.OutputFlag, flags.FlagNode, flags.FlagBroadcastMode} {
		t.Run(key, func(t *testing.T) {
			path := t.TempDir()
			conf := defaultClientConfig()
			want := *conf
			file := filepath.Join(path, "client.toml")
			require.NoError(t, writeConfigToFile(file, conf))
			before, err := os.ReadFile(file)
			require.NoError(t, err)
			beforeInfo, err := os.Stat(file)
			require.NoError(t, err)

			err = SetClientConfig(key, string([]byte{'x', 0xff, 'y'}), path, conf)
			require.ErrorContains(t, err, "not valid UTF-8")
			after, err := os.ReadFile(file)
			require.NoError(t, err)
			require.Equal(t, before, after)
			afterInfo, err := os.Stat(file)
			require.NoError(t, err)
			require.True(t, os.SameFile(beforeInfo, afterInfo), "validation must precede replacement")
			reopened, err := GetClientConfig(path, viper.New())
			require.NoError(t, err)
			require.Equal(t, want, *reopened)
			entries, err := os.ReadDir(path)
			require.NoError(t, err)
			require.Len(t, entries, 1, "invalid input must not leave temporary files")
		})
	}
}

func TestRepairM_1460_AtomicReplacementRecovery(t *testing.T) {
	home := t.TempDir()
	path := filepath.Join(home, "config")
	require.NoError(t, ensureConfigPath(path))
	file := filepath.Join(path, "client.toml")
	conf := defaultClientConfig()
	conf.KeyringBackend = "memory"
	require.NoError(t, writeConfigToFile(file, conf))
	before, err := os.ReadFile(file)
	require.NoError(t, err)
	// An open reader and a hard link retain the last committed inode.
	oldReader, err := os.Open(file)
	require.NoError(t, err)
	t.Cleanup(func() { _ = oldReader.Close() })
	oldInfo, err := oldReader.Stat()
	require.NoError(t, err)
	snapshot := filepath.Join(t.TempDir(), "client.toml")
	require.NoError(t, os.Link(file, snapshot))
	require.NoError(t, os.Chmod(file, 0644))

	chain := "restarted-\"chain\"\\id\nnext"
	require.NoError(t, SetClientConfig(flags.FlagChainID, chain, path, conf))
	newInfo, err := os.Stat(file)
	require.NoError(t, err)
	require.False(t, os.SameFile(oldInfo, newInfo), "replacement must not truncate the committed inode")
	require.Equal(t, os.FileMode(0600), newInfo.Mode().Perm())
	oldBytes, err := io.ReadAll(oldReader)
	require.NoError(t, err)
	require.Equal(t, before, oldBytes)
	snapshotBytes, err := os.ReadFile(snapshot)
	require.NoError(t, err)
	require.Equal(t, before, snapshotBytes)
	require.NoError(t, oldReader.Close())
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	require.Len(t, entries, 1, "successful replacement must not leave a temporary sibling")

	// Restart the actual context consumer, including keyring and node setup.
	ctx := client.Context{}.WithHomeDir(home)
	ctx.Viper = viper.New()
	ctx, err = ReadFromClientConfig(ctx)
	require.NoError(t, err)
	require.Equal(t, chain, ctx.ChainID)
	require.Equal(t, conf.Node, ctx.NodeURI)
	require.Equal(t, conf.Output, ctx.OutputFormat)
	require.Equal(t, conf.BroadcastMode, ctx.BroadcastMode)
	require.NotNil(t, ctx.Keyring)
	keys, err := ctx.Keyring.List()
	require.NoError(t, err)
	require.Empty(t, keys)
}

func TestRepairM_1460_RefusedReplacementRecovery(t *testing.T) {
	path := t.TempDir()
	// A nonempty directory refuses rename even when the test runs as root.
	// Its existing configuration must remain usable after the refused update.
	protected := filepath.Join(path, "client.toml")
	require.NoError(t, ensureConfigPath(protected))
	file := filepath.Join(protected, "client.toml")
	prior := defaultClientConfig()
	prior.ChainID = "previous-chain"
	require.NoError(t, writeConfigToFile(file, prior))
	before, err := os.ReadFile(file)
	require.NoError(t, err)
	conf := *prior
	err = SetClientConfig(flags.FlagChainID, "replacement-chain", path, &conf)
	require.ErrorContains(t, err, "could not replace client config")
	after, err := os.ReadFile(file)
	require.NoError(t, err)
	require.Equal(t, before, after)
	entries, err := os.ReadDir(path)
	require.NoError(t, err)
	require.Len(t, entries, 1, "refused replacement must remove its temporary sibling")
	require.Equal(t, "client.toml", entries[0].Name())
	require.True(t, entries[0].IsDir())

	reopened, err := GetClientConfig(protected, viper.New())
	require.NoError(t, err)
	require.Equal(t, *prior, *reopened)
	// A process can stop with a partial temporary sibling; a fresh reader must
	// still select the committed file, and a later update must remain possible.
	staged := filepath.Join(protected, ".client.toml-interrupted")
	require.NoError(t, writeConfigToFile(staged, &conf))
	stagedFile, err := os.OpenFile(staged, os.O_WRONLY|os.O_TRUNC, 0600)
	require.NoError(t, err)
	_, err = stagedFile.WriteString("chain-id = \"")
	require.NoError(t, err)
	require.NoError(t, stagedFile.Close())
	reopened, err = GetClientConfig(protected, viper.New())
	require.NoError(t, err)
	require.Equal(t, *prior, *reopened)
	require.NoError(t, SetClientConfig(flags.FlagChainID, "recovered-chain", protected, reopened))
	reopened, err = GetClientConfig(protected, viper.New())
	require.NoError(t, err)
	require.Equal(t, "recovered-chain", reopened.ChainID)
	require.Equal(t, prior.Node, reopened.Node)

	partial, err := os.ReadFile(staged)
	require.NoError(t, err)
	require.Equal(t, "chain-id = \"", string(partial), "uncommitted siblings must not be consumed or replaced")
}
