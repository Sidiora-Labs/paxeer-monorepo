package evmrpc

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	rpclocal "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	rpctest "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/test"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/eth/filters"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestGetLogsUnknownBlockHashErrors(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	conf, err := rpctest.CreateConfig(t, t.Name())
	require.NoError(t, err)
	kv := kvstore.NewApplication()
	t.Cleanup(func() { require.NoError(t, kv.Close()) })
	nodeService, closer, err := rpctest.StartTendermint(ctx, conf, kv, rpctest.SuppressStdout)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, closer(context.Background())) })
	nodeWithEnv, ok := nodeService.(rpclocal.NodeService)
	require.True(t, ok)
	tm, err := rpclocal.New(nodeWithEnv)
	require.NoError(t, err)
	for {
		status, err := tm.Status(ctx)
		require.NoError(t, err)
		if status.SyncInfo.LatestBlockHeight >= 1 {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}

	watermarks := NewWatermarkManager(tm, nil, nil, nil)
	filterConfig := NewFilterConfig(0, DefaultMaxLogLimit, DefaultMaxBlockRange)
	fetcher := &LogFetcher{tmClient: tm, filterConfig: filterConfig, watermarks: watermarks}
	api := &FilterAPI{
		namespace:      "eth",
		connectionType: ConnectionTypeHTTP,
		filterConfig:   filterConfig,
		logFetcher:     fetcher,
	}
	server := rpc.NewServer()
	t.Cleanup(server.Stop)
	require.NoError(t, server.RegisterName("eth", api))
	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)
	client, err := rpc.DialHTTP(httpServer.URL)
	require.NoError(t, err)
	t.Cleanup(client.Close)

	for _, tc := range []struct {
		name string
		hash common.Hash
	}{
		{name: "unknown", hash: common.HexToHash("0xdeadbeef")},
		{name: "zero", hash: common.Hash{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.hash != (common.Hash{}) {
				block, err := tm.BlockByHash(ctx, tc.hash.Bytes())
				require.NoError(t, err)
				require.Nil(t, block.Block)
			}
			var logs []*FilterLog
			err := client.CallContext(ctx, &logs, "eth_getLogs", map[string]string{"blockHash": tc.hash.Hex()})
			require.EqualError(t, err, "unknown block")
			require.Nil(t, logs)
			var rpcErr rpc.Error
			require.ErrorAs(t, err, &rpcErr)
			require.Equal(t, -32000, rpcErr.ErrorCode())
		})
	}

	height := int64(1)
	block, err := tm.Block(ctx, &height)
	require.NoError(t, err)
	require.NotNil(t, block.Block)
	hash := common.BytesToHash(block.BlockID.Hash)
	blocks, end, applyLimit, err := fetcher.fetchBlocksByCrit(ctx, filters.FilterCriteria{BlockHash: &hash}, 0, nil)
	require.NoError(t, err)
	require.Zero(t, end)
	require.False(t, applyLimit)
	require.Equal(t, 1, cap(blocks))
	found, ok := <-blocks
	require.True(t, ok)
	require.Equal(t, block.BlockID, found.BlockID)
	_, ok = <-blocks
	require.False(t, ok)
}
