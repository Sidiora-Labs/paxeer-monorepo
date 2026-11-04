package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/tcp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/app"
	noderuntime "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/node/runtime"
	e2e "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/test/e2e/pkg"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func repair123Testnet(t *testing.T, protocol string) *e2e.Testnet {
	t.Helper()
	manifest := e2e.Manifest{
		ABCIProtocol:    protocol,
		InitialState:    map[string]string{"protocol": "builtin"},
		LeaderPin:       []string{"validator01"},
		LeaderPinHeight: 1,
		ValidatorUpdates: map[string]map[string]int64{
			"0": {"validator01": 100},
		},
		Nodes: map[string]*e2e.ManifestNode{"validator01": {}},
	}
	file := filepath.Join(t.TempDir(), "protocol.toml")
	require.NoError(t, manifest.Save(file))
	testnet, err := e2e.LoadTestnet(file)
	require.NoError(t, err)
	return testnet
}

func TestRepair_123_GeneratedBuiltinConfigBootstrap(t *testing.T) {
	for _, tc := range []struct {
		name     string
		protocol string
	}{
		{name: "explicit", protocol: "builtin"},
		{name: "default"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testnet := repair123Testnet(t, tc.protocol)
			require.Equal(t, e2e.ProtocolBuiltin, testnet.ABCIProtocol)
			require.NoError(t, Setup(testnet))
			node := testnet.Nodes[0]
			home := filepath.Join(testnet.Dir, node.Name)
			appFile := filepath.Join(home, "config", "app.toml")
			generated, err := noderuntime.LoadConfig(appFile)
			require.NoError(t, err)
			require.Equal(t, "builtin", generated.Protocol)

			// Keep the real generated node isolated from other tests and host services.
			cfg, err := MakeConfig(node)
			require.NoError(t, err)
			cfg.P2P.ListenAddress = "tcp://" + tcp.TestReserveAddr().String()
			cfg.P2P.ExternalAddress = ""
			cfg.RPC.ListenAddress = ""
			cfg.RPC.PprofListenAddress = ""
			cfg.Instrumentation.Prometheus = false
			require.NoError(t, config.WriteConfigFile(home, cfg))

			appDir := filepath.Join(home, "data", "app")
			watcher, err := fsnotify.NewWatcher()
			require.NoError(t, err)
			defer watcher.Close()
			require.NoError(t, watcher.Add(appDir))

			ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
			done := make(chan error, 1)
			go func() { done <- noderuntime.Run(ctx, appFile, home) }()
			defer func() {
				cancel()
				require.NoError(t, <-done)
			}()

			for {
				select {
				case err := <-done:
					done <- err
					t.Fatalf("node exited before committing generated builtin configuration: %v", err)
				case <-ctx.Done():
					t.Fatalf("node did not commit generated builtin configuration: %v", ctx.Err())
				case err := <-watcher.Errors:
					require.NoError(t, err)
				case event := <-watcher.Events:
					if event.Name != filepath.Join(appDir, "app_state.json") || !event.Has(fsnotify.Create) {
						continue
					}
					state, err := app.NewState(appDir, node.PersistInterval)
					require.NoError(t, err)
					if state.Height == 0 {
						continue // InitChain persists genesis before the first block.
					}
					require.GreaterOrEqual(t, state.Height, uint64(testnet.InitialHeight))
					require.Equal(t, "builtin", state.Get("protocol"))

					var appCfg app.Config
					_, err = toml.DecodeFile(appFile, &appCfg)
					require.NoError(t, err)
					appCfg.Dir = appDir
					application, err := app.NewApplication(&appCfg)
					require.NoError(t, err)
					var source types.LeaderSource = application
					activation, leaders, err := source.LeaderSchedule(testnet.InitialHeight)
					require.NoError(t, err)
					require.Equal(t, testnet.LeaderPinHeight, activation)
					require.Equal(t, [][32]byte{[32]byte(node.PrivvalKey.Public().Bytes())}, leaders)
					return
				}
			}
		})
	}
}

func TestRepair_123_UnsupportedTransports(t *testing.T) {
	for _, protocol := range []string{"unix", "tcp", "grpc"} {
		t.Run(protocol, func(t *testing.T) {
			testnet := repair123Testnet(t, protocol)
			require.Equal(t, e2e.Protocol(protocol), testnet.ABCIProtocol)
			for _, mode := range []e2e.Mode{e2e.ModeValidator, e2e.ModeFull, e2e.ModeSeed, e2e.ModeLight} {
				t.Run(string(mode), func(t *testing.T) {
					node := *testnet.Nodes[0]
					node.Mode = mode
					encoded, err := MakeAppConfig(&node)
					require.Error(t, err)
					require.Nil(t, encoded)
				})
			}
		})
	}
	for _, protocol := range []e2e.Protocol{"socket", "file", "unknown", ""} {
		t.Run(string(protocol), func(t *testing.T) {
			testnet := repair123Testnet(t, "builtin")
			testnet.ABCIProtocol = protocol
			encoded, err := MakeAppConfig(testnet.Nodes[0])
			require.Error(t, err)
			require.Nil(t, encoded)
		})
	}
}

func TestRepair_123_ConsumerRefusesUnsupportedTransports(t *testing.T) {
	for _, protocol := range []string{"unix", "tcp", "grpc", "socket", "file", "unknown", ""} {
		t.Run(protocol, func(t *testing.T) {
			testnet := repair123Testnet(t, "builtin")
			encoded, err := MakeAppConfig(testnet.Nodes[0])
			require.NoError(t, err)
			var generated map[string]any
			_, err = toml.Decode(string(encoded), &generated)
			require.NoError(t, err)
			generated["protocol"] = protocol
			var invalid bytes.Buffer
			require.NoError(t, toml.NewEncoder(&invalid).Encode(generated))
			file := filepath.Join(t.TempDir(), "app.toml")
			require.NoError(t, os.WriteFile(file, invalid.Bytes(), 0600))
			_, err = noderuntime.LoadConfig(file)
			require.Error(t, err)
			require.Error(t, noderuntime.Run(t.Context(), file, t.TempDir()))
		})
	}
}
