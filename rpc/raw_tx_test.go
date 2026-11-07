package evmrpc_test

import (
	"context"
	"math/big"
	"sync"
	"testing"
	"time"

	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	evmtypes "github.com/Sidiora-Labs/Paxeer-X-Network/modules/evm/types"
	evmrpc "github.com/Sidiora-Labs/Paxeer-X-Network/rpc"
	"github.com/Sidiora-Labs/Paxeer-X-Network/sdk/client"
	sdk "github.com/Sidiora-Labs/Paxeer-X-Network/sdk/types"
	occutils "github.com/Sidiora-Labs/Paxeer-X-Network/tests/occ/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestGetRawTransactionByHashReturnsEnvelope(t *testing.T) {
	ctxProvider := func(height int64) sdk.Context {
		ctx := Ctx
		if height == MockHeight2 {
			ctx = MultiTxCtx
		}
		if height == evmrpc.LatestCtxHeight {
			height = MockHeight103
		}
		return ctx.WithBlockHeight(height).WithIsTracing(true).WithClosestUpgradeName(LatestCtxUpgradeName)
	}
	txConfigProvider := func(int64) client.TxConfig { return TxConfig }
	tmClient := &MockClient{}
	watermarks := evmrpc.NewWatermarkManager(tmClient, ctxProvider, nil, nil)
	api := evmrpc.NewRawTransactionAPI(tmClient, EVMKeeper, ctxProvider, txConfigProvider, "", evmrpc.ConnectionTypeHTTP, utils.None[time.Duration](), watermarks, evmrpc.NewBlockCache(8), &sync.Mutex{})

	assertEnvelope := func(t *testing.T, api *evmrpc.RawTransactionAPI, want *types.Transaction) {
		t.Helper()
		raw, err := api.GetRawTransactionByHash(context.Background(), want.Hash())
		require.NoError(t, err)
		require.NotEmpty(t, raw, "transaction %s", want.Hash())
		var decoded types.Transaction
		require.NoError(t, decoded.UnmarshalBinary(raw))
		require.Equal(t, want.Hash(), decoded.Hash())
		wantRaw, err := want.MarshalBinary()
		require.NoError(t, err)
		require.Equal(t, wantRaw, []byte(raw))
	}

	t.Run("committed", func(t *testing.T) {
		for _, tx := range []*types.Transaction{multiTxBlockTx1, multiTxBlockTx2, multiTxBlockTx3, block100NormalTx} {
			t.Run(tx.Hash().Hex(), func(t *testing.T) {
				assertEnvelope(t, api, tx)
			})
		}
	})
	t.Run("unknown", func(t *testing.T) {
		raw, err := api.GetRawTransactionByHash(context.Background(), common.HexToHash("0xdeadbeef"))
		require.NoError(t, err)
		require.Nil(t, raw)
	})
	t.Run("above watermark", func(t *testing.T) {
		laggingClient := &MockClient{latestOverride: 1}
		wm := evmrpc.NewWatermarkManager(laggingClient, ctxProvider, nil, nil)
		laggingAPI := evmrpc.NewRawTransactionAPI(laggingClient, EVMKeeper, ctxProvider, txConfigProvider, "", evmrpc.ConnectionTypeHTTP, utils.None[time.Duration](), wm, evmrpc.NewBlockCache(8), &sync.Mutex{})
		raw, err := laggingAPI.GetRawTransactionByHash(context.Background(), multiTxBlockTx1.Hash())
		require.NoError(t, err)
		require.Nil(t, raw)
	})
	t.Run("mempool", func(t *testing.T) {
		msg, ok := UnconfirmedTx.GetMsgs()[0].(*evmtypes.MsgEVMTransaction)
		require.True(t, ok)
		tx, _ := msg.AsTransaction()
		require.NotNil(t, tx)
		assertEnvelope(t, api, tx)
	})
	t.Run("sequencer pending", func(t *testing.T) {
		sender := occutils.NewSigner()
		s := newPendingSequencer(t, 2, sender)
		txBytes := pendingTransfer(t, sender, 0, occutils.NewSigner().EvmAddress, big.NewInt(pendingOnePaxWei))
		accepted, err := s.exec.Accept(txBytes)
		require.NoError(t, err)
		require.Equal(t, abci.CodeTypeOK, accepted.Result.Code)
		require.Equal(t, s.committed+1, accepted.Height)
		txConfig := s.app.GetTxConfig()
		decoded, err := txConfig.TxDecoder()(txBytes)
		require.NoError(t, err)
		msg := s.app.GetEVMMsg(decoded)
		require.NotNil(t, msg)
		tx, _ := msg.AsTransaction()
		require.NotNil(t, tx)
		pendingAPI := evmrpc.NewRawTransactionAPI(&MockClient{latestOverride: s.committed}, &s.app.EvmKeeper, s.ctxProvider, func(int64) client.TxConfig { return txConfig }, "", evmrpc.ConnectionTypeHTTP, utils.None[time.Duration](), s.wm, evmrpc.NewBlockCache(1), &sync.Mutex{})
		assertEnvelope(t, pendingAPI, tx)
	})
}
