package evmrpc_test

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http/httptest"
	"testing"
	"time"

	occutils "github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestSendRawTransactionSyncReturnsReceipt(t *testing.T) {
	to := common.HexToAddress("010203")
	_, transfer := buildTx(ethtypes.DynamicFeeTx{
		Nonce:     2,
		GasFeeCap: big.NewInt(10),
		Gas:       1000,
		To:        &to,
		Value:     big.NewInt(1000),
		Data:      []byte("abc"),
		ChainID:   EVMKeeper.ChainID(Ctx),
	})
	require.Equal(t, multiTxBlockTx1.Hash(), transfer.Hash())
	_, r, s := transfer.RawSignatureValues()
	require.Positive(t, r.Sign())
	require.Positive(t, s.Sign())
	raw, err := transfer.MarshalBinary()
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	client, err := rpc.DialHTTP(fmt.Sprintf("http://%s:%d", TestAddr, TestPort))
	require.NoError(t, err)
	defer client.Close()

	stored, err := EVMKeeper.GetReceipt(Ctx, transfer.Hash())
	require.NoError(t, err)
	require.Equal(t, transfer.Hash().Hex(), stored.TxHashHex)
	var receipt map[string]interface{}
	require.NoError(t, client.CallContext(ctx, &receipt, "eth_sendRawTransactionSync", hexutil.Bytes(raw)))
	require.Equal(t, transfer.Hash().Hex(), receipt["transactionHash"])

	var standard map[string]interface{}
	require.NoError(t, client.CallContext(ctx, &standard, "eth_getTransactionReceipt", transfer.Hash()))
	require.NotEmpty(t, standard)
	require.Equal(t, standard, receipt)

	sender := occutils.NewSigner()
	h := newSendSeqHarness(t, sender)
	_, refusedRaw := sendSeqTransfer(t, sender, 0, to, big.NewInt(1000), sendSeqTransferGas)
	_, err = h.api.SendRawTransaction(context.Background(), refusedRaw)
	require.NoError(t, err)
	server := rpc.NewServer()
	require.NoError(t, server.RegisterName("eth", h.api))
	defer server.Stop()
	httpServer := httptest.NewServer(server)
	defer httpServer.Close()

	for _, refused := range []struct {
		name     string
		endpoint string
		raw      hexutil.Bytes
	}{
		{name: "refused transfer", endpoint: httpServer.URL, raw: refusedRaw},
		{name: "invalid encoding", endpoint: fmt.Sprintf("http://%s:%d", TestAddr, TestPort), raw: hexutil.Bytes{0x12, 0x34}},
	} {
		t.Run(refused.name, func(t *testing.T) {
			client, err := rpc.DialHTTP(refused.endpoint)
			require.NoError(t, err)
			defer client.Close()
			ctx, cancel := context.WithTimeout(context.Background(), time.Second)
			defer cancel()
			var hash common.Hash
			sendErr := client.CallContext(ctx, &hash, "eth_sendRawTransaction", refused.raw)
			var sendRPCError rpc.Error
			require.True(t, errors.As(sendErr, &sendRPCError), "send error: %v", sendErr)
			var receipt map[string]interface{}
			syncErr := client.CallContext(ctx, &receipt, "eth_sendRawTransactionSync", refused.raw)
			var syncRPCError rpc.Error
			require.True(t, errors.As(syncErr, &syncRPCError), "sync error: %v", syncErr)
			require.Equal(t, sendRPCError.ErrorCode(), syncRPCError.ErrorCode())
			require.Equal(t, sendErr.Error(), syncErr.Error())
			require.Nil(t, receipt)
		})
	}
	require.Len(t, h.accepter.receipts, 1)
}
