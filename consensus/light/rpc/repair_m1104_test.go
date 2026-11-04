package rpc_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/config"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/eventbus"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/pubsub"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/rpc/core"
	lrpc "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/light/rpc"
	rpcclient "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client"
	rpchttp "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/http"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	rpcserver "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/server"
	rpctypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/jsonrpc/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

const repairM1104Query = "tm.event = 'BlockSyncStatus'"
const repairM1104OtherQuery = "tm.event = 'StateSyncStatus'"

type repairM1104Fixture struct {
	client *lrpc.Client
	bus    *eventbus.EventBus
	server *httptest.Server
}

func repairM1104Server(routes map[string]*rpcserver.RPCFunc, disconnect func(string)) *httptest.Server {
	mux := http.NewServeMux()
	wm := rpcserver.NewWebsocketManager(routes, rpcserver.OnDisconnect(disconnect))
	mux.HandleFunc("/websocket", wm.WebsocketHandler)
	return httptest.NewServer(mux)
}

func repairM1104New(t *testing.T, httpUpstream, disconnectCallback bool) *repairM1104Fixture {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	bus := eventbus.NewDefault()
	env := &core.Environment{EventBus: bus, Config: *config.DefaultRPCConfig()}
	var next rpcclient.Client = &local.Local{EventBus: bus, Environment: env}
	var upstream *httptest.Server
	var httpClient *rpchttp.HTTP
	if httpUpstream {
		require.NoError(t, bus.Start(ctx))
		upstream = repairM1104Server(map[string]*rpcserver.RPCFunc{
			"subscribe":       rpcserver.NewWSRPCFunc(env.Subscribe),
			"unsubscribe":     rpcserver.NewWSRPCFunc(env.Unsubscribe),
			"unsubscribe_all": rpcserver.NewWSRPCFunc(env.UnsubscribeAll),
		}, func(addr string) { _ = bus.UnsubscribeAll(context.Background(), addr) })
		var err error
		httpClient, err = rpchttp.New(upstream.URL)
		require.NoError(t, err)
		next = httpClient
	}
	client := lrpc.NewClient(next, nil)
	require.NoError(t, client.Start(ctx))
	var disconnect func(string)
	if disconnectCallback {
		disconnect = func(addr string) { _ = client.UnsubscribeAll(context.Background(), addr) }
	}
	server := repairM1104Server(map[string]*rpcserver.RPCFunc{
		"subscribe": rpcserver.NewWSRPCFunc(func(ctx context.Context, req *coretypes.RequestSubscribe) (*coretypes.ResultSubscribe, error) {
			return client.SubscribeWS(ctx, req.Query)
		}),
		"unsubscribe": rpcserver.NewWSRPCFunc(func(ctx context.Context, req *coretypes.RequestUnsubscribe) (*coretypes.ResultUnsubscribe, error) {
			return client.UnsubscribeWS(ctx, req.Query)
		}),
		"unsubscribe_all": rpcserver.NewWSRPCFunc(client.UnsubscribeAllWS),
	}, disconnect)
	t.Cleanup(func() {
		client.Stop()
		cancel()
		if httpClient != nil {
			require.NoError(t, httpClient.Stop())
		}
		server.Close()
		if upstream != nil {
			upstream.Close()
		}
		bus.Stop()
	})
	return &repairM1104Fixture{client: client, bus: bus, server: server}
}

func (f *repairM1104Fixture) dial(t *testing.T) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(f.server.URL, "http")+"/websocket", nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func repairM1104Call(conn *websocket.Conn, method, query string) (rpctypes.RPCResponse, error) {
	req := rpctypes.NewRequest(1)
	var params interface{}
	if method != "unsubscribe_all" {
		params = &coretypes.RequestSubscribe{Query: query}
	}
	if err := req.SetMethodAndParams(method, params); err != nil {
		return rpctypes.RPCResponse{}, err
	}
	if err := conn.SetWriteDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return rpctypes.RPCResponse{}, err
	}
	if err := conn.WriteJSON(req); err != nil {
		return rpctypes.RPCResponse{}, err
	}
	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		return rpctypes.RPCResponse{}, err
	}
	var resp rpctypes.RPCResponse
	err := conn.ReadJSON(&resp)
	return resp, err
}

func repairM1104OK(t *testing.T, conn *websocket.Conn, method, query string) {
	t.Helper()
	resp, err := repairM1104Call(conn, method, query)
	require.NoError(t, err)
	require.Nil(t, resp.Error)
}

