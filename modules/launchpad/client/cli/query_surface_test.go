package cli_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
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
	clitestutil "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/cli"
	sdknetwork "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	sdkerrors "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/errors"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCmdQuerySurfaceArgs(t *testing.T) {
	address := "0x0000000000000000000000000000000000000001"
	role := "0x" + strings.Repeat("0", 64)
	for _, tc := range []struct {
		newCmd func() *cobra.Command
		use    string
		args   []string
	}{
		{cli.CmdQuerySurface, "surface [address]", []string{address}},
		{cli.CmdQueryRetired, "retired [address]", []string{address}},
		{cli.CmdQueryHasRole, "has-role [contract] [role] [account]", []string{address, role, address}},
	} {
		t.Run(tc.use, func(t *testing.T) {
			cmd := tc.newCmd()
			require.Equal(t, tc.use, cmd.Use)
			require.NotNil(t, cmd.RunE)
			require.NotNil(t, cmd.Args)
			require.NoError(t, cmd.ValidateArgs(tc.args))
			for count := 0; count < len(tc.args); count++ {
				want := fmt.Sprintf("accepts %d arg(s), received %d", len(tc.args), count)
				require.EqualError(t, cmd.ValidateArgs(tc.args[:count]), want)
				_, err := clitestutil.ExecTestCLICmd(client.Context{}, tc.newCmd(), tc.args[:count])
				require.ErrorContains(t, err, want)
			}
			extra := append(append([]string{}, tc.args...), "extra")
			want := fmt.Sprintf("accepts %d arg(s), received %d", len(tc.args), len(extra))
			require.EqualError(t, cmd.ValidateArgs(extra), want)
			_, err := clitestutil.ExecTestCLICmd(client.Context{}, tc.newCmd(), extra)
			require.ErrorContains(t, err, want)

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

func TestCmdQuerySurfaceNetwork(t *testing.T) {
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
	address := "0x0000000000000000000000000000000000000001"
	account := "0x0000000000000000000000000000000000000002"
	role := "0x" + strings.Repeat("0", 64)
	run := func(cmd *cobra.Command, args []string) (string, error) {
		cmd.SilenceUsage = true
		cmd.SilenceErrors = true
		args = append(append([]string{}, args...), "--node", validator.RPCAddress,
			"--height", strconv.FormatInt(height, 10), "--output", "json")
		output, err := clitestutil.ExecTestCLICmd(validator.ClientCtx, cmd, args)
		return output.String(), err
	}
	for _, tc := range []struct {
		name   string
		newCmd func() *cobra.Command
		args   []string
		field  string
	}{
		{"surface", cli.CmdQuerySurface, []string{address}, "found"},
		{"retired", cli.CmdQueryRetired, []string{address}, "found"},
		{"has-role", cli.CmdQueryHasRole, []string{address, role, account}, "has_role"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, err := run(tc.newCmd(), tc.args)
			require.NoError(t, err)
			var response map[string]interface{}
			require.NoError(t, json.Unmarshal([]byte(output), &response))
			require.Contains(t, response, tc.field)
			require.Equal(t, false, response[tc.field])
		})
	}

	// Compare refusals with direct ABCI queries to prove the CLI passes the
	// original address and role strings to the node for validation.
	invalidAddress := strings.Repeat("1", 40)
	zeroAddress := "0x" + strings.Repeat("0", 40)
	for _, tc := range []struct {
		name     string
		newCmd   func() *cobra.Command
		args     []string
		method   string
		request  codec.ProtoMarshaler
		sdkErr   *sdkerrors.Error
		grpcCode codes.Code
	}{
		{"surface/address", cli.CmdQuerySurface, []string{invalidAddress}, "Surface", &types.QuerySurfaceRequest{Address: invalidAddress}, sdkerrors.ErrInvalidRequest, codes.InvalidArgument},
		{"surface/zero", cli.CmdQuerySurface, []string{zeroAddress}, "Surface", &types.QuerySurfaceRequest{Address: zeroAddress}, sdkerrors.ErrInvalidRequest, codes.InvalidArgument},
		{"retired/address", cli.CmdQueryRetired, []string{invalidAddress}, "Retired", &types.QueryRetiredRequest{Address: invalidAddress}, sdkerrors.ErrInvalidRequest, codes.InvalidArgument},
		{"retired/zero", cli.CmdQueryRetired, []string{zeroAddress}, "Retired", &types.QueryRetiredRequest{Address: zeroAddress}, sdkerrors.ErrInvalidRequest, codes.InvalidArgument},
		{"has-role/contract", cli.CmdQueryHasRole, []string{invalidAddress, role, account}, "HasRole", &types.QueryHasRoleRequest{Contract: invalidAddress, Role: role, Account: account}, sdkerrors.ErrInvalidAddress, codes.Unknown},
		{"has-role/account", cli.CmdQueryHasRole, []string{address, role, invalidAddress}, "HasRole", &types.QueryHasRoleRequest{Contract: address, Role: role, Account: invalidAddress}, sdkerrors.ErrInvalidAddress, codes.Unknown},
		{"has-role/role", cli.CmdQueryHasRole, []string{address, "0x01", account}, "HasRole", &types.QueryHasRoleRequest{Contract: address, Role: "0x01", Account: account}, sdkerrors.ErrInvalidRequest, codes.InvalidArgument},
	} {
		t.Run(tc.name, func(t *testing.T) {
			data, err := cfg.Codec.Marshal(tc.request)
			require.NoError(t, err)
			response, err := validator.RPCClient.ABCIQueryWithOptions(context.Background(),
				"/paxprotocol.paxchain.launchpad.Query/"+tc.method, data, rpcclient.ABCIQueryOptions{Height: height})
			require.NoError(t, err)
			require.NotZero(t, response.Response.Code)
			require.Equal(t, tc.sdkErr.Codespace(), response.Response.Codespace)
			require.Equal(t, tc.sdkErr.ABCICode(), response.Response.Code)
			require.NotEmpty(t, response.Response.Log)
			output, err := run(tc.newCmd(), tc.args)
			require.Error(t, err)
			require.Equal(t, tc.grpcCode, status.Code(err))
			require.Equal(t, response.Response.Log, status.Convert(err).Message())
			require.Empty(t, output)
		})
	}
}
