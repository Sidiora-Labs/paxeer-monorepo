package sequencer

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tmconfig "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/privval"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
)

func nonceLifetimeOptions() *viper.Viper {
	opts := viper.New()
	opts.Set(FlagEnable, true)
	opts.Set(FlagWALDir, "wal")
	opts.Set(FlagReceiptKeyFile, "receipt.key")
	opts.Set(FlagNonceQueueMaxPerSender, 4)
	opts.Set(FlagNonceQueueMaxTotal, 16)
	opts.Set(FlagLeaseDuration, "10s")
	return opts
}

func TestNonceLifetimeReadsOptions(t *testing.T) {
	for _, value := range []any{"725ms", 725 * time.Millisecond} {
		opts := nonceLifetimeOptions()
		opts.Set(FlagNonceQueueLifetime, value)
		cfg, err := ReadConfig(opts)
		require.NoError(t, err)
		require.Equal(t, 725*time.Millisecond, cfg.NonceQueueLifetime)
		require.Equal(t, 10*time.Second, cfg.LeaseDuration)
	}
	cmd := &cobra.Command{Use: "start"}
	AddStartFlags(cmd)
	require.NoError(t, cmd.ParseFlags([]string{"--sequencer.nonce-queue-lifetime=725ms"}))
	opts := nonceLifetimeOptions()
	require.NoError(t, opts.BindPFlags(cmd.Flags()))
	cfg, err := ReadConfig(opts)
	require.NoError(t, err)
	require.Equal(t, 725*time.Millisecond, cfg.NonceQueueLifetime)
}

func TestNonceLifetimeOmitted(t *testing.T) {
	for _, lease := range []string{"1ms", "10s", "1h"} {
		for _, bound := range []bool{false, true} {
			t.Run(lease+"/"+map[bool]string{false: "options", true: "flags"}[bound], func(t *testing.T) {
				opts := nonceLifetimeOptions()
				opts.Set(FlagLeaseDuration, lease)
				if bound {
					cmd := &cobra.Command{Use: "start"}
					AddStartFlags(cmd)
					// An unchanged bound flag is omitted even if its exposed default differs.
					require.NoError(t, cmd.Flags().Lookup(FlagNonceQueueLifetime).Value.Set("9s"))
					require.NoError(t, opts.BindPFlags(cmd.Flags()))
					require.False(t, opts.IsSet(FlagNonceQueueLifetime))
					require.Equal(t, 9*time.Second, opts.GetDuration(FlagNonceQueueLifetime))
				}
				cfg, err := ReadConfig(opts)
				require.NoError(t, err)
				require.Equal(t, 3*time.Second, cfg.NonceQueueLifetime)
			})
		}
	}
}

func TestNonceLifetimeRefusesInvalid(t *testing.T) {
	for _, bound := range []bool{false, true} {
		for _, value := range []string{"0s", "-1s", "soon"} {
			opts := nonceLifetimeOptions()
			if bound {
				cmd := &cobra.Command{Use: "start"}
				AddStartFlags(cmd)
				if value == "soon" {
					require.Error(t, cmd.ParseFlags([]string{"--sequencer.nonce-queue-lifetime=" + value}))
					continue
				}
				require.NoError(t, cmd.ParseFlags([]string{"--sequencer.nonce-queue-lifetime=" + value}))
				require.NoError(t, opts.BindPFlags(cmd.Flags()))
			} else {
				opts.Set(FlagNonceQueueLifetime, value)
			}
			_, err := ReadConfig(opts)
			require.ErrorIs(t, err, ErrConfigValue)
			require.ErrorContains(t, err, FlagNonceQueueLifetime)
		}
	}
	valid, err := ReadConfig(nonceLifetimeOptions())
	require.NoError(t, err)
	for _, value := range []time.Duration{0, -time.Second} {
		cfg := valid
		cfg.NonceQueueLifetime = value
		require.ErrorIs(t, cfg.Validate(), ErrConfigValue)
		cfg.Enable = false
		require.NoError(t, cfg.Validate())
	}
}

func nonceLifetimeNode(t *testing.T, opts *viper.Viper, sender utils.TestAcct) *Node {
	t.Helper()
	h := newConsensusFenceHarness(t, 42, sender)
	a := h.src.app
	a.ChainID = h.opts.ChainID
	info, err := a.UpgradeKeeper.GetUpgradeInfoPath()
	require.NoError(t, err)
	files := tmconfig.DefaultConfig().SetRoot(filepath.Dir(filepath.Dir(info))).PrivValidator
	require.NoError(t, os.MkdirAll(filepath.Dir(files.KeyFile()), 0700))
	pv := privval.NewFilePV(h.consensus.Key.PrivKey, files.KeyFile(), files.StateFile())
	require.NoError(t, pv.Save())
	opts.Set(FlagWALDir, t.TempDir())
	keyPath := filepath.Join(t.TempDir(), "receipt-key")
	require.NoError(t, os.WriteFile(keyPath, []byte(hex.EncodeToString(h.src.key.private.Seed())), 0600))
	opts.Set(FlagReceiptKeyFile, keyPath)
	cfg, err := ReadConfig(opts)
	require.NoError(t, err)
	n, err := NewNode(cfg, a)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, n.Stop()) })
	return n
}