func (f *repairM1104Fixture) subscription(t *testing.T, conn *websocket.Conn, query string) *lrpc.RepairM1104Subscription {
	t.Helper()
	sub := lrpc.RepairM1104LookupSubscription(f.client, conn.LocalAddr().String(), query)
	require.NotNil(t, sub)
	return sub
}

func repairM1104Finished(t *testing.T, sub *lrpc.RepairM1104Subscription) {
	t.Helper()
	select {
	case <-sub.RepairM1104Done():
	case <-time.After(5 * time.Second):
		t.Fatal("forwarding goroutine did not terminate")
	}
}

func repairM1104Active(t *testing.T, sub *lrpc.RepairM1104Subscription) {
	t.Helper()
	select {
	case <-sub.RepairM1104Done():
		t.Fatal("unrelated forwarding goroutine terminated")
	default:
	}
}

func (f *repairM1104Fixture) count(t *testing.T, want int) {
	t.Helper()
	require.Eventually(t, func() bool {
		return lrpc.RepairM1104SubscriptionCount(f.client) == want
	}, 5*time.Second, time.Millisecond, "per-subscription state was not released")
}

func repairM1104Event(t *testing.T, conn *websocket.Conn, query string, height int64) {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	var resp rpctypes.RPCResponse
	require.NoError(t, conn.ReadJSON(&resp))
	require.Nil(t, resp.Error)
	var event coretypes.ResultEvent
	require.NoError(t, json.Unmarshal(resp.Result, &event))
	require.Equal(t, query, event.Query)
	if query == repairM1104Query {
		require.Equal(t, types.EventDataBlockSyncStatus{Height: height, Complete: true}, event.Data)
	} else {
		require.Equal(t, types.EventDataStateSyncStatus{Height: height, Complete: true}, event.Data)
	}
}

func TestRepairM_1104_Unsubscribe(t *testing.T) {
	f := repairM1104New(t, true, true)
	conn := f.dial(t)
	repairM1104OK(t, conn, "subscribe", repairM1104Query)
	sub := f.subscription(t, conn, repairM1104Query)
	require.Eventually(t, func() bool { return f.bus.NumClients() == 1 }, 5*time.Second, time.Millisecond)
	require.NoError(t, f.bus.PublishEventBlockSyncStatus(types.EventDataBlockSyncStatus{Height: 41, Complete: true}))
	repairM1104Event(t, conn, repairM1104Query, 41)
	repairM1104OK(t, conn, "unsubscribe", repairM1104Query)
	repairM1104Finished(t, sub)
	f.count(t, 0)
	require.True(t, f.client.IsRunning())
	// Reusing the same subscriber and query must not retain the previous cancel.
	repairM1104OK(t, conn, "subscribe", repairM1104Query)
	replacement := f.subscription(t, conn, repairM1104Query)
	require.NotSame(t, sub, replacement)
	repairM1104OK(t, conn, "unsubscribe", repairM1104Query)
	repairM1104Finished(t, replacement)
	f.count(t, 0)
}

func TestRepairM_1104_UnsubscribeAll(t *testing.T) {
	f := repairM1104New(t, true, true)
	conn := f.dial(t)
	repairM1104OK(t, conn, "subscribe", repairM1104Query)
	repairM1104OK(t, conn, "subscribe", repairM1104OtherQuery)
	first := f.subscription(t, conn, repairM1104Query)
	second := f.subscription(t, conn, repairM1104OtherQuery)
	repairM1104OK(t, conn, "unsubscribe_all", "")
	repairM1104Finished(t, first)
	repairM1104Finished(t, second)
	f.count(t, 0)
	require.True(t, f.client.IsRunning())
}

func TestRepairM_1104_Disconnect(t *testing.T) {
	for _, callback := range []bool{true, false} {
		t.Run(fmt.Sprintf("callback_%t", callback), func(t *testing.T) {
			f := repairM1104New(t, true, callback)
			conn := f.dial(t)
			repairM1104OK(t, conn, "subscribe", repairM1104Query)
			repairM1104OK(t, conn, "subscribe", repairM1104OtherQuery)
			first := f.subscription(t, conn, repairM1104Query)
			second := f.subscription(t, conn, repairM1104OtherQuery)
			require.NoError(t, conn.Close())
			repairM1104Finished(t, first)
			repairM1104Finished(t, second)
			f.count(t, 0)
			require.True(t, f.client.IsRunning())
		})
	}
}

