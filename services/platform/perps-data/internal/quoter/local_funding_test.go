package quoter

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func fundLocalAccount(t *testing.T, ctx context.Context, owner *RPC, to common.Address, nonce uint64, gasPrice *big.Int) {
	t.Helper()
	amount := new(big.Int).Mul(big.NewInt(10), unit)
	gas, err := owner.client.EstimateGas(ctx, ethereum.CallMsg{From: owner.account, To: &to, Value: amount, GasPrice: gasPrice})
	require.NoError(t, err)
	tx := types.NewTransaction(nonce, to, amount, gas+gas/5, gasPrice, nil)
	signed, err := types.SignTx(tx, types.LatestSignerForChainID(owner.chain), owner.key)
	require.NoError(t, err)
	require.NoError(t, owner.client.SendTransaction(ctx, signed))
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		receipt, err := owner.client.TransactionReceipt(ctx, signed.Hash())
		if err == nil {
			require.Equal(t, types.ReceiptStatusSuccessful, receipt.Status)
			return
		}
		require.ErrorIs(t, err, ethereum.NotFound)
		select {
		case <-ctx.Done():
			t.Fatal("local gas funding timed out")
		case <-ticker.C:
		}
	}
}
