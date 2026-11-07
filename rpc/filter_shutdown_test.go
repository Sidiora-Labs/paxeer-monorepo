package evmrpc

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func newFilterShutdownServer(t *testing.T) (*HTTPServer, map[string]*FilterAPI, *rpc.Client) {
	t.Helper()
	server := NewHTTPServer(rpc.DefaultHTTPTimeouts)
	t.Cleanup(server.Stop)
	filterAPIs := make(map[string]*FilterAPI)
	apis := make([]rpc.API, 0, 2)
	for _, namespace := range []string{"eth", "pax"} {
		api := NewFilterAPI(nil, nil, nil, nil, NewFilterConfig(time.Minute, 0, 0),
			ConnectionTypeHTTP, namespace, make(chan struct{}, MaxDBReadConcurrency),
			NewBlockCache(16), &sync.Mutex{}, NewLogSlicePool(), nil)
		api.setReceiptLogFeed(NewReceiptLogFeed(0))
		filterAPIs[namespace] = api
		server.addStopHook(api.shutdown)
		apis = append(apis, rpc.API{Namespace: namespace, Service: api})
	}
	require.NoError(t, server.EnableRPC(apis, HTTPConfig{Vhosts: []string{"*"}}))
	require.NoError(t, server.SetListenAddr("127.0.0.1", 0))
	require.NoError(t, server.Start())
	client, err := rpc.DialHTTP("http://" + server.ListenAddr())
	require.NoError(t, err)
	t.Cleanup(client.Close)
	return server, filterAPIs, client
}

func TestFilterCleanupNotServed(t *testing.T) {
	server, filterAPIs, client := newFilterShutdownServer(t)
	methods := server.HTTPMethods()
	for _, namespace := range []string{"eth", "pax"} {
		require.Contains(t, methods, namespace+"_newFilter")
		require.NotContains(t, methods, namespace+"_cleanup")
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		var result interface{}
		err := client.CallContext(ctx, &result, namespace+"_cleanup")
		cancel()
		require.Error(t, err)
		var rpcErr rpc.Error
		require.ErrorAs(t, err, &rpcErr)
		require.Equal(t, -32601, rpcErr.ErrorCode())
		require.NoError(t, filterAPIs[namespace].shutdownCtx.Err())
	}
}

func TestHTTPServerStopCancelsFilters(t *testing.T) {
	server, filterAPIs, client := newFilterShutdownServer(t)
	filterContexts := make(map[string]context.Context)
	for _, namespace := range []string{"eth", "pax"} {
		api := filterAPIs[namespace]
		filterCtx, cancel := context.WithCancel(api.shutdownCtx)
		t.Cleanup(cancel)
		filterContexts[namespace] = filterCtx
		for i := 0; i < 2; i++ {
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			var id rpc.ID
			err := client.CallContext(ctx, &id, namespace+"_newFilter", map[string]string{"toBlock": "pending"})
			cancel()
			require.NoError(t, err)
			require.NotEmpty(t, id)
			api.filtersMu.RLock()
			_, installed := api.filters[id]
			api.filtersMu.RUnlock()
			require.True(t, installed)
		}
		require.NoError(t, filterCtx.Err())
		api.receiptLogs.mu.Lock()
		subscribers := len(api.receiptLogs.filters)
		api.receiptLogs.mu.Unlock()
		require.Equal(t, 2, subscribers)
	}

	server.Stop()
	for _, namespace := range []string{"eth", "pax"} {
		api := filterAPIs[namespace]
		require.ErrorIs(t, filterContexts[namespace].Err(), context.Canceled)
		api.filtersMu.RLock()
		remaining := len(api.filters)
		api.filtersMu.RUnlock()
		require.Zero(t, remaining)
		api.receiptLogs.mu.Lock()
		subscribers := len(api.receiptLogs.filters)
		api.receiptLogs.mu.Unlock()
		require.Zero(t, subscribers)
		_, open := <-api.toDelete
		require.False(t, open)
		require.NotPanics(t, api.shutdown)
	}
	require.Empty(t, server.onStop)
	require.NotPanics(t, server.Stop)
}
