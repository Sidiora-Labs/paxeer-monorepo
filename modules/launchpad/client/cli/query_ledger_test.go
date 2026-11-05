package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
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

func TestCmdQueryLedgerArgs(t *testing.T) {
	address := types.DefaultGenesis().Params.QuoteToken
	for _, tc := range []struct {
		newCmd func() *cobra.Command
		use    string
		args   []string
	}{
		{cli.CmdQueryTreasury, "treasury [token]", []string{address}},
		{cli.CmdQueryQuoteLedger, "quote-ledger [held|fee-held|treasury-held] [address]", []string{"treasury-held", address}},
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
				output, err := executeLedgerQuery(client.Context{}, tc.newCmd(), args...)
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
}

func TestCmdQueryLedgerNetwork(t *testing.T) {
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
	genesis := types.DefaultGenesis()
	genesisJSON, err := json.Marshal(genesis)
	require.NoError(t, err)
	require.JSONEq(t, string(genesisJSON), string(cfg.GenesisState[types.ModuleName]))
	net := network.New(t, cfg)
	require.Len(t, net.Validators, 1)
	_, err = net.WaitForHeight(2)
	require.NoError(t, err)
	validator := net.Validators[0]
	address := genesis.Params.QuoteToken
	run := func(cmd *cobra.Command, args ...string) (string, error) {
		args = append(args, "--node", validator.RPCAddress, "--output", "json", "--height", "1")
		return executeLedgerQuery(validator.ClientCtx, cmd, args...)
	}
	t.Run("treasury", func(t *testing.T) {
		output, err := run(cli.CmdQueryTreasury(), address)
		require.NoError(t, err)
		require.JSONEq(t, `{"balance":"0","held":"0"}`, output)
		var response types.QueryTreasuryResponse
		require.NoError(t, cfg.Codec.UnmarshalAsJSON([]byte(output), &response))
		require.Equal(t, "0", response.Balance)
		require.Equal(t, "0", response.Held)
	})
	for _, kind := range []string{"held", "fee-held", "treasury-held"} {
		t.Run(kind, func(t *testing.T) {
			output, err := run(cli.CmdQueryQuoteLedger(), kind, address)
			require.NoError(t, err)
			require.JSONEq(t, `{"amount":"0"}`, output)
			var response types.QueryQuoteLedgerResponse
			require.NoError(t, cfg.Codec.UnmarshalAsJSON([]byte(output), &response))
			require.Equal(t, "0", response.Amount)
		})
	}
	for _, tc := range []struct {
		name    string
		newCmd  func() *cobra.Command
		args    []string
		method  string
		request codec.ProtoMarshaler
		message string
	}{
		{"unknown kind", cli.CmdQueryQuoteLedger, []string{"other", address}, "QuoteLedger", &types.QueryQuoteLedgerRequest{Kind: "other", Address: address}, `unknown quote ledger kind "other"`},
		{"invalid token", cli.CmdQueryTreasury, []string{address[2:]}, "Treasury", &types.QueryTreasuryRequest{Token: address[2:]}, "token must be a 0x-prefixed 40-hex-digit address"},
		{"invalid held address", cli.CmdQueryQuoteLedger, []string{"held", "invalid"}, "QuoteLedger", &types.QueryQuoteLedgerRequest{Kind: "held", Address: "invalid"}, "invalid quote ledger address"},
		{"invalid fee-held address", cli.CmdQueryQuoteLedger, []string{"fee-held", "invalid"}, "QuoteLedger", &types.QueryQuoteLedgerRequest{Kind: "fee-held", Address: "invalid"}, "invalid quote ledger address"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output, commandErr := run(tc.newCmd(), tc.args...)
			require.ErrorContains(t, commandErr, tc.message)
			require.Empty(t, output)
			data, err := cfg.Codec.Marshal(tc.request)
			require.NoError(t, err)
			response, err := validator.RPCClient.ABCIQueryWithOptions(context.Background(),
				"/paxprotocol.paxchain.launchpad.Query/"+tc.method, data, rpcclient.ABCIQueryOptions{Height: 1})
			require.NoError(t, err)
			require.NotZero(t, response.Response.Code)
			require.Contains(t, response.Response.Log, tc.message)
			require.Equal(t, response.Response.Log, status.Convert(commandErr).Message())
		})
	}
}

func executeLedgerQuery(clientCtx client.Context, cmd *cobra.Command, args ...string) (string, error) {
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
