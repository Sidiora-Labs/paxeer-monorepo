package evmrpc_test

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestEthclientTransactionReceiptDecodes(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := ethclient.DialContext(ctx, fmt.Sprintf("http://%s:%d", TestAddr, TestPort))
	require.NoError(t, err)
	defer client.Close()

	receipt, err := client.TransactionReceipt(ctx, multiTxBlockTx1.Hash())
	require.NoError(t, err)
	require.NotNil(t, receipt)
	require.Equal(t, multiTxBlockTx1.Hash(), receipt.TxHash)
	require.Equal(t, big.NewInt(MockHeight2), receipt.BlockNumber)
	require.Equal(t, common.HexToHash(MultiTxBlockHash), receipt.BlockHash)
	require.Equal(t, uint(0), receipt.TransactionIndex)
	require.Equal(t, uint64(21000), receipt.GasUsed)
	require.Equal(t, uint64(21000), receipt.CumulativeGasUsed)
	require.Equal(t, uint8(ethtypes.LegacyTxType), receipt.Type)
	require.Equal(t, evmtypes.DefaultMinFeePerGas.TruncateInt().BigInt(), receipt.EffectiveGasPrice)
	require.Len(t, receipt.Logs, 2)
	for i, log := range receipt.Logs {
		require.NotNil(t, log)
		require.Equal(t, common.HexToAddress("0x1111111111111111111111111111111111111112"), log.Address)
		topics := []common.Hash{common.HexToHash("0x123")}
		if i == 0 {
			topics = append(topics, common.HexToHash("0x456"))
		}
		require.Equal(t, topics, log.Topics)
		require.Empty(t, log.Data)
		require.Equal(t, receipt.BlockHash, log.BlockHash)
		require.Equal(t, uint64(MockHeight2), log.BlockNumber)
		require.Equal(t, receipt.TxHash, log.TxHash)
		require.Equal(t, receipt.TransactionIndex, log.TxIndex)
		require.Equal(t, uint(0), log.Index)
		require.False(t, log.Removed)
	}
	require.Equal(t, ethtypes.CreateBloom(receipt), receipt.Bloom)
	require.Equal(t, uint64(ethtypes.ReceiptStatusSuccessful), receipt.Status)
}

func TestEthclientBlockReceiptsDecode(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	client, err := ethclient.DialContext(ctx, fmt.Sprintf("http://%s:%d", TestAddr, TestPort))
	require.NoError(t, err)
	defer client.Close()

	receipts, err := client.BlockReceipts(ctx, rpc.BlockNumberOrHashWithNumber(rpc.BlockNumber(MockHeight2)))
	require.NoError(t, err)
	blockHash := common.HexToHash(MultiTxBlockHash)
	count, err := client.TransactionCount(ctx, blockHash)
	require.NoError(t, err)
	require.Equal(t, uint(3), count)
	require.Len(t, receipts, int(count))

	expected := []struct {
		hash              common.Hash
		gasUsed           uint64
		cumulativeGasUsed uint64
		status            uint64
		address           common.Address
		logs              int
		logIndex          uint
	}{
		{multiTxBlockTx1.Hash(), 21000, 21000, ethtypes.ReceiptStatusSuccessful, common.HexToAddress("0x1111111111111111111111111111111111111112"), 2, 0},
		{multiTxBlockTx2.Hash(), 30000, 51000, ethtypes.ReceiptStatusFailed, common.HexToAddress("0x1111111111111111111111111111111111111113"), 1, 2},
		{multiTxBlockTx3.Hash(), 25000, 76000, ethtypes.ReceiptStatusFailed, common.HexToAddress("0x1111111111111111111111111111111111111114"), 1, 3},
	}
	for i, want := range expected {
		t.Run(fmt.Sprintf("transaction_%d", i), func(t *testing.T) {
			receipt := receipts[i]
			require.NotNil(t, receipt)
			require.Equal(t, want.hash, receipt.TxHash)
			require.NotEqual(t, block2SyntheticTx.Hash(), receipt.TxHash)
			require.Equal(t, blockHash, receipt.BlockHash)
			require.Equal(t, big.NewInt(MockHeight2), receipt.BlockNumber)
			require.Equal(t, uint(i), receipt.TransactionIndex)
			require.Equal(t, want.gasUsed, receipt.GasUsed)
			require.Equal(t, want.cumulativeGasUsed, receipt.CumulativeGasUsed)
			require.Equal(t, uint8(ethtypes.LegacyTxType), receipt.Type)
			require.Equal(t, evmtypes.DefaultMinFeePerGas.TruncateInt().BigInt(), receipt.EffectiveGasPrice)
			require.Equal(t, want.status, receipt.Status)
			require.Len(t, receipt.Logs, want.logs)
			for j, log := range receipt.Logs {
				require.NotNil(t, log)
				require.Equal(t, want.address, log.Address)
				topics := []common.Hash{common.HexToHash("0x123")}
				if j == 0 {
					topics = append(topics, common.HexToHash("0x456"))
				}
				require.Equal(t, topics, log.Topics)
				require.Equal(t, blockHash, log.BlockHash)
				require.Equal(t, uint64(MockHeight2), log.BlockNumber)
				require.Equal(t, want.hash, log.TxHash)
				require.Equal(t, uint(i), log.TxIndex)
				require.Equal(t, want.logIndex, log.Index)
				require.Empty(t, log.Data)
				require.False(t, log.Removed)
			}
			require.Equal(t, ethtypes.CreateBloom(receipt), receipt.Bloom)
		})
	}
}
