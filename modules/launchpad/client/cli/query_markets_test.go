package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strconv"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/testutil/network"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/params"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdknetwork "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
)

func executeMarketsQuery(clientCtx client.Context, cmd *cobra.Command, args ...string) (string, error) {
	var output bytes.Buffer
	clientCtx = clientCtx.WithOutput(&output)
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))
	return output.String(), err
}

func TestCmdQueryMarketsArgs(t *testing.T) {
	for _, tc := range []struct {
		name        string
		newCmd      func() *cobra.Command
		use         string
		args        []string
		invalidArgs [][]string
	}{
		{"markets", cli.CmdQueryMarkets, "markets", nil, [][]string{{"extra"}, {"one", "two"}}},
		{"markets-by-creator", cli.CmdQueryMarketsByCreator, "markets-by-creator [creator]", []string{"creator"}, [][]string{nil, {"creator", "extra"}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.newCmd()
			require.Equal(t, tc.use, cmd.Use)
			require.NotNil(t, cmd.RunE)
			require.NoError(t, cmd.ValidateArgs(tc.args))
			for _, args := range tc.invalidArgs {
				require.Error(t, cmd.ValidateArgs(args))
				output, err := executeMarketsQuery(client.Context{}, tc.newCmd(), args...)
				require.Error(t, err)
				require.Empty(t, output)
			}
			queryFlags := &cobra.Command{}
			flags.AddQueryFlagsToCmd(queryFlags)
			for _, name := range []string{flags.FlagNode, flags.FlagHeight, flags.FlagOutput} {
				want := queryFlags.Flags().Lookup(name)
				got := cmd.Flags().Lookup(name)
				require.NotNil(t, got)
				require.Equal(t, want.DefValue, got.DefValue)
				require.Equal(t, want.Value.Type(), got.Value.Type())
				require.Equal(t, want.Shorthand, got.Shorthand)
			}
			require.Equal(t, "0", cmd.Flags().Lookup(flags.FlagOffset).DefValue)
			require.Equal(t, "100", cmd.Flags().Lookup(flags.FlagLimit).DefValue)
			for _, name := range []string{flags.FlagOffset, flags.FlagLimit} {
				for _, value := range []string{"", "abc", "-1", "1.5", "0x10", "18446744073709551616"} {
					t.Run(name+"="+value, func(t *testing.T) {
						args := append([]string{}, tc.args...)
						args = append(args, "--"+name+"="+value)
						output, err := executeMarketsQuery(client.Context{}, tc.newCmd(), args...)
						require.ErrorContains(t, err, "invalid --"+name)
						var parseErr *strconv.NumError
						require.ErrorAs(t, err, &parseErr)
						require.Equal(t, value, parseErr.Num)
						require.Empty(t, output)
					})
				}
			}
		})
	}
}

