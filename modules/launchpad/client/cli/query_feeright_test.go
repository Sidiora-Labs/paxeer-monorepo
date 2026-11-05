package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	rpcclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/testutil/network"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/params"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdknetwork "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
	"google.golang.org/grpc/status"
)

func TestCmdQueryFeeRightArgs(t *testing.T) {
	address := "0x0000000000000000000000000000000000000001"
	for _, tc := range []struct {
		newCmd func() *cobra.Command
		use    string
		args   []string
	}{
		{cli.CmdQueryFeeRight, "fee-right [collection] [id]", []string{address, "1"}},
		{cli.CmdQueryPoolMetadata, "pool-metadata [pool]", []string{address}},
	} {
		t.Run(tc.use, func(t *testing.T) {
			cmd := tc.newCmd()
			require.Equal(t, tc.use, cmd.Use)
			require.NotNil(t, cmd.RunE)
			require.NotNil(t, cmd.Args)
			require.NoError(t, cmd.ValidateArgs(tc.args))
			for count := 0; count <= len(tc.args)+1; count++ {
				if count == len(tc.args) {
					continue
				}
				args := append(append([]string{}, tc.args...), "extra")[:count]
				want := fmt.Sprintf("accepts %d arg(s), received %d", len(tc.args), count)
				require.EqualError(t, cmd.ValidateArgs(args), want)
				output, err := executeFeeRightQuery(client.Context{}, tc.newCmd(), args...)
				require.EqualError(t, err, want)
				require.Empty(t, output)
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
	for _, value := range []string{"", "abc", "-1", "+1", "1.5", "0x10", "18446744073709551616"} {
		t.Run("id="+value, func(t *testing.T) {
			output, err := executeFeeRightQuery(client.Context{}, cli.CmdQueryFeeRight(), "--", address, value)
			var numberError *strconv.NumError
			require.ErrorAs(t, err, &numberError)
			require.Equal(t, value, numberError.Num)
			require.Equal(t, "ParseUint", numberError.Func)
			require.ErrorContains(t, err, "id")
			require.Empty(t, output)
		})
	}
}

func TestCmdQueryFeeRightNetwork(t *testing.T) {
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
	genesis, err := json.Marshal(types.DefaultGenesis())
	require.NoError(t, err)
	require.JSONEq(t, string(genesis), string(cfg.GenesisState[types.ModuleName]))
	net := network.New(t, cfg)
	require.Len(t, net.Validators, 1)
	_, err = net.WaitForHeight(2)
	require.NoError(t, err)
	validator := net.Validators[0]
	address := "0x0000000000000000000000000000000000000001"
	run := func(cmd *cobra.Command, args ...string) (string, error) {
		args = append(args, "--node", validator.RPCAddress, "--output", "json", "--height", "1")
		return executeFeeRightQuery(validator.ClientCtx, cmd, args...)
	}
	for _, id := range []string{"0", "1", "18446744073709551615"} {
		t.Run("fee-right/id="+id, func(t *testing.T) {
			output, err := run(cli.CmdQueryFeeRight(), address, id)
			require.NoError(t, err)
			require.JSONEq(t, `{"found":false,"fee_right":null}`, output)
			var response types.QueryFeeRightResponse
			require.NoError(t, cfg.Codec.UnmarshalAsJSON([]byte(output), &response))
			require.False(t, response.Found)
			require.Nil(t, response.FeeRight)
		})
	}
	t.Run("pool-metadata", func(t *testing.T) {
		output, err := run(cli.CmdQueryPoolMetadata(), address)
		require.NoError(t, err)
		require.JSONEq(t, `{"found":false,"metadata":null}`, output)
		var response types.QueryPoolMetadataResponse
		require.NoError(t, cfg.Codec.UnmarshalAsJSON([]byte(output), &response))
		require.False(t, response.Found)
		require.Nil(t, response.Metadata)
	})
	for _, invalid := range []string{address[2:], "invalid", "0x0000000000000000000000000000000000000000"} {
		for _, tc := range []struct {
			newCmd  func() *cobra.Command
			args    []string
			method  string
			request codec.ProtoMarshaler
		}{
			{cli.CmdQueryFeeRight, []string{invalid, "1"}, "FeeRight", &types.QueryFeeRightRequest{Collection: invalid, Id: 1}},
			{cli.CmdQueryPoolMetadata, []string{invalid}, "PoolMetadata", &types.QueryPoolMetadataRequest{Pool: invalid}},
		} {
			t.Run(tc.method+"/address="+invalid, func(t *testing.T) {
				output, commandErr := run(tc.newCmd(), tc.args...)
				require.Error(t, commandErr)
				require.Empty(t, output)
				data, err := cfg.Codec.Marshal(tc.request)
				require.NoError(t, err)
				response, err := validator.RPCClient.ABCIQueryWithOptions(context.Background(),
					"/paxprotocol.paxchain.launchpad.Query/"+tc.method, data, rpcclient.ABCIQueryOptions{Height: 1})
				require.NoError(t, err)
				require.NotZero(t, response.Response.Code)
				if invalid == "0x0000000000000000000000000000000000000000" {
					require.Contains(t, response.Response.Log, "must not be the zero address")
				} else {
					require.Contains(t, response.Response.Log, "must be a 0x-prefixed 40-hex-digit address")
				}
				require.Equal(t, response.Response.Log, status.Convert(commandErr).Message())
			})
		}
	}
}

func executeFeeRightQuery(clientCtx client.Context, cmd *cobra.Command, args ...string) (string, error) {
	var output bytes.Buffer
	clientCtx = clientCtx.WithOutput(&output)
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SilenceUsage = true
	cmd.SilenceErrors = true
	cmd.SetArgs(args)
	err := cmd.ExecuteContext(context.WithValue(context.Background(), client.ClientContextKey, &clientCtx))
	return output.String(), err
}
