package evmrpc_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
)

func TestEthclientSubscribeNewHeadHashMatchesBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	wsClient, err := ethclient.DialContext(ctx, fmt.Sprintf("ws://%s:%d", TestAddr, TestNotifierWSPort))
	require.NoError(t, err)
	defer wsClient.Close()
	httpClient, err := ethclient.DialContext(ctx, fmt.Sprintf("http://%s:%d", TestAddr, TestPort))
	require.NoError(t, err)
	defer httpClient.Close()

	heads := make(chan *ethtypes.Header, 5)
	sub, err := wsClient.SubscribeNewHead(ctx, heads)
	require.NoError(t, err)
	defer sub.Unsubscribe()

	type headPayload struct {
		Hash   common.Hash  `json:"hash"`
		Number *hexutil.Big `json:"number"`
	}
	rawHeads := make(chan *headPayload, 5)
	rawSub, err := wsClient.Client().EthSubscribe(ctx, rawHeads, "newHeads")
	require.NoError(t, err)
	defer rawSub.Unsubscribe()

	height := int64(MockHeight8)
	client := &MockClient{}
	block, err := client.Block(ctx, &height)
	require.NoError(t, err)
	require.NotNil(t, block)
	require.NotNil(t, block.Block)
	results, err := client.BlockResults(ctx, &height)
	require.NoError(t, err)
	require.NotNil(t, results)
	committedHeader := block.Block.Header.ToProto()
	committedHeader.ProposerAddress = common.BytesToAddress(block.Block.ProposerAddress).Bytes()
	NotifierForTest.OnBlockCommitted(block.BlockID.Hash, committedHeader, &abci.ResponseFinalizeBlock{
		AppHash:   block.Block.AppHash,
		TxResults: results.TxsResults,
	})

	var header *ethtypes.Header
	select {
	case header = <-heads:
	case err := <-sub.Err():
		t.Fatalf("newHeads subscription ended before receiving a head: %v", err)
	case <-ctx.Done():
		t.Fatalf("waiting for newHeads notification: %v", ctx.Err())
	}
	require.NotNil(t, header)
	require.NotNil(t, header.Number)
	require.Positive(t, header.Number.Sign())
	require.Equal(t, height, header.Number.Int64(), "head must come from the committed fixture")

	var rawHead *headPayload
	select {
	case rawHead = <-rawHeads:
	case err := <-rawSub.Err():
		t.Fatalf("raw newHeads subscription ended before receiving a head: %v", err)
	case <-ctx.Done():
		t.Fatalf("waiting for raw newHeads notification: %v", ctx.Err())
	}
	require.NotNil(t, rawHead)
	require.NotNil(t, rawHead.Number)
	require.Zero(t, header.Number.Cmp(rawHead.Number.ToInt()), "both subscriptions must receive the committed height")

	httpBlock, err := httpClient.BlockByNumber(ctx, header.Number)
	require.NoError(t, err)
	require.NotNil(t, httpBlock)
	require.Zero(t, header.Number.Cmp(httpBlock.Number()), "HTTP block must have the subscribed height")

	var served *headPayload
	err = httpClient.Client().CallContext(ctx, &served, "eth_getBlockByNumber", hexutil.EncodeBig(header.Number), false)
	require.NoError(t, err)
	require.NotNil(t, served)
	require.NotNil(t, served.Number)
	require.Zero(t, header.Number.Cmp(served.Number.ToInt()))
	require.NotEqual(t, common.Hash{}, served.Hash)
	assert.Equal(t, httpBlock.Hash(), header.Hash(), "SubscribeNewHead and BlockByNumber hashes at height %s", header.Number)
	assert.Equal(t, common.BytesToHash(block.BlockID.Hash), served.Hash, "HTTP block must carry the committed consensus hash")
	assert.Equal(t, served.Hash, rawHead.Hash, "raw newHeads hash and eth_getBlockByNumber hash at height %s", header.Number)
}
