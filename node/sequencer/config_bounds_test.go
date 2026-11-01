package sequencer_test

import (
	"math"
	"strconv"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

const boundedSequencerSection = `
[sequencer]
enable = true
wal-dir = "data/sequencer-wal"
receipt-key-file = "config/receipt_key.hex"
replication-timeout = "250ms"
wal-retain-blocks = 7
nonce-queue-max-per-sender = 16
nonce-queue-max-total = 64
`

func TestSequencerConfigReadsRetentionAndQueueBounds(t *testing.T) {
	opts := seqConfigOptions(t, boundedSequencerSection)
	opts.Set(sequencer.FlagLeaseDuration, "2s")
	cfg, err := sequencer.ReadConfig(opts)
	require.NoError(t, err)
	require.Equal(t, uint64(7), cfg.WALRetainBlocks)
	require.Equal(t, 16, cfg.NonceQueueMaxPerSender)
	require.Equal(t, 64, cfg.NonceQueueMaxTotal)
	require.Equal(t, 2*time.Second, cfg.LeaseDuration)
	require.NoError(t, cfg.Validate())

	derived, err := sequencer.ReadConfig(seqConfigOptions(t, boundedSequencerSection))
	require.NoError(t, err)
	require.Equal(t, 3*derived.ReplicationTimeout, derived.LeaseDuration)
	require.Equal(t, 750*time.Millisecond, derived.LeaseDuration)

	cmd := &cobra.Command{Use: "start"}
	sequencer.AddStartFlags(cmd)
	bound := seqConfigOptions(t, boundedSequencerSection)
	require.NoError(t, bound.BindPFlags(cmd.Flags()))
	derivedFromFlags, err := sequencer.ReadConfig(bound)
	require.NoError(t, err)
	require.Equal(t, derived, derivedFromFlags)

	defaults := sequencer.DefaultConfig()
	queue := sequencer.NonceQueueConfig{}
	require.Equal(t, uint64(2), defaults.WALRetainBlocks)
	require.Equal(t, queue.PerSender, defaults.NonceQueueMaxPerSender)
	require.Equal(t, queue.Total, defaults.NonceQueueMaxTotal)
	require.Equal(t, 3*defaults.ReplicationTimeout, defaults.LeaseDuration)
	for _, opts := range []*viper.Viper{viper.New(), seqConfigOptions(t, sequencer.ConfigTemplate)} {
		actual, err := sequencer.ReadConfig(opts)
		require.NoError(t, err)
		require.Equal(t, defaults, actual)
	}
	templateOpts := seqConfigOptions(t, sequencer.ConfigTemplate)
	for _, key := range []string{
		sequencer.FlagWALRetainBlocks, sequencer.FlagNonceQueueMaxPerSender,
		sequencer.FlagNonceQueueMaxTotal, sequencer.FlagLeaseDuration,
	} {
		require.True(t, templateOpts.IsSet(key), key)
	}
}

func TestSequencerConfigRefusesZeroBounds(t *testing.T) {
	valid, err := sequencer.ReadConfig(seqConfigOptions(t, boundedSequencerSection))
	require.NoError(t, err)
	for _, tc := range []struct {
		key string
		set func(*sequencer.Config, int)
	}{
		{sequencer.FlagWALRetainBlocks, func(c *sequencer.Config, n int) { c.WALRetainBlocks = uint64(n) }},
		{sequencer.FlagNonceQueueMaxPerSender, func(c *sequencer.Config, n int) { c.NonceQueueMaxPerSender = n }},
		{sequencer.FlagNonceQueueMaxTotal, func(c *sequencer.Config, n int) { c.NonceQueueMaxTotal = n }},
		{sequencer.FlagLeaseDuration, func(c *sequencer.Config, n int) { c.LeaseDuration = time.Duration(n) }},
	} {
		t.Run(tc.key, func(t *testing.T) {
			for _, value := range []int{0, -1} {
				if value == 0 || tc.key != sequencer.FlagWALRetainBlocks {
					bad := valid
					tc.set(&bad, value)
					err := bad.Validate()
					require.ErrorIs(t, err, sequencer.ErrConfigValue)
					require.ErrorContains(t, err, tc.key)
				}
				opts := seqConfigOptions(t, boundedSequencerSection)
				opts.Set(tc.key, value)
				_, err := sequencer.ReadConfig(opts)
				require.ErrorIs(t, err, sequencer.ErrConfigValue)
				require.ErrorContains(t, err, tc.key)
			}
			opts := seqConfigOptions(t, boundedSequencerSection)
			opts.Set(tc.key, "invalid")
			_, err := sequencer.ReadConfig(opts)
			require.ErrorIs(t, err, sequencer.ErrConfigValue)
			require.ErrorContains(t, err, tc.key)
		})
	}
	bad := valid
	bad.NonceQueueMaxTotal = bad.NonceQueueMaxPerSender - 1
	require.ErrorIs(t, bad.Validate(), sequencer.ErrConfigValue)

	overflow := seqConfigOptions(t, boundedSequencerSection)
	overflow.Set(sequencer.FlagReplicationTimeout, time.Duration(math.MaxInt64/3+1))
	_, err = sequencer.ReadConfig(overflow)
	require.ErrorIs(t, err, sequencer.ErrConfigValue)
	require.ErrorContains(t, err, sequencer.FlagLeaseDuration)
}

func TestAddStartFlagsRegistersSequencerFlags(t *testing.T) {
	defaults := sequencer.DefaultConfig()
	cmd := &cobra.Command{Use: "start"}
	sequencer.AddStartFlags(cmd)
	want := map[string]struct{ kind, value string }{
		sequencer.FlagEnable:                 {"bool", strconv.FormatBool(defaults.Enable)},
		sequencer.FlagMode:                   {"string", defaults.Mode.String()},
		sequencer.FlagWALDir:                 {"string", defaults.WALDir},
		sequencer.FlagReceiptKeyFile:         {"string", defaults.ReceiptKeyFile},
		"sequencer.receipt-forward-url":      {"string", defaults.ReceiptForwardURL},
		"sequencer.role":                     {"string", defaults.Role.String()},
		sequencer.FlagStandbyAddress:         {"string", defaults.StandbyAddress},
		sequencer.FlagReplicationTimeout:     {"duration", defaults.ReplicationTimeout.String()},
		sequencer.FlagMicroBatchWindow:       {"duration", defaults.MicroBatchWindow.String()},
		sequencer.FlagMicroBatchMaxCount:     {"int", strconv.Itoa(defaults.MicroBatchMaxCount)},
		sequencer.FlagWALRetainBlocks:        {"uint64", strconv.FormatUint(defaults.WALRetainBlocks, 10)},
		sequencer.FlagNonceQueueMaxPerSender: {"int", strconv.Itoa(defaults.NonceQueueMaxPerSender)},
		sequencer.FlagNonceQueueMaxTotal:     {"int", strconv.Itoa(defaults.NonceQueueMaxTotal)},
		sequencer.FlagLeaseDuration:          {"duration", defaults.LeaseDuration.String()},
	}
	for name, expected := range want {
		flag := cmd.Flags().Lookup(name)
		require.NotNil(t, flag, name)
		require.Equal(t, expected.kind, flag.Value.Type(), name)
		require.Equal(t, expected.value, flag.DefValue, name)
		require.Equal(t, expected.value, flag.Value.String(), name)
		require.NotEmpty(t, flag.Usage, name)
	}
	opts := viper.New()
	require.NoError(t, opts.BindPFlags(cmd.Flags()))
	cfg, err := sequencer.ReadConfig(opts)
	require.NoError(t, err)
	require.Equal(t, defaults, cfg)

	require.NoError(t, cmd.Flags().Parse([]string{
		"--sequencer.enable", "--sequencer.mode=microbatch", "--sequencer.role=standby",
		"--sequencer.wal-dir=w", "--sequencer.receipt-key-file=k",
		"--sequencer.standby-address=localhost:26700", "--sequencer.replication-timeout=400ms",
		"--sequencer.microbatch-window=5ms", "--sequencer.microbatch-max-count=32",
		"--sequencer.wal-retain-blocks=9", "--sequencer.nonce-queue-max-per-sender=8",
		"--sequencer.nonce-queue-max-total=128", "--sequencer.lease-duration=2s",
	}))
	cfg, err = sequencer.ReadConfig(opts)
	require.NoError(t, err)
	require.Equal(t, sequencer.Config{
		Enable: true, Mode: sequencer.ModeMicroBatch, Role: sequencer.RoleStandby,
		WALDir: "w", ReceiptKeyFile: "k", StandbyAddress: "localhost:26700",
		ReplicationTimeout: 400 * time.Millisecond, MicroBatchWindow: 5 * time.Millisecond,
		MicroBatchMaxCount: 32, WALRetainBlocks: 9, NonceQueueMaxPerSender: 8,
		NonceQueueMaxTotal: 128, LeaseDuration: 2 * time.Second,
	}, cfg)

	follower := &cobra.Command{Use: "start"}
	sequencer.AddStartFlags(follower)
	require.NoError(t, follower.Flags().Parse([]string{"--sequencer.receipt-forward-url=https://sequencer.example/receipts"}))
	opts = viper.New()
	require.NoError(t, opts.BindPFlags(follower.Flags()))
	cfg, err = sequencer.ReadConfig(opts)
	require.NoError(t, err)
	require.Equal(t, "https://sequencer.example/receipts", cfg.ReceiptForwardURL)

	count := 0
	cmd.Flags().VisitAll(func(*pflag.Flag) { count++ })
	require.Equal(t, len(want), count)
}
