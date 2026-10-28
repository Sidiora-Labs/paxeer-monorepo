package cli_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	rpcclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/params"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	clitestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCmdQueryPriceArgs(t *testing.T) {
	for _, tc := range []struct {
		newCmd func() *cobra.Command
		use    string
		args   []string
	}{
		{cli.CmdQueryPriceHistory, "price-history [denom]", []string{"unknown-denom"}},
		{cli.CmdQueryQuote, "quote [denom] [buy|sell] [amount-in]", []string{"unknown-denom", "buy", "1"}},
	} {
		t.Run(tc.use, func(t *testing.T) {
			cmd := tc.newCmd()
			require.Equal(t, tc.use, cmd.Use)
			require.NotNil(t, cmd.RunE)
			require.NoError(t, cmd.ValidateArgs(tc.args))
			for count := 0; count < len(tc.args); count++ {
				require.EqualError(t, cmd.ValidateArgs(tc.args[:count]), fmt.Sprintf("accepts %d arg(s), received %d", len(tc.args), count))
			}
			extra := append(append([]string{}, tc.args...), "extra")
			require.EqualError(t, cmd.ValidateArgs(extra), fmt.Sprintf("accepts %d arg(s), received %d", len(tc.args), len(extra)))
			standard := &cobra.Command{}
			flags.AddQueryFlagsToCmd(standard)
			standard.Flags().VisitAll(func(want *pflag.Flag) {
				got := cmd.Flags().Lookup(want.Name)
				require.NotNil(t, got, "missing query flag %s", want.Name)
				require.Equal(t, want.Value.Type(), got.Value.Type())
				require.Equal(t, want.DefValue, got.DefValue)
				require.Equal(t, want.Shorthand, got.Shorthand)
			})
		})
	}
	for _, flag := range []string{"from-height", "to-height"} {
		require.NotNil(t, cli.CmdQueryPriceHistory().Flags().Lookup(flag))
		for _, value := range []string{"", "abc", "-1", "1.5", "0x10", "18446744073709551616"} {
			t.Run(flag+"="+value, func(t *testing.T) {
				_, err := clitestutil.ExecTestCLICmd(client.Context{}, cli.CmdQueryPriceHistory(), []string{"unknown-denom", "--" + flag + "=" + value})
				var numberError *strconv.NumError
				require.ErrorAs(t, err, &numberError)
				require.Equal(t, value, numberError.Num)
				require.ErrorContains(t, err, flag)
			})
		}
	}
	for _, value := range []string{"", "abc", "-1", "1.5", "0x10", "18446744073709551616"} {
		t.Run("amount="+value, func(t *testing.T) {
			_, err := clitestutil.ExecTestCLICmd(client.Context{}, cli.CmdQueryQuote(), []string{"--", "unknown-denom", "buy", value})
			var numberError *strconv.NumError
			require.ErrorAs(t, err, &numberError)
			require.Equal(t, value, numberError.Num)
			require.ErrorContains(t, err, "amount-in")
		})
	}
	_, err := clitestutil.ExecTestCLICmd(client.Context{}, cli.CmdQueryQuote(), []string{"unknown-denom", "hold", "1"})
	require.ErrorContains(t, err, "side must be buy or sell")
}

func TestCmdQueryPriceNetwork(t *testing.T) {
	cfg := network.DefaultConfig(t)
	cfg.NumValidators = 1
	cfg.AppConstructor = func(val network.Validator) servertypes.Application {
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
	genesis, err := json.Marshal(types.DefaultGenesis())
	require.NoError(t, err)
	require.JSONEq(t, string(genesis), string(cfg.GenesisState[types.ModuleName]))
	net := network.New(t, cfg)
	t.Cleanup(net.Cleanup)
	require.Len(t, net.Validators, 1)
	_, err = net.WaitForHeight(1)
	require.NoError(t, err)
	validator := net.Validators[0]

	for _, tc := range []struct {
		name    string
		newCmd  func() *cobra.Command
		args    []string
		method  string
		request codec.ProtoMarshaler
	}{
		{"history", cli.CmdQueryPriceHistory, []string{"unknown-denom", "--from-height=0", "--to-height=1"}, "PriceHistory", &types.QueryPriceHistoryRequest{Denom: "unknown-denom", ToHeight: 1}},
		{"buy", cli.CmdQueryQuote, []string{"unknown-denom", "buy", "1000000"}, "Quote", &types.QueryQuoteRequest{Denom: "unknown-denom", IsBuy: true, AmountIn: "1000000"}},
		{"sell", cli.CmdQueryQuote, []string{"unknown-denom", "sell", "1000000"}, "Quote", &types.QueryQuoteRequest{Denom: "unknown-denom", AmountIn: "1000000"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append(append([]string{}, tc.args...), "--output=json")
			_, commandErr := clitestutil.ExecTestCLICmd(validator.ClientCtx, tc.newCmd(), args)
			require.ErrorContains(t, commandErr, types.ErrUnknownMarket.Error())
			require.Equal(t, codes.Unknown, status.Code(commandErr))
			data, err := cfg.Codec.Marshal(tc.request)
			require.NoError(t, err)
			response, err := validator.RPCClient.ABCIQueryWithOptions(context.Background(), "/paxprotocol.paxchain.launchpad.Query/"+tc.method, data, rpcclient.ABCIQueryOptions{})
			require.NoError(t, err)
			require.Equal(t, uint32(12), response.Response.Code)
			require.Equal(t, types.ModuleName, response.Response.Codespace)
			require.Contains(t, response.Response.Log, types.ErrUnknownMarket.Error())
			require.Equal(t, response.Response.Log, status.Convert(commandErr).Message())
		})
	}
	t.Run("invalid amount before RPC", func(t *testing.T) {
		for _, clientCtx := range []client.Context{validator.ClientCtx, {}} {
			_, err := clitestutil.ExecTestCLICmd(clientCtx, cli.CmdQueryQuote(), []string{"unknown-denom", "buy", "abc", "--output=json"})
			var numberError *strconv.NumError
			require.ErrorAs(t, err, &numberError)
			require.Equal(t, "abc", numberError.Num)
			require.ErrorIs(t, err, strconv.ErrSyntax)
		}
	})
}
