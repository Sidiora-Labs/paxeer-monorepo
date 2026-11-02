package evmrpc_test

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

func dialEthclientBlocks(t *testing.T) *ethclient.Client {
	t.Helper()
	client, err := ethclient.Dial(fmt.Sprintf("http://%s:%d", TestAddr, TestPort))
	require.NoError(t, err)
	t.Cleanup(client.Close)
	return client
}

func TestEthclientBlockByNumberDecodes(t *testing.T) {
	client := dialEthclientBlocks(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	block, err := client.BlockByNumber(ctx, big.NewInt(2))
	require.NoError(t, err)
	require.NotNil(t, block)
	require.Equal(t, uint64(2), block.NumberU64())
	require.Equal(t, ethtypes.EmptyUncleHash, block.UncleHash())
	require.Empty(t, block.Uncles())
	require.NotEqual(t, ethtypes.EmptyTxsHash, block.TxHash())

	wantTransactions := []*ethtypes.Transaction{multiTxBlockTx1, multiTxBlockTx2, multiTxBlockTx3}
	require.Len(t, block.Transactions(), len(wantTransactions))
	for i, want := range wantTransactions {
		got := block.Transactions()[i]
		require.NotNil(t, got)
		require.Equal(t, want.Hash(), got.Hash(), "transaction %d", i)
		wantBytes, err := want.MarshalBinary()
		require.NoError(t, err)
		gotBytes, err := got.MarshalBinary()
		require.NoError(t, err)
		require.Equal(t, wantBytes, gotBytes, "transaction %d", i)
	}
}

func TestEthclientHeaderByHashMatchesBlock(t *testing.T) {
	client := dialEthclientBlocks(t)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	block, err := client.BlockByNumber(ctx, big.NewInt(2))
	require.NoError(t, err)
	require.NotNil(t, block)
	require.Equal(t, uint64(2), block.NumberU64())

	header, err := client.HeaderByHash(ctx, common.HexToHash(MultiTxBlockHash))
	require.NoError(t, err)
	require.NotNil(t, header)
	require.Equal(t, block.Header(), ethtypes.CopyHeader(header))
	require.Equal(t, block.Hash(), header.Hash())
}