func TestNonceLifetimeQueueExpiry(t *testing.T) {
	for _, tc := range []struct {
		name     string
		setting  string
		lifetime time.Duration
	}{
		{"omitted", "", 3 * time.Second},
		{"configured", "5s", 5 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			opts := nonceLifetimeOptions()
			if tc.setting != "" {
				opts.Set(FlagNonceQueueLifetime, tc.setting)
			}
			sender := utils.NewSigner()
			n := nonceLifetimeNode(t, opts, sender)
			require.NoError(t, n.Start(t.Context()))
			require.Equal(t, 10*time.Second, n.Config.LeaseDuration)
			tx, _ := runtimeAdmissionSigned(t, n, sender, 1)
			_, err := n.Accept(consensusFenceTransfer(t, n.App, sender, 1))
			require.ErrorIs(t, err, ErrNonceHeld)
			pending := n.NonceAdmission.queue.Pending()
			require.Len(t, pending, 1)
			arrived := pending[0].Arrived
			require.Empty(t, n.NonceAdmission.Open(arrived.Add(tc.lifetime-time.Nanosecond)))
			require.Equal(t, 1, n.NonceAdmission.queue.Len())
			require.Empty(t, n.NonceAdmission.Open(arrived.Add(tc.lifetime)))
			dropped := n.NonceAdmission.Open(arrived.Add(tc.lifetime + time.Nanosecond))
			require.Equal(t, pending, dropped)
			require.Zero(t, n.NonceAdmission.queue.Len())
			receipt, err := n.ReceiptSource.SequencerSend(tx.Hash())
			require.NoError(t, err)
			require.Nil(t, receipt)
		})
	}
}

func TestNonceLifetimeStartupSettings(t *testing.T) {
	if os.Getenv("PAX_NONCE_LIFETIME_LOG_CHILD") == "1" {
		n := nonceLifetimeNode(t, nonceLifetimeOptions(), utils.NewSigner())
		require.NoError(t, n.Start(t.Context()))
		require.ErrorContains(t, n.Start(t.Context()), "runtime already installed")
		cfg := n.Config
		cfg.NonceQueueLifetime = 0
		_, err := NewNode(cfg, n.App)
		require.ErrorIs(t, err, ErrConfigValue)
		cfg.Enable = false
		disabled, err := NewNode(cfg, n.App)
		require.NoError(t, err)
		require.Nil(t, disabled)
		return
	}
	path := filepath.Join(t.TempDir(), "settings.jsonl")
	child := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^TestNonceLifetimeStartupSettings$", "-test.count=1")
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "PAX_LOG_") || strings.HasPrefix(entry, "PAX_NONCE_LIFETIME_LOG_CHILD=") {
			continue
		}
		child.Env = append(child.Env, entry)
	}
	child.Env = append(child.Env, "PAX_NONCE_LIFETIME_LOG_CHILD=1", "PAX_LOG_FORMAT=json", "PAX_LOG_LEVEL=info", "PAX_LOG_ADD_SOURCE=false", "PAX_LOG_OUTPUT="+path)
	output, err := child.CombinedOutput()
	require.NoError(t, err, "%s", output)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var settings []map[string]any
	for _, line := range bytes.Split(bytes.TrimSpace(data), []byte("\n")) {
		var record map[string]any
		require.NoError(t, json.Unmarshal(line, &record))
		if record["msg"] == "sequencer settings" {
			settings = append(settings, record)
		}
	}
	require.Len(t, settings, 1)
	record := settings[0]
	require.Equal(t, float64(10*time.Second), record["lease-duration"])
	require.Equal(t, float64(3*time.Second), record["nonce-queue-lifetime"])
	require.Equal(t, float64(4), record["nonce-queue-max-per-sender"])
	require.Equal(t, float64(16), record["nonce-queue-max-total"])
	for _, key := range []string{"time", "level", "msg", "logger", "lease-duration", "nonce-queue-lifetime", "nonce-queue-max-per-sender", "nonce-queue-max-total"} {
		require.Contains(t, record, key)
	}
	require.Len(t, record, 8)
}
