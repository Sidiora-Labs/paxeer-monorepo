// Proves: cmd.InitCmd, sequencer.AddStartFlags and cmd.SequencerCmd -> app.toml/Viper and receipt seed file -> sequencer.ReadConfig, LoadReceiptKey and ReceiptKey.SignReceipt
//
// Repairs ConfigTemplate in node/sequencer/config.go: an explicit zero lease
// prevented the documented timeout-derived lease when enabling an initialized
// node. The template now omits that override; explicitly configured zero still
// refuses start. Repairs InitCmd in daemon/paxd/cmd/init.go: duplicate config
// detection must precede validator recovery and client configuration writes.
// The proof drops commands, Viper instances and loaded keys, then reopens their
// actual files; no configuration reader, key generator or signer is replaced.
package sequencer_test

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/daemon/paxd/cmd"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/layerxproof/verify"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/params"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	clientconfig "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/module"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func proofSequencerInit(t *testing.T, home string, recover bool) error {
	t.Helper()
	clientCtx := client.Context{}.
		WithCodec(params.MakeEncodingConfig().Marshaler).
		WithHomeDir(home).WithViper("")
	init := cmd.InitCmd(module.BasicManager{}, home)
	init.SetContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))
	init.SetOut(&bytes.Buffer{})
	init.SetErr(&bytes.Buffer{})
	init.SetIn(bytes.NewReader(nil))
	args := []string{"config-proof", "--chain-id=config-proof", "--mode=validator"}
	if recover {
		args = append(args, "--recover")
	}
	init.SetArgs(args)
	return init.Execute()
}

func proofSequencerOptions(t *testing.T, path string) *viper.Viper {
	t.Helper()
	opts := viper.New()
	opts.SetConfigFile(path)
	require.NoError(t, opts.ReadInConfig())
	return opts
}

func proofSequencerGenerateKey(path string) ([32]byte, error) {
	var output bytes.Buffer
	producer := cmd.SequencerCmd()
	producer.SilenceErrors, producer.SilenceUsage = true, true
	producer.SetOut(&output)
	producer.SetErr(&bytes.Buffer{})
	producer.SetArgs([]string{"gen-receipt-key", "--output", path})
	if err := producer.Execute(); err != nil {
		return [32]byte{}, err
	}
	var result struct {
		PublicKey string `json:"public_key"`
	}
	if err := json.Unmarshal(output.Bytes(), &result); err != nil {
		return [32]byte{}, err
	}
	public, err := hex.DecodeString(result.PublicKey)
	if err != nil {
		return [32]byte{}, err
	}
	return [32]byte(public), nil
}

