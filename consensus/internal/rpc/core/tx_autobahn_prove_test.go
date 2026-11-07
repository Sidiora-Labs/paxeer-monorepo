package core

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	dbm "github.com/tendermint/tm-db"

	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/example/kvstore"
	abci "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/abci/types"
	atypes "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/autobahn/types"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/proxy"
	sm "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer"
	kvsink "github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/state/indexer/sink/kv"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/internal/store"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/libs/utils/scope"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/rpc/coretypes"
	"github.com/Sidiora-Labs/Paxeer-X-Network/consensus/types"
)

func TestTxProveUnderAutobahnReturnsProof(t *testing.T) {
	rng := utils.TestRng()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	app := proxy.New(kvstore.NewApplication(), proxy.NopMetrics())
	stateStore := sm.NewStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, stateStore.Close()) })
	router, giga := newAutobahnTestRouter(t, rng, "autobahn-tx-proof-test", app, stateStore, 3, 1_000)
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, blockStore.Close()) })
	sink := kvsink.NewEventSink(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, sink.Stop()) })
	env := &Environment{Router: router, BlockStore: blockStore, EventSinks: []indexer.EventSink{sink}}
	txs := types.Txs{types.Tx("first=one"), types.Tx("second=two"), types.Tx("third=three")}

	err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBgNamed("router", func() error { return utils.IgnoreCancel(router.Run(ctx)) })
		for _, tx := range append(txs, types.Tx("next=block")) {
			response, err := giga.InsertTx(ctx, tx)
			if err != nil {
				return err
			}
			require.True(t, response.IsOK())
		}
		for height := int64(1); ; height++ {
			block, err := giga.BlockByNumber(ctx, atypes.GlobalBlockNumber(height))
			if err != nil {
				return err
			}
			if len(block.Block.Data.Txs) == 0 {
				continue
			}
			require.Equal(t, txs, block.Block.Data.Txs)
			require.Nil(t, blockStore.LoadBlock(height))
			for i, tx := range txs {
				indexed := &abci.TxResultV2{Height: height, Index: uint32(i), Tx: tx, Result: abci.ExecTxResult{Code: abci.CodeTypeOK}}
				require.NoError(t, sink.IndexTxEvents([]*abci.TxResultV2{indexed}))
				result, err := env.Tx(ctx, &coretypes.RequestTx{Hash: tx.Hash().Bytes(), Prove: true})
				require.NoError(t, err)
				require.NotNil(t, result)
				require.Equal(t, tx.Hash().Bytes(), result.Hash)
				require.Equal(t, height, result.Height)
				require.Equal(t, uint32(i), result.Index)
				require.Equal(t, tx, result.Tx)
				require.Equal(t, indexed.Result, result.TxResult)
				require.Equal(t, txs.Proof(i), result.Proof)
				require.NoError(t, result.Proof.Validate(block.Block.Data.Txs.Hash()))
			}
			return nil
		}
	})
	require.NoError(t, err)
}

func TestTxProveMissingBlockReturnsErrHeightNotAvailable(t *testing.T) {
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, blockStore.Close()) })
	sink := kvsink.NewEventSink(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, sink.Stop()) })
	env := &Environment{BlockStore: blockStore, EventSinks: []indexer.EventSink{sink}}
	tx := types.Tx("missing=block")
	indexed := &abci.TxResultV2{Height: 12, Index: 1, Tx: tx, Result: abci.ExecTxResult{Code: abci.CodeTypeOK}}
	require.NoError(t, sink.IndexTxEvents([]*abci.TxResultV2{indexed}))
	require.Nil(t, blockStore.LoadBlock(indexed.Height))

	result, err := env.Tx(t.Context(), &coretypes.RequestTx{Hash: tx.Hash().Bytes(), Prove: true})
	require.ErrorIs(t, err, coretypes.ErrHeightNotAvailable)
	require.Nil(t, result)

	result, err = env.Tx(t.Context(), &coretypes.RequestTx{Hash: tx.Hash().Bytes()})
	require.NoError(t, err)
	require.Equal(t, indexed.Height, result.Height)
	require.Equal(t, indexed.Index, result.Index)
	require.Equal(t, tx, result.Tx)
	require.Equal(t, indexed.Result, result.TxResult)
	require.Equal(t, types.TxProof{}, result.Proof)
}