func TestRepairM_1104_SubscriberAndQueryIsolation(t *testing.T) {
	f := repairM1104New(t, false, true)
	first, second := f.dial(t), f.dial(t)
	repairM1104OK(t, first, "subscribe", repairM1104Query)
	repairM1104OK(t, first, "subscribe", repairM1104OtherQuery)
	repairM1104OK(t, second, "subscribe", repairM1104Query)
	a := f.subscription(t, first, repairM1104Query)
	b := f.subscription(t, first, repairM1104OtherQuery)
	c := f.subscription(t, second, repairM1104Query)
	repairM1104OK(t, first, "unsubscribe", repairM1104Query)
	repairM1104Finished(t, a)
	repairM1104Active(t, b)
	repairM1104Active(t, c)
	f.count(t, 2)
	require.NoError(t, f.bus.PublishEventBlockSyncStatus(types.EventDataBlockSyncStatus{Height: 42, Complete: true}))
	repairM1104Event(t, second, repairM1104Query, 42)
	require.NoError(t, f.bus.PublishEventStateSyncStatus(types.EventDataStateSyncStatus{Height: 43, Complete: true}))
	repairM1104Event(t, first, repairM1104OtherQuery, 43)
	repairM1104OK(t, first, "unsubscribe_all", "")
	repairM1104Finished(t, b)
	repairM1104Active(t, c)
	f.count(t, 1)
	require.NoError(t, second.Close())
	repairM1104Finished(t, c)
	f.count(t, 0)
}

func TestRepairM_1104_UnsubscribeErrorStillReleasesForwarder(t *testing.T) {
	f := repairM1104New(t, false, true)
	conn := f.dial(t)
	repairM1104OK(t, conn, "subscribe", repairM1104Query)
	sub := f.subscription(t, conn, repairM1104Query)
	// Remove the real upstream subscription before the proxy's unsubscribe.
	require.NoError(t, f.bus.UnsubscribeAll(t.Context(), conn.LocalAddr().String()))
	resp, err := repairM1104Call(conn, "unsubscribe_all", "")
	require.NoError(t, err)
	require.NotNil(t, resp.Error)
	require.Contains(t, resp.Error.Data, pubsub.ErrSubscriptionNotFound.Error())
	repairM1104Finished(t, sub)
	f.count(t, 0)
}

func TestRepairM_1104_FailedAndDuplicateSubscribe(t *testing.T) {
	f := repairM1104New(t, false, true)
	conn := f.dial(t)
	resp, err := repairM1104Call(conn, "subscribe", "invalid query")
	require.NoError(t, err)
	require.NotNil(t, resp.Error)
	f.count(t, 0)
	repairM1104OK(t, conn, "subscribe", repairM1104Query)
	sub := f.subscription(t, conn, repairM1104Query)
	resp, err = repairM1104Call(conn, "subscribe", repairM1104Query)
	require.NoError(t, err)
	require.NotNil(t, resp.Error)
	require.Same(t, sub, f.subscription(t, conn, repairM1104Query))
	repairM1104Active(t, sub)
	f.count(t, 1)
	repairM1104OK(t, conn, "unsubscribe_all", "")
	repairM1104Finished(t, sub)
	f.count(t, 0)
}

func TestRepairM_1104_ConcurrentLifecycleAndShutdown(t *testing.T) {
	f := repairM1104New(t, false, true)
	var conns []*websocket.Conn
	var subs []*lrpc.RepairM1104Subscription
	for i := 0; i < 12; i++ {
		conn := f.dial(t)
		repairM1104OK(t, conn, "subscribe", repairM1104Query)
		conns = append(conns, conn)
		subs = append(subs, f.subscription(t, conn, repairM1104Query))
	}
	var wg sync.WaitGroup
	failures := make(chan error, len(conns))
	for i, conn := range conns {
		wg.Add(1)
		go func(i int, conn *websocket.Conn) {
			defer wg.Done()
			if i%2 == 0 {
				resp, err := repairM1104Call(conn, "unsubscribe_all", "")
				if err != nil {
					failures <- err
				} else if resp.Error != nil {
					failures <- resp.Error
				}
			} else {
				if err := conn.Close(); err != nil {
					failures <- err
				}
			}
		}(i, conn)
	}
	wg.Wait()
	close(failures)
	for err := range failures {
		require.NoError(t, err)
	}
	for _, sub := range subs {
		repairM1104Finished(t, sub)
	}
	f.count(t, 0)
	conn := f.dial(t)
	repairM1104OK(t, conn, "subscribe", repairM1104Query)
	sub := f.subscription(t, conn, repairM1104Query)
	f.client.Stop()
	repairM1104Finished(t, sub)
	f.count(t, 0)
	resp, err := repairM1104Call(conn, "subscribe", repairM1104OtherQuery)
	require.NoError(t, err)
	require.NotNil(t, resp.Error)
	require.Contains(t, resp.Error.Data, "client is stopped")
	f.count(t, 0)
}