func TestProofSequencerConfig(t *testing.T) {
	home := t.TempDir()
	configDir := filepath.Join(home, "config")
	require.NoError(t, os.MkdirAll(configDir, 0o700))
	// Use the client config writer that normally precedes InitCmd on the root command.
	require.NoError(t, clientconfig.SetClientConfig(flags.FlagChainID, "config-proof", configDir, &clientconfig.ClientConfig{}))
	require.NoError(t, proofSequencerInit(t, home, false))
	appPath := filepath.Join(configDir, "app.toml")
	consensusPath := filepath.Join(configDir, "priv_validator_key.json")
	keyPath := filepath.Join(configDir, "receipt_key.hex")
	public, err := proofSequencerGenerateKey(keyPath)
	require.NoError(t, err)

	// Both producer commands have ended. Consume only their persisted output.
	opts := proofSequencerOptions(t, appPath)
	defaults, err := sequencer.ReadConfig(opts)
	require.NoError(t, err)
	require.Equal(t, sequencer.DefaultConfig(), defaults)
	require.False(t, opts.IsSet(sequencer.FlagLeaseDuration), "the generated template must allow lease derivation")

	// The init node mode and the sequencer executor mode occupy distinct keys.
	require.Equal(t, "mode", cmd.FlagMode)
	require.Equal(t, "sequencer.mode", sequencer.FlagMode)
	start := &cobra.Command{Use: "start"}
	sequencer.AddStartFlags(start)
	for flag, name := range map[string]string{
		sequencer.FlagEnable:             "sequencer.enable",
		sequencer.FlagMode:               "sequencer.mode",
		sequencer.FlagReceiptKeyFile:     "sequencer.receipt-key-file",
		sequencer.FlagStandbyAddress:     "sequencer.standby-address",
		sequencer.FlagReplicationTimeout: "sequencer.replication-timeout",
		sequencer.FlagMicroBatchWindow:   "sequencer.microbatch-window",
		sequencer.FlagMicroBatchMaxCount: "sequencer.microbatch-max-count",
	} {
		require.Equal(t, name, flag)
		require.NotNil(t, start.Flags().Lookup(name))
		require.True(t, opts.IsSet(name), "InitCmd must produce %s", name)
	}
	require.NoError(t, start.ParseFlags([]string{
		"--sequencer.enable=true", "--sequencer.mode=microbatch",
		"--sequencer.wal-dir=" + filepath.Join(home, "data", "sequencer-wal"),
		"--sequencer.receipt-key-file=" + keyPath,
		"--sequencer.standby-address=127.0.0.1:26700",
		"--sequencer.replication-timeout=250ms",
		"--sequencer.microbatch-window=5ms", "--sequencer.microbatch-max-count=17",
		"--sequencer.nonce-queue-max-per-sender=4", "--sequencer.nonce-queue-max-total=16",
	}))
	require.NoError(t, opts.BindPFlags(start.Flags()))
	cfg, err := sequencer.ReadConfig(opts)
	require.NoError(t, err)
	require.Equal(t, sequencer.Config{
		Enable: true, Role: sequencer.RolePrimary, Mode: sequencer.ModeMicroBatch,
		WALDir: filepath.Join(home, "data", "sequencer-wal"), WALRetainBlocks: 2,
		NonceQueueMaxPerSender: 4, NonceQueueMaxTotal: 16,
		ReceiptKeyFile: keyPath, StandbyAddress: "127.0.0.1:26700",
		ReplicationTimeout: 250 * time.Millisecond, LeaseDuration: 750 * time.Millisecond,
		MicroBatchWindow: 5 * time.Millisecond, MicroBatchMaxCount: 17,
	}, cfg)
	require.Equal(t, sequencer.MicroBatchOptions{Window: 5 * time.Millisecond, MaxCount: 17}, cfg.MicroBatchOptions())

	// Persist the selected lease explicitly: Viper also serializes bound flag defaults.
	require.NoError(t, start.Flags().Set(sequencer.FlagLeaseDuration, cfg.LeaseDuration.String()))
	require.NoError(t, opts.WriteConfig())
	firstConfig, err := os.ReadFile(appPath)
	require.NoError(t, err)
	firstSeed, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	pins := []verify.SequencerAuthorization{{
		SequencerID: testKeyID, PublicKey: public, FirstBatchNumber: 1, LastBatchNumber: 10,
	}}
	key, err := sequencer.LoadReceiptKey(cfg.ReceiptKeyFile, consensusPath, pins, 2)
	require.NoError(t, err)
	require.Equal(t, public, key.PublicKey())
	in := vectorInput()
	in.PaymentDenom = "upax"
	in.Accepted.Height = 2
	firstReceipt := assertReceiptKeySignature(t, key, in)
	decoded, err := codec.DecodeReceipt(firstReceipt)
	require.NoError(t, err)
	require.Equal(t, testKeyID, decoded.ContextHash)

	// Drop every in-memory producer and consumer before restarting from disk.
	opts, start, key = nil, nil, nil
	reopened := proofSequencerOptions(t, appPath)
	restarted, err := sequencer.ReadConfig(reopened)
	require.NoError(t, err)
	require.Equal(t, cfg, restarted)
	key, err = sequencer.LoadReceiptKey(restarted.ReceiptKeyFile, consensusPath, pins, 2)
	require.NoError(t, err)
	require.Equal(t, public, key.PublicKey())
	require.Equal(t, firstReceipt, assertReceiptKeySignature(t, key, in))

	// Consume the persisted executor selection through the real application's host.
	h := newHarness(t)
	exec, err := sequencer.NewExecutor(h.app, sequencer.Options{
		Mode: restarted.Mode, ChainID: testChainID, ProposerAddress: h.proposer,
		LastSeq: startSeq, LastSeqHash: startSeqHash, MicroBatch: restarted.MicroBatchOptions(),
	})
	require.NoError(t, err)
	require.Equal(t, sequencer.ModeMicroBatch, exec.Mode())

	// A repeated key producer must refuse and preserve the signing identity.
	_, err = proofSequencerGenerateKey(keyPath)
	require.ErrorIs(t, err, os.ErrExist)
	again, err := os.ReadFile(keyPath)
	require.NoError(t, err)
	require.Equal(t, firstSeed, again)
	// A repeated init must refuse before attempting mnemonic recovery or any write.
	protected := []string{appPath, consensusPath, filepath.Join(configDir, "config.toml"), filepath.Join(configDir, "client.toml")}
	before := make(map[string][]byte)
	for _, path := range protected {
		before[path], err = os.ReadFile(path)
		require.NoError(t, err)
	}
	err = proofSequencerInit(t, home, true)
	require.ErrorContains(t, err, "configuration files already exist")
	for _, path := range protected {
		after, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, before[path], after, path)
	}
	require.Equal(t, firstConfig, before[appPath])

	t.Run("refuses malformed persisted configuration", func(t *testing.T) {
		for _, tc := range []struct {
			key   string
			value any
			want  error
		}{
			{sequencer.FlagMode, "unknown", sequencer.ErrConfigMode},
			{sequencer.FlagEnable, "sometimes", sequencer.ErrConfigValue},
			{sequencer.FlagStandbyAddress, "missing-port", sequencer.ErrConfigStandbyAddress},
			{sequencer.FlagReplicationTimeout, "soon", sequencer.ErrConfigValue},
			{sequencer.FlagMicroBatchWindow, "0s", sequencer.ErrMicroBatchWindow},
			{sequencer.FlagMicroBatchMaxCount, 0, sequencer.ErrMicroBatchMaxCount},
			{sequencer.FlagReceiptKeyFile, "", sequencer.ErrConfigReceiptKeyFile},
			{sequencer.FlagLeaseDuration, "0s", sequencer.ErrConfigValue},
		} {
			t.Run(tc.key, func(t *testing.T) {
				producer := proofSequencerOptions(t, appPath)
				producer.Set(tc.key, tc.value)
				path := filepath.Join(t.TempDir(), "app.toml")
				require.NoError(t, producer.WriteConfigAs(path))
				producer = nil
				_, err := sequencer.ReadConfig(proofSequencerOptions(t, path))
				require.ErrorIs(t, err, tc.want)
			})
		}
	})

	t.Run("refuses wrong signer and retired pin", func(t *testing.T) {
		_, err := verify.ReceiptSignature(firstReceipt, publicOf(consensusKeySeed))
		require.ErrorIs(t, err, verify.ErrSequencerSignature)
		foreign := append([]verify.SequencerAuthorization(nil), pins...)
		foreign[0].PublicKey = publicOf(consensusKeySeed)
		refused, err := sequencer.LoadReceiptKey(restarted.ReceiptKeyFile, consensusPath, foreign, 2)
		require.ErrorIs(t, err, sequencer.ErrReceiptKeyNotPinned)
		require.Nil(t, refused)
		refused, err = sequencer.LoadReceiptKey(restarted.ReceiptKeyFile, consensusPath, pins, 11)
		require.ErrorIs(t, err, sequencer.ErrReceiptKeyNotPinned)
		require.Nil(t, refused)
	})

	t.Run("rechecks persisted key permissions and bytes", func(t *testing.T) {
		require.NoError(t, os.Chmod(keyPath, 0o644))
		refused, err := sequencer.LoadReceiptKey(keyPath, consensusPath, pins, 2)
		require.ErrorIs(t, err, sequencer.ErrReceiptKeyFile)
		require.Nil(t, refused)
		require.NoError(t, os.Chmod(keyPath, 0o600))
		require.NoError(t, os.WriteFile(keyPath, []byte("malformed\n"), 0o600))
		refused, err = sequencer.LoadReceiptKey(keyPath, consensusPath, pins, 2)
		require.ErrorIs(t, err, sequencer.ErrReceiptKeyFile)
		require.Nil(t, refused)
	})
}