func TestTxSearchProveUnderAutobahnReturnsProof(t *testing.T) {
	rng := utils.TestRng()
	ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
	defer cancel()
	app := proxy.New(kvstore.NewApplication(), proxy.NopMetrics())
	stateStore := sm.NewStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, stateStore.Close()) })
	router, giga := newAutobahnTestRouter(t, rng, "autobahn-tx-search-proof-test", app, stateStore, 3, 1_000)
	blockStore := store.NewBlockStore(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, blockStore.Close()) })
	sink := kvsink.NewEventSink(dbm.NewMemDB())
	t.Cleanup(func() { require.NoError(t, sink.Stop()) })
	env := &Environment{Router: router, BlockStore: blockStore, EventSinks: []indexer.EventSink{sink}}
	txs := types.Txs{types.Tx("first=one"), types.Tx("second=two"), types.Tx("third=three")}

	err := scope.Run(ctx, func(ctx context.Context, s scope.Scope) error {
		s.SpawnBgNamed("router", func() error { return utils.IgnoreCancel(router.Run(ctx)) })
		for _, tx := range append(txs, types.Tx("next=block")) {
			response, err := giga.InsertTx(ctx, tx)
			if err != nil {
				return err
			}
			require.True(t, response.IsOK())
		}
		for height := int64(1); ; height++ {
			block, err := giga.BlockByNumber(ctx, atypes.GlobalBlockNumber(height))
			if err != nil {
				return err
			}
			if len(block.Block.Data.Txs) == 0 {
				continue
			}
			require.Equal(t, txs, block.Block.Data.Txs)
			require.Nil(t, blockStore.LoadBlock(height))
			indexed := make([]*abci.TxResultV2, len(txs))
			for i, tx := range txs {
				indexed[i] = &abci.TxResultV2{Height: height, Index: uint32(i), Tx: tx, Result: abci.ExecTxResult{Code: abci.CodeTypeOK}}
			}
			require.NoError(t, sink.IndexTxEvents(indexed))
			req := &coretypes.RequestTxSearch{Query: fmt.Sprintf("tx.height=%d", height), Prove: true, OrderBy: "asc"}
			result, err := env.TxSearch(ctx, req)
			require.NoError(t, err)
			require.NotNil(t, result)
			require.Equal(t, len(txs), result.TotalCount)
			require.Len(t, result.Txs, len(txs))
			for i, tx := range txs {
				r := result.Txs[i]
				require.Equal(t, tx.Hash().Bytes(), r.Hash)
				require.Equal(t, height, r.Height)
				require.Equal(t, uint32(i), r.Index)
				require.Equal(t, tx, r.Tx)
				require.Equal(t, indexed[i].Result, r.TxResult)
				require.Equal(t, txs.Proof(i), r.Proof)
				require.NoError(t, r.Proof.Validate(block.Block.Data.Txs.Hash()))
			}

			classic := &Environment{BlockStore: blockStore, EventSinks: []indexer.EventSink{sink}}
			result, err = classic.TxSearch(ctx, req)
			require.ErrorIs(t, err, coretypes.ErrHeightNotAvailable)
			require.Nil(t, result)
			req.Prove = false
			result, err = classic.TxSearch(ctx, req)
			require.NoError(t, err)
			require.Equal(t, len(txs), result.TotalCount)
			require.Len(t, result.Txs, len(txs))
			for i, r := range result.Txs {
				require.Equal(t, txs[i], r.Tx)
				require.Equal(t, types.TxProof{}, r.Proof)
			}
			return nil
		}
	})
	require.NoError(t, err)
}
