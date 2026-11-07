package evmrpc_test

import (
	"context"
	"encoding/json"
	"math"
	"math/big"
	"sync"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	occutils "github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/export"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestPendingTransactionByBlockNumberAndIndex(t *testing.T) {
	const committed = int64(2)
	sender := occutils.NewSigner()
	recipient := occutils.NewSigner().EvmAddress
	s := newPendingSequencer(t, committed, sender)
	txConfig := func(int64) client.TxConfig { return s.app.GetTxConfig() }
	api := evmrpc.NewTransactionAPI(nil, &s.app.EvmKeeper, s.ctxProvider, txConfig, "",
		evmrpc.ConnectionTypeHTTP, utils.None[time.Duration](), s.wm, nil, &sync.Mutex{})
	paxAPI := evmrpc.NewPaxTransactionAPI(nil, &s.app.EvmKeeper, s.ctxProvider, txConfig, "",
		evmrpc.ConnectionTypeHTTP, utils.None[time.Duration](), nil, s.wm, nil, &sync.Mutex{})
	server := rpc.NewServer()
	require.NoError(t, server.RegisterName("eth", api))
	require.NoError(t, server.RegisterName("pax", paxAPI))
	rpcClient := rpc.DialInProc(server)
	t.Cleanup(func() {
		rpcClient.Close()
		server.Stop()
	})
	ctx := context.Background()
	lookup := func(namespace string, index hexutil.Uint) json.RawMessage {
		t.Helper()
		var result json.RawMessage
		require.NoError(t, rpcClient.CallContext(ctx, &result,
			namespace+"_getTransactionByBlockNumberAndIndex", "pending", index))
		return result
	}
	for _, namespace := range []string{"eth", "pax"} {
		require.JSONEq(t, "null", string(lookup(namespace, 0)))
	}

	for nonce := uint64(0); nonce < 3; nonce++ {
		value := big.NewInt(int64(nonce+1) * pendingOnePaxWei)
		txBytes := pendingTransfer(t, sender, nonce, recipient, value)
		accepted, err := s.exec.Accept(txBytes)
		require.NoError(t, err)
		require.Equal(t, abci.CodeTypeOK, accepted.Result.Code)
		hash := pendingBlockEthHash(t, s, txBytes)
		for _, namespace := range []string{"eth", "pax"} {
			raw := lookup(namespace, hexutil.Uint(nonce))
			var got export.RPCTransaction
			require.NoError(t, json.Unmarshal(raw, &got))
			require.Equal(t, hash, got.Hash)
			require.Equal(t, sender.EvmAddress, got.From)
			require.Equal(t, &recipient, got.To)
			require.Equal(t, hexutil.Uint64(nonce), got.Nonce)
			require.Equal(t, (*hexutil.Big)(value), got.Value)
			require.Equal(t, (*hexutil.Big)(big.NewInt(pendingGasPriceWei)), got.GasPrice)
			require.Equal(t, hexutil.Uint64(pendingTransferGas), got.Gas)
			require.Equal(t, (*hexutil.Big)(big.NewInt(committed+1)), got.BlockNumber)
			require.NotNil(t, got.TransactionIndex)
			require.Equal(t, hexutil.Uint64(accepted.TxIndex), *got.TransactionIndex)
			require.Nil(t, got.BlockHash)
			var fields map[string]json.RawMessage
			require.NoError(t, json.Unmarshal(raw, &fields))
			require.JSONEq(t, "null", string(fields["blockHash"]))

			var byHash json.RawMessage
			require.NoError(t, rpcClient.CallContext(ctx, &byHash,
				namespace+"_getTransactionByHash", hash))
			require.JSONEq(t, string(byHash), string(raw))
		}
	}

	for _, namespace := range []string{"eth", "pax"} {
		first := lookup(namespace, 0)
		var got export.RPCTransaction
		require.NoError(t, json.Unmarshal(first, &got))
		require.Zero(t, got.Nonce)
		for _, index := range []hexutil.Uint{3, math.MaxUint32, math.MaxUint32 + 1} {
			require.JSONEq(t, "null", string(lookup(namespace, index)))
		}
	}

	_, err := s.exec.Seal(committed + 1)
	require.NoError(t, err)
	result, err := api.GetTransactionByBlockNumberAndIndex(ctx, rpc.PendingBlockNumber, 0)
	require.ErrorIs(t, err, sequencer.ErrLayerNotOpen)
	require.Nil(t, result)

	fullNodeAPI := evmrpc.NewTransactionAPI(nil, &s.app.EvmKeeper, s.ctxProvider, txConfig, "",
		evmrpc.ConnectionTypeHTTP, utils.None[time.Duration](), nil, nil, &sync.Mutex{})
	result, err = fullNodeAPI.GetTransactionByBlockNumberAndIndex(ctx, rpc.PendingBlockNumber, 0)
	require.ErrorIs(t, err, evmrpc.ErrPendingServedBySequencer)
	require.Nil(t, result)
}
