package evmrpc_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/stretchr/testify/require"
)

func TestEthclientPendingNonceAndPendingBlock(t *testing.T) {
	sender := utils.NewSigner()
	recipient := utils.NewSigner().EvmAddress
	servers := newSequencerOptionsServers(t, sender)
	client := ethclient.NewClient(servers.http)
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()

	nonce, err := client.PendingNonceAt(ctx, sender.EvmAddress)
	require.NoError(t, err)
	require.Equal(t, uint64(0), nonce)

	type pendingBlock struct {
		Number       hexutil.Uint64 `json:"number"`
		Hash         *common.Hash   `json:"hash"`
		Transactions []common.Hash  `json:"transactions"`
	}
	var empty *pendingBlock
	require.NoError(t, client.Client().CallContext(ctx, &empty, "eth_getBlockByNumber", "pending", false))
	require.NotNil(t, empty)
	require.Equal(t, hexutil.Uint64(servers.s.committed+1), empty.Number)
	require.Nil(t, empty.Hash)
	require.Equal(t, []common.Hash{}, empty.Transactions)

	raw, wantHash := subscribeRawTx(t, sender, nonce, &recipient, big.NewInt(pendingOnePaxWei), pendingTransferGas, nil)
	var transfer ethtypes.Transaction
	require.NoError(t, transfer.UnmarshalBinary(raw))
	require.Equal(t, wantHash, transfer.Hash())
	require.NoError(t, client.SendTransaction(ctx, &transfer))

	nonce, err = client.PendingNonceAt(ctx, sender.EvmAddress)
	require.NoError(t, err)
	require.Equal(t, uint64(1), nonce, "the pending nonce must count the accepted transfer")
	committedNonce, err := client.NonceAt(ctx, sender.EvmAddress, nil)
	require.NoError(t, err)
	require.Equal(t, uint64(0), committedNonce, "the transfer must remain uncommitted")
	require.Equal(t, committedNonce+1, nonce, "the pending nonce must be the committed nonce plus one")
	require.Equal(t, servers.s.committed, servers.s.app.LastBlockHeight())

	var block *pendingBlock
	require.NoError(t, client.Client().CallContext(ctx, &block, "eth_getBlockByNumber", "pending", false))
	require.NotNil(t, block)
	require.Equal(t, hexutil.Uint64(servers.s.committed+1), block.Number)
	require.Nil(t, block.Hash)
	require.Equal(t, []common.Hash{wantHash}, block.Transactions)
}
