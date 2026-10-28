package rest_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Sidiora-Labs/Paxeer-X-Network/engine/deps/testutil/network"
	"github.com/Sidiora-Labs/Paxeer-X-Network/modules/launchpad/types"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/params"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/baseapp"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/hd"
	servertypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/server/types"
	storetypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/store/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil"
	sdknetwork "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/testutil/network"
	grpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types/grpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/wasm/x/wasm"
	"github.com/gogo/protobuf/proto"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
)

func TestRESTLedgerRoutes(t *testing.T) {
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
	val := net.Validators[0]
	val.ClientCtx = val.ClientCtx.WithHeight(1)
	queryClient := types.NewQueryClient(val.ClientCtx)
	headers := map[string]string{grpctypes.GRPCBlockHeightHeader: "1"}
	baseURL := val.APIAddress + "/pax-protocol/paxchain/launchpad/"
	compare := func(t *testing.T, route string, response, expected proto.Message) []byte {
		t.Helper()
		body, err := testutil.GetRequestWithHeaders(baseURL+route, headers)
		require.NoError(t, err)
		require.NoError(t, val.ClientCtx.Codec.UnmarshalAsJSON(body, response), "%s: %s", route, body)
		require.Equal(t, expected, response)
		want, err := val.ClientCtx.Codec.MarshalAsJSON(expected)
		require.NoError(t, err)
		require.JSONEq(t, string(want), string(body))
		return body
	}

	t.Run("treasury", func(t *testing.T) {
		request := &types.QueryTreasuryRequest{Token: genesis.Params.QuoteToken}
		expected, err := queryClient.Treasury(context.Background(), request)
		require.NoError(t, err)
		var response types.QueryTreasuryResponse
		compare(t, "treasury/"+request.Token, &response, expected)
		require.Equal(t, "0", response.Balance)
		require.Equal(t, "0", response.Held)
	})

	t.Run("quote_ledger", func(t *testing.T) {
		request := &types.QueryQuoteLedgerRequest{Kind: "treasury-held"}
		expected, err := queryClient.QuoteLedger(context.Background(), request)
		require.NoError(t, err)
		var response types.QueryQuoteLedgerResponse
		compare(t, "quote_ledger/"+request.Kind, &response, expected)
		require.Equal(t, "0", response.Amount)
	})

	t.Run("fee_accounting", func(t *testing.T) {
		expected, err := queryClient.FeeAccounting(context.Background(), &types.QueryFeeAccountingRequest{})
		require.NoError(t, err)
		var response types.QueryFeeAccountingResponse
		body := compare(t, "fee_accounting", &response, expected)
		require.False(t, response.Accounting.TotalLiabilities.IsNil())
		require.True(t, response.Accounting.TotalLiabilities.IsZero())
		var printed struct {
			Accounting map[string]json.RawMessage `json:"accounting"`
		}
		require.NoError(t, json.Unmarshal(body, &printed))
		require.JSONEq(t, `"0"`, string(printed.Accounting["total_liabilities"]))
	})
}
