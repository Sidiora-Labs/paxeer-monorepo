package evmrpc_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	rpclocal "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/client/local"
	rpctest "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/test"
	app "github.com/Sidiora-Labs/Paxeer-X-Network/node"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestBlockLookupHasNoLaunchHeightFloor(t *testing.T) {
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
		if status.SyncInfo.LatestBlockHeight >= 2 {
			break
		}
		select {
		case <-ctx.Done():
			require.FailNow(t, "chain did not reach height 2", "%v", ctx.Err())
		case <-time.After(20 * time.Millisecond):
		}
	}
	height := int64(2)
	block, err := tm.Block(ctx, &height)
	require.NoError(t, err)
	require.NotNil(t, block.Block)
	expectedHash := common.BytesToHash(block.BlockID.Hash).Hex()

	application := app.Setup(t, false, true, false)
	sdkCtx := application.GetContextForDeliverTx(nil).
		WithChainID("pacific-1").
		WithBlockTime(block.Block.Time).
		WithClosestUpgradeName("v6.0.0")
	require.Equal(t, "pacific-1", sdkCtx.ChainID())
	ctxProvider := func(h int64) sdk.Context {
		if h == evmrpc.LatestCtxHeight {
			return sdkCtx
		}
		return sdkCtx.WithBlockHeight(h)
	}
	txConfigProvider := func(int64) client.TxConfig { return TxConfig }
	watermarks := evmrpc.NewWatermarkManager(tm, nil, nil, nil)
	api := evmrpc.NewBlockAPI(tm, &application.EvmKeeper, ctxProvider, txConfigProvider,
		evmrpc.ConnectionTypeHTTP, watermarks, evmrpc.NewBlockCache(3000), &sync.Mutex{})
	server := rpc.NewServer()
	t.Cleanup(server.Stop)
	require.NoError(t, server.RegisterName("eth", api))
	rpcClient := rpc.DialInProc(server)
	t.Cleanup(rpcClient.Close)

	for _, fullTx := range []bool{false, true} {
		var byNumber map[string]interface{}
		require.NoError(t, rpcClient.CallContext(ctx, &byNumber, "eth_getBlockByNumber", "0x2", fullTx))
		require.NotNil(t, byNumber)
		require.Equal(t, "0x2", byNumber["number"])
		require.Equal(t, expectedHash, byNumber["hash"])
		require.Empty(t, byNumber["transactions"])

		var byHash map[string]interface{}
		require.NoError(t, rpcClient.CallContext(ctx, &byHash, "eth_getBlockByHash", expectedHash, fullTx))
		require.NotNil(t, byHash)
		require.Equal(t, byNumber, byHash)
	}
}
