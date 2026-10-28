package evmrpc_test

import (
	"context"
	"sort"
	"strings"
	"testing"
	"time"

	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPaxServedSetEqualsGatedSet(t *testing.T) {
	gatedNames := evmrpc.PaxLegacyAllGatedMethodNames()
	require.Len(t, gatedNames, 36)
	require.NotContains(t, gatedNames, evmrpc.SequencerReceiptMethod)
	gated := make(map[string]struct{}, len(gatedNames))
	for _, name := range gatedNames {
		gated[name] = struct{}{}
	}

	testApp := app.Setup(t, false, false, false)
	config := evmrpcconfig.DefaultConfig
	config.HTTPAddress = TestAddr
	config.HTTPPort = 0
	config.FilterTimeout = 500 * time.Millisecond
	config.MaxLogNoBlock = 10
	config.EnabledLegacyPaxApis = gatedNames
	ctxProvider := func(height int64) sdk.Context {
		if height == MockHeight2 {
			return MultiTxCtx.WithIsTracing(true)
		}
		if height == evmrpc.LatestCtxHeight {
			return Ctx.WithIsTracing(true).WithClosestUpgradeName(LatestCtxUpgradeName)
		}
		return Ctx.WithIsTracing(true)
	}
	txConfigProvider := func(int64) client.TxConfig { return TxConfig }

	for _, tc := range []struct {
		name    string
		options []evmrpc.ServerOption
	}{
		{name: "without_sequencer_receipt_option"},
		{
			name:    "with_sequencer_receipt_option",
			options: []evmrpc.ServerOption{evmrpc.SequencerReceiptOption{}},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			service, err := evmrpc.NewEVMHTTPServer(config, &MockClient{}, EVMKeeper,
				testApp.BeginBlockKeepers, testApp.BaseApp, testApp.TracerAnteHandler,
				ctxProvider, txConfigProvider, "", nil, tc.options...)
			require.NoError(t, err)
			t.Cleanup(service.Stop)
			require.NoError(t, service.Start())
			server, ok := service.(*evmrpc.HTTPServer)
			require.True(t, ok, "HTTP server must expose its registered methods")
			methods := server.HTTPMethods()
			require.NotEmpty(t, methods)

			served := make(map[string]struct{})
			for _, name := range methods {
				if name != evmrpc.SequencerReceiptMethod &&
					(strings.HasPrefix(name, "pax_") || strings.HasPrefix(name, "pax2_")) {
					served[name] = struct{}{}
				}
			}
			var servedNotGated, gatedNotServed []string
			for name := range served {
				if _, ok := gated[name]; !ok {
					servedNotGated = append(servedNotGated, name)
				}
			}
			for name := range gated {
				if _, ok := served[name]; !ok {
					gatedNotServed = append(gatedNotServed, name)
				}
			}
			sort.Strings(servedNotGated)
			sort.Strings(gatedNotServed)
			assert.True(t, len(servedNotGated) == 0 && len(gatedNotServed) == 0,
				"legacy method sets differ: served but not gated: %v; gated but not served: %v",
				servedNotGated, gatedNotServed)
			assert.Contains(t, methods, evmrpc.SequencerReceiptMethod,
				"the ungated receipt method must be registered with and without SequencerReceiptOption")

			rpcClient, err := rpc.DialHTTP("http://" + server.ListenAddr())
			require.NoError(t, err)
			t.Cleanup(rpcClient.Close)
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			var result evmrpc.SequencerReceiptResult
			err = rpcClient.CallContext(ctx, &result, evmrpc.SequencerReceiptMethod, common.Hash{})
			var rpcError rpc.Error
			require.ErrorAs(t, err, &rpcError)
			assert.Equal(t, -32000, rpcError.ErrorCode())
			assert.EqualError(t, err, evmrpc.SequencerReceiptMethod+" served by sequencer")
		})
	}
}