func TestCmdQueryMarketsNetwork(t *testing.T) {
	cfg := network.DefaultConfig()
	cfg.SigningAlgo = string(hd.Secp256k1Type)
	cfg.AppConstructor = func(val sdknetwork.Validator) servertypes.Application {
		opts := viper.New()
		sc, ss := val.AppConfig.StateCommit, val.AppConfig.StateStore
		for name, value := range map[string]interface{}{
			"chain-id":                          cfg.ChainID,
			"evm.http_enabled":                  false,
			"evm.ws_enabled":                    false,
			app.FlagSCEnable:                    sc.Enable,
			app.FlagSCAsyncCommitBuffer:         sc.MemIAVLConfig.AsyncCommitBuffer,
			app.FlagSCSnapshotKeepRecent:        sc.MemIAVLConfig.SnapshotKeepRecent,
			app.FlagSCSnapshotInterval:          sc.MemIAVLConfig.SnapshotInterval,
			app.FlagSCSnapshotMinTimeInterval:   sc.MemIAVLConfig.SnapshotMinTimeInterval,
			app.FlagSCSnapshotWriterLimit:       sc.MemIAVLConfig.SnapshotWriterLimit,
			app.FlagSCSnapshotPrefetchThreshold: sc.MemIAVLConfig.SnapshotPrefetchThreshold,
			app.FlagSCSnapshotWriteRateMBps:     sc.MemIAVLConfig.SnapshotWriteRateMBps,
			app.FlagSSEnable:                    ss.Enable,
			app.FlagSSBackend:                   ss.Backend,
			app.FlagSSAsyncWriterBuffer:         ss.AsyncWriteBuffer,
			app.FlagSSKeepRecent:                ss.KeepRecent,
			app.FlagSSPruneInterval:             ss.PruneIntervalSeconds,
			app.FlagSSImportNumWorkers:          ss.ImportNumWorkers,
		} {
			opts.Set(name, value)
		}
		encoding := params.EncodingConfig{
			Marshaler: cfg.Codec, Amino: cfg.LegacyAmino,
			TxConfig: cfg.TxConfig, InterfaceRegistry: cfg.InterfaceRegistry,
		}
		return app.New(tmdb.NewMemDB(), nil, true, map[int64]bool{}, val.Ctx.Config.RootDir,
			0, false, val.Ctx.Config, encoding, wasm.EnableAllProposals, opts, nil, app.EmptyAppOptions,
			baseapp.SetPruning(storetypes.NewPruningOptionsFromString(val.AppConfig.Pruning)),
			baseapp.SetMinGasPrices(val.AppConfig.MinGasPrices))
	}
	require.Equal(t, 1, cfg.NumValidators)
	var genesis types.GenesisState
	require.NoError(t, json.Unmarshal(cfg.GenesisState[types.ModuleName], &genesis))
	require.Equal(t, *types.DefaultGenesis(), genesis)
	net := network.New(t, cfg)
	require.Len(t, net.Validators, 1)
	_, err := net.WaitForHeight(2)
	require.NoError(t, err)
	val := net.Validators[0]
	run := func(cmd *cobra.Command, args ...string) (string, error) {
		args = append(args, "--node", val.RPCAddress, "--output", "json")
		return executeMarketsQuery(val.ClientCtx, cmd, args...)
	}

	t.Run("markets", func(t *testing.T) {
		for _, args := range [][]string{
			{"--limit", "10"},
			nil,
			{"--offset", "10", "--limit", "100", "--height", "1"},
			{"--offset", "18446744073709551614", "--limit", "1"},
		} {
			output, err := run(cli.CmdQueryMarkets(), args...)
			require.NoError(t, err)
			require.JSONEq(t, `{"markets":[],"total":"0"}`, output)
			var response types.QueryMarketsResponse
			require.NoError(t, val.ClientCtx.Codec.UnmarshalAsJSON([]byte(output), &response))
			require.Empty(t, response.Markets)
			require.Zero(t, response.Total)
		}
	})
	t.Run("markets-by-creator", func(t *testing.T) {
		for _, args := range [][]string{
			{val.Address.String(), "--limit", "10"},
			{val.Address.String()},
			{val.Address.String(), "--offset", "10", "--limit", "100"},
		} {
			output, err := run(cli.CmdQueryMarketsByCreator(), args...)
			require.NoError(t, err)
			require.JSONEq(t, `{"markets":[]}`, output)
			var response types.QueryMarketsByCreatorResponse
			require.NoError(t, val.ClientCtx.Codec.UnmarshalAsJSON([]byte(output), &response))
			require.Empty(t, response.Markets)
		}
		output, err := run(cli.CmdQueryMarketsByCreator(), "invalid-creator", "--limit", "10")
		require.ErrorContains(t, err, "invalid address")
		require.Empty(t, output)
	})
	t.Run("invalid range", func(t *testing.T) {
		for _, tc := range []struct {
			cmd  *cobra.Command
			args []string
		}{
			{cli.CmdQueryMarkets(), []string{"--limit", "101"}},
			{cli.CmdQueryMarketsByCreator(), []string{val.Address.String(), "--limit", "101"}},
			{cli.CmdQueryMarkets(), []string{"--limit", "0"}},
			{cli.CmdQueryMarkets(), []string{"--offset", "18446744073709551615", "--limit", "1"}},
		} {
			output, err := run(tc.cmd, tc.args...)
			require.ErrorContains(t, err, types.ErrInvalidRange.Error())
			require.Empty(t, output)
		}
		node, err := client.NewClientFromNode(val.RPCAddress)
		require.NoError(t, err)
		data, err := (&types.QueryMarketsRequest{Limit: 101}).Marshal()
		require.NoError(t, err)
		response, err := node.ABCIQuery(context.Background(), "/paxprotocol.paxchain.launchpad.Query/Markets", data)
		require.NoError(t, err)
		require.Equal(t, uint32(30), response.Response.Code)
		require.Equal(t, types.ModuleName, response.Response.Codespace)
		require.Contains(t, response.Response.Log, types.ErrInvalidRange.Error())
	})
}
