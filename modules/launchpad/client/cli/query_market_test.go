package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"strconv"
	"testing"

	rpcclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/testutil/network"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	sdknetwork "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/gogo/protobuf/proto"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestCmdQueryMarketArgs(t *testing.T) {
	address := "0x0000000000000000000000000000000000000001"
	for _, tc := range []struct {
		newCmd func() *cobra.Command
		use    string
		arg    string
	}{
		{cli.CmdQueryMarket, "market [denom]", "launchpad/" + address},
		{cli.CmdQueryMarketByAddress, "market-by-address [address]", address},
	} {
		t.Run(tc.use, func(t *testing.T) {
			cmd := tc.newCmd()
			require.Equal(t, tc.use, cmd.Use)
			require.NotNil(t, cmd.RunE)
			require.NotNil(t, cmd.Args)
			require.EqualError(t, cmd.ValidateArgs(nil), "accepts 1 arg(s), received 0")
			require.NoError(t, cmd.ValidateArgs([]string{tc.arg}))
			require.EqualError(t, cmd.ValidateArgs([]string{tc.arg, "extra"}), "accepts 1 arg(s), received 2")

			standard := &cobra.Command{}
			flags.AddQueryFlagsToCmd(standard)
			standard.Flags().VisitAll(func(want *pflag.Flag) {
				got := cmd.Flags().Lookup(want.Name)
				require.NotNil(t, got, "missing query flag %s", want.Name)
				require.Equal(t, want.Value.Type(), got.Value.Type())
				require.Equal(t, want.DefValue, got.DefValue)
				require.Equal(t, want.Shorthand, got.Shorthand)
			})
			for _, args := range [][]string{nil, {tc.arg, "extra"}} {
				output, err := executeMarketQuery(tc.newCmd(), client.Context{}, args...)
				require.ErrorContains(t, err, "accepts 1 arg(s)")
				require.Empty(t, output)
			}
		})
	}
}

func TestCmdQueryMarketNetwork(t *testing.T) {
	cfg := network.DefaultConfig()
	cfg.SigningAlgo = string(hd.Secp256k1Type)
	encoding := app.MakeEncodingConfig()
	cfg.AppConstructor = func(val sdknetwork.Validator) servertypes.Application {
		return app.New(
			tmdb.NewMemDB(), nil, true, map[int64]bool{}, val.Ctx.Config.RootDir, 0, false,
			nil, encoding, wasm.EnableAllProposals, app.TestAppOpts{}, nil, app.EmptyAppOptions,
			baseapp.SetPruning(storetypes.NewPruningOptionsFromString(val.AppConfig.Pruning)),
			baseapp.SetMinGasPrices(val.AppConfig.MinGasPrices),
		)
	}
	require.Equal(t, 1, cfg.NumValidators)
	genesis, err := json.Marshal(types.DefaultGenesis())
	require.NoError(t, err)
	require.JSONEq(t, string(genesis), string(cfg.GenesisState[types.ModuleName]))
	net := network.New(t, cfg)
	height, err := net.WaitForHeight(1)
	require.NoError(t, err)
	require.Len(t, net.Validators, 1)
	validator := net.Validators[0]
	address := "0x0000000000000000000000000000000000000001"
	for _, tc := range []struct {
		newCmd  func() *cobra.Command
		arg     string
		method  string
		request proto.Message
	}{
		{cli.CmdQueryMarket, "launchpad/" + address, "Market", &types.QueryMarketRequest{Denom: "launchpad/" + address}},
		{cli.CmdQueryMarketByAddress, address, "MarketByAddress", &types.QueryMarketByAddressRequest{Address: address}},
	} {
		t.Run(tc.method, func(t *testing.T) {
			data, err := proto.Marshal(tc.request)
			require.NoError(t, err)
			response, err := validator.RPCClient.ABCIQueryWithOptions(context.Background(),
				"/paxprotocol.paxchain.launchpad.Query/"+tc.method, data, rpcclient.ABCIQueryOptions{Height: height})
			require.NoError(t, err)
			require.Equal(t, "launchpad", response.Response.Codespace)
			require.Equal(t, uint32(12), response.Response.Code)
			require.Contains(t, response.Response.Log, types.ErrUnknownMarket.Error())

			output, err := executeMarketQuery(tc.newCmd(), validator.ClientCtx,
				tc.arg, "--"+flags.FlagHeight, strconv.FormatInt(height, 10), "--"+flags.FlagOutput, "json")
			require.Error(t, err)
			require.Equal(t, codes.Unknown, status.Code(err))
			require.Equal(t, response.Response.Log, status.Convert(err).Message())
			require.Empty(t, output)
		})
	}
}

func executeMarketQuery(cmd *cobra.Command, clientCtx client.Context, args ...string) (string, error) {
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
