package evmrpc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	evmrpcconfig "github.com/Sidiora-Labs/Paxeer-X-Network/rpc/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
)

func newWSTransportServers(t *testing.T, denyList []string) (*evmrpc.HTTPServer, *rpc.Client, *rpc.Client) {
	t.Helper()
	testApp := app.Setup(t, false, false, false)
	config := evmrpcconfig.DefaultConfig
	config.HTTPAddress = TestAddr
	config.HTTPPort = 0
	config.WSPort = 0
	config.FilterTimeout = time.Minute
	config.DenyList = denyList
	ctxProvider := func(int64) sdk.Context {
		return Ctx.WithBlockHeight(MockHeight103).WithIsTracing(true).WithClosestUpgradeName(LatestCtxUpgradeName)
	}
	txConfigProvider := func(int64) client.TxConfig { return TxConfig }
	wsService, err := evmrpc.NewEVMWebSocketServer(config, &MockClient{}, EVMKeeper, testApp.BeginBlockKeepers,
		testApp.BaseApp, testApp.TracerAnteHandler, ctxProvider, txConfigProvider, "", nil, nil)
	require.NoError(t, err)
	t.Cleanup(wsService.Stop)
	wsServer := wsService.(*evmrpc.HTTPServer)
	require.NoError(t, wsServer.Start())
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	wsClient, err := rpc.DialWebsocket(ctx, "ws://"+wsServer.ListenAddr(), "http://localhost")
	require.NoError(t, err)
	t.Cleanup(wsClient.Close)

	httpService, err := evmrpc.NewEVMHTTPServer(config, &MockClient{}, EVMKeeper, testApp.BeginBlockKeepers,
		testApp.BaseApp, testApp.TracerAnteHandler, ctxProvider, txConfigProvider, "", nil)
	require.NoError(t, err)
	t.Cleanup(httpService.Stop)
	httpServer := httpService.(*evmrpc.HTTPServer)
	require.NoError(t, httpServer.Start())
	httpClient, err := rpc.DialHTTP("http://" + httpServer.ListenAddr())
	require.NoError(t, err)
	t.Cleanup(httpClient.Close)
	return wsServer, wsClient, httpClient
}

func TestWSBindsHTTPAddress(t *testing.T) {
	server, wsClient, _ := newWSTransportServers(t, []string{"eth_chainId"})
	host, port, err := net.SplitHostPort(server.ListenAddr())
	require.NoError(t, err)
	require.Equal(t, TestAddr, host)
	require.True(t, net.ParseIP(host).IsLoopback())
	require.NotEqual(t, "0", port)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var echoed string
	require.NoError(t, wsClient.CallContext(ctx, &echoed, "echo_echo", "loopback"))
	require.Equal(t, "loopback", echoed)
}

func TestWSAppliesDenyList(t *testing.T) {
	_, allowedWS, _ := newWSTransportServers(t, nil)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var chainID hexutil.Big
	require.NoError(t, allowedWS.CallContext(ctx, &chainID, "eth_chainId"))
	require.Equal(t, EVMKeeper.ChainID(Ctx), chainID.ToInt())

	_, deniedWS, deniedHTTP := newWSTransportServers(t, []string{"eth_chainId"})
	ctx, cancelDenied := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelDenied()
	var wsResult, httpResult interface{}
	wsErr := deniedWS.CallContext(ctx, &wsResult, "eth_chainId")
	httpErr := deniedHTTP.CallContext(ctx, &httpResult, "eth_chainId")
	var wsRPCError, httpRPCError rpc.Error
	require.ErrorAs(t, wsErr, &wsRPCError)
	require.ErrorAs(t, httpErr, &httpRPCError)
	require.Equal(t, -32601, wsRPCError.ErrorCode())
	require.EqualError(t, wsErr, "the method eth_chainId does not exist/is not available")
	require.Equal(t, httpRPCError.ErrorCode(), wsRPCError.ErrorCode())
	require.Equal(t, httpErr.Error(), wsErr.Error())
	require.Nil(t, wsResult)
	require.Nil(t, httpResult)
	var echoed string
	require.NoError(t, deniedWS.CallContext(ctx, &echoed, "echo_echo", "allowed"))
	require.Equal(t, "allowed", echoed)
	batch := []rpc.BatchElem{
		{Method: "eth_chainId", Result: new(hexutil.Big)},
		{Method: "eth_chainId", Result: new(hexutil.Big)},
	}
	require.NoError(t, deniedWS.BatchCallContext(ctx, batch))
	for _, call := range batch {
		var rpcError rpc.Error
		require.ErrorAs(t, call.Error, &rpcError)
		require.Equal(t, -32601, rpcError.ErrorCode())
		require.EqualError(t, call.Error, "the method eth_chainId does not exist/is not available")
	}
}

