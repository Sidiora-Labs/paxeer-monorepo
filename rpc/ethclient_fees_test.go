package evmrpc_test

import (
	"context"
	"fmt"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

func TestEthclientSuggestGasPriceAndTip(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client, err := ethclient.DialContext(ctx, fmt.Sprintf("http://%s:%d", TestAddr, TestFeePort))
	require.NoError(t, err)
	t.Cleanup(client.Close)

	price, err := client.SuggestGasPrice(ctx)
	require.NoError(t, err)
	require.NotNil(t, price)
	require.Positive(t, price.Sign(), "suggested gas price must be positive")

	tip, err := client.SuggestGasTipCap(ctx)
	require.NoError(t, err)
	require.NotNil(t, tip)
	require.Positive(t, tip.Sign(), "suggested gas tip must be positive")
	require.GreaterOrEqual(t, price.Cmp(tip), 0, "suggested gas price %s must be at least the tip %s", price, tip)

	const defaultPriorityFeePerGas = 1000000000
	require.Equal(t, big.NewInt(defaultPriorityFeePerGas), tip)
	history, err := client.FeeHistory(ctx, 1, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, history)
	require.Len(t, history.BaseFee, 2)
	require.NotNil(t, history.BaseFee[1])
	expectedPrice := new(big.Int).Mul(history.BaseFee[1], big.NewInt(110))
	expectedPrice.Div(expectedPrice, big.NewInt(100))
	expectedPrice.Add(expectedPrice, big.NewInt(defaultPriorityFeePerGas))
	require.Equal(t, expectedPrice, price)
}

func TestEthclientFeeHistory(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	client, err := ethclient.DialContext(ctx, fmt.Sprintf("http://%s:%d", TestAddr, TestFeePort))
	require.NoError(t, err)
	t.Cleanup(client.Close)

	const blockCount = 2
	percentiles := []float64{50}
	history, err := client.FeeHistory(ctx, blockCount, nil, percentiles)
	require.NoError(t, err)
	require.NotNil(t, history)
	require.Equal(t, big.NewInt(MockHeight8-blockCount+1), history.OldestBlock)
	require.Len(t, history.BaseFee, blockCount+1, "base fees must include the next block")
	require.Len(t, history.Reward, blockCount, "each block must have a reward row")
	require.Len(t, history.GasUsedRatio, blockCount)
	for i, fee := range history.BaseFee {
		require.NotNil(t, fee, "base fee %d must decode", i)
		require.Positive(t, fee.Sign(), "base fee %d must be positive", i)
	}
	for i, rewards := range history.Reward {
		require.Len(t, rewards, len(percentiles), "reward row %d must contain each requested percentile", i)
		for _, reward := range rewards {
			require.NotNil(t, reward, "reward in row %d must decode", i)
			require.GreaterOrEqual(t, reward.Sign(), 0, "reward in row %d must be nonnegative", i)
		}
		require.GreaterOrEqual(t, history.GasUsedRatio[i], 0.0)
		require.LessOrEqual(t, history.GasUsedRatio[i], 1.0)
	}
}
