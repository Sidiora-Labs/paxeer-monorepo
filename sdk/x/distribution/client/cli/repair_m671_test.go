package cli_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/flags"
	clienttx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec"
	codectypes "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/codec/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/crypto/keyring"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	authtx "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/auth/tx"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/client/cli"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/x/distribution/types"
	"github.com/stretchr/testify/require"
)

func TestRepairM_671_NegativeChunks(t *testing.T) {
	del := sdk.AccAddress(bytes.Repeat([]byte{1}, 20))
	val := sdk.ValAddress(bytes.Repeat([]byte{2}, 20))
	for _, chunkSize := range []int{-1, -int(^uint(0)>>1) - 1} {
		for _, msgs := range [][]sdk.Msg{nil, {types.NewMsgWithdrawDelegatorReward(del, val)}} {
			t.Run(fmt.Sprintf("chunk_%d/messages_%d", chunkSize, len(msgs)), func(t *testing.T) {
				var err error
				require.NotPanics(t, func() {
					err = cli.NewSplitAndApply(t.Context(), clienttx.GenerateOrBroadcastTxCLI, client.Context{}, nil, msgs, chunkSize)
				})
				require.EqualError(t, err, "max-msgs must be non-negative (0 for unlimited)")
			})
		}
	}
}

func TestRepairM_671_WithdrawAllRewards(t *testing.T) {
	del := sdk.AccAddress(bytes.Repeat([]byte{1}, 20))
	validators := []string{
		sdk.ValAddress(bytes.Repeat([]byte{2}, 20)).String(),
		sdk.ValAddress(bytes.Repeat([]byte{3}, 20)).String(),
		sdk.ValAddress(bytes.Repeat([]byte{4}, 20)).String(),
	}
	for _, tc := range []struct {
		name       string
		mode       string
		chunkSize  int
		validators []string
		wantChunks []int
		wantError  string
	}{
		{"negative_block", flags.BroadcastBlock, -1, validators, nil, "max-msgs must be non-negative (0 for unlimited)"},
		{"negative_sync", flags.BroadcastSync, -1, validators, nil, "max-msgs must be non-negative (0 for unlimited)"},
		{"negative_async", flags.BroadcastAsync, -1, validators, nil, "max-msgs must be non-negative (0 for unlimited)"},
		{"negative_empty", flags.BroadcastBlock, -1, nil, nil, "max-msgs must be non-negative (0 for unlimited)"},
		{"unlimited_block", flags.BroadcastBlock, 0, validators, []int{3}, ""},
		{"unlimited_sync", flags.BroadcastSync, 0, validators, []int{3}, ""},
		{"unlimited_async", flags.BroadcastAsync, 0, validators, []int{3}, ""},
		{"split_block", flags.BroadcastBlock, 2, validators, []int{2, 1}, ""},
		{"oversized_block", flags.BroadcastBlock, 4, validators, []int{3}, ""},
		{"split_sync_rejected", flags.BroadcastSync, 2, validators, nil, "cannot use broadcast mode sync with max-msgs != 0"},
		{"split_async_rejected", flags.BroadcastAsync, 2, validators, nil, "cannot use broadcast mode async with max-msgs != 0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			registry := codectypes.NewInterfaceRegistry()
			types.RegisterInterfaces(registry)
			cdc := codec.NewProtoCodec(registry)
			txConfig := authtx.NewTxConfig(cdc, authtx.DefaultSignModes)
			response, err := cdc.Marshal(&types.QueryDelegatorValidatorsResponse{Validators: tc.validators})
			require.NoError(t, err)
			var queries atomic.Int32
			mux := http.NewServeMux()
			rpcserver.RegisterRPCFuncs(mux, map[string]*rpcserver.RPCFunc{
				"abci_query": rpcserver.NewRPCFunc(func(_ context.Context, query *coretypes.RequestABCIQuery) (*coretypes.ResultABCIQuery, error) {
					queries.Add(1)
					if query.Path != "/cosmos.distribution.v1beta1.Query/DelegatorValidators" || query.Height != 0 || query.Prove {
						return nil, fmt.Errorf("unexpected query: %+v", query)
					}
					var request types.QueryDelegatorValidatorsRequest
					if err := cdc.Unmarshal(query.Data, &request); err != nil {
						return nil, err
					}
					if request.DelegatorAddress != del.String() {
						return nil, fmt.Errorf("unexpected delegator %q", request.DelegatorAddress)
					}
					return &coretypes.ResultABCIQuery{Response: abci.ResponseQuery{Value: response, Height: 1}}, nil
				}),
			})
			server := httptest.NewServer(mux)
			t.Cleanup(server.Close)
			rpcClient, err := rpchttp.New(server.URL)
			require.NoError(t, err)
			var output bytes.Buffer
			clientCtx := client.Context{}.
				WithCodec(cdc).
				WithInterfaceRegistry(registry).
				WithTxConfig(txConfig).
				WithKeyring(keyring.NewInMemory()).
				WithClient(rpcClient).
				WithFrom(del.String()).
				WithFromAddress(del).
				WithOutput(&output).
				WithChainID("repair-m671")
			cmd := cli.NewWithdrawAllRewardsCmd()
			cmd.SetOut(&output)
			cmd.SetErr(&output)
			cmd.SilenceErrors = true
			cmd.SilenceUsage = true
			cmd.SetArgs([]string{"--generate-only", "--broadcast-mode=" + tc.mode, fmt.Sprintf("--max-msgs=%d", tc.chunkSize)})
			ctx := context.WithValue(t.Context(), client.ClientContextKey, &clientCtx)
			require.NotPanics(t, func() { err = cmd.ExecuteContext(ctx) })
			require.EqualValues(t, 1, queries.Load(), "the public command must consume the validator query fixture")
			if tc.wantError != "" {
				require.EqualError(t, err, tc.wantError)
				require.Empty(t, output.String(), "invalid limits must not produce any transaction")
				return
			}
			require.NoError(t, err)
			transactions := strings.Split(strings.TrimSpace(output.String()), "\n")
			require.Len(t, transactions, len(tc.wantChunks))
			var gotValidators []string
			for i, transaction := range transactions {
				decoded, err := txConfig.TxJSONDecoder()([]byte(transaction))
				require.NoError(t, err)
				require.Len(t, decoded.GetMsgs(), tc.wantChunks[i])
				for _, msg := range decoded.GetMsgs() {
					withdrawal, ok := msg.(*types.MsgWithdrawDelegatorReward)
					require.True(t, ok)
					require.Equal(t, del.String(), withdrawal.DelegatorAddress)
					gotValidators = append(gotValidators, withdrawal.ValidatorAddress)
				}
			}
			require.Equal(t, validators, gotValidators, "all queried validators must appear once and in order")
		})
	}
}
