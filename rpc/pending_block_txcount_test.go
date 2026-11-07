package evmrpc_test

import (
	"context"
	"encoding/json"
	"math/big"
	"testing"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/node/sequencer"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func TestPendingBlockTransactionCount(t *testing.T) {
	const committed = int64(1)
	sender := utils.NewSigner()
	recipient := utils.NewSigner().EvmAddress
	s := newPendingSequencer(t, committed, sender)
	api := pendingBlockAPI(s)
	server := rpc.NewServer()
	t.Cleanup(server.Stop)
	require.NoError(t, server.RegisterName("eth", api))
	client := rpc.DialInProc(server)
	t.Cleanup(client.Close)
	ctx := context.Background()

	var result json.RawMessage
	require.NoError(t, client.CallContext(ctx, &result, "eth_getBlockTransactionCountByNumber", "pending"))
	require.JSONEq(t, `"0x0"`, string(result))

	for nonce, want := range []string{`"0x1"`, `"0x2"`} {
		accepted, err := s.exec.Accept(pendingTransfer(t, sender, uint64(nonce), recipient, big.NewInt(pendingOnePaxWei)))
		require.NoError(t, err)
		require.Equal(t, committed+1, accepted.Height)
		require.Equal(t, abci.CodeTypeOK, accepted.Result.Code)
		require.NoError(t, client.CallContext(ctx, &result, "eth_getBlockTransactionCountByNumber", "pending"))
		require.JSONEq(t, want, string(result))
	}
	require.Equal(t, committed, s.app.LastBlockHeight())

	_, err := s.exec.Seal(committed + 1)
	require.NoError(t, err)
	count, err := api.GetBlockTransactionCountByNumber(ctx, rpc.PendingBlockNumber)
	require.ErrorIs(t, err, sequencer.ErrLayerNotOpen)
	require.Nil(t, count)

	s.wm.SetPendingSnapshot(nil)
	require.NoError(t, client.CallContext(ctx, &result, "eth_getBlockTransactionCountByNumber", "pending"))
	require.JSONEq(t, "null", string(result))

	withoutWatermarks := evmrpc.NewBlockAPI(nil, nil, nil, nil, evmrpc.ConnectionTypeHTTP, nil, nil, nil)
	count, err = withoutWatermarks.GetBlockTransactionCountByNumber(ctx, rpc.PendingBlockNumber)
	require.NoError(t, err)
	require.Nil(t, count)
}
