package evmrpc_test

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

func TestEthclientFilterLogs(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := ethclient.DialContext(ctx, fmt.Sprintf("http://%s:%d", TestAddr, TestPort))
	require.NoError(t, err)
	defer client.Close()

	rangeLogs, err := client.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: big.NewInt(MockHeight2),
		ToBlock:   big.NewInt(MockHeight2),
	})
	require.NoError(t, err, "decode eth_getLogs for the block-2 range")
	require.NotEmpty(t, rangeLogs, "block 2 contains EVM logs")

	blockHash := common.HexToHash(MultiTxBlockHash)
	hashLogs, err := client.FilterLogs(ctx, ethereum.FilterQuery{BlockHash: &blockHash})
	require.NoError(t, err, "decode eth_getLogs for the block-2 hash")
	require.NotEmpty(t, hashLogs, "block 2 contains EVM logs")
	require.Equal(t, rangeLogs, hashLogs, "range and block-hash queries return identical decoded logs")

	for i, log := range rangeLogs {
		require.Equal(t, uint64(MockHeight2), log.BlockNumber, "log %d block number", i)
		require.Equal(t, blockHash, log.BlockHash, "log %d block hash", i)
	}
}