func dialWSDenyList(t *testing.T, server *evmrpc.HTTPServer) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial("ws://"+server.ListenAddr(), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func requireWSDenyListReply(t *testing.T, conn *websocket.Conn, body, want string) {
	t.Helper()
	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(body)))
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	_, got, err := conn.ReadMessage()
	require.NoError(t, err)
	require.JSONEq(t, want, string(got))
	var gotMessages, wantMessages []struct {
		ID json.RawMessage `json:"id"`
	}
	if strings.HasPrefix(want, "[") {
		require.NoError(t, json.Unmarshal(got, &gotMessages))
		require.NoError(t, json.Unmarshal([]byte(want), &wantMessages))
	} else {
		gotMessages = make([]struct {
			ID json.RawMessage `json:"id"`
		}, 1)
		wantMessages = make([]struct {
			ID json.RawMessage `json:"id"`
		}, 1)
		require.NoError(t, json.Unmarshal(got, &gotMessages[0]))
		require.NoError(t, json.Unmarshal([]byte(want), &wantMessages[0]))
	}
	require.Len(t, gotMessages, len(wantMessages))
	for i := range wantMessages {
		require.Equal(t, string(wantMessages[i].ID), string(gotMessages[i].ID))
	}
}

func TestWSAppliesDenyListMixedBatches(t *testing.T) {
	server, wsClient, _ := newWSTransportServers(t, []string{"eth_chainId", "eth_getBalance", "eth_subscribe", "eth_unsubscribe"})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	var blockNumber hexutil.Uint64
	require.NoError(t, wsClient.CallContext(ctx, &blockNumber, "eth_blockNumber"))
	blockResult, err := json.Marshal(blockNumber)
	require.NoError(t, err)
	for _, tc := range []struct{ name, body, want string }{
		{
			name: "denied_then_allowed",
			body: `[{"jsonrpc":"2.0","id":1,"method":"eth_chainId","params":[]},{"jsonrpc":"2.0","id":2,"method":"eth_blockNumber","params":[]}]`,
			want: fmt.Sprintf(`[{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"the method eth_chainId does not exist/is not available"}},{"jsonrpc":"2.0","id":2,"result":%s}]`, blockResult),
		},
		{
			name: "allowed_denied_allowed_duplicate_ids",
			body: `[{"jsonrpc":"2.0","id":"same","method":"echo_echo","params":["before"]},{"jsonrpc":"2.0","id":"same","method":"eth_chainId"},{"jsonrpc":"2.0","id":"same","method":"echo_echo","params":["after"]}]`,
			want: `[{"jsonrpc":"2.0","id":"same","result":"before"},{"jsonrpc":"2.0","id":"same","error":{"code":-32601,"message":"the method eth_chainId does not exist/is not available"}},{"jsonrpc":"2.0","id":"same","result":"after"}]`,
		},
		{
			name: "all_denied_distinct_methods_and_id_types",
			body: `[{"jsonrpc":"2.0","id":null,"method":"eth_chainId"},{"jsonrpc":"2.0","id":9007199254740993,"method":"eth_getBalance"},{"jsonrpc":"2.0","id":"text","method":"eth_chainId"}]`,
			want: `[{"jsonrpc":"2.0","id":null,"error":{"code":-32601,"message":"the method eth_chainId does not exist/is not available"}},{"jsonrpc":"2.0","id":9007199254740993,"error":{"code":-32601,"message":"the method eth_getBalance does not exist/is not available"}},{"jsonrpc":"2.0","id":"text","error":{"code":-32601,"message":"the method eth_chainId does not exist/is not available"}}]`,
		},
		{
			name: "denied_subscription_builtins",
			body: `[{"jsonrpc":"2.0","id":1,"method":"eth_subscribe","params":["newHeads"]},{"jsonrpc":"2.0","id":2,"method":"eth_unsubscribe","params":["0x1"]},{"jsonrpc":"2.0","id":3,"method":"echo_echo","params":["allowed"]}]`,
			want: `[{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"the method eth_subscribe does not exist/is not available"}},{"jsonrpc":"2.0","id":2,"error":{"code":-32601,"message":"the method eth_unsubscribe does not exist/is not available"}},{"jsonrpc":"2.0","id":3,"result":"allowed"}]`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn := dialWSDenyList(t, server)
			requireWSDenyListReply(t, conn, tc.body, tc.want)
			requireWSDenyListReply(t, conn, `{"jsonrpc":"2.0","id":"next","method":"echo_echo","params":["still open"]}`, `{"jsonrpc":"2.0","id":"next","result":"still open"}`)
		})
	}
}

