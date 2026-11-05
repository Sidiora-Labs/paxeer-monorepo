package config_test

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/cmd/tendermint/commands"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/mempool"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func repairM1087Read(t *testing.T, cfg *config.Config) {
	t.Helper()
	viper.Reset()
	t.Cleanup(viper.Reset)
	root := t.TempDir()
	config.EnsureRoot(root)
	require.NoError(t, config.WriteConfigFile(root, cfg))
	viper.SetConfigFile(filepath.Join(root, "config", "config.toml"))
	require.NoError(t, viper.ReadInConfig())
}

func TestRepairM_1087_P2PValidation(t *testing.T) {
	cases := []struct {
		key string
		set func(*config.P2PConfig, int64)
	}{
		{"flush-throttle-timeout", func(c *config.P2PConfig, n int64) { c.FlushThrottleTimeout = time.Duration(n) }},
		{"max-packet-msg-payload-size", func(c *config.P2PConfig, n int64) { c.MaxPacketMsgPayloadSize = int(n) }},
		{"send-rate", func(c *config.P2PConfig, n int64) { c.SendRate = n }},
		{"recv-rate", func(c *config.P2PConfig, n int64) { c.RecvRate = n }},
	}
	for _, tc := range cases {
		t.Run(tc.key, func(t *testing.T) {
			cfg := config.DefaultConfig()
			tc.set(cfg.P2P, -1)
			repairM1087Read(t, cfg)
			parsed, err := commands.ParseConfig(config.DefaultConfig())
			require.Nil(t, parsed)
			require.EqualError(t, err, "error in config file: error in [p2p] section: "+tc.key+" can't be negative")

			tc.set(cfg.P2P, 0)
			repairM1087Read(t, cfg)
			parsed, err = commands.ParseConfig(config.DefaultConfig())
			require.NoError(t, err)
			require.Equal(t, cfg.P2P, parsed.P2P)
		})
	}
}

func TestRepairM_1087_MempoolBounds(t *testing.T) {
	cases := []struct {
		name string
		set  func(*config.MempoolConfig)
		want string
	}{
		{"reservoir-zero", func(c *config.MempoolConfig) { c.DropPriorityReservoirSize = 0 }, "drop-priority-reservoir-size must be positive"},
		{"reservoir-negative", func(c *config.MempoolConfig) { c.DropPriorityReservoirSize = -1 }, "drop-priority-reservoir-size must be positive"},
		{"pending-size", func(c *config.MempoolConfig) { c.PendingSize = -1 }, "pending-size can't be negative"},
		{"pending-bytes", func(c *config.MempoolConfig) { c.MaxPendingTxsBytes = -1 }, "max-pending-txs-bytes can't be negative"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := config.DefaultConfig()
			tc.set(cfg.Mempool)
			require.EqualError(t, cfg.Mempool.ValidateBasic(), tc.want)
			repairM1087Read(t, cfg)
			parsed, err := commands.ParseConfig(config.DefaultConfig())
			require.Nil(t, parsed)
			require.EqualError(t, err, "error in config file: error in [mempool] section: "+tc.want)
		})
	}
	for _, field := range []string{"drop-priority-threshold", "drop-utilisation-threshold"} {
		for _, value := range []float64{-0.01, 1.01, math.NaN(), math.Inf(1), math.Inf(-1)} {
			t.Run(fmt.Sprintf("%s/%v", field, value), func(t *testing.T) {
				cfg := config.DefaultConfig()
				repairM1087Read(t, cfg)
				// Viper overrides follow the same decoding and startup validation as file values.
				viper.Set("mempool."+field, value)
				decoded := config.DefaultConfig()
				require.NoError(t, viper.Unmarshal(decoded))
				want := field + " must be between 0.0 and 1.0"
				require.EqualError(t, decoded.Mempool.ValidateBasic(), want)
				parsed, err := commands.ParseConfig(config.DefaultConfig())
				require.Nil(t, parsed)
				require.EqualError(t, err, "error in config file: error in [mempool] section: "+want)
			})
		}
	}

	for _, value := range []float64{0, 0.5, 1} {
		t.Run(fmt.Sprintf("construct/%v", value), func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.Mempool.DropPriorityReservoirSize = 1
			cfg.Mempool.DropPriorityThreshold = value
			cfg.Mempool.DropUtilisationThreshold = value
			cfg.Mempool.PendingSize = 0
			cfg.Mempool.MaxPendingTxsBytes = 0
			repairM1087Read(t, cfg)
			parsed, err := commands.ParseConfig(config.DefaultConfig())
			require.NoError(t, err)
			require.Equal(t, cfg.Mempool, parsed.Mempool)
			mcfg := parsed.Mempool.ToMempoolConfig()
			require.Equal(t, 1, mcfg.DropPriorityReservoirSize)
			require.Equal(t, value, mcfg.DropPriorityThreshold)
			require.Equal(t, value, mcfg.DropUtilisationThreshold)
			require.Zero(t, mcfg.PendingSize)
			require.Zero(t, mcfg.MaxPendingTxsBytes)

			app := kvstore.NewApplication()
			t.Cleanup(func() { require.NoError(t, app.Close()) })
			pool := mempool.NewTxMempool(mcfg, proxy.New(app, proxy.NopMetrics()), mempool.NopMetrics(), mempool.NopTxConstraintsFetcher)
			tx := types.Tx("key=value")
			response, err := pool.CheckTx(t.Context(), tx)
			require.NoError(t, err)
			require.True(t, response.IsOK())
			require.Equal(t, 1, pool.Size())
			require.Zero(t, pool.PendingSize())
			reaped, gas := pool.ReapTxs(mempool.ReapLimits{}, false)
			require.Equal(t, types.Txs{tx}, reaped)
			require.Equal(t, int64(1), gas)
			require.Equal(t, types.Txs{tx}, pool.RecentSnapshot())
		})
	}
}

