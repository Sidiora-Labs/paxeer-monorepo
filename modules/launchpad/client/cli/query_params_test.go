package cli_test

import (
	"encoding/json"
	"fmt"
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
	clitestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/cli"
	sdknetwork "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
)

func TestCmdQueryParamsArgs(t *testing.T) {
	for _, tc := range []struct {
		use    string
		newCmd func() *cobra.Command
	}{
		{"params", cli.CmdQueryParams},
		{"fee-accounting", cli.CmdQueryFeeAccounting},
	} {
		t.Run(tc.use, func(t *testing.T) {
			cmd := tc.newCmd()
			require.Equal(t, tc.use, cmd.Use)
			require.NotNil(t, cmd.RunE)
			require.NotNil(t, cmd.Args)
			require.NoError(t, cmd.ValidateArgs(nil))
			for _, args := range [][]string{{"extra"}, {"one", "two"}} {
				want := fmt.Sprintf("accepts 0 arg(s), received %d", len(args))
				require.EqualError(t, cmd.ValidateArgs(args), want)
				invalid := tc.newCmd()
				invalid.SilenceUsage = true
				invalid.SilenceErrors = true
				output, err := clitestutil.ExecTestCLICmd(client.Context{}, invalid, args)
				require.EqualError(t, err, want)
				require.Empty(t, output.String())
			}
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
}

func TestCmdQueryParamsNetwork(t *testing.T) {
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
	height, err := net.WaitForHeight(1)
	require.NoError(t, err)
	validator := net.Validators[0]

	for _, tc := range []struct {
		name   string
		newCmd func() *cobra.Command
		check  func(*testing.T, []byte)
	}{
		{
			name: "params", newCmd: cli.CmdQueryParams,
			check: func(t *testing.T, output []byte) {
				var response types.QueryParamsResponse
				require.NoError(t, cfg.Codec.UnmarshalAsJSON(output, &response))
				require.Equal(t, genesis.Params, response.Params)
				want, err := cfg.Codec.MarshalAsJSON(&types.QueryParamsResponse{Params: genesis.Params})
				require.NoError(t, err)
				require.JSONEq(t, string(want), string(output))
			},
		},
		{
			name: "fee-accounting", newCmd: cli.CmdQueryFeeAccounting,
			check: func(t *testing.T, output []byte) {
				var response types.QueryFeeAccountingResponse
				require.NoError(t, cfg.Codec.UnmarshalAsJSON(output, &response))
				require.False(t, response.Accounting.TotalLiabilities.IsNil())
				require.True(t, response.Accounting.TotalLiabilities.IsZero())
				require.False(t, response.Accounting.PendingProtocolFees.IsNil())
				require.True(t, response.Accounting.PendingProtocolFees.IsZero())
				var printed struct {
					Accounting map[string]json.RawMessage `json:"accounting"`
				}
				require.NoError(t, json.Unmarshal(output, &printed))
				require.JSONEq(t, `"0"`, string(printed.Accounting["total_liabilities"]))
				require.JSONEq(t, `"0"`, string(printed.Accounting["pending_protocol_fees"]))
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := tc.newCmd()
			output, err := clitestutil.ExecTestCLICmd(validator.ClientCtx, cmd, []string{
				"--node", validator.RPCAddress,
				"--height", strconv.FormatInt(height, 10), "--output", "json",
			})
			require.NoError(t, err)
			tc.check(t, output.Bytes())
		})
	}
}