func TestWSAppliesDenyListNotifications(t *testing.T) {
	server, _, _ := newWSTransportServers(t, []string{"eth_chainId", "eth_getBalance"})
	for _, tc := range []struct{ name, body, want string }{
		{
			name: "denied_notification_with_allowed_call",
			body: `[{"jsonrpc":"2.0","method":"eth_chainId"},{"jsonrpc":"2.0","id":1,"method":"echo_echo","params":["allowed"]}]`,
			want: `[{"jsonrpc":"2.0","id":1,"result":"allowed"}]`,
		},
		{
			name: "allowed_notification_with_denied_call",
			body: `[{"jsonrpc":"2.0","method":"echo_echo","params":["notification"]},{"jsonrpc":"2.0","id":1,"method":"eth_chainId"}]`,
			want: `[{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"the method eth_chainId does not exist/is not available"}}]`,
		},
		{
			name: "notifications_and_null_id_call",
			body: `[{"jsonrpc":"2.0","method":"eth_chainId"},{"jsonrpc":"2.0","method":"echo_echo","params":["notification"]},{"jsonrpc":"2.0","id":null,"method":"eth_getBalance"}]`,
			want: `[{"jsonrpc":"2.0","id":null,"error":{"code":-32601,"message":"the method eth_getBalance does not exist/is not available"}}]`,
		},
		{name: "single_denied_notification", body: `{"jsonrpc":"2.0","method":"eth_chainId"}`},
		{name: "all_denied_notifications", body: `[{"jsonrpc":"2.0","method":"eth_chainId"},{"jsonrpc":"2.0","method":"eth_getBalance"}]`},
		{name: "mixed_notifications_only", body: `[{"jsonrpc":"2.0","method":"eth_chainId"},{"jsonrpc":"2.0","method":"echo_echo","params":["notification"]}]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn := dialWSDenyList(t, server)
			if tc.want == "" {
				require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(tc.body)))
			} else {
				requireWSDenyListReply(t, conn, tc.body, tc.want)
			}
			requireWSDenyListReply(t, conn, `{"jsonrpc":"2.0","id":"next","method":"echo_echo","params":["still open"]}`, `{"jsonrpc":"2.0","id":"next","result":"still open"}`)
			require.NoError(t, conn.SetReadDeadline(time.Now().Add(100*time.Millisecond)))
			_, _, err := conn.ReadMessage()
			var timeout net.Error
			require.ErrorAs(t, err, &timeout)
			require.True(t, timeout.Timeout())
		})
	}
}

func TestWSAppliesDenyListProtocolErrors(t *testing.T) {
	server, _, _ := newWSTransportServers(t, []string{"eth_chainId", ""})
	for _, tc := range []struct{ name, body, want string }{
		{
			name: "unknown_method",
			body: `{"jsonrpc":"2.0","id":"unknown","method":"unknown_method"}`,
			want: `{"jsonrpc":"2.0","id":"unknown","error":{"code":-32601,"message":"the method unknown_method does not exist/is not available"}}`,
		},
		{
			name: "unknown_method_without_separator",
			body: `{"jsonrpc":"2.0","id":"unknown","method":"denied:0"}`,
			want: `{"jsonrpc":"2.0","id":"unknown","error":{"code":-32601,"message":"the method denied:0 does not exist/is not available"}}`,
		},
		{
			name: "denied_invalid_and_allowed",
			body: `[{"jsonrpc":"2.0","id":1,"method":"eth_chainId"},7,{"jsonrpc":"2.0","id":2,"method":"echo_echo","params":["allowed"]}]`,
			want: `[{"jsonrpc":"2.0","id":1,"error":{"code":-32601,"message":"the method eth_chainId does not exist/is not available"}},{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"invalid request"}},{"jsonrpc":"2.0","id":2,"result":"allowed"}]`,
		},
		{
			name: "invalid_version_for_denied_method",
			body: `{"jsonrpc":"1.0","id":1,"method":"eth_chainId"}`,
			want: `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"invalid request"}}`,
		},
		{
			name: "missing_method",
			body: `{"jsonrpc":"2.0","id":1}`,
			want: `{"jsonrpc":"2.0","id":1,"error":{"code":-32600,"message":"invalid request"}}`,
		},
		{
			name: "invalid_id_for_denied_method",
			body: `{"jsonrpc":"2.0","id":[],"method":"eth_chainId"}`,
			want: `{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"invalid request"}}`,
		},
		{
			name: "empty_batch",
			body: `[]`,
			want: `{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"empty batch"}}`,
		},
		{
			name: "null_request",
			body: `null`,
			want: `{"jsonrpc":"2.0","id":null,"error":{"code":-32600,"message":"invalid request"}}`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			conn := dialWSDenyList(t, server)
			requireWSDenyListReply(t, conn, tc.body, tc.want)
		})
	}
	t.Run("denied_items_count_toward_batch_limit", func(t *testing.T) {
		items := make([]json.RawMessage, evmrpcconfig.DefaultConfig.BatchRequestLimit+1)
		for i := range items {
			items[i] = json.RawMessage(fmt.Sprintf(`{"jsonrpc":"2.0","id":%d,"method":"eth_chainId"}`, i))
		}
		body, err := json.Marshal(items)
		require.NoError(t, err)
		conn := dialWSDenyList(t, server)
		requireWSDenyListReply(t, conn, string(body), `[{"jsonrpc":"2.0","id":0,"error":{"code":-32600,"message":"batch too large"}}]`)
	})
	t.Run("malformed_json", func(t *testing.T) {
		conn := dialWSDenyList(t, server)
		require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"jsonrpc":"2.0","method":"eth_chainId",]`)))
		require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
		var response struct {
			ID    json.RawMessage `json:"id"`
			Error struct {
				Code int `json:"code"`
			} `json:"error"`
		}
		require.NoError(t, conn.ReadJSON(&response))
		require.Equal(t, "null", string(response.ID))
		require.Equal(t, -32700, response.Error.Code)
		_, _, err := conn.ReadMessage()
		var closed *websocket.CloseError
		require.ErrorAs(t, err, &closed)
	})
}