func TestRepairM_1087_TOMLStrings(t *testing.T) {
	values := []string{
		"node \"quoted\" \\ directory\nnext line",
		"\"\nmode = \"seed\"\n#",
		"\x00\x01\b\t\n\v\f\r\x1f\x7f café 世界 😀",
		`C:\node\new\autobahn.json`,
	}
	fields := []string{
		"BaseConfig.Moniker", "BaseConfig.Mode", "BaseConfig.DBBackend", "BaseConfig.DBPath",
		"BaseConfig.LogLevel", "BaseConfig.LogFormat", "BaseConfig.Genesis", "BaseConfig.NodeKey",
		"AutobahnConfigFile", "RPC.ListenAddress", "RPC.TLSCertFile", "RPC.TLSKeyFile", "RPC.PprofListenAddress",
		"P2P.QueueType", "P2P.ListenAddress", "P2P.ExternalAddress", "P2P.BootstrapPeers", "P2P.PersistentPeers",
		"P2P.BlockSyncPeers", "P2P.PrivatePeerIDs", "P2P.UnconditionalPeerIDs", "StateSync.TrustHash",
		"Consensus.WalPath", "TxIndex.PsqlConn", "Instrumentation.PrometheusListenAddr", "Instrumentation.Namespace",
		"PrivValidator.Key", "PrivValidator.State", "PrivValidator.ListenAddr", "PrivValidator.ClientCertificate",
		"PrivValidator.ClientKey", "PrivValidator.RootCA",
	}
	field := func(cfg *config.Config, name string) reflect.Value {
		v := reflect.ValueOf(cfg)
		for _, part := range strings.Split(name, ".") {
			v = reflect.Indirect(v).FieldByName(part)
		}
		return v
	}
	for i, value := range values {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			cfg := config.DefaultConfig()
			for _, name := range fields {
				field(cfg, name).SetString(value)
			}
			cfg.RPC.CORSAllowedOrigins = []string{value, "https://example.com"}
			cfg.RPC.CORSAllowedMethods = []string{value, "GET"}
			cfg.RPC.CORSAllowedHeaders = []string{value, "Content-Type"}
			cfg.TxIndex.Indexer = []string{value, "kv"}
			cfg.StateSync.RPCServers = []string{value, "http://localhost:26657"}
			repairM1087Read(t, cfg)
			decoded := config.DefaultConfig()
			require.NoError(t, viper.Unmarshal(decoded))
			for _, name := range fields {
				want, got := field(cfg, name).String(), field(decoded, name).String()
				require.Equal(t, want, got, name)
			}
			require.Equal(t, cfg.RPC.CORSAllowedOrigins, decoded.RPC.CORSAllowedOrigins)
			require.Equal(t, cfg.RPC.CORSAllowedMethods, decoded.RPC.CORSAllowedMethods)
			require.Equal(t, cfg.RPC.CORSAllowedHeaders, decoded.RPC.CORSAllowedHeaders)
			require.Equal(t, cfg.TxIndex.Indexer, decoded.TxIndex.Indexer)
			require.Equal(t, cfg.StateSync.RPCServers, decoded.StateSync.RPCServers)
		})
	}

	t.Run("startup", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.Moniker = values[0]
		cfg.AutobahnConfigFile = values[3]
		repairM1087Read(t, cfg)
		parsed, err := commands.ParseConfig(config.DefaultConfig())
		require.NoError(t, err)
		require.Equal(t, cfg.Moniker, parsed.Moniker)
		require.Equal(t, cfg.AutobahnConfigFile, parsed.AutobahnConfigFile)
		require.Equal(t, config.ModeFull, parsed.Mode)
	})

	t.Run("invalid-utf8", func(t *testing.T) {
		cfg := config.DefaultConfig()
		cfg.Moniker = string([]byte{0xff})
		path := filepath.Join(t.TempDir(), "config.toml")
		require.Error(t, cfg.WriteToTemplate(path))
		_, err := os.Stat(path)
		require.ErrorIs(t, err, os.ErrNotExist)
	})
}

func TestRepairM_1087_AutobahnSequencerRoundTrip(t *testing.T) {
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			cfg := config.DefaultConfig()
			cfg.AutobahnConfigFile = "config/autobahn \"network\"\\file\n.json"
			cfg.Consensus.Sequencer = config.SequencerConfig{
				Enable:              enabled,
				TimeoutPropose:      2 * time.Second,
				TimeoutProposeDelta: 250 * time.Millisecond,
				TimeoutVote:         30 * time.Millisecond,
				TimeoutCommit:       40 * time.Millisecond,
			}
			repairM1087Read(t, cfg)
			decoded := config.DefaultConfig()
			require.NoError(t, viper.Unmarshal(decoded))
			require.Equal(t, cfg.AutobahnConfigFile, decoded.AutobahnConfigFile)
			require.Equal(t, cfg.Consensus.Sequencer, decoded.Consensus.Sequencer)
			parsed, err := commands.ParseConfig(config.DefaultConfig())
			if enabled {
				require.EqualError(t, decoded.ValidateBasic(), "error in [consensus] section: sequencer mode does not run with autobahn-config-file")
				require.EqualError(t, err, "error in config file: error in [consensus] section: sequencer mode does not run with autobahn-config-file")
				require.Nil(t, parsed)
				decoded.AutobahnConfigFile = ""
				require.NoError(t, decoded.ValidateBasic())
			} else {
				require.NoError(t, err)
				require.Equal(t, cfg.AutobahnConfigFile, parsed.AutobahnConfigFile)
				require.NoError(t, decoded.ValidateBasic())
			}
		})
	}
}
