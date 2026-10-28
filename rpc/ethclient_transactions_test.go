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
	"github.com/stretchr/testify/require"
)

func TestEthclientTransactionByHash(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := ethclient.DialContext(ctx, fmt.Sprintf("http://%s:%d", TestAddr, TestPort))
	require.NoError(t, err)
	t.Cleanup(client.Close)

	want := multiTxBlockTx1
	receipt := waitForReceipt(t, Ctx, want.Hash())
	require.Equal(t, uint64(MockHeight2), receipt.BlockNumber)

	got, isPending, err := client.TransactionByHash(ctx, want.Hash())
	require.NoError(t, err)
	require.False(t, isPending)
	requireEthclientTransactionEqual(t, want, got)
}

func TestEthclientTransactionInBlock(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := ethclient.DialContext(ctx, fmt.Sprintf("http://%s:%d", TestAddr, TestPort))
	require.NoError(t, err)
	t.Cleanup(client.Close)

	var block struct {
		Number       hexutil.Uint64 `json:"number"`
		Hash         common.Hash    `json:"hash"`
		Transactions []common.Hash  `json:"transactions"`
	}
	err = client.Client().CallContext(ctx, &block, "eth_getBlockByNumber", hexutil.EncodeUint64(MockHeight2), false)
	require.NoError(t, err)
	require.Equal(t, hexutil.Uint64(MockHeight2), block.Number)
	require.Equal(t, common.HexToHash(MultiTxBlockHash), block.Hash)
	require.NotEmpty(t, block.Transactions)
	require.Equal(t, multiTxBlockTx1.Hash(), block.Transactions[0])

	got, err := client.TransactionInBlock(ctx, block.Hash, 0)
	require.NoError(t, err)
	requireEthclientTransactionEqual(t, multiTxBlockTx1, got)
	require.Equal(t, block.Transactions[0], got.Hash())
}

func requireEthclientTransactionEqual(t *testing.T, want, got *ethtypes.Transaction) {
	t.Helper()
	require.NotNil(t, got)
	require.Equal(t, want.Hash(), got.Hash())
	require.Equal(t, want.Type(), got.Type())
	require.Equal(t, want.ChainId(), got.ChainId())
	require.Equal(t, want.Nonce(), got.Nonce())
	require.Equal(t, want.To(), got.To())
	require.Equal(t, want.Value(), got.Value())
	require.Equal(t, want.Data(), got.Data())

	wantBytes, err := want.MarshalBinary()
	require.NoError(t, err)
	gotBytes, err := got.MarshalBinary()
	require.NoError(t, err)
	require.Equal(t, wantBytes, gotBytes)
}
