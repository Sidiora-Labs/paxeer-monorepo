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

const blockSyncQuery = "tm.event = 'BlockSyncStatus'"
const stateSyncQuery = "tm.event = 'StateSyncStatus'"

type subscriptionFixture struct {
	client *lrpc.Client
	bus    *eventbus.EventBus
	server *httptest.Server
}

func subscriptionServer(routes map[string]*rpcserver.RPCFunc, disconnect func(string)) *httptest.Server {
	mux := http.NewServeMux()
	wm := rpcserver.NewWebsocketManager(routes, rpcserver.OnDisconnect(disconnect))
	mux.HandleFunc("/websocket", wm.WebsocketHandler)
	return httptest.NewServer(mux)
}

func newSubscriptionFixture(t *testing.T, httpUpstream, disconnectCallback bool) *subscriptionFixture {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	bus := eventbus.NewDefault()
	env := &core.Environment{EventBus: bus, Config: *config.DefaultRPCConfig()}
	var next rpcclient.Client = &local.Local{EventBus: bus, Environment: env}
	var upstream *httptest.Server
	var httpClient *rpchttp.HTTP
	if httpUpstream {
		require.NoError(t, bus.Start(ctx))
		upstream = subscriptionServer(map[string]*rpcserver.RPCFunc{
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
	server := subscriptionServer(map[string]*rpcserver.RPCFunc{
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
	return &subscriptionFixture{client: client, bus: bus, server: server}
}

func (f *subscriptionFixture) dial(t *testing.T) *websocket.Conn {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(f.server.URL, "http")+"/websocket", nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func callSubscriptionMethod(conn *websocket.Conn, method, query string) (rpctypes.RPCResponse, error) {
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

func requireSubscriptionCallOK(t *testing.T, conn *websocket.Conn, method, query string) {
	t.Helper()
	resp, err := callSubscriptionMethod(conn, method, query)
	require.NoError(t, err)
	require.Nil(t, resp.Error)
}

func (f *subscriptionFixture) subscription(t *testing.T, conn *websocket.Conn, query string) *lrpc.SubscriptionForTest {
	t.Helper()
	sub := lrpc.LookupTestSubscription(f.client, conn.LocalAddr().String(), query)
	require.NotNil(t, sub)
	return sub
}

func requireSubscriptionFinished(t *testing.T, sub *lrpc.SubscriptionForTest) {
	t.Helper()
	select {
	case <-sub.DoneForTest():
	case <-time.After(5 * time.Second):
		t.Fatal("forwarding goroutine did not terminate")
	}
}

func requireSubscriptionActive(t *testing.T, sub *lrpc.SubscriptionForTest) {
	t.Helper()
	select {
	case <-sub.DoneForTest():
		t.Fatal("unrelated forwarding goroutine terminated")
	default:
	}
}

func (f *subscriptionFixture) count(t *testing.T, want int) {
	t.Helper()
	require.Eventually(t, func() bool {
		return lrpc.SubscriptionCountForTest(f.client) == want
	}, 5*time.Second, time.Millisecond, "per-subscription state was not released")
}

func requireSubscriptionEvent(t *testing.T, conn *websocket.Conn, query string, height int64) {
	t.Helper()
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(5*time.Second)))
	var resp rpctypes.RPCResponse
	require.NoError(t, conn.ReadJSON(&resp))
	require.Nil(t, resp.Error)
	var event coretypes.ResultEvent
	require.NoError(t, json.Unmarshal(resp.Result, &event))
	require.Equal(t, query, event.Query)
	if query == blockSyncQuery {
		require.Equal(t, types.EventDataBlockSyncStatus{Height: height, Complete: true}, event.Data)
	} else {
		require.Equal(t, types.EventDataStateSyncStatus{Height: height, Complete: true}, event.Data)
	}
}

func TestSubscriptionLifecycle_Unsubscribe(t *testing.T) {
	f := newSubscriptionFixture(t, true, true)
	conn := f.dial(t)
	requireSubscriptionCallOK(t, conn, "subscribe", blockSyncQuery)
	sub := f.subscription(t, conn, blockSyncQuery)
	require.Eventually(t, func() bool { return f.bus.NumClients() == 1 }, 5*time.Second, time.Millisecond)
	require.NoError(t, f.bus.PublishEventBlockSyncStatus(types.EventDataBlockSyncStatus{Height: 41, Complete: true}))
	requireSubscriptionEvent(t, conn, blockSyncQuery, 41)
	requireSubscriptionCallOK(t, conn, "unsubscribe", blockSyncQuery)
	requireSubscriptionFinished(t, sub)
	f.count(t, 0)
	require.True(t, f.client.IsRunning())
	// Reusing the same subscriber and query must not retain the previous cancel.
	requireSubscriptionCallOK(t, conn, "subscribe", blockSyncQuery)
	replacement := f.subscription(t, conn, blockSyncQuery)
	require.NotSame(t, sub, replacement)
	requireSubscriptionCallOK(t, conn, "unsubscribe", blockSyncQuery)
	requireSubscriptionFinished(t, replacement)
	f.count(t, 0)
}

func TestSubscriptionLifecycle_UnsubscribeAll(t *testing.T) {
	f := newSubscriptionFixture(t, true, true)
	conn := f.dial(t)
	requireSubscriptionCallOK(t, conn, "subscribe", blockSyncQuery)
	requireSubscriptionCallOK(t, conn, "subscribe", stateSyncQuery)
	first := f.subscription(t, conn, blockSyncQuery)
	second := f.subscription(t, conn, stateSyncQuery)
	requireSubscriptionCallOK(t, conn, "unsubscribe_all", "")
	requireSubscriptionFinished(t, first)
	requireSubscriptionFinished(t, second)
	f.count(t, 0)
	require.True(t, f.client.IsRunning())
}

func TestSubscriptionLifecycle_Disconnect(t *testing.T) {
	for _, callback := range []bool{true, false} {
		t.Run(fmt.Sprintf("callback_%t", callback), func(t *testing.T) {
			f := newSubscriptionFixture(t, true, callback)
			conn := f.dial(t)
			requireSubscriptionCallOK(t, conn, "subscribe", blockSyncQuery)
			requireSubscriptionCallOK(t, conn, "subscribe", stateSyncQuery)
			first := f.subscription(t, conn, blockSyncQuery)
			second := f.subscription(t, conn, stateSyncQuery)
			require.NoError(t, conn.Close())
			requireSubscriptionFinished(t, first)
			requireSubscriptionFinished(t, second)
			f.count(t, 0)
			require.True(t, f.client.IsRunning())
		})
	}
}

func TestSubscriptionLifecycle_SubscriberAndQueryIsolation(t *testing.T) {
	f := newSubscriptionFixture(t, false, true)
	first, second := f.dial(t), f.dial(t)
	requireSubscriptionCallOK(t, first, "subscribe", blockSyncQuery)
	requireSubscriptionCallOK(t, first, "subscribe", stateSyncQuery)
	requireSubscriptionCallOK(t, second, "subscribe", blockSyncQuery)
	a := f.subscription(t, first, blockSyncQuery)
	b := f.subscription(t, first, stateSyncQuery)
	c := f.subscription(t, second, blockSyncQuery)
	requireSubscriptionCallOK(t, first, "unsubscribe", blockSyncQuery)
	requireSubscriptionFinished(t, a)
	requireSubscriptionActive(t, b)
	requireSubscriptionActive(t, c)
	f.count(t, 2)
	require.NoError(t, f.bus.PublishEventBlockSyncStatus(types.EventDataBlockSyncStatus{Height: 42, Complete: true}))
	requireSubscriptionEvent(t, second, blockSyncQuery, 42)
	require.NoError(t, f.bus.PublishEventStateSyncStatus(types.EventDataStateSyncStatus{Height: 43, Complete: true}))
	requireSubscriptionEvent(t, first, stateSyncQuery, 43)
	requireSubscriptionCallOK(t, first, "unsubscribe_all", "")
	requireSubscriptionFinished(t, b)
	requireSubscriptionActive(t, c)
	f.count(t, 1)
	require.NoError(t, second.Close())
	requireSubscriptionFinished(t, c)
	f.count(t, 0)
}

func TestSubscriptionLifecycle_UnsubscribeErrorStillReleasesForwarder(t *testing.T) {
	f := newSubscriptionFixture(t, false, true)
	conn := f.dial(t)
	requireSubscriptionCallOK(t, conn, "subscribe", blockSyncQuery)
	sub := f.subscription(t, conn, blockSyncQuery)
	// Remove the real upstream subscription before the proxy's unsubscribe.
	require.NoError(t, f.bus.UnsubscribeAll(t.Context(), conn.LocalAddr().String()))
	resp, err := callSubscriptionMethod(conn, "unsubscribe_all", "")
	require.NoError(t, err)
	require.NotNil(t, resp.Error)
	require.Contains(t, resp.Error.Data, pubsub.ErrSubscriptionNotFound.Error())
	requireSubscriptionFinished(t, sub)
	f.count(t, 0)
}

func TestSubscriptionLifecycle_FailedAndDuplicateSubscribe(t *testing.T) {
	f := newSubscriptionFixture(t, false, true)
	conn := f.dial(t)
	resp, err := callSubscriptionMethod(conn, "subscribe", "invalid query")
	require.NoError(t, err)
	require.NotNil(t, resp.Error)
	f.count(t, 0)
	requireSubscriptionCallOK(t, conn, "subscribe", blockSyncQuery)
	sub := f.subscription(t, conn, blockSyncQuery)
	resp, err = callSubscriptionMethod(conn, "subscribe", blockSyncQuery)
	require.NoError(t, err)
	require.NotNil(t, resp.Error)
	require.Same(t, sub, f.subscription(t, conn, blockSyncQuery))
	requireSubscriptionActive(t, sub)
	f.count(t, 1)
	requireSubscriptionCallOK(t, conn, "unsubscribe_all", "")
	requireSubscriptionFinished(t, sub)
	f.count(t, 0)
}

func TestSubscriptionLifecycle_ConcurrentLifecycleAndShutdown(t *testing.T) {
	f := newSubscriptionFixture(t, false, true)
	var conns []*websocket.Conn
	var subs []*lrpc.SubscriptionForTest
	for i := 0; i < 12; i++ {
		conn := f.dial(t)
		requireSubscriptionCallOK(t, conn, "subscribe", blockSyncQuery)
		conns = append(conns, conn)
		subs = append(subs, f.subscription(t, conn, blockSyncQuery))
	}
	var wg sync.WaitGroup
	failures := make(chan error, len(conns))
	for i, conn := range conns {
		wg.Add(1)
		go func(i int, conn *websocket.Conn) {
			defer wg.Done()
			if i%2 == 0 {
				resp, err := callSubscriptionMethod(conn, "unsubscribe_all", "")
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
		requireSubscriptionFinished(t, sub)
	}
	f.count(t, 0)
	conn := f.dial(t)
	requireSubscriptionCallOK(t, conn, "subscribe", blockSyncQuery)
	sub := f.subscription(t, conn, blockSyncQuery)
	f.client.Stop()
	requireSubscriptionFinished(t, sub)
	f.count(t, 0)
	resp, err := callSubscriptionMethod(conn, "subscribe", stateSyncQuery)
	require.NoError(t, err)
	require.NotNil(t, resp.Error)
	require.Contains(t, resp.Error.Data, "client is stopped")
	f.count(t, 0)
}
