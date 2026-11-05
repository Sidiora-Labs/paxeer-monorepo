package rest_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tmdb "github.com/tendermint/tm-db"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

func TestRESTRegistryRoutes(t *testing.T) {
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
	_, err = net.WaitForHeight(2)
	require.NoError(t, err)
	require.Len(t, net.Validators, 1)
	val := net.Validators[0]
	val.ClientCtx = val.ClientCtx.WithHeight(1)
	queryClient := types.NewQueryClient(val.ClientCtx)
	headers := map[string]string{grpctypes.GRPCBlockHeightHeader: "1"}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	ctx = metadata.NewOutgoingContext(ctx, metadata.Pairs(grpctypes.GRPCBlockHeightHeader, "1"))

	const address = "0x1111111111111111111111111111111111111111"
	const account = "0x2222222222222222222222222222222222222222"
	const malformed = "not-an-address"
	role := "0x" + strings.Repeat("00", 32)
	testCases := []struct {
		name             string
		path             func(string, string) string
		query            func(context.Context, string, string) (proto.Message, error)
		response         proto.Message
		expected         proto.Message
		secondaryAddress string
	}{
		{
			name: "surface",
			path: func(a, _ string) string { return "surface/" + a },
			query: func(ctx context.Context, a, _ string) (proto.Message, error) {
				return queryClient.Surface(ctx, &types.QuerySurfaceRequest{Address: a})
			},
			response: &types.QuerySurfaceResponse{},
			expected: &types.QuerySurfaceResponse{Found: false},
		},
		{
			name: "retired",
			path: func(a, _ string) string { return "retired/" + a },
			query: func(ctx context.Context, a, _ string) (proto.Message, error) {
				return queryClient.Retired(ctx, &types.QueryRetiredRequest{Address: a})
			},
			response: &types.QueryRetiredResponse{},
			expected: &types.QueryRetiredResponse{Found: false},
		},
		{
			name: "fee_right",
			path: func(a, _ string) string { return "fee_right/" + a + "/1" },
			query: func(ctx context.Context, a, _ string) (proto.Message, error) {
				return queryClient.FeeRight(ctx, &types.QueryFeeRightRequest{Collection: a, Id: 1})
			},
			response: &types.QueryFeeRightResponse{},
			expected: &types.QueryFeeRightResponse{Found: false},
		},
		{
			name: "pool_metadata",
			path: func(a, _ string) string { return "pool_metadata/" + a },
			query: func(ctx context.Context, a, _ string) (proto.Message, error) {
				return queryClient.PoolMetadata(ctx, &types.QueryPoolMetadataRequest{Pool: a})
			},
			response: &types.QueryPoolMetadataResponse{},
			expected: &types.QueryPoolMetadataResponse{Found: false},
		},
		{
			name: "airdrop_epoch",
			path: func(a, _ string) string { return "airdrop_epoch/" + a + "/1" },
			query: func(ctx context.Context, a, _ string) (proto.Message, error) {
				return queryClient.AirdropEpoch(ctx, &types.QueryAirdropEpochRequest{Pool: a, Epoch: 1})
			},
			response: &types.QueryAirdropEpochResponse{},
			expected: &types.QueryAirdropEpochResponse{Found: false},
		},
		{
			name: "airdrop_claimed",
			path: func(a, holder string) string { return fmt.Sprintf("airdrop_claimed/%s/%s/1", a, holder) },
			query: func(ctx context.Context, a, holder string) (proto.Message, error) {
				return queryClient.AirdropClaimed(ctx, &types.QueryAirdropClaimedRequest{Pool: a, Holder: holder, Epoch: 1})
			},
			response:         &types.QueryAirdropClaimedResponse{},
			expected:         &types.QueryAirdropClaimedResponse{Claimed: false},
			secondaryAddress: "holder",
		},
		{
			name: "has_role",
			path: func(a, holder string) string { return fmt.Sprintf("has_role/%s/%s/%s", a, role, holder) },
			query: func(ctx context.Context, a, holder string) (proto.Message, error) {
				return queryClient.HasRole(ctx, &types.QueryHasRoleRequest{Contract: a, Role: role, Account: holder})
			},
			response:         &types.QueryHasRoleResponse{},
			expected:         &types.QueryHasRoleResponse{HasRole: false},
			secondaryAddress: "account",
		},
	}

	baseURL := val.APIAddress + "/pax-protocol/paxchain/launchpad/"
	httpClient := &http.Client{Timeout: 10 * time.Second}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Run("unregistered", func(t *testing.T) {
				grpcResponse, err := tc.query(ctx, address, account)
				require.NoError(t, err)
				require.Equal(t, tc.expected, grpcResponse)
				body, err := testutil.GetRequestWithHeaders(baseURL+tc.path(address, account), headers)
				require.NoError(t, err)
				require.NoError(t, val.ClientCtx.Codec.UnmarshalAsJSON(body, tc.response), "body: %s", body)
				require.Equal(t, grpcResponse, tc.response)
				require.Equal(t, tc.expected, tc.response)
				want, err := val.ClientCtx.Codec.MarshalAsJSON(grpcResponse)
				require.NoError(t, err)
				require.JSONEq(t, string(want), string(body))
			})

			invalidCases := []struct {
				name    string
				primary string
				other   string
			}{
				{name: "malformed_address", primary: malformed, other: account},
			}
			if tc.secondaryAddress != "" {
				invalidCases = append(invalidCases, struct {
					name    string
					primary string
					other   string
				}{name: "malformed_" + tc.secondaryAddress, primary: address, other: malformed})
			}
			for _, invalid := range invalidCases {
				t.Run(invalid.name, func(t *testing.T) {
					_, grpcErr := tc.query(ctx, invalid.primary, invalid.other)
					require.Error(t, grpcErr)
					assert.Equal(t, codes.InvalidArgument, status.Code(grpcErr))
					url := baseURL + tc.path(invalid.primary, invalid.other)
					body, err := testutil.GetRequestWithHeaders(url, headers)
					require.NoError(t, err)
					var errorBody struct {
						Code    int    `json:"code"`
						Message string `json:"message"`
					}
					require.NoError(t, json.Unmarshal(body, &errorBody), "body: %s", body)
					assert.NotEqual(t, int(codes.OK), errorBody.Code)
					assert.Equal(t, int(codes.InvalidArgument), errorBody.Code)
					assert.Equal(t, int(status.Code(grpcErr)), errorBody.Code)
					assert.Equal(t, status.Convert(grpcErr).Message(), errorBody.Message)
					assert.Contains(t, errorBody.Message, "40-hex-digit address")

					req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
					require.NoError(t, err)
					for name, value := range headers {
						req.Header.Set(name, value)
					}
					res, err := httpClient.Do(req)
					require.NoError(t, err)
					defer res.Body.Close()
					assert.Equal(t, http.StatusBadRequest, res.StatusCode, "body: %s", body)
					assert.GreaterOrEqual(t, res.StatusCode, http.StatusBadRequest, "body: %s", body)
					assert.Less(t, res.StatusCode, http.StatusInternalServerError, "body: %s", body)
				})
			}
		})
	}
}
