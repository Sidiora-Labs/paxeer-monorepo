package evmrpc_test

import (
	"context"
	"math/big"
	"net/http/httptest"
	"testing"
	"time"

	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/fixtures/example/contracts/simplestorage"
	"github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/stretchr/testify/require"
)

func repair105Client(t *testing.T, s *pendingSequencer, wm *evmrpc.WatermarkManager) *rpc.Client {
	t.Helper()
	node := pendingSimulateAPIs(s, wm)
	server := rpc.NewServer()
	require.NoError(t, server.RegisterName("eth", node.sim))
	t.Cleanup(server.Stop)
	httpServer := httptest.NewServer(server)
	t.Cleanup(httpServer.Close)
	client, err := rpc.DialHTTP(httpServer.URL)
	require.NoError(t, err)
	t.Cleanup(client.Close)
	return client
}

func TestRepair_105_CallOmittedTagReadsOpenState(t *testing.T) {
	sender := utils.NewSigner()
	s := newPendingSequencer(t, 2, sender)
	creation, set, get := pendingSimulateStorage(t)
	pendingSimulateAccept(t, s.exec, pendingSimulateTx(t, sender, 0, nil, big.NewInt(0), creation, pendingSimulateDeployGas))
	contract := crypto.CreateAddress(sender.EvmAddress, 0)
	commit := func() {
		t.Helper()
		batch, err := s.exec.Seal(s.committed + 1)
		require.NoError(t, err)
		batch.Store.Write()
		s.committed++
		require.Equal(t, s.committed, s.app.CommitMultiStore().Commit(true).Version)
	}
	commit()

	wm := evmrpc.NewWatermarkManager(nil, s.ctxProvider, nil, nil)
	wm.SetPendingSnapshot(s.exec)
	client := repair105Client(t, s, wm)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	args := pendingSimulateArgs(sender.EvmAddress, contract, big.NewInt(0), get)
	assertCalls := func(t *testing.T, client *rpc.Client, open, latest int64) {
		t.Helper()
		for _, test := range []struct {
			name string
			tag  string
			want int64
		}{
			{name: "omitted", want: open},
			{name: "pending", tag: "pending", want: open},
			{name: "latest", tag: "latest", want: latest},
		} {
			t.Run(test.name, func(t *testing.T) {
				params := []any{args}
				if test.tag != "" {
					params = append(params, test.tag)
				}
				var result hexutil.Bytes
				require.NoError(t, client.CallContext(ctx, &result, "eth_call", params...))
				require.Equal(t, pendingSimulateWord(test.want), result)
			})
		}
	}

	t.Run("idle", func(t *testing.T) { assertCalls(t, client, 0, 0) })
	require.NoError(t, s.exec.Open(s.committed+1, time.Now().UTC().UnixMilli()))
	pendingSimulateAccept(t, s.exec, pendingSimulateTx(t, sender, 1, &contract, big.NewInt(0), set, pendingSimulateSetGas))
	t.Run("before_seal", func(t *testing.T) { assertCalls(t, client, pendingSimulateStored, 0) })

	fullNode := repair105Client(t, s, evmrpc.NewWatermarkManager(nil, s.ctxProvider, nil, nil))
	t.Run("without_pending_provider", func(t *testing.T) {
		var result hexutil.Bytes
		require.NoError(t, fullNode.CallContext(ctx, &result, "eth_call", args))
		require.Equal(t, pendingSimulateWord(0), result)
		require.NoError(t, fullNode.CallContext(ctx, &result, "eth_call", args, "latest"))
		require.Equal(t, pendingSimulateWord(0), result)
		err := fullNode.CallContext(ctx, &result, "eth_call", args, "pending")
		var rpcErr rpc.Error
		require.ErrorAs(t, err, &rpcErr)
		require.Equal(t, -32000, rpcErr.ErrorCode())
		require.EqualError(t, err, evmrpc.ErrPendingServedBySequencer.Error())
	})

	batch, err := s.exec.Seal(s.committed + 1)
	require.NoError(t, err)
	t.Run("sealed_before_commit", func(t *testing.T) { assertCalls(t, client, 0, 0) })
	batch.Store.Write()
	s.committed++
	require.Equal(t, s.committed, s.app.CommitMultiStore().Commit(true).Version)
	client = repair105Client(t, s, wm)
	t.Run("committed_idle", func(t *testing.T) { assertCalls(t, client, pendingSimulateStored, pendingSimulateStored) })

	require.NoError(t, s.exec.Open(s.committed+1, time.Now().UTC().UnixMilli()))
	abi, err := simplestorage.SimplestorageMetaData.GetAbi()
	require.NoError(t, err)
	setAgain, err := abi.Pack("set", big.NewInt(37))
	require.NoError(t, err)
	pendingSimulateAccept(t, s.exec, pendingSimulateTx(t, sender, 2, &contract, big.NewInt(0), setAgain, pendingSimulateSetGas))
	t.Run("reopened", func(t *testing.T) { assertCalls(t, client, 37, pendingSimulateStored) })

	snapshot, err := s.exec.Snapshot()
	require.NoError(t, err)
	require.Equal(t, common.BigToHash(big.NewInt(37)), s.app.EvmKeeper.GetState(snapshot, contract, common.Hash{}))
	require.Equal(t, common.BigToHash(big.NewInt(pendingSimulateStored)), s.app.EvmKeeper.GetState(s.ctxProvider(evmrpc.LatestCtxHeight), contract, common.Hash{}))
}
