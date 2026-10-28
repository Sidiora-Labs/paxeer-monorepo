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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCmdQueryAirdropArgs(t *testing.T) {
	pool := "0x0000000000000000000000000000000000000001"
	holder := "0x0000000000000000000000000000000000000002"
	for _, tc := range []struct {
		newCmd func() *cobra.Command
		use    string
		args   []string
	}{
		{cli.CmdQueryAirdropEpoch, "airdrop-epoch [pool] [epoch]", []string{pool, "1"}},
		{cli.CmdQueryAirdropClaimed, "airdrop-claimed [pool] [holder] [epoch]", []string{pool, holder, "1"}},
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
				output, err := executeAirdropQuery(client.Context{}, tc.newCmd(), args...)
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
			for _, value := range []string{"", "abc", "-1", "+1", "1.5", "0x10", "18446744073709551616"} {
				t.Run("epoch="+value, func(t *testing.T) {
					args := append([]string{}, tc.args...)
					args[len(args)-1] = value
					output, err := executeAirdropQuery(client.Context{}, tc.newCmd(), append([]string{"--"}, args...)...)
					var numberError *strconv.NumError
					require.ErrorAs(t, err, &numberError)
					require.Equal(t, value, numberError.Num)
					require.Equal(t, "ParseUint", numberError.Func)
					require.ErrorContains(t, err, "epoch")
					require.Empty(t, output)
				})
			}
		})
	}
}

func TestCmdQueryAirdropNetwork(t *testing.T) {
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
	pool := "0x0000000000000000000000000000000000000001"
	holder := "0x0000000000000000000000000000000000000002"
	run := func(cmd *cobra.Command, args ...string) (string, error) {
		args = append(args, "--node", validator.RPCAddress, "--output", "json", "--height", "1")
		return executeAirdropQuery(validator.ClientCtx, cmd, args...)
	}
	for _, epoch := range []string{"1", "18446744073709551615"} {
		t.Run("epoch="+epoch, func(t *testing.T) {
			output, err := run(cli.CmdQueryAirdropEpoch(), pool, epoch)
			require.NoError(t, err)
			require.JSONEq(t, `{"found":false,"airdrop_epoch":null}`, output)
			var response types.QueryAirdropEpochResponse
			require.NoError(t, cfg.Codec.UnmarshalAsJSON([]byte(output), &response))
			require.False(t, response.Found)
			require.Nil(t, response.AirdropEpoch)

			output, err = run(cli.CmdQueryAirdropClaimed(), pool, holder, epoch)
			require.NoError(t, err)
			require.JSONEq(t, `{"claimed":false}`, output)
			var claimed types.QueryAirdropClaimedResponse
			require.NoError(t, cfg.Codec.UnmarshalAsJSON([]byte(output), &claimed))
			require.False(t, claimed.Claimed)
		})
	}
	t.Run("invalid epoch on node", func(t *testing.T) {
		output, commandErr := run(cli.CmdQueryAirdropEpoch(), pool, "0")
		require.ErrorContains(t, commandErr, types.ErrInvalidAirdropEpoch.Error())
		require.Equal(t, codes.Unknown, status.Code(commandErr))
		require.Empty(t, output)
		data, err := cfg.Codec.Marshal(&types.QueryAirdropEpochRequest{Pool: pool, Epoch: 0})
		require.NoError(t, err)
		response, err := validator.RPCClient.ABCIQueryWithOptions(context.Background(),
			"/paxprotocol.paxchain.launchpad.Query/AirdropEpoch", data, rpcclient.ABCIQueryOptions{Height: 1})
		require.NoError(t, err)
		require.Equal(t, uint32(27), response.Response.Code)
		require.Equal(t, types.ModuleName, response.Response.Codespace)
		require.Contains(t, response.Response.Log, types.ErrInvalidAirdropEpoch.Error())
		require.Equal(t, response.Response.Log, status.Convert(commandErr).Message())
	})
	t.Run("invalid addresses on node", func(t *testing.T) {
		for _, tc := range []struct {
			newCmd   func() *cobra.Command
			args     []string
			method   string
			request  codec.ProtoMarshaler
			grpcCode codes.Code
			abciCode uint32
		}{
			{cli.CmdQueryAirdropEpoch, []string{pool[2:], "1"}, "AirdropEpoch", &types.QueryAirdropEpochRequest{Pool: pool[2:], Epoch: 1}, codes.InvalidArgument, 18},
			{cli.CmdQueryAirdropClaimed, []string{pool[2:], holder, "1"}, "AirdropClaimed", &types.QueryAirdropClaimedRequest{Pool: pool[2:], Holder: holder, Epoch: 1}, codes.Unknown, 7},
			{cli.CmdQueryAirdropClaimed, []string{pool, holder[2:], "1"}, "AirdropClaimed", &types.QueryAirdropClaimedRequest{Pool: pool, Holder: holder[2:], Epoch: 1}, codes.Unknown, 7},
		} {
			output, commandErr := run(tc.newCmd(), tc.args...)
			require.ErrorContains(t, commandErr, "invalid address")
			require.Equal(t, tc.grpcCode, status.Code(commandErr))
			require.Empty(t, output)
			data, err := cfg.Codec.Marshal(tc.request)
			require.NoError(t, err)
			response, err := validator.RPCClient.ABCIQueryWithOptions(context.Background(),
				"/paxprotocol.paxchain.launchpad.Query/"+tc.method, data, rpcclient.ABCIQueryOptions{Height: 1})
			require.NoError(t, err)
			require.Equal(t, tc.abciCode, response.Response.Code)
			require.Equal(t, "sdk", response.Response.Codespace)
			require.Equal(t, response.Response.Log, status.Convert(commandErr).Message())
		}
	})
}

func executeAirdropQuery(clientCtx client.Context, cmd *cobra.Command, args ...string) (string, error) {
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
