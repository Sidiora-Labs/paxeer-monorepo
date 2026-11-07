package tests

import (
	"math/big"
	"testing"

	tmproto "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/proto/tendermint/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	testkeeper "github.com/Sidiora-Labs/Paxeer-X-Network/tests/testutil/keeper"
	"github.com/stretchr/testify/require"
)

func TestGasPriceCongestionThreshold(t *testing.T) {
	// Verify threshold uses > 80% (strictly greater). At exactly 80% → not congested
	// Above 80% → congested.

	// Create a ctx provider with explicit MaxGas to make threshold deterministic
	baseCtx := testkeeper.EVMTestApp.GetContextForDeliverTx(nil)
	consensusParams := &tmproto.ConsensusParams{Block: &tmproto.BlockParams{MaxGas: 10000000}}
	baseCtx = baseCtx.WithConsensusParams(consensusParams)
	ctxProvider := func(int64) sdk.Context { return baseCtx }

	i := evmrpc.NewInfoAPI(nil, nil, ctxProvider, nil, t.TempDir(), 1024, evmrpc.ConnectionTypeHTTP, nil, nil)

	oneGwei := big.NewInt(1000000000)
	median := big.NewInt(2000000000) // 2 gwei

	// Exactly at 80% → not congested → 10% bump on base fee plus the default priority fee
	gasPrice, err := evmrpc.GasPriceHelper(
		i,
		baseCtx.Context(),
		oneGwei,
		8000000, // 80% of 10,000,000
		median,
	)
	require.NoError(t, err)
	// expected = baseFee * 110 / 100 + 1 gwei default priority fee
	expectedNotCongested := new(big.Int).Mul(new(big.Int).Set(oneGwei), big.NewInt(110))
	expectedNotCongested.Div(expectedNotCongested, big.NewInt(100))
	expectedNotCongested.Add(expectedNotCongested, oneGwei)
	require.Equal(t, expectedNotCongested, gasPrice.ToInt())

	// Just above 80% → congested → baseFee + median
	gasPrice, err = evmrpc.GasPriceHelper(
		i,
		baseCtx.Context(),
		oneGwei,
		8000001,
		median,
	)
	require.NoError(t, err)
	require.Equal(t, new(big.Int).Add(oneGwei, median), gasPrice.ToInt())
}

func TestGasPriceInvokesFeeHistoryMedian(t *testing.T) {
	// Single empty block is sufficient; FeeHistory will still execute and return zeros
	// and GasPrice will compute based on baseFee without error.
	SetupTestServer(t, nil).Run(func(port int) {
		res := sendRequestWithNamespace("eth", port, "gasPrice")

		// Ensure no top-level error is returned by the RPC
		if errObj, hasErr := res["error"]; hasErr {
			t.Fatalf("eth_gasPrice returned error: %v", errObj)
		}

		// Result should be a hex string; validate it's a positive integer
		result, ok := res["result"].(string)
		require.True(t, ok, "result should be a hex string")
		n := new(big.Int)
		_, ok = n.SetString(result, 0)
		require.True(t, ok, "result should parse as hex big.Int")
		require.NotEqual(t, 0, n.Sign(), "gas price should be non-zero")
	})
}
