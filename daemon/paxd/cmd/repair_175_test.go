package cmd

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	tmcfg "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	tmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/params"
	svrcmd "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/cmd"
	"github.com/cosmos/go-bip39"
	"github.com/stretchr/testify/require"
)

func TestRepair_175_InitRecoveryMnemonicRestart(t *testing.T) {
	// Re-execute the test binary with paxd's production entry path, without an
	// extra build or a replacement command, input reader, or validator store.
	if len(os.Args) > 3 && os.Args[2] == "--" {
		params.SetAddressPrefixes()
		root, _ := NewRootCmd()
		root.SetArgs(os.Args[3:])
		require.NoError(t, svrcmd.Execute(root, app.DefaultNodeHome))
		return
	}

	executable, err := os.Executable()
	require.NoError(t, err)
	run := func(t *testing.T, home, mnemonic string) ([]byte, error) {
		t.Helper()
		ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
		defer cancel()
		process := exec.CommandContext(ctx, executable,
			"-test.run=^TestRepair_175_InitRecoveryMnemonicRestart$", "--",
			"init", "recovered-validator", "--recover", "--mode", "validator",
			"--home", home, "--chain-id", "repair-175")
		for _, env := range os.Environ() {
			if !strings.HasPrefix(env, "PAX_") && !strings.HasPrefix(env, "PAXD_") {
				process.Env = append(process.Env, env)
			}
		}
		process.Stdin = strings.NewReader(mnemonic)
		output, err := process.CombinedOutput()
		require.NoError(t, ctx.Err(), "init must terminate within its deadline")
		return output, err
	}

	for _, tc := range []struct {
		name     string
		mnemonic string
		input    string
	}{
		{
			name:     "newline terminated",
			mnemonic: "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about",
			input:    "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about\n",
		},
		{
			name:     "trimmed at EOF",
			mnemonic: "legal winner thank year wave sausage worth useful legal winner thank yellow",
			input:    "  legal winner thank year wave sausage worth useful legal winner thank yellow  ",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			require.True(t, bip39.IsMnemonicValid(tc.mnemonic))
			seed := sha256.Sum256([]byte(tc.mnemonic))
			expected := ed25519.NewKeyFromSeed(seed[:]).Public().(ed25519.PublicKey)
			home := t.TempDir()
			config := tmcfg.DefaultConfig().SetRoot(home)
			for attempt := 0; attempt < 2; attempt++ {
				output, err := run(t, home, tc.input)
				require.NoError(t, err, "real init --recover subprocess: %s", output)
				validator, err := privval.LoadFilePV(config.PrivValidator.KeyFile(), config.PrivValidator.StateFile())
				require.NoError(t, err, "reopen the recovered validator after the init process has exited")
				pubKey, err := validator.GetPubKey(t.Context())
				require.NoError(t, err)
				require.Equal(t, []byte(expected), pubKey.Bytes(), "operator input must determine the durable consensus identity")
				require.Equal(t, pubKey.Address(), validator.GetAddress())
				require.Zero(t, validator.LastSignState.Height)
				for _, path := range []string{config.PrivValidator.KeyFile(), config.PrivValidator.StateFile()} {
					info, err := os.Stat(path)
					require.NoError(t, err)
					require.Equal(t, os.FileMode(0600), info.Mode().Perm())
				}

				vote := &tmproto.Vote{Type: tmproto.PrevoteType, Height: 1, Round: 0, Timestamp: time.Now().UTC()}
				require.NoError(t, validator.SignVote(t.Context(), "repair-175", vote))
				require.True(t, ed25519.Verify(expected, tmtypes.VoteSignBytes("repair-175", vote), vote.Signature))
				reopened, err := privval.LoadFilePV(config.PrivValidator.KeyFile(), config.PrivValidator.StateFile())
				require.NoError(t, err)
				require.Equal(t, []byte(expected), reopened.Key.PubKey.Bytes())
				require.Equal(t, int64(1), reopened.LastSignState.Height)
				require.Equal(t, vote.Signature, reopened.LastSignState.Signature)
				require.NoError(t, reopened.SignVote(t.Context(), "repair-175", vote))

				// The next recovery starts in another fresh home, without any
				// key file that could hide loss of the supplied mnemonic.
				home = t.TempDir()
				config = tmcfg.DefaultConfig().SetRoot(home)
			}
		})
	}

	t.Run("invalid mnemonic", func(t *testing.T) {
		home := t.TempDir()
		const invalidMnemonic = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon invalid-word"
		require.False(t, bip39.IsMnemonicValid(invalidMnemonic))
		output, err := run(t, home, invalidMnemonic+"\n")
		require.Error(t, err)
		require.Contains(t, string(output), "invalid mnemonic")
		config := tmcfg.DefaultConfig().SetRoot(home)
		require.NoFileExists(t, config.PrivValidator.KeyFile())
		require.NoFileExists(t, config.PrivValidator.StateFile())
	})

	t.Run("recovery persistence failure", func(t *testing.T) {
		home := t.TempDir()
		config := tmcfg.DefaultConfig().SetRoot(home)
		require.NoError(t, os.MkdirAll(config.PrivValidator.StateFile(), 0700))
		output, err := run(t, home, "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about\n")
		require.Error(t, err)
		require.Contains(t, string(output), "failed to save recovered validator")
	})
}
